package venice

import (
	"encoding/json"
	"net/http"

	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
)

// VeniceProviderFactory creates Venice channels. Venice serves an OpenAI compatible API only
// (chat completions, responses, embeddings, images, speech, transcriptions); there is no
// Anthropic Messages endpoint, so Claude format requests go through the Claude to OpenAI
// conversion.
//
// Prompt caching (checked 2026-11-19, https://docs.venice.ai/guides/features/prompt-caching):
// most models cache a repeated prefix of about 1,024 tokens or more on their own; for Claude
// models Venice adds the breakpoints itself (system prompt and the second-to-last user message)
// and keeps cache_control blocks a client sends. prompt_cache_key passes through as a routing
// hint. Usage reports reads in prompt_tokens_details.cached_tokens and writes in
// prompt_tokens_details.cache_creation_input_tokens.
//
// `GET /models?type=all` (no auth) lists every model with its USD prices; `GET /billing/balance`
// returns the account's balances.
// https://docs.venice.ai/
type VeniceProviderFactory struct{}

// Requests are sent with the channel key. The balance is read with the channel's management key
// when it has one, else with the channel key.
func (f VeniceProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	manageKey := channel.GetManageKey()
	if manageKey == "" {
		manageKey = channel.Key
	}
	return &VeniceProvider{
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

// DefaultBaseURL is Venice's API root; the OpenAI paths are appended to it.
const DefaultBaseURL = "https://api.venice.ai/api/v1"

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:             DefaultBaseURL,
		ChatCompletions:     "/chat/completions",
		Responses:           "/responses",
		Embeddings:          "/embeddings",
		ImagesGenerations:   "/images/generations",
		AudioSpeech:         "/audio/speech",
		AudioTranscriptions: "/audio/transcriptions",
		ModelList:           "/models",
	}
}

type VeniceProvider struct {
	openai.OpenAIProvider
	manageKey string
}

// RequestErrorHandle parses Venice's errors. Most are `{"error": "message"}` with an optional
// `details` object; some endpoints return OpenAI's error object instead.
func RequestErrorHandle(resp *http.Response) *types.OpenAIError {
	var body struct {
		Error json.RawMessage `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || len(body.Error) == 0 {
		return nil
	}
	return parseError(body.Error)
}

func parseError(raw json.RawMessage) *types.OpenAIError {
	var message string
	if json.Unmarshal(raw, &message) == nil {
		if message == "" {
			return nil
		}
		return &types.OpenAIError{Message: message, Type: "venice_error"}
	}
	var obj types.OpenAIError
	if json.Unmarshal(raw, &obj) != nil || obj.Message == "" {
		return nil
	}
	return &obj
}
