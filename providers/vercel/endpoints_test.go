package vercel

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
)

func ep(provider, prompt, completion string) modelEndpoint {
	return modelEndpoint{ProviderName: provider, Pricing: endpointPricing{Prompt: prompt, Completion: completion}}
}

// The vendor's own hosting wins; without it (or without a token price there) nothing is picked
// and the listed price stays.
func TestRepresentativePricing(t *testing.T) {
	qwen := []modelEndpoint{
		ep("wafer", "0.00000006", "0.0000022"),
		ep("alibaba", "0.0000005", "0.000003"),
		ep("deepinfra", "0.00000015", "0.000001875"),
	}
	if got, ok := representativePricing("alibaba/qwen3.8-27b", qwen); !ok || got.Prompt != "0.0000005" {
		t.Errorf("first party: %+v %v", got, ok)
	}
	grok := []modelEndpoint{ep("vertex", "0.000002", "0.000006"), ep("XAI", "0.00000125", "0.0000025")}
	if got, _ := representativePricing("spacexai/grok-4.20-reasoning", grok); got.Prompt != "0.00000125" {
		t.Errorf("aliased first party: %+v", got)
	}
	if _, ok := representativePricing("meta/llama-4-scout", []modelEndpoint{ep("bedrock", "0.00000017", "0.00000066")}); ok {
		t.Error("picked a provider other than the vendor")
	}
	if _, ok := representativePricing("x/y", []modelEndpoint{ep("x", "0", "")}); ok {
		t.Error("picked a vendor endpoint without a token price")
	}
}

// Models hosted by several providers take the representative provider's price in both the price
// sync and the channel price list; the others, and a model whose endpoints fail, keep the list.
func TestFetchPricedModels(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models":
			_, _ = w.Write([]byte(`{"data":[
			 {"id":"alibaba/qwen3.8-27b","type":"language","pricing":{"input":"0.00000006","output":"0.0000022","input_cache_read":"0.000000059","varies_by_provider":true}},
			 {"id":"broken/model","type":"language","pricing":{"input":"0.000001","output":"0.000002","varies_by_provider":true}},
			 {"id":"openai/gpt-x","type":"language","pricing":{"input":"0.000005","output":"0.00003"}}]}`))
		case "/v1/models/alibaba/qwen3.8-27b/endpoints":
			_, _ = w.Write([]byte(`{"data":{"endpoints":[
			 {"provider_name":"wafer","pricing":{"prompt":"0.00000006","completion":"0.0000022","input_cache_read":"0.000000059"}},
			 {"provider_name":"alibaba","pricing":{"prompt":"0.0000005","completion":"0.000003","input_cache_read":"0.0000001","input_cache_write":"0.000000625"}}]}}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
		}
	}))
	defer upstream.Close()
	baseURL := upstream.URL
	p := VercelProviderFactory{}.Create(&model.Channel{Type: config.ChannelTypeVercelAIGateway, Key: "vck_test", BaseURL: &baseURL}).(*VercelProvider)

	prices, err := p.GetChannelPrices()
	if err != nil {
		t.Fatal(err)
	}
	byModel := map[string]*model.ChannelPrice{}
	for _, cp := range prices {
		byModel[cp.Model] = cp
	}
	near := func(a, b float64) bool { return a > b-1e-9 && a < b+1e-9 }
	if q := byModel["alibaba/qwen3.8-27b"]; q == nil || !near(q.Input, 0.5) || !near(q.Output, 3) || !near(q.CacheRead, 0.1) || !near(q.CacheWrite, 0.625) {
		t.Errorf("qwen = %+v", q)
	}
	if b := byModel["broken/model"]; b == nil || !near(b.Input, 1) {
		t.Errorf("failed endpoints keep the list price: %+v", b)
	}
	if g := byModel["openai/gpt-x"]; g == nil || !near(g.Input, 5) {
		t.Errorf("single-provider model: %+v", g)
	}
}
