package model

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestModelsDevChannelPrices(t *testing.T) {
	raw := `{"togetherai":{"models":{
		"meta-llama/Llama-3.3-70B-Instruct-Turbo":{"id":"meta-llama/Llama-3.3-70B-Instruct-Turbo","cost":{"input":0.88,"output":0.88,"cache_read":0.44,"cache_write":1.1},"modalities":{"output":["text"]}},
		"black-forest-labs/FLUX.1-schnell":{"cost":{"input":0.003},"modalities":{"output":["image"]}},
		"unpriced":{"modalities":{"output":["text"]}}
	}}}`
	calls := 0
	old := modelsDevFetch
	modelsDevFetch = func() (map[string]catalogProvider, error) {
		calls++
		var data map[string]catalogProvider
		err := json.Unmarshal([]byte(raw), &data)
		return data, err
	}
	modelsDevCache.data = nil
	t.Cleanup(func() { modelsDevFetch = old; modelsDevCache.data = nil })

	prices, err := ModelsDevChannelPrices("togetherai")
	if err != nil || len(prices) != 2 {
		t.Fatalf("prices = %+v, %v", prices, err)
	}
	flux, llama := prices[0], prices[1]
	if llama.Input != 0.88 || llama.CacheRead != 0.44 || llama.CacheWrite != 1.1 || llama.Kind != "" {
		t.Errorf("llama = %+v", llama)
	}
	if flux.Model != "black-forest-labs/FLUX.1-schnell" || flux.Kind != ModelKindImage {
		t.Errorf("flux = %+v", flux)
	}
	ids, _ := ModelsDevModelIDs("togetherai")
	if len(ids) != 3 {
		t.Errorf("ids = %v", ids)
	}
	if calls != 1 {
		t.Errorf("one download should serve both lookups, got %d", calls)
	}

	// A failed refresh keeps the last good copy.
	modelsDevCache.fetched = modelsDevCache.fetched.Add(-2 * modelsDevCacheTTL)
	modelsDevFetch = func() (map[string]catalogProvider, error) { return nil, errors.New("down") }
	if prices, err := ModelsDevChannelPrices("togetherai"); err != nil || len(prices) != 2 {
		t.Errorf("stale copy = %d, %v", len(prices), err)
	}
}
