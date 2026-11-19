package cheaperinference

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/session"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
)

// CheaperInferenceProviderFactory creates Cheaper Inference channels. Cheaper Inference is a
// router that sends each request to the cheapest route it has for the model (pooled lab
// discounts, resold capacity, third-party sellers) and falls back to the next one before output
// starts. It serves an OpenAI compatible API (chat completions, responses, embeddings) and an
// Anthropic compatible Messages endpoint.
//
// Prompt caching (checked 2026-11-19, https://www.cheaperinference.com/docs): Claude cache_control
// blocks pass through on every endpoint. The x-ci-prompt-cache header picks how requests without
// them are marked: "on" adds breakpoints at the system / tools prefix and at the last block,
// "passthrough" sends the body as it is, "off" removes the client's markers. Without the header
// only Claude chat requests served on one kind of route get breakpoints, so OpenAI format
// requests to Claude models mostly go uncached. x-ci-prompt-cache-session keeps one conversation
// on one route, where its cache entry is. Usage reports reads in cached_tokens (OpenAI formats) or
// cache_read_input_tokens, writes in cache_creation_input_tokens (Messages), and the amount the
// wallet was charged in usage.cost (USD).
//
// `GET /public/models` (no auth) lists every model with the price a request pays;
// `GET /v1/account/balance` returns the wallet, with a key that has the account:read scope.
type CheaperInferenceProviderFactory struct{}

// Requests are sent with the channel key. The balance is read with the channel's management key
// when it has one, so an inference-only key can still be the one requests are sent with.
func (f CheaperInferenceProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	manageKey := channel.GetManageKey()
	if manageKey == "" {
		manageKey = channel.Key
	}
	req := requester.NewHTTPRequester(channel.GetProxy(), RequestErrorHandle)
	req.RequestHook = routingHook(channel)
	return &CheaperInferenceProvider{
		OpenAIProvider: openai.OpenAIProvider{
			BaseProvider: base.BaseProvider{
				Config:         getConfig(),
				Channel:        channel,
				Requester:      req,
				DefaultHeaders: openAIHeaders(channel),
			},
			ReasoningHandler:     true,
			SupportStreamOptions: true,
		},
		manageKey: manageKey,
		routing:   routingOf(channel),
	}
}

// DefaultBaseURL is the API root. The OpenAI paths carry /v1 themselves, as the Messages path
// does, so the one root serves both formats.
const DefaultBaseURL = "https://api.cheaperinference.com"

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         DefaultBaseURL,
		ChatCompletions: "/v1/chat/completions",
		Completions:     "/v1/completions",
		Responses:       "/v1/responses",
		Embeddings:      "/v1/embeddings",
		ModelList:       "/v1/models",
	}
}

type CheaperInferenceProvider struct {
	openai.OpenAIProvider
	manageKey string
	routing   routing
}

// RequestErrorHandle parses the errors of the OpenAI endpoints, which use OpenAI's error object.
func RequestErrorHandle(resp *http.Response) *types.OpenAIError {
	return openai.RequestErrorHandle(resp)
}

const (
	headerPromptCache        = "x-ci-prompt-cache"
	headerPromptCacheSession = "x-ci-prompt-cache-session"
)

// promptCacheMode is the x-ci-prompt-cache value OpenAI format requests carry: "on" unless the
// channel's plugin says "passthrough" or "off". Coding agents resend the same long context every
// turn; a cache read costs a tenth of input and a write a quarter more, so one long prompt sent
// once costs 25% more and every repeated one far less.
func promptCacheMode(channel *model.Channel) string {
	switch mode, _ := pluginOptions(channel, "cheaperinference")["prompt_cache"].(string); mode {
	case "passthrough", "off":
		return mode
	}
	return "on"
}

// openAIHeaders marks OpenAI format requests for caching and keeps each session on one route.
func openAIHeaders(channel *model.Channel) func(*gin.Context, map[string]string) {
	mode := promptCacheMode(channel)
	return func(c *gin.Context, headers map[string]string) {
		headers[headerPromptCache] = mode
		sessionHeader(c, headers)
	}
}

// sessionHeader sends the gateway's session key (a hash scoped to the API key and the model, the
// one session affinity uses) as the cache session, so turns of one conversation stay on the route
// that holds their cache entry. Nothing is sent for a request with nothing that names a session.
func sessionHeader(c *gin.Context, headers map[string]string) {
	if c == nil {
		return
	}
	if id := session.GenerateGenericSessionHash(c, c.GetString("original_model")); id != "" {
		headers[headerPromptCacheSession] = id
	}
}
