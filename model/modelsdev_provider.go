package model

import (
	"sort"
	"sync"
	"time"
)

// modelsDevCacheTTL is how long one download of models.dev serves per-provider lookups.
const modelsDevCacheTTL = 6 * time.Hour

var modelsDevCache struct {
	sync.Mutex
	data    map[string]catalogProvider
	fetched time.Time
}

// modelsDevFetch downloads models.dev; tests replace it.
var modelsDevFetch = func() (map[string]catalogProvider, error) { return fetchCatalogData("") }

// presetModelsDevModels returns one models.dev provider's models, from a download shared for
// modelsDevCacheTTL. A failed download keeps serving the last good copy.
func presetModelsDevModels(provider string) (map[string]catalogModel, error) {
	modelsDevCache.Lock()
	defer modelsDevCache.Unlock()
	if modelsDevCache.data == nil || time.Since(modelsDevCache.fetched) > modelsDevCacheTTL {
		data, err := modelsDevFetch()
		if err != nil {
			if modelsDevCache.data == nil {
				return nil, err
			}
		} else {
			modelsDevCache.data, modelsDevCache.fetched = data, time.Now()
		}
	}
	return modelsDevCache.data[provider].Models, nil
}

// ModelsDevModelIDs lists the model ids models.dev has for a provider, sorted.
func ModelsDevModelIDs(provider string) ([]string, error) {
	models, err := presetModelsDevModels(provider)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(models))
	for key, m := range models {
		id := m.Id
		if id == "" {
			id = key
		}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids, nil
}

// ModelsDevChannelPrices is a provider's price list as models.dev records it, in USD per million
// tokens, for a channel's own price list. Models without a price are left out.
func ModelsDevChannelPrices(provider string) ([]*ChannelPrice, error) {
	models, err := presetModelsDevModels(provider)
	if err != nil {
		return nil, err
	}
	out := make([]*ChannelPrice, 0, len(models))
	for key, m := range models {
		if m.Cost.Input == nil && m.Cost.Output == nil {
			continue
		}
		id := m.Id
		if id == "" {
			id = key
		}
		out = append(out, &ChannelPrice{
			Model:      id,
			Input:      costValue(m.Cost.Input),
			Output:     costValue(m.Cost.Output),
			CacheRead:  costValue(m.Cost.CacheRead),
			CacheWrite: costValue(m.Cost.CacheWrite),
			Kind:       ModelKindFromOutputModalities(m.Modalities.Output),
		})
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Model < out[b].Model })
	return out, nil
}
