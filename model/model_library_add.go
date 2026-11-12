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
}

// channelOffers indexes which enabled channels list which public names: a listed name counts for
// itself and for every public name the library ties it to.
func channelOffers() (map[string][]LibraryChannel, error) {
	channels, err := GetAllChannels()
	if err != nil {
		return nil, err
	}
	out := map[string][]LibraryChannel{}
	for _, ch := range channels {
		if ch.Status != config.ChannelStatusEnabled {
			continue
		}
		seen := map[string]bool{}
		add := func(public, upstream string) {
			key := strings.ToLower(public)
			if seen[key] {
				return
			}
			seen[key] = true
			out[key] = append(out[key], LibraryChannel{Id: ch.Id, Name: ch.Name, Type: ch.Type, UpstreamModel: upstream})
		}
		for _, name := range SplitChannelModels(ch.Models) {
			add(name, name)
			add(bareModelID(name), name)
			for _, public := range LibraryPublicNames(name) {
				add(public, name)
			}
		}
	}
	return out, nil
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
	offers, err := channelOffers()
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
	// Supplies creates a supply on every enabled channel that lists the model.
	Supplies bool `json:"supplies"`
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
// every enabled channel that lists the model. Models already in the catalog are skipped.
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
		if offers, err = channelOffers(); err != nil {
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
		info.SupplyMode = SupplyModeBalance
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
		for _, ch := range offers[strings.ToLower(e.Model)] {
			channel, err := GetChannelById(ch.Id)
			if err != nil {
				continue
			}
			supply := &ModelSupply{
				Model: e.Model, ChannelId: ch.Id, Enabled: true, Weight: 1,
				UpstreamModel: SuggestSupplyUpstream(e.Model, channel).UpstreamModel,
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

// jsonUnmarshalStrings reads a JSON string array; an empty string is an empty array.
func jsonUnmarshalStrings(raw string, out *[]string) error {
	if raw == "" {
		*out = nil
		return nil
	}
	return json.Unmarshal([]byte(raw), out)
}
