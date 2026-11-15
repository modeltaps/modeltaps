package relay

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/utils"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/relay/protocol"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// Anthropic Messages and OpenAI Responses convert to each other in one hop (relay/protocol):
// an Anthropic-entry request for a GPT model goes to a Responses upstream as Responses, and a
// Responses-entry request for a Claude model goes to a Claude upstream as Messages.

// responsesChannelTypes speak the OpenAI Responses protocol for GPT models. Other
// OpenAI-compatible channels may not serve /v1/responses, so they keep the chat conversion.
var responsesChannelTypes = []int{
	config.ChannelTypeOpenAI,
	config.ChannelTypeAzure,
	config.ChannelTypeAzureV1,
}

var responsesModelPattern = regexp.MustCompile(`^(gpt-|chatgpt-|codex-|o[1-9](-|$))`)

// isResponsesModelName reports whether a model is an OpenAI model whose native protocol is
// Responses (GPT, o-series, Codex), ignoring a vendor prefix such as "openai/".
func isResponsesModelName(name string) bool {
	name = strings.ToLower(name)
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return responsesModelPattern.MatchString(name)
}

// claudeEntryResponsesProvider returns the Responses implementation an Anthropic-entry request
// should use: a Codex channel for every model, or an OpenAI / Azure channel for GPT models.
func claudeEntryResponsesProvider(channelType int, modelName string, provider providersBase.ProviderInterface) providersBase.ResponsesInterface {
	responsesProvider, ok := provider.(providersBase.ResponsesInterface)
	if !ok || !provider.GetSupportedResponse() {
		return nil
	}
	if channel := provider.GetChannel(); channel != nil && channel.CompatibleResponse {
		return nil
	}
	if channelType == config.ChannelTypeCodex {
		return responsesProvider
	}
	if utils.Contains(channelType, responsesChannelTypes) && isResponsesModelName(modelName) {
		return responsesProvider
	}
	return nil
}

// claudeMessagesChannelTypes serve Claude models over the Anthropic Messages protocol through
// ClaudeChatInterface. Bedrock and Vertex AI host other models too, so the model must be Claude.
var claudeMessagesChannelTypes = []int{
	config.ChannelTypeAnthropic,
	config.ChannelTypeClaudeCode,
	config.ChannelTypeBedrock,
	config.ChannelTypeVertexAI,
}

// responsesEntryClaudeProvider returns the Messages implementation a Responses-entry request
// should use for a Claude model: a first-party Claude channel, or a custom channel that declares
// the Anthropic protocol for the model.
func responsesEntryClaudeProvider(provider providersBase.ProviderInterface, modelName string) claude.ClaudeChatInterface {
	if delegate := customClaudeProvider(provider, modelName); delegate != nil {
		return delegate
	}
	channel := provider.GetChannel()
	if channel == nil || !utils.Contains(channel.Type, claudeMessagesChannelTypes) {
		return nil
	}
	if (channel.Type == config.ChannelTypeBedrock || channel.Type == config.ChannelTypeVertexAI) &&
		!model_utils.IsAnthropicModelName(modelName) {
		return nil
	}
	claudeProvider, ok := provider.(claude.ClaudeChatInterface)
	if !ok {
		return nil
	}
	return claudeProvider
}

// clearRawResponseBody drops upstream bytes a provider kept for pass-through: a converted reply
// must be written from the converted struct.
func clearRawResponseBody(c *gin.Context) {
	c.Set(config.GinRawResponseBodyKey, nil)
	c.Set(config.GinBedrockRawResponseBodyKey, nil)
}

// streamConverter turns upstream stream chunks of one protocol into SSE text of another.
type streamConverter interface {
	Process(chunk string) string
	Finish() string
	Error(message string) string
}

