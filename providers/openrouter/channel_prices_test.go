package openrouter

import (
	"encoding/json"
	"testing"
)

func TestBuildOpenRouterChannelPrices(t *testing.T) {
	resp := &modelPricingResponse{}
	raw := `{"data": [
	  {"id": "anthropic/claude-3.5-sonnet", "pricing": {"prompt": "0.000003", "completion": "0.000015", "request": "0", "input_cache_read": "0.0000003", "input_cache_write": "0.00000375"}},
	  {"id": "vendor/free-model:free", "pricing": {"prompt": "0", "completion": "0", "request": "0"}},
	  {"id": "vendor/per-call", "pricing": {"prompt": "0", "completion": "0", "request": "0.01"}},
	  {"id": "vendor/unpriced", "pricing": {}}
	]}`
	if err := json.Unmarshal([]byte(raw), resp); err != nil {
		t.Fatal(err)
	}
	prices := buildOpenRouterChannelPrices(resp)
	if len(prices) != 3 {
		t.Fatalf("prices = %d, want 3: %+v", len(prices), prices)
	}
	sonnet := prices[0]
	if sonnet.Input != 3 || sonnet.Output != 15 || sonnet.CacheRead != 0.3 || sonnet.CacheWrite != 3.75 {
		t.Errorf("sonnet = %+v, want 3 / 15 / 0.3 / 3.75 per million", sonnet)
	}
	if free := prices[1]; free.Model != "vendor/free-model:free" || free.Input != 0 || free.Output != 0 {
		t.Errorf("free model = %+v, want kept with zero prices", free)
	}
	if perCall := prices[2]; perCall.Request != 0.01 {
		t.Errorf("per-call model = %+v, want 0.01 per request", perCall)
	}
}
