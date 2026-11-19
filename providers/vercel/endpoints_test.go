package vercel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/tidwall/gjson"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func ep(provider, prompt, completion string) modelEndpoint {
	return modelEndpoint{ProviderName: provider, Pricing: endpointPricing{Prompt: prompt, Completion: completion}, Tags: []string{"implicit-caching"}}
}

func names(order []string) string { return strings.Join(order, ",") }

// The vendor's own hosting leads; qualifying hosts follow cheapest first. Hosts that are down,
// up too little, quantized, short of the model's context, without caching when the vendor caches,
// or priced far below the vendor are left out. Without the vendor there is no plan.
func TestPlanHosts(t *testing.T) {
	oldOrder, oldSuccess, oldSaving := config.SupplyCostOrder, config.SupplyCostMinSuccessPercent, config.SupplyCostMinSavingPercent
	t.Cleanup(func() {
		config.SupplyCostOrder, config.SupplyCostMinSuccessPercent, config.SupplyCostMinSavingPercent = oldOrder, oldSuccess, oldSaving
	})
	config.SupplyCostOrder, config.SupplyCostMinSuccessPercent, config.SupplyCostMinSavingPercent = model.SupplyCostOrderPriority, 95, 5
	fp8, low, short := "fp8", 90.0, modelEndpoint{}
	short = ep("short", "0.0000004", "0.0000025")
	short.ContextLength = 131072
	quant := ep("quant", "0.0000003", "0.0000022")
	quant.Quantization = &fp8
	flaky := ep("flaky", "0.0000004", "0.0000024")
	flaky.Uptime1d = &low
	down := ep("down", "0.0000004", "0.0000024")
	down.Status = 1
	nocache := ep("nocache", "0.0000004", "0.0000024")
	nocache.Tags = nil
	m := gatewayModel{Id: "alibaba/qwen3.8-27b", ContextWindow: 262144}
	endpoints := []modelEndpoint{
		ep("novita", "0.00000042", "0.000003"),
		ep("deepinfra", "0.00000015", "0.000001875"),
		ep("alibaba", "0.0000005", "0.000003"),
		ep("runinfra", "0.00000008", "0.00000032"), // 11% of the vendor: left out
		ep("relace", "0.00000003", "0.0000029"),    // input at 6% of the vendor: left out
		short, quant, flaky, down, nocache,
	}
	order, price, ok := planHosts(m, endpoints)
	if !ok || names(order) != "alibaba,deepinfra,novita" || price.Prompt != "0.0000005" {
		t.Fatalf("priority first: %v %+v %v", order, price, ok)
	}

	// Cost first: hosts cheaper than the vendor by the minimum saving go ahead of it.
	config.SupplyCostOrder = model.SupplyCostOrderCost
	order, price, _ = planHosts(m, endpoints)
	if names(order) != "deepinfra,alibaba,novita" || price.Prompt != "0.00000015" {
		t.Fatalf("cost first: %v %+v", order, price)
	}
	config.SupplyCostMinSavingPercent = 50
	if order, _, _ = planHosts(m, endpoints); names(order) != "alibaba,deepinfra,novita" {
		t.Fatalf("cost first, 50%% saving: %v", order)
	}
	config.SupplyCostOrder = model.SupplyCostOrderPriority

	// A vendor that is down goes last; the first qualifying host's price is the cost.
	vendorDown := append([]modelEndpoint{}, endpoints...)
	vendorDown[2].Status = 1
	order, price, _ = planHosts(m, vendorDown)
	if names(order) != "deepinfra,novita,alibaba" || price.Prompt != "0.00000015" {
		t.Fatalf("vendor down: %v %+v", order, price)
	}

	// The gateway's alias for the vendor's own hosting.
	grok := []modelEndpoint{ep("vertex", "0.000002", "0.000006"), ep("xai", "0.00000125", "0.0000025")}
	if order, _, _ := planHosts(gatewayModel{Id: "spacexai/grok-4.20-reasoning"}, grok); names(order) != "xai,vertex" {
		t.Errorf("aliased vendor: %v", order)
	}
	// At most maxHostOrder hosts are named.
	many := []modelEndpoint{ep("zai", "0.0000014", "0.0000044")}
	for _, h := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		many = append(many, ep(h, "0.0000014", "0.0000044"))
	}
	if order, _, _ := planHosts(gatewayModel{Id: "zai/glm-5.3"}, many); len(order) != maxHostOrder || order[0] != "zai" {
		t.Errorf("long order: %v", order)
	}
	if _, _, ok := planHosts(gatewayModel{Id: "meta/llama-4-scout"}, []modelEndpoint{ep("bedrock", "0.00000017", "0.00000066")}); ok {
		t.Error("planned without the vendor")
	}
}