// convertedStreamClient writes a converted upstream stream to the client, keeping the
// disconnect handling of responseGeneralStreamClient.
func convertedStreamClient(c *gin.Context, stream requester.StreamReaderInterface[string], converter streamConverter) (firstResponseTime time.Time) {
	requester.SetEventStreamHeaders(c)
	applyUpstreamRequestIdHeader(c)
	dataChan, errChan := stream.Recv()
	defer stream.Close()

	ctxDone := c.Request.Context().Done()
	clientDisconnected := false
	write := func(text string) {
		if text == "" || clientDisconnected {
			return
		}
		fmt.Fprint(c.Writer, text)
		c.Writer.Flush()
	}

	for {
		select {
		case data, ok := <-dataChan:
			if !ok {
				write(converter.Finish())
				return
			}
			if firstResponseTime.IsZero() {
				firstResponseTime = time.Now()
			}
			write(converter.Process(data))
		case err := <-errChan:
			if errors.Is(err, io.EOF) {
				write(converter.Finish())
			} else if err != nil {
				logger.LogError(c.Request.Context(), "Stream err:"+err.Error())
				write(converter.Error(utils.MaskSensitiveInfo(err.Error())))
			}
			return
		case <-ctxDone:
			clientDisconnected = true
			ctxDone = nil
			if stopOnClientDisconnect(c) {
				return
			}
		}
	}
}

// sendResponsesWithClaudeFormat serves an Anthropic-entry request from a Responses upstream:
// Messages → Responses, then the reply Responses → Messages.
func (r *relayClaudeOnly) sendResponsesWithClaudeFormat(responsesProvider providersBase.ResponsesInterface) (err *types.OpenAIErrorWithStatusCode, done bool) {
	r.claudeRequest.Model = r.modelName
	if safetyErr := r.performContentSafety(); safetyErr != nil {
		return safetyErr, true
	}

	request := protocol.MessagesToResponsesRequest(r.claudeRequest)
	request.Converted = true

	if r.claudeRequest.Stream {
		stream, errWithCode := responsesProvider.CreateResponsesStream(request)
		if errWithCode != nil {
			return errWithCode, false
		}
		if r.heartbeat != nil {
			r.heartbeat.Stop()
		}
		converter := protocol.NewResponsesToMessagesStream(r.responseModelName(), r.provider.GetUsage())
		r.SetFirstResponseTime(convertedStreamClient(r.c, stream, converter))
		return nil, false
	}

	response, errWithCode := responsesProvider.CreateResponses(request)
	if errWithCode != nil {
		return errWithCode, false
	}
	if r.heartbeat != nil {
		r.heartbeat.Stop()
	}
	r.SetFirstResponseTime(time.Now())
	clearRawResponseBody(r.c)
	_ = responseJsonClient(r.c, protocol.ResponsesToMessagesResponse(response, r.responseModelName(), r.provider.GetUsage()))
	return nil, false
}

// responseModelName is the model name the reply reports: the one the client asked for.
func (r *relayClaudeOnly) responseModelName() string {
	if r.originalModel != "" {
		return r.originalModel
	}
	return r.modelName
}

// sendClaudeWithResponsesFormat serves a Responses-entry request from a Messages upstream:
// Responses → Messages, then the reply Messages → Responses.
func (r *relayResponses) sendClaudeWithResponsesFormat(claudeProvider claude.ClaudeChatInterface) (err *types.OpenAIErrorWithStatusCode, done bool) {
	request, convertErr := protocol.ResponsesToMessagesRequest(&r.responsesRequest,
		config.ClaudeSettingsInstance.GetDefaultMaxTokens(r.modelName))
	if convertErr != nil {
		return common.ErrorWrapperLocal(convertErr, "invalid_request", http.StatusBadRequest), true
	}

	model := r.originalModel
	if model == "" {
		model = r.modelName
	}

	if request.Stream {
		stream, errWithCode := claudeProvider.CreateClaudeChatStream(request)
		if errWithCode != nil {
			return errWithCode, false
		}
		converter := protocol.NewMessagesToResponsesStream(&r.responsesRequest, model, r.provider.GetUsage())
		r.SetFirstResponseTime(convertedStreamClient(r.c, stream, converter))
		return nil, false
	}

	response, errWithCode := claudeProvider.CreateClaudeChat(request)
	if errWithCode != nil {
		return errWithCode, false
	}
	r.SetFirstResponseTime(time.Now())
	clearRawResponseBody(r.c)
	_ = responseJsonClient(r.c, protocol.MessagesToResponsesResponse(response, &r.responsesRequest, model, r.provider.GetUsage()))
	return nil, false
}
