package github

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/openai"
)

type GithubProviderFactory struct{}

// 创建 GithubProvider
func (f GithubProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	config := getGithubConfig()
	return &GithubProvider{
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

func getGithubConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         "https://models.inference.ai.azure.com",
		ChatCompletions: "/chat/completions",
		Embeddings:      "/embeddings",
		ModelList:       "/models",
	}
}

type GithubProvider struct {
	openai.OpenAIProvider
}
