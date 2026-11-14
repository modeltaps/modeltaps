package model

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// ModelInfoSourceLibrary marks catalog rows added from the model library; the catalog sync keeps
// refreshing them from the library.
const ModelInfoSourceLibrary = "library"

// ModelSupplySourceLibrary marks supplies created when a model was added from the library.
const ModelSupplySourceLibrary = "library"

var ErrLibraryAddEmpty = errors.New("no models to add")

// LibraryItem is one library model as the add-models dialog shows it.
type LibraryItem struct {
	*ModelLibraryEntry
	// InCatalog is true when a catalog row with this name already exists.
	InCatalog bool `json:"in_catalog"`
	// Channels are the enabled channels that list the model or a platform spelling of it.
	Channels []LibraryChannel `json:"channels"`
}

type LibraryChannel struct {
	Id            int    `json:"id"`
	Name          string `json:"name"`
	Type          int    `json:"type"`
	UpstreamModel string `json:"upstream_model"`
	NameMatch
	// Excluded is true when an admin rejected this match before; it is listed unselected.
	Excluded bool `json:"excluded,omitempty"`
}

// selectedByDefault reports whether the channel should be selected by default.
func (c LibraryChannel) selectedByDefault() bool { return c.Default && !c.Excluded }

// channelOffers works out, for each public name, which enabled channels can supply it and under
// which upstream name, with how sure the match is (design/model-library.md §7). Per channel and
// public name only the best upstream name is kept. An upstream name that is exactly or by
// platform spelling some public name is not offered as a variant or pending match of another.
func channelOffers(publicNames []string) (map[string][]LibraryChannel, error) {
	channels, err := GetAllChannels()
	if err != nil {
		return nil, err
	}
	aliases, err := loadChannelAliases()
	if err != nil {
		return nil, err
	}
	index := newPublicNameIndex(publicNames)
	out := map[string][]LibraryChannel{}
	for _, ch := range channels {
		if ch.Status != config.ChannelStatusEnabled {
			continue
		}
		best := map[string]LibraryChannel{} // by lower-cased public name
		var order []string
		for _, upstream := range SplitChannelModels(ch.Models) {
			if upstream == "" || strings.HasSuffix(upstream, "*") {
				continue
			}
			found := map[string]NameMatch{}
			consider := func(public string, m NameMatch) {
				key := strings.ToLower(public)
				if cur, ok := found[key]; !ok || tierRank(m.Tier) < tierRank(cur.Tier) {
					found[key] = m
				}
			}
			consider(upstream, NameMatch{Tier: MatchExact, Default: true})
			for _, public := range LibraryPublicNames(upstream) {
				consider(public, NameMatch{Tier: MatchPlatform, Default: true})
			}
			for _, public := range index.candidates(upstream, ch.Type) {
				if m, ok := matchModelName(upstream, public, ch.Type); ok {
					consider(public, m)
				}
			}
			for key := range found {
				if excluded, ok := aliases[newChannelAliasKey(ch.Id, upstream, key)]; ok && !excluded {
					found[key] = NameMatch{Tier: MatchExact, Default: true}
				}
			}
			// The upstream name is someone else's variant only if it is not a public model itself.
			strong := false
			for key, m := range found {
				if (m.Tier == MatchExact || m.Tier == MatchPlatform) && index.has(key) {
					strong = true
				}
			}
			for key, m := range found {
				if strong && (m.Tier == MatchVariant || m.Tier == MatchPending) {
					continue
				}
				offer := LibraryChannel{Id: ch.Id, Name: ch.Name, Type: ch.Type, UpstreamModel: upstream, NameMatch: m}
				if excluded, ok := aliases[newChannelAliasKey(ch.Id, upstream, key)]; ok && excluded {
					offer.Excluded = true
				}
				cur, ok := best[key]
				switch {
				case !ok:
					order = append(order, key)
					best[key] = offer
				case betterOffer(offer, cur):
					best[key] = offer
				}
			}
		}
		for _, key := range order {
			out[key] = append(out[key], best[key])
		}
	}
	for key := range out {
		list := out[key]
		sort.SliceStable(list, func(i, j int) bool { return tierRank(list[i].Tier) < tierRank(list[j].Tier) })
	}
	return out, nil
}

