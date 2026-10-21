package vercel

import (
	"net/http"

	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
)

// VercelProviderFactory 创建 Vercel AI Gateway 渠道。
// 网关提供 OpenAI 兼容接口（chat / responses / embeddings / images），模型名形如 "anthropic/claude-sonnet-4"，
// 另有 `GET /v1/models`（自带价格与元数据，无需鉴权）和 `GET /v1/credits`（团队余额）。
// https://vercel.com/docs/ai-gateway/sdks-and-apis/rest-api
type VercelProviderFactory struct{}

func (f VercelProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	return &VercelProvider{
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

// RequestErrorHandle 复用 OpenAI 的错误解析（网关返回 OpenAI 格式的 error 对象）。
func RequestErrorHandle(resp *http.Response) *types.OpenAIError {
	return openai.RequestErrorHandle(resp)
}

func getConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:           "https://ai-gateway.vercel.sh",
		ChatCompletions:   "/v1/chat/completions",
		Responses:         "/v1/responses",
		Embeddings:        "/v1/embeddings",
		ImagesGenerations: "/v1/images/generations",
		ModelList:         "/v1/models",
	}
}

type VercelProvider struct {
	openai.OpenAIProvider
}
