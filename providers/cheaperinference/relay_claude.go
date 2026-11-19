package cheaperinference

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
)

// getClaudeConfig is the Anthropic compatible Messages endpoint on the same root. Claude format
// requests go there as they are instead of through the Claude to OpenAI conversion, which would
// drop the client's cache breakpoints.
func getClaudeConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         DefaultBaseURL,
		ChatCompletions: "/v1/messages",
		ModelList:       "/v1/models",
	}
}

// claudeProvider is a ClaudeProvider over the same BaseProvider (channel, context, usage) with
// the Messages endpoint and Anthropic's error shape. Messages requests carry the client's own
// cache_control breakpoints, so no x-ci-prompt-cache mode is sent: "on" would add breakpoints
// next to them. The cache session is still sent.
func (p *CheaperInferenceProvider) claudeProvider() *claude.ClaudeProvider {
	provider := &claude.ClaudeProvider{BaseProvider: p.BaseProvider}
	provider.DefaultHeaders = sessionHeader
	provider.Config = getClaudeConfig()
	provider.Requester = requester.NewHTTPRequester(p.Channel.GetProxy(), claude.RequestErrorHandle)
	return provider
}

func (p *CheaperInferenceProvider) CreateClaudeChat(request *claude.ClaudeRequest) (*claude.ClaudeResponse, *types.OpenAIErrorWithStatusCode) {
	return p.claudeProvider().CreateClaudeChat(request)
}

func (p *CheaperInferenceProvider) CreateClaudeChatStream(request *claude.ClaudeRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	return p.claudeProvider().CreateClaudeChatStream(request)
}