// The hook writes the order kept with the model's channel price into chat, responses and
// messages bodies, replacing a client's order and keeping its other gateway options; other
// paths and models without an order are sent as they are.
func TestHostOrderHook(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.ChannelPrice{}); err != nil {
		t.Fatal(err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
	if err := model.ReplaceChannelPrices(7, []*model.ChannelPrice{
		{Model: "alibaba/qwen3.8-27b", Input: 0.5, Output: 3, ProviderOrder: "alibaba,deepinfra"},
		{Model: "openai/gpt-x", Input: 5, Output: 30},
	}); err != nil {
		t.Fatal(err)
	}
	hook := hostOrderHook(&model.Channel{Id: 7})
	send := func(path, body string) string {
		req := httptest.NewRequest(http.MethodPost, "https://ai-gateway.vercel.sh"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if err := hook(req); err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(req.Body)
		if req.ContentLength != int64(len(got)) {
			t.Errorf("content length %d for %d bytes", req.ContentLength, len(got))
		}
		return string(got)
	}
	for _, path := range []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"} {
		got := send(path, `{"model":"alibaba/qwen3.8-27b","stream":true}`)
		if gjson.Get(got, "providerOptions.gateway.order").Raw != `["alibaba","deepinfra"]` || !gjson.Get(got, "stream").Bool() {
			t.Errorf("%s: %s", path, got)
		}
	}
	got := send("/v1/chat/completions", `{"model":"alibaba/qwen3.8-27b","providerOptions":{"gateway":{"order":["wafer"],"zdr":true}}}`)
	if gjson.Get(got, "providerOptions.gateway.order").Raw != `["alibaba","deepinfra"]` || !gjson.Get(got, "providerOptions.gateway.zdr").Bool() {
		t.Errorf("client options: %s", got)
	}
	for _, c := range []struct{ path, body string }{
		{"/v1/chat/completions", `{"model":"openai/gpt-x"}`},
		{"/v1/embeddings", `{"model":"alibaba/qwen3.8-27b"}`},
		{"/v1/chat/completions", `not json`},
	} {
		if got := send(c.path, c.body); got != c.body {
			t.Errorf("%s %s changed: %s", c.path, c.body, got)
		}
	}
}

// Models hosted by several providers take the first planned host's price and keep the order in
// the channel price list; the others, and a model whose endpoints fail, keep the list.
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
	if q := byModel["alibaba/qwen3.8-27b"]; q == nil || !near(q.Input, 0.5) || !near(q.Output, 3) || !near(q.CacheRead, 0.1) ||
		!near(q.CacheWrite, 0.625) || q.ProviderOrder != "alibaba" {
		t.Errorf("qwen = %+v", q)
	}
	if b := byModel["broken/model"]; b == nil || !near(b.Input, 1) || b.ProviderOrder != "" {
		t.Errorf("failed endpoints keep the list price: %+v", b)
	}
	if g := byModel["openai/gpt-x"]; g == nil || !near(g.Input, 5) {
		t.Errorf("single-provider model: %+v", g)
	}
}
