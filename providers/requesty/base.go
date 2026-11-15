package requesty

import (
	"net/http"

	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
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
				Config:    getConfig(),
				Channel:   channel,
				Requester: requester.NewHTTPRequester(channel.GetProxy(), RequestErrorHandle),
			},
			ReasoningHandler:     true,
			SupportStreamOptions: true,
		},
		manageKey: manageKey,
	}
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
