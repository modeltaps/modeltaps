package openrouter

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

func ep(tag, prompt, completion string) modelEndpoint {
	return modelEndpoint{Tag: tag, Pricing: endpointPricing{Prompt: prompt, Completion: completion}, SupportsImplicitCaching: true}
}

func quantized(e modelEndpoint, q string) modelEndpoint {
	e.Quantization = &q
	return e
}

func names(order []string) string { return strings.Join(order, ",") }

func withCostSettings(t *testing.T, order string, success, saving float64) {
	t.Helper()
	oldOrder, oldSuccess, oldSaving := config.SupplyCostOrder, config.SupplyCostMinSuccessPercent, config.SupplyCostMinSavingPercent
	t.Cleanup(func() {
		config.SupplyCostOrder, config.SupplyCostMinSuccessPercent, config.SupplyCostMinSavingPercent = oldOrder, oldSuccess, oldSaving
	})
	config.SupplyCostOrder, config.SupplyCostMinSuccessPercent, config.SupplyCostMinSavingPercent = order, success, saving
}

// The vendor's own endpoint leads; qualifying endpoints follow cheapest first. Endpoints that are
// down, up too little, at another tier, less precise than the vendor, short of the model's
// context, without caching when the vendor caches, or priced far below the vendor are left out.
func TestPlanHostsWithVendor(t *testing.T) {
	withCostSettings(t, model.SupplyCostOrderPriority, 95, 5)
	low, flex := 90.0, "flex"
	flaky := ep("flaky", "0.0000013", "0.000004")
	flaky.Uptime1d = &low
	down := ep("down", "0.0000013", "0.000004")
	down.Status = 1
	short := ep("short", "0.0000013", "0.000004")
	short.ContextLength = 131072
	nocache := ep("nocache", "0.0000013", "0.000004")
	nocache.SupportsImplicitCaching = false
	tier := ep("deepinfra/fp8/flex", "0.0000009", "0.000003")
	tier.ServiceTier = &flex
	m := modelWithPricing{Id: "z-ai/glm-5.3", ContextLen: 1048576}
	endpoints := []modelEndpoint{
		quantized(ep("wafer/us", "0.0000000699", "0.0000014"), "fp4"), // fp4 and input at 5% of the vendor
		quantized(ep("deepinfra/fp4", "0.0000005625", "0.0000025"), "fp4"),
		quantized(ep("novita/fp8", "0.00000063", "0.00000198"), "fp8"),
		quantized(ep("z-ai/fp8", "0.0000014", "0.0000044"), "fp8"),
		ep("together", "0.0000014", "0.0000044"),
		ep("cheap", "0.0000002", "0.000004"), // input at 14% of the vendor
		flaky, down, short, nocache, tier,
	}
	order, price, ok := planHosts(m, endpoints)
	if !ok || names(order) != "z-ai/fp8,novita/fp8,together" || price.Prompt != "0.0000014" {
		t.Fatalf("priority first: %v %+v %v", order, price, ok)
	}

	// Cost first: endpoints cheaper than the vendor by the minimum saving go ahead of it.
	config.SupplyCostOrder = model.SupplyCostOrderCost
	order, price, _ = planHosts(m, endpoints)
	if names(order) != "novita/fp8,z-ai/fp8,together" || price.Prompt != "0.00000063" {
		t.Fatalf("cost first: %v %+v", order, price)
	}
	config.SupplyCostOrder = model.SupplyCostOrderPriority

	// A vendor that is down goes last; the first qualifying endpoint's price is the cost.
	vendorDown := append([]modelEndpoint{}, endpoints...)
	vendorDown[3].Status = 1
	order, price, _ = planHosts(m, vendorDown)
	if names(order) != "novita/fp8,together,z-ai/fp8" || price.Prompt != "0.00000063" {
		t.Fatalf("vendor down: %v %+v", order, price)
	}

	// The vendor's hosting under another slug.
	qwen := []modelEndpoint{ep("deepinfra", "0.0000006", "0.000003"), ep("alibaba", "0.0000005", "0.000003")}
	if order, _, _ := planHosts(modelWithPricing{Id: "qwen/qwen3.8-27b"}, qwen); names(order) != "alibaba,deepinfra" {
		t.Errorf("aliased vendor: %v", order)
	}
	// At most maxHostOrder endpoints are named.
	many := []modelEndpoint{ep("z-ai", "0.0000014", "0.0000044")}
	for _, h := range []string{"a", "b", "c", "d", "e", "f", "g"} {
		many = append(many, ep(h, "0.0000014", "0.0000044"))
	}
	if order, _, _ := planHosts(m, many); len(order) != maxHostOrder || order[0] != "z-ai" {
		t.Errorf("long order: %v", order)
	}
}

