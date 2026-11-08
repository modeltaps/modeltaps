package category

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

const AnthropicVersion = "vertex-2023-10-16"

type ClaudeRequest struct {
	*claude.ClaudeRequest
	AnthropicVersion string `json:"anthropic_version"`
}

// claudeModelOverrides holds the model IDs that the generic date rule cannot derive
// (Vertex AI IDs that differ from the Anthropic ones). A regular "-YYYYMMDD" suffix is
// converted to "@YYYYMMDD" by NormalizeClaudeModelName and needs no entry here.
var claudeModelOverrides = map[string]string{
	"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-v2@20241022",
}

var claudeDatedModelPattern = regexp.MustCompile(`^(.+)-(\d{8})$`)

func init() {
	CategoryMap["claude"] = &Category{
		Category:                  "claude",
		Publisher:                 PublisherAnthropic,
		ChatComplete:              ConvertClaudeFromChatOpenai,
		ResponseChatComplete:      ConvertClaudeToChatOpenai,
		ResponseChatCompleteStrem: ClaudeChatCompleteStrem,
		ErrorHandler:              claude.RequestErrorHandle,
		GetModelName:              NormalizeClaudeModelName,
		GetOtherUrl:               getClaudeOtherUrl,
	}
}

func ConvertClaudeFromChatOpenai(request *types.ChatCompletionRequest) (any, *types.OpenAIErrorWithStatusCode) {
	rawRequest, err := claude.ConvertFromChatOpenai(request)
	if err != nil {
		return nil, err
	}

	claudeRequest := &ClaudeRequest{}
	claudeRequest.ClaudeRequest = rawRequest
	claudeRequest.AnthropicVersion = AnthropicVersion

	// 删除model字段
	claudeRequest.Model = ""

	return claudeRequest, nil
}

func ConvertClaudeToChatOpenai(provider base.ProviderInterface, response *http.Response, request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
	claudeResponse := &claude.ClaudeResponse{}
	err := json.NewDecoder(response.Body).Decode(claudeResponse)
	if err != nil {
		return nil, common.ErrorWrapper(err, "decode_response_failed", http.StatusInternalServerError)
	}

	return claude.ConvertToChatOpenai(provider, claudeResponse, request)
}

func ClaudeChatCompleteStrem(provider base.ProviderInterface, request *types.ChatCompletionRequest) requester.HandlerPrefix[string] {
	chatHandler := &claude.ClaudeStreamHandler{
		Usage:   provider.GetUsage(),
		Request: request,
		Prefix:  "data:",
	}

	return chatHandler.HandlerStream
}

// NormalizeClaudeModelName converts an Anthropic model ID into the Vertex AI model ID:
//   - an explicit override wins (irregular names such as the v2 model)
//   - an ID that already contains "@" is returned unchanged
//   - "claude-xxx-YYYYMMDD" becomes "claude-xxx@YYYYMMDD"
//   - anything else (models without a date suffix) is returned unchanged
func NormalizeClaudeModelName(modelName string) string {
	if mapped, ok := claudeModelOverrides[modelName]; ok {
		return mapped
	}
	if strings.Contains(modelName, "@") {
		return modelName
	}
	if m := claudeDatedModelPattern.FindStringSubmatch(modelName); len(m) == 3 {
		return m[1] + "@" + m[2]
	}
	return modelName
}

// getClaudeOtherUrl returns the Vertex AI action for Claude models: streamRawPredict when
// streaming, rawPredict otherwise.
func getClaudeOtherUrl(stream bool) string {
	if stream {
		return "streamRawPredict"
	}
	return "rawPredict"
}
