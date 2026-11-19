package cheaperinference

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/types"
)

// Streamed chat completions and completions carry no charge (the docs say to reconcile them by
// request id), and a responses stream reports it only in its last event: each started stream
// is queued for a charge lookup, which fills the log's upstream cost only where it has none.

func (p *CheaperInferenceProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	stream, errWithCode := p.OpenAIProvider.CreateChatCompletionStream(request)
	if errWithCode == nil {
		p.queueCostLookup()
	}
	return stream, errWithCode
}

func (p *CheaperInferenceProvider) CreateCompletionStream(request *types.CompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	stream, errWithCode := p.OpenAIProvider.CreateCompletionStream(request)
	if errWithCode == nil {
		p.queueCostLookup()
	}
	return stream, errWithCode
}

func (p *CheaperInferenceProvider) CreateResponsesStream(request *types.OpenAIResponsesRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	stream, errWithCode := p.OpenAIProvider.CreateResponsesStream(request)
	if errWithCode == nil {
		p.queueCostLookup()
	}
	return stream, errWithCode
}
