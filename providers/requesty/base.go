package requesty

import (
	"net/http"
	"strings"

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

// The channel key is "apiKey" or "apiKey|manageKey": the second part, when given, is only used to
// read the balance, so an inference key without manage permission can still be the one requests
// are sent with.
func splitKey(key string) (apiKey, manageKey string) {
	apiKey, manageKey, _ = strings.Cut(strings.TrimSpace(key), "|")
	return strings.TrimSpace(apiKey), strings.TrimSpace(manageKey)
}

func (f RequestyProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	apiKey, manageKey := splitKey(channel.Key)
	ch := channel
	if apiKey != channel.Key {
		// The OpenAI and Claude providers read the key from the channel, so they get a copy that
		// carries the inference key alone. Writes through it (balance, status) go to the same row.
		clone := *channel
		clone.Key = apiKey
		ch = &clone
	}
	if manageKey == "" {
		manageKey = apiKey
	}
	return &RequestyProvider{
		OpenAIProvider: openai.OpenAIProvider{
			BaseProvider: base.BaseProvider{
				Config:    getConfig(),
				Channel:   ch,
				Requester: requester.NewHTTPRequester(ch.GetProxy(), RequestErrorHandle),
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
