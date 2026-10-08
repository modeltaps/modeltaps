package deepseek

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/openai"
)

type DeepseekProviderFactory struct{}

// 创建 DeepseekProvider
func (f DeepseekProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	config := getDeepseekConfig()
	return &DeepseekProvider{
		OpenAIProvider: openai.OpenAIProvider{
			BaseProvider: base.BaseProvider{
				Config:    config,
				Channel:   channel,
				Requester: requester.NewHTTPRequester(channel.GetProxy(), openai.RequestErrorHandle),
			},
			BalanceAction: false,
		},
	}
}

func getDeepseekConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         "https://api.deepseek.com",
		ChatCompletions: "/v1/chat/completions",
		ModelList:       "/v1/models",
		Responses:       "/v1/responses",
	}
}

type DeepseekProvider struct {
	openai.OpenAIProvider
}