// Without the vendor the median-priced serving endpoint is the yardstick: endpoints far below it
// are left out and the rest, the reference among them, are ordered cheapest first.
func TestPlanHostsWithoutVendor(t *testing.T) {
	withCostSettings(t, model.SupplyCostOrderPriority, 95, 5)
	m := modelWithPricing{Id: "minimax/minimax-m3.1-flash"}
	endpoints := []modelEndpoint{
		ep("wafer", "0.00000003", "0.0000012"), // input at 10% of the median
		ep("nebius", "0.0000003", "0.0000012"),
		ep("together", "0.0000003", "0.0000011"),
		ep("parasail", "0.0000004", "0.0000015"),
		ep("fireworks", "0.0000004", "0.0000016"),
	}
	order, price, ok := planHosts(m, endpoints)
	if !ok || names(order) != "together,nebius,parasail,fireworks" || price.Completion != "0.0000011" {
		t.Fatalf("no vendor: %v %+v %v", order, price, ok)
	}
	// A model served by one endpoint is pinned to it.
	if order, price, ok := planHosts(modelWithPricing{Id: "meta-llama/llama-guard-4-12b"}, []modelEndpoint{ep("deepinfra", "0.00000018", "0.00000018")}); !ok ||
		names(order) != "deepinfra" || price.Prompt != "0.00000018" {
		t.Errorf("single endpoint: %v %+v %v", order, price, ok)
	}
	// The most precise stated weights set the floor: endpoints stating less are left out, those
	// not saying stay.
	mixed := []modelEndpoint{
		quantized(ep("wafer/mxfp4", "0.00000005", "0.00000057"), "mxfp4"),
		quantized(ep("relace/fp4", "0.00000012", "0.0000005"), "fp4"),
		quantized(ep("deepinfra/fp8", "0.00000014", "0.00000042"), "fp8"),
		ep("river", "0.00000012", "0.00000048"),
		quantized(ep("novita/fp8", "0.00000015", "0.0000006"), "fp8"),
	}
	if order, _, _ := planHosts(modelWithPricing{Id: "deepseek/deepseek-v4.1-flash"}, mixed); names(order) != "river,deepinfra/fp8,novita/fp8" {
		t.Errorf("precision floor: %v", order)
	}
	// A vendor not stating its precision is taken as full weights.
	kimi := []modelEndpoint{ep("moonshotai", "0.000003", "0.000015"), quantized(ep("deepinfra/mxfp4", "0.00000285", "0.00001425"), "mxfp4"), ep("together", "0.000003", "0.000015")}
	if order, _, _ := planHosts(modelWithPricing{Id: "moonshotai/kimi-k3"}, kimi); names(order) != "moonshotai,together" {
		t.Errorf("unstated vendor precision: %v", order)
	}
	// Every endpoint short of the listed context: the reference is pinned.
	shortA, shortB := ep("a", "0.0000003", "0.0000012"), ep("b", "0.0000004", "0.0000015")
	shortA.ContextLength, shortB.ContextLength = 131072, 131072
	if order, _, ok := planHosts(modelWithPricing{Id: "minimax/minimax-m3.1-flash", ContextLen: 1048576}, []modelEndpoint{shortA, shortB}); !ok || names(order) != "b" {
		t.Errorf("all short: %v %v", order, ok)
	}
	// Nothing serving and no vendor: no plan.
	down := ep("deepinfra", "0.00000018", "0.00000018")
	down.Status = 1
	if _, _, ok := planHosts(m, []modelEndpoint{down}); ok {
		t.Error("planned with nothing serving")
	}
}

