package openrouter

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
	"net/http"
)

// 定义供应商工厂
type OpenRouterProviderFactory struct{}

// 创建 OpenRouterProvider
// https://openrouter.ai/docs/api-reference/overview
func (f OpenRouterProviderFactory) Create(channel *model.Channel) base.ProviderInterface {

	return &OpenRouterProvider{
		OpenAIProvider: openai.OpenAIProvider{
			BaseProvider: base.BaseProvider{
				Config:    getConfig(),
				Channel:   channel,
				Requester: requester.NewHTTPRequester(channel.GetProxy(), RequestErrorHandle),
			},

			ReasoningHandler:     true,
			SupportStreamOptions: true,
		},
	}
}

// RequestErrorHandle 转发到OpenAI的错误处理函数
func RequestErrorHandle(resp *http.Response) *types.OpenAIError {
	return openai.RequestErrorHandle(resp)
}

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:             "https://openrouter.ai/api",
		ChatCompletions:     "/v1/chat/completions",
		AudioSpeech:         "/v1/audio/speech",
		AudioTranscriptions: "/v1/audio/transcriptions",
		ModelList:           "/v1/models",
	}
}

type OpenRouterProvider struct {
	openai.OpenAIProvider
}
