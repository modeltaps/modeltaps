package gemini

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"
)

type GeminiChatInterface interface {
	base.ProviderInterface
	CreateGeminiChat(request *GeminiChatRequest) (*GeminiChatResponse, *types.OpenAIErrorWithStatusCode)
	CreateGeminiChatStream(request *GeminiChatRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode)
}

type GeminiVeoInterface interface {
	base.ProviderInterface
	CreateVeoVideoAndDownload(request *VeoVideoRequest, modelName string) ([]byte, string, *types.OpenAIErrorWithStatusCode)
}
