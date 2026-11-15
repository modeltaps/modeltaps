package requesty

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
)

// getClaudeConfig is Requesty's Anthropic Messages endpoint, /v1/messages on the router (the docs
// name /anthropic/v1/messages, which answers 404). Claude format requests go there as they are
// instead of through the Claude to OpenAI conversion, which would drop the client's cache
// breakpoints.
func getClaudeConfig(baseURL string) base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         baseURL,
		ChatCompletions: "/v1/messages",
		ModelList:       "/v1/models",
	}
}

// claudeProvider is a ClaudeProvider over the same BaseProvider (channel, context, usage) with
// the Anthropic endpoint and error parsing. It sends the key as x-api-key, as the Anthropic SDK
// pointed at Requesty does.
func (p *RequestyProvider) claudeProvider() *claude.ClaudeProvider {
	provider := &claude.ClaudeProvider{BaseProvider: p.BaseProvider}
	provider.Config = getClaudeConfig(p.Config.BaseURL)
	provider.Requester = requester.NewHTTPRequester(p.Channel.GetProxy(), claude.RequestErrorHandle)
	return provider
}

func (p *RequestyProvider) CreateClaudeChat(request *claude.ClaudeRequest) (*claude.ClaudeResponse, *types.OpenAIErrorWithStatusCode) {
	return p.claudeProvider().CreateClaudeChat(request)
}

func (p *RequestyProvider) CreateClaudeChatStream(request *claude.ClaudeRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	return p.claudeProvider().CreateClaudeChatStream(request)
}