// The hook writes the order kept with the model's channel price into chat, responses and messages
// bodies, keeping the other provider preferences; an order already in the body, other paths and
// models without an order are sent as they are.
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
	if err := model.ReplaceChannelPrices(9, []*model.ChannelPrice{
		{Model: "z-ai/glm-5.3", Input: 1.4, Output: 4.4, ProviderOrder: "z-ai/fp8,novita/fp8"},
		{Model: "openai/gpt-x", Input: 5, Output: 30},
	}); err != nil {
		t.Fatal(err)
	}
	hook := hostOrderHook(&model.Channel{Id: 9})
	send := func(path, body string) string {
		req := httptest.NewRequest(http.MethodPost, "https://openrouter.ai/api"+path, strings.NewReader(body))
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
		got := send(path, `{"model":"z-ai/glm-5.3","stream":true,"provider":{}}`)
		if gjson.Get(got, "provider.order").Raw != `["z-ai/fp8","novita/fp8"]` || !gjson.Get(got, "stream").Bool() {
			t.Errorf("%s: %s", path, got)
		}
	}
	got := send("/v1/chat/completions", `{"model":"z-ai/glm-5.3","provider":{"data_collection":"deny"}}`)
	if gjson.Get(got, "provider.order").Raw != `["z-ai/fp8","novita/fp8"]` || gjson.Get(got, "provider.data_collection").String() != "deny" {
		t.Errorf("other preferences: %s", got)
	}
	for _, c := range []struct{ path, body string }{
		{"/v1/chat/completions", `{"model":"z-ai/glm-5.3","provider":{"order":["together"]}}`},
		{"/v1/chat/completions", `{"model":"openai/gpt-x"}`},
		{"/v1/audio/speech", `{"model":"z-ai/glm-5.3"}`},
		{"/v1/chat/completions", `not json`},
	} {
		if got := send(c.path, c.body); got != c.body {
			t.Errorf("%s %s changed: %s", c.path, c.body, got)
		}
	}
}

// Planned models take the first endpoint's price and keep the order in the channel price list;
// a model whose endpoints fail keeps the listed price, and free models are not planned.
func TestFetchPricedModels(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	withCostSettings(t, model.SupplyCostOrderPriority, 95, 5)
	var asked []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		asked = append(asked, r.URL.Path)
		switch r.URL.Path {
		case "/v1/models":
			if r.URL.Query().Get("output_modalities") != "" {
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[
			 {"id":"z-ai/glm-5.3","context_length":1048576,"pricing":{"prompt":"0.0000000699","completion":"0.0000014","input_cache_read":"0.0000000689"}},
			 {"id":"broken/model","pricing":{"prompt":"0.000001","completion":"0.000002"}},
			 {"id":"z-ai/glm-5.3-flash:free","pricing":{"prompt":"0","completion":"0"}}]}`))
		case "/v1/models/z-ai/glm-5.3/endpoints":
			_, _ = w.Write([]byte(`{"data":{"endpoints":[
			 {"tag":"wafer/us","quantization":"fp4","status":0,"supports_implicit_caching":true,"pricing":{"prompt":"0.0000000699","completion":"0.0000014","input_cache_read":"0.0000000689"}},
			 {"tag":"z-ai/fp8","quantization":"fp8","status":0,"supports_implicit_caching":true,"pricing":{"prompt":"0.0000014","completion":"0.0000044","input_cache_read":"0.00000026"}}]}}`))
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
		}
	}))
	defer upstream.Close()
	baseURL := upstream.URL
	p := OpenRouterProviderFactory{}.Create(&model.Channel{Type: config.ChannelTypeOpenRouter, Key: "sk-or-test", BaseURL: &baseURL}).(*OpenRouterProvider)

	prices, err := p.GetChannelPrices()
	if err != nil {
		t.Fatal(err)
	}
	byModel := map[string]*model.ChannelPrice{}
	for _, cp := range prices {
		byModel[cp.Model] = cp
	}
	near := func(a, b float64) bool { return a > b-1e-9 && a < b+1e-9 }
	if g := byModel["z-ai/glm-5.3"]; g == nil || !near(g.Input, 1.4) || !near(g.Output, 4.4) || !near(g.CacheRead, 0.26) || g.ProviderOrder != "z-ai/fp8" {
		t.Errorf("glm = %+v", g)
	}
	if b := byModel["broken/model"]; b == nil || !near(b.Input, 1) || b.ProviderOrder != "" {
		t.Errorf("failed endpoints keep the list price: %+v", b)
	}
	for _, path := range asked {
		if strings.Contains(path, ":free") {
			t.Errorf("free model planned: %s", path)
		}
	}
}
