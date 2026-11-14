package model

import (
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// ChannelModelPublic is a public model one listed model of a channel serves: through a supply
// while supplies route requests, otherwise the listed name itself. InputPrice / OutputPrice are
// its base sell prices in USD per million tokens, nil when it is not sold per token.
type ChannelModelPublic struct {
	Model       string   `json:"model"`
	Enabled     bool     `json:"enabled"`
	InputPrice  *float64 `json:"input_price"`
	OutputPrice *float64 `json:"output_price"`
}

// ChannelModelDetail is one model of a channel's list with what the channel panel shows next to
// it: catalog metadata (from the model picker), its place in the usual order and the public
// models it serves.
type ChannelModelDetail struct {
	ModelPickerItem
	// Rank is the 1-based position in the usual order: families most requested on this site in
	// the last ModelPickerHeatDays days first (then the mainstream fallback tiers), the latest
	// version first inside a family.
	Rank int `json:"rank"`
	// FamilyRequests is the site's requests to the model's family in that window.
	FamilyRequests int64                `json:"family_requests"`
	Public         []ChannelModelPublic `json:"public"`
}

// tokenSellPrice is the base sell price of a public model per million tokens; nil when it has no
// configured per-token price.
func tokenSellPrice(name string) (*float64, *float64) {
	if PricingInstance == nil {
		return nil, nil
	}
	price := PricingInstance.GetPrice(name)
	if price == nil || price.Unconfigured || price.Type != TokensPriceType {
		return nil, nil
	}
	return tokenPricePerMillion(price.Input), tokenPricePerMillion(price.Output)
}

// BuildChannelModelDetails describes `models` (a channel's list, possibly unsaved) for the channel
// panel, in the usual order. `mapping` is the list's model mapping (listed name → name sent
// upstream, a leading "+" allowed): supplies are matched by the name sent upstream.
func BuildChannelModelDetails(channelID int, models []string, mapping map[string]string) ([]ChannelModelDetail, error) {
	familyRequests, err := FamilyRequestCounts(ModelPickerHeatDays)
	if err != nil {
		return nil, err
	}
	picker, err := BuildModelPicker(models, familyRequests, map[string]int{})
	if err != nil {
		return nil, err
	}

	bySupply := map[string][]*ModelSupply{}
	if config.ModelSupplyEnabled && channelID > 0 {
		var supplies []*ModelSupply
		if err := DB.Where("channel_id = ?", channelID).Order("model").Find(&supplies).Error; err != nil {
			return nil, err
		}
		for _, s := range supplies {
			bySupply[s.UpstreamModel] = append(bySupply[s.UpstreamModel], s)
		}
	}

	rank := make(map[string]int, len(picker.Families))
	for _, f := range picker.Families {
		rank[f.Family] = f.Rank
	}
	items := picker.Models
	sort.SliceStable(items, func(a, b int) bool {
		if ra, rb := rank[items[a].Family], rank[items[b].Family]; ra != rb {
			return ra < rb
		}
		if items[a].Latest != items[b].Latest {
			return items[a].Latest
		}
		return newerModel(&items[a], &items[b])
	})

	out := make([]ChannelModelDetail, 0, len(items))
	for i, item := range items {
		detail := ChannelModelDetail{
			ModelPickerItem: item,
			Rank:            i + 1,
			FamilyRequests:  familyRequests[item.Family],
			Public:          []ChannelModelPublic{},
		}
		if config.ModelSupplyEnabled {
			upstream := strings.TrimPrefix(strings.TrimSpace(mapping[item.Model]), "+")
			if upstream == "" {
				upstream = item.Model
			}
			seen := map[int]bool{}
			for _, key := range []string{upstream, item.Model} {
				for _, s := range bySupply[key] {
					if seen[s.Id] {
						continue
					}
					seen[s.Id] = true
					in, outPrice := tokenSellPrice(s.Model)
					detail.Public = append(detail.Public, ChannelModelPublic{Model: s.Model, Enabled: s.Enabled, InputPrice: in, OutputPrice: outPrice})
				}
			}
		} else {
			in, outPrice := tokenSellPrice(item.Model)
			detail.Public = append(detail.Public, ChannelModelPublic{Model: item.Model, Enabled: true, InputPrice: in, OutputPrice: outPrice})
		}
		out = append(out, detail)
	}
	return out, nil
}
