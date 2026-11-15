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
	// A preset whose balance needs more than the API key reads them from the channel's
	// management key; several credentials are separated by "|".
	var extra []string
	if manageKey := channel.GetManageKey(); f.Preset.ManageKey && manageKey != "" {
		extra = strings.Split(manageKey, "|")
		for i := range extra {
			extra[i] = strings.TrimSpace(extra[i])
		}
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
	// extraKeys are the parts of the channel's management key, read by the preset's Balance.
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