// betterOffer prefers a stronger tier, then a default-selected one, then the shorter upstream name
// (the plain spelling over a decorated one).
func betterOffer(a, b LibraryChannel) bool {
	if ra, rb := tierRank(a.Tier), tierRank(b.Tier); ra != rb {
		return ra < rb
	}
	if a.selectedByDefault() != b.selectedByDefault() {
		return a.selectedByDefault()
	}
	return len(a.UpstreamModel) < len(b.UpstreamModel)
}

// ListLibraryItems returns the whole library with catalog membership and the channels that can
// supply each model. Filtering is left to the dialog (the library is a few hundred rows).
func ListLibraryItems() ([]*LibraryItem, error) {
	var entries []*ModelLibraryEntry
	if err := DB.Order("released_at desc, model").Find(&entries).Error; err != nil {
		return nil, err
	}
	var names []string
	if err := DB.Model(&ModelInfo{}).Pluck("model", &names).Error; err != nil {
		return nil, err
	}
	inCatalog := make(map[string]bool, len(names))
	for _, n := range names {
		inCatalog[strings.ToLower(n)] = true
	}
	publicNames := make([]string, 0, len(entries))
	for _, e := range entries {
		publicNames = append(publicNames, e.Model)
	}
	offers, err := channelOffers(publicNames)
	if err != nil {
		return nil, err
	}
	items := make([]*LibraryItem, 0, len(entries))
	for _, e := range entries {
		chs := offers[strings.ToLower(e.Model)]
		if chs == nil {
			chs = []LibraryChannel{}
		}
		items = append(items, &LibraryItem{ModelLibraryEntry: e, InCatalog: inCatalog[strings.ToLower(e.Model)], Channels: chs})
	}
	return items, nil
}

// LibraryAddRequest adds library models to the public catalog.
type LibraryAddRequest struct {
	// Canonicals are library canonical ids ("anthropic/claude-fable-5-2").
	Canonicals []string `json:"canonicals"`
	// Status is draft or published.
	Status string `json:"status"`
	// Supplies creates supplies for the added models.
	Supplies bool `json:"supplies"`
	// Channels, by canonical id, are the channels the admin picked for that model. A model
	// missing here gets the channels selected by default. Picking a pending or variant match
	// confirms it as a channel alias; leaving out a default one records it as rejected.
	Channels map[string][]LibraryAddChannel `json:"channels"`
}

// LibraryAddChannel is one picked channel and the upstream name it was offered under.
type LibraryAddChannel struct {
	Id            int    `json:"id"`
	UpstreamModel string `json:"upstream_model"`
}

type LibraryAddResult struct {
	Added    []string `json:"added"`
	Skipped  []string `json:"skipped"`
	Priced   int      `json:"priced"`
	Supplies int      `json:"supplies"`
}

// libraryModelInfo builds the catalog row for a library model.
func libraryModelInfo(e *ModelLibraryEntry) *ModelInfo {
	return &ModelInfo{
		Model:            e.Model,
		Name:             e.Name,
		ContextLength:    e.ContextLength,
		MaxTokens:        e.MaxTokens,
		InputModalities:  e.InputModalities,
		OutputModalities: e.OutputModalities,
		Capabilities:     e.Capabilities,
		Endpoints:        e.Endpoints,
		VendorID:         e.VendorID,
		OpenWeights:      e.OpenWeights,
		ReleasedAt:       e.ReleasedAt,
	}
}

// libraryPrice is the lab's list price turned into a price row, nil when the library has none.
func libraryPrice(e *ModelLibraryEntry) *Price {
	m := catalogModel{Cost: catalogCost{Input: e.InputPrice, Output: e.OutputPrice, CacheRead: e.CacheReadPrice}}
	if err := jsonUnmarshalStrings(e.InputModalities, &m.Modalities.Input); err != nil {
		return nil
	}
	if err := jsonUnmarshalStrings(e.OutputModalities, &m.Modalities.Output); err != nil {
		return nil
	}
	price, _ := catalogPrice(e.Model, catalogProviderToChannelType[e.Lab], m)
	return price
}

