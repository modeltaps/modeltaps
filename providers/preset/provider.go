package preset

import (
	"errors"
	"strings"

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
	// A preset with a balance may take more credentials after the API key ("apiKey|a|b"); the
	// requests get a copy of the channel that carries the API key alone.
	var extra []string
	if f.Preset.Balance != nil && strings.Contains(channel.Key, "|") {
		parts := strings.Split(strings.TrimSpace(channel.Key), "|")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		clone := *channel
		clone.Key = parts[0]
		channel, extra = &clone, parts[1:]
	}
	return &Provider{
		extraKeys: extra,
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
	// extraKeys are the credentials after the API key, read by the preset's Balance.
	extraKeys []string
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

// Balance implements base.BalanceInterface for presets with a balance endpoint.
func (p *Provider) Balance() (float64, error) {
	if p.preset.Balance == nil {
		return 0, errors.New(p.preset.Name + " has no balance endpoint")
	}
	balance, err := p.preset.Balance(p)
	if err != nil {
		return 0, err
	}
	p.Channel.UpdateBalance(balance)
	return balance, nil
}

// CanQueryBalance implements base.BalanceCapability.
func (p *Provider) CanQueryBalance() bool {
	return p.preset.Balance != nil
}
