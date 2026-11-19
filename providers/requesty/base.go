package requesty

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
)

// RequestyProviderFactory creates Requesty channels. Requesty is a router with an OpenAI
// compatible API (chat completions, responses) and an Anthropic compatible Messages endpoint.
// `GET /v1/models` (no auth) lists every provider-pinned model id with its price, lab and canonical
// name; `GET /v1/models/managed` lists the bare names Requesty routes across providers with
// failover. The balance is only readable with a key that has manage permission:
// `GET https://api-v2.requesty.ai/v1/manage/org`.
// https://docs.requesty.ai/
type RequestyProviderFactory struct{}

// Requests are sent with the channel key. The balance is read with the channel's management key
// when it has one, so an inference key without manage permission can still be the one requests
// are sent with; without one the channel key is tried.
func (f RequestyProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	manageKey := channel.GetManageKey()
	if manageKey == "" {
		manageKey = channel.Key
	}
	return &RequestyProvider{
		OpenAIProvider: openai.OpenAIProvider{
			BaseProvider: base.BaseProvider{
				Config:              getConfig(),
				Channel:             channel,
				Requester:           requester.NewHTTPRequester(channel.GetProxy(), RequestErrorHandle),
				DefaultCustomParams: autoCacheParams(channel),
			},
			ReasoningHandler:     true,
			SupportStreamOptions: true,
		},
		manageKey: manageKey,
	}
}

// autoCacheParams turns on Requesty's automatic cache breakpoints for chat completions and
// responses (`"requesty": {"auto_cache": true}`) unless the channel switches them off. Without it
// Claude models reached over the OpenAI format are not cached at all, and coding agents resend
// the same long context on every turn: cache reads cost a tenth of input, a write a quarter more,
// so a one-off long prompt costs 25% more while every repeated one costs far less. The channel's
// own custom parameters win, and so does a client's own `requesty` object on channels that pass
// extra body fields through. A prompt cache mode the API key or the request chose (PROTO-9)
// wins over the channel's switch: "on" asks for breakpoints, "off" for none.
// https://docs.requesty.ai/features/auto-caching
func autoCacheParams(channel *model.Channel) func(*gin.Context) map[string]interface{} {
	enabled := true
	if channel != nil && channel.Plugin != nil {
		if opts, ok := channel.Plugin.Data()["requesty"]; ok {
			if mode, _ := opts["auto_cache"].(string); mode == "off" {
				enabled = false
			}
		}
	}
	return func(c *gin.Context) map[string]interface{} {
		switch claude.RequestPromptCacheMode(c) {
		case config.PromptCacheModeOn:
			return autoCacheOn()
		case config.PromptCacheModeOff:
			return nil
		}
		if !enabled {
			return nil
		}
		return autoCacheOn()
	}
}

func autoCacheOn() map[string]interface{} {
	return map[string]interface{}{"requesty": map[string]interface{}{"auto_cache": true}}
}

// RequestErrorHandle parses Requesty's errors, which use OpenAI's error object.
func RequestErrorHandle(resp *http.Response) *types.OpenAIError {
	return openai.RequestErrorHandle(resp)
}

const defaultBaseURL = "https://router.requesty.ai"

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         defaultBaseURL,
		ChatCompletions: "/v1/chat/completions",
		Responses:       "/v1/responses",
		ModelList:       "/v1/models",
	}
}

type RequestyProvider struct {
	openai.OpenAIProvider
	manageKey string
}