// AddLibraryModels adds library models to the catalog: one row each (taken from the library),
// a price from the lab's list price when the model has none yet, and with Supplies a supply on
// each picked channel (see LibraryAddRequest.Channels). Models already in the catalog are skipped.
func AddLibraryModels(req LibraryAddRequest) (*LibraryAddResult, error) {
	if len(req.Canonicals) == 0 {
		return nil, ErrLibraryAddEmpty
	}
	status := req.Status
	if status == "" {
		status = ModelStatusDraft
	}
	if status != ModelStatusDraft && status != ModelStatusPublished {
		return nil, ErrModelStatus
	}
	var entries []*ModelLibraryEntry
	if err := DB.Where("canonical IN ?", req.Canonicals).Find(&entries).Error; err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Model < entries[j].Model })
	var offers map[string][]LibraryChannel
	if req.Supplies {
		var err error
		publicNames := make([]string, 0, len(entries))
		for _, e := range entries {
			publicNames = append(publicNames, e.Model)
		}
		if offers, err = channelOffers(publicNames); err != nil {
			return nil, err
		}
	}
	result := &LibraryAddResult{Added: []string{}, Skipped: []string{}}
	var prices []*Price
	for _, e := range entries {
		if existing, _ := GetModelInfoByModel(e.Model); existing != nil {
			result.Skipped = append(result.Skipped, e.Model)
			continue
		}
		info := libraryModelInfo(e)
		info.Status = status
		info.Hidden = status != ModelStatusPublished
		info.SupplyMode = SupplyModeDefault
		info.Source = ModelInfoSourceLibrary
		if err := DB.Create(info).Error; err != nil {
			return nil, err
		}
		result.Added = append(result.Added, e.Model)
		if _, ok := PricingInstance.priceSnapshot(e.Model); !ok {
			if p := libraryPrice(e); p != nil {
				prices = append(prices, p)
			}
		}
		for _, ch := range pickLibraryChannels(e, offers[strings.ToLower(e.Model)], req.Channels) {
			channel, err := GetChannelById(ch.Id)
			if err != nil {
				continue
			}
			upstream := ch.UpstreamModel
			if strings.EqualFold(upstream, e.Model) {
				// The channel lists the public name itself; its model mapping may still rename it.
				upstream = SuggestSupplyUpstream(e.Model, channel).UpstreamModel
			}
			supply := &ModelSupply{
				Model: e.Model, ChannelId: ch.Id, Enabled: true, Weight: 1,
				UpstreamModel: upstream,
				Source:        ModelSupplySourceLibrary,
			}
			if err := CreateModelSupply(supply); err == nil {
				result.Supplies++
			}
		}
	}
	if len(prices) > 0 && PricingInstance != nil {
		if err := PricingInstance.UpsertPrices(prices); err != nil {
			return nil, err
		}
		result.Priced = len(prices)
	}
	if len(result.Added) > 0 {
		refreshCatalogCaches()
	}
	return result, nil
}

// pickLibraryChannels returns the offers to supply a model from. Without a pick for the model the
// default-selected offers are used. With one, only picked offers count (anything not offered is
// ignored), and the decisions are remembered per channel: a picked match that was not selected by
// default because the rules were unsure is confirmed, and a default one left out is rejected.
func pickLibraryChannels(e *ModelLibraryEntry, offers []LibraryChannel, picks map[string][]LibraryAddChannel) []LibraryChannel {
	picked, ok := picks[e.Canonical]
	if !ok {
		var out []LibraryChannel
		for _, o := range offers {
			if o.selectedByDefault() {
				out = append(out, o)
			}
		}
		return out
	}
	chosen := map[channelAliasKey]bool{}
	for _, p := range picked {
		chosen[newChannelAliasKey(p.Id, p.UpstreamModel, e.Model)] = true
	}
	var out []LibraryChannel
	for _, o := range offers {
		key := newChannelAliasKey(o.Id, o.UpstreamModel, e.Model)
		switch {
		case chosen[key]:
			out = append(out, o)
			if o.Excluded || o.Tier == MatchPending || o.Tier == MatchVariant {
				_ = SaveChannelAlias(o.Id, o.UpstreamModel, e.Model, false)
			}
		case o.selectedByDefault():
			_ = SaveChannelAlias(o.Id, o.UpstreamModel, e.Model, true)
		}
	}
	return out
}

// jsonUnmarshalStrings reads a JSON string array; an empty string is an empty array.
func jsonUnmarshalStrings(raw string, out *[]string) error {
	if raw == "" {
		*out = nil
		return nil
	}
	return json.Unmarshal([]byte(raw), out)
}
