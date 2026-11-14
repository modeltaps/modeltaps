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
	// Unoffered says why a listed model serves no public model while supplies route requests
	// (one of the ChannelModelUnoffered* values); empty when it serves one.
	Unoffered string `json:"unoffered,omitempty"`
	// Suggested is the public model the name would be offered as: the catalog model it belongs to
	// for ChannelModelNoSupply, the name a catalog row of its own would take for
	// ChannelModelNotInCatalog and ChannelModelDuplicate.
	Suggested string `json:"suggested,omitempty"`
	// For ChannelModelDuplicate: the public model the name belongs to and the upstream name the
	// channel already supplies it with.
	SameAs     string `json:"same_as,omitempty"`
	SuppliedBy string `json:"supplied_by,omitempty"`
}

// Why a listed model serves no public model.
const (
	ChannelModelNotInCatalog = "not_in_catalog" // no public model in the catalog matches the name
	ChannelModelNoSupply     = "no_supply"      // a public model matches but the channel does not supply it
	// The public model it matches is already supplied by this channel under another upstream name
	// (a channel supplies a model once), as "openai/gpt-5.5-fast" next to "openai/gpt-5.5".
	ChannelModelDuplicate = "duplicate"
	ChannelModelWildcard     = "wildcard"       // "claude-*" style entries are never supplied
)

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
	// suppliedBy is the upstream name each public model is supplied with on this channel.
	suppliedBy := map[string]string{}
	if config.ModelSupplyEnabled && channelID > 0 {
		var supplies []*ModelSupply
		if err := DB.Where("channel_id = ?", channelID).Order("model").Find(&supplies).Error; err != nil {
			return nil, err
		}
		for _, s := range supplies {
			bySupply[s.UpstreamModel] = append(bySupply[s.UpstreamModel], s)
			suppliedBy[s.Model] = s.UpstreamModel
		}
	}

	var resolver *CanonicalResolver
	if config.ModelSupplyEnabled {
		if resolver, err = NewCanonicalResolver(); err != nil {
			return nil, err
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
			if len(detail.Public) == 0 {
				applyUnofferedReason(&detail, resolver, suppliedBy)
			}
		} else {
			in, outPrice := tokenSellPrice(item.Model)
			detail.Public = append(detail.Public, ChannelModelPublic{Model: item.Model, Enabled: true, InputPrice: in, OutputPrice: outPrice})
		}
		out = append(out, detail)
	}
	return out, nil
}

// applyUnofferedReason tells why a listed name serves no public model, matching names to public
// models the way AutoSupplyChannel does, and what it would be offered as. suppliedBy maps the
// public models the channel supplies to the upstream name it sends for each.
func applyUnofferedReason(detail *ChannelModelDetail, resolver *CanonicalResolver, suppliedBy map[string]string) {
	name := detail.Model
	if strings.HasSuffix(name, "*") {
		detail.Unoffered = ChannelModelWildcard
		return
	}
	info := resolver.publicModel(resolver.Resolve(name).Canonical)
	if info == nil {
		detail.Unoffered, detail.Suggested = ChannelModelNotInCatalog, resolver.PublicName(name)
		return
	}
	if other, ok := suppliedBy[info.Model]; ok && other != name {
		own := name
		if bare := bareModelID(name); bare != "" {
			own = bare
		}
		detail.Unoffered, detail.Suggested, detail.SameAs, detail.SuppliedBy = ChannelModelDuplicate, own, info.Model, other
		return
	}
	detail.Unoffered, detail.Suggested = ChannelModelNoSupply, info.Model
}
