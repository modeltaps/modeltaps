package preset

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/openai"
)

// ProviderFactory creates the channels of one preset.
type ProviderFactory struct {
	Preset Preset
}

func (f ProviderFactory) Create(channel *model.Channel) base.ProviderInterface {
	return &Provider{
		OpenAIProvider: openai.OpenAIProvider{
			BaseProvider: base.BaseProvider{
				Config:          f.Preset.config(),
				Channel:         channel,
				Requester:       requester.NewHTTPRequester(channel.GetProxy(), openai.RequestErrorHandle),
				SupportResponse: f.Preset.Responses,
			},
			SupportStreamOptions: !f.Preset.NoStreamOptions,
		},
		preset: f.Preset,
	}
}

func (p Preset) config() base.ProviderConfig {
	cfg := base.ProviderConfig{
		BaseURL:         p.Root,
		ChatCompletions: "/chat/completions",
		ModelList:       "/models",
	}
	if p.Embeddings {
		cfg.Embeddings = "/embeddings"
	}
	if p.Responses {
		cfg.Responses = "/responses"
	}
	return cfg
}

// Provider is an OpenAI compatible platform.
type Provider struct {
	openai.OpenAIProvider
	preset Preset
}

// GetModelList asks the platform; when it cannot list (no key yet, or no /models), the models
// models.dev records for it are offered instead.
func (p *Provider) GetModelList() ([]string, error) {
	list, err := p.OpenAIProvider.GetModelList()
	if err == nil && len(list) > 0 {
		return list, nil
	}
	if fallback, ferr := model.ModelsDevModelIDs(p.preset.ModelsDev); ferr == nil && len(fallback) > 0 {
		return fallback, nil
	}
	return list, err
}

// GetChannelPrices implements base.ChannelPriceListInterface with models.dev's price list for
// the platform.
func (p *Provider) GetChannelPrices() ([]*model.ChannelPrice, error) {
	return model.ModelsDevChannelPrices(p.preset.ModelsDev)
}
