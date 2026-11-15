package claudecode

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
)

// CreateClaudeChat sends a Messages request with the account's OAuth token. The embedded
// ClaudeProvider method would authenticate with the channel key (the credentials JSON) as an
// API key, which the subscription endpoint rejects.
func (p *ClaudeCodeProvider) CreateClaudeChat(request *claude.ClaudeRequest) (*claude.ClaudeResponse, *types.OpenAIErrorWithStatusCode) {
	p.applyClaudeCodeCompatibility(request)
	req, errWithCode := p.getChatRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	response := &claude.ClaudeResponse{}
	if _, errWithCode = p.Requester.SendRequest(req, response, false); errWithCode != nil {
		return nil, errWithCode
	}

	usage := p.GetUsage()
	base.ObserveUpstreamModel(p.Context, response.Model)
	if !claude.ClaudeUsageToOpenaiUsage(&response.Usage, usage) {
		usage.CompletionTokens = claude.ClaudeOutputUsage(response)
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return response, nil
}

// CreateClaudeChatStream is the streamed CreateClaudeChat: Messages events pass through as they
// arrive while their usage is recorded for billing.
func (p *ClaudeCodeProvider) CreateClaudeChatStream(request *claude.ClaudeRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	p.applyClaudeCodeCompatibility(request)
	req, errWithCode := p.getChatRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	handler := &claude.ClaudeRelayStreamHandler{
		Usage:     p.Usage,
		ModelName: request.Model,
		Prefix:    `data: {`,
		Context:   p.Context,
	}
	return requester.RequestNoTrimStream(p.Requester, resp, handler.HandlerStream)
}
