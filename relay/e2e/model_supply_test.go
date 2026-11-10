package e2e

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// useSupplies turns supply routing on for the test and loads the given catalog rows and supplies.
func useSupplies(t *testing.T, rows []model.ModelInfo, supplies []model.ModelSupply) []model.ModelSupply {
	t.Helper()
	setAndRestore(t, &config.ModelSupplyEnabled, true)
	for i := range rows {
		if err := model.DB.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	for i := range supplies {
		if err := model.DB.Create(&supplies[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	model.ChannelGroup.Load()
	return supplies
}

func chatFor(modelName string) string {
	return `{"model":"` + modelName + `","messages":[{"role":"user","content":"hi"}]}`
}

// A supply decides the upstream name (the channel's model_mapping is ignored) and the request
// is billed by the public model, not by the upstream name.
func TestSupplySendsUpstreamNameAndBillsPublicModel(t *testing.T) {
	upstream := NewFakeProvider(t)
	upstream.SetResponse(FakeResponse{JSON: ChatCompletionJSON("vendor/gpt-4o-mini", "ok", 11, 7)})
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{{Name: "c", BaseURL: upstream.Server.URL, Models: "unrelated", ModelMapping: `{"gpt-4o-mini":"wrong"}`}},
		Prices: []PriceSpec{
			{Model: "gpt-4o-mini", Input: billingInputRatio, Output: billingOutputRatio},
			{Model: "vendor/gpt-4o-mini", Input: 50, Output: 50},
		},
	})
	supplies := useSupplies(t,
		[]model.ModelInfo{{Model: "gpt-4o-mini", Status: model.ModelStatusPublished}},
		[]model.ModelSupply{{Model: "gpt-4o-mini", ChannelId: h.Channel.Id, UpstreamModel: "vendor/gpt-4o-mini", Enabled: true}})

	quotaBefore := h.UserQuota(t)
	if w := h.Post("/v1/chat/completions", chatFor("gpt-4o-mini")); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if got := upstream.LastRequest(t).JSONBody(t)["model"]; got != "vendor/gpt-4o-mini" {
		t.Fatalf("upstream model = %v, want the supply's upstream name", got)
	}
	want := expectQuota(11, 7, 1)
	if spent := quotaBefore - h.UserQuota(t); spent != want {
		t.Fatalf("spent %d, want %d (the public model's price, not the upstream name's)", spent, want)
	}
	logs := consumeLogs(t)
	if len(logs) != 1 || logs[0].ModelName != "gpt-4o-mini" {
		t.Fatalf("consume logs = %+v, want one log for the public model", logs)
	}
	meta := logs[0].Metadata.Data()
	if meta["upstream_model"] != "vendor/gpt-4o-mini" || meta["supply_id"] != float64(supplies[0].Id) {
		t.Fatalf("metadata = %v, want the upstream name and supply id", meta)
	}
}

// Tiers come from the supplies' priorities, not from the channels'.
func TestSupplyPriorityOverridesChannelPriority(t *testing.T) {
	high := NewFakeProvider(t)
	high.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "high-channel", 11, 7)})
	low := NewFakeProvider(t)
	low.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "low-channel", 11, 7)})
	h := NewHarness(t, HarnessOptions{Channels: []ChannelSpec{
		{Name: "channel-priority-10", BaseURL: high.Server.URL, Priority: 10},
		{Name: "channel-priority-0", BaseURL: low.Server.URL, Priority: 0},
	}})
	useSupplies(t,
		[]model.ModelInfo{{Model: "gpt-4o-mini", Status: model.ModelStatusPublished}},
		[]model.ModelSupply{
			{Model: "gpt-4o-mini", ChannelId: h.Channels[0].Id, UpstreamModel: "gpt-4o-mini", Priority: 0, Enabled: true},
			{Model: "gpt-4o-mini", ChannelId: h.Channels[1].Id, UpstreamModel: "gpt-4o-mini", Priority: 10, Enabled: true},
		})

	for i := 0; i < 5; i++ {
		if w := h.Post("/v1/chat/completions", chatFor("gpt-4o-mini")); w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	if len(high.Requests()) != 0 || len(low.Requests()) != 5 {
		t.Fatalf("requests: channel-priority-10 %d, channel-priority-0 %d; want all on the supply with priority 10",
			len(high.Requests()), len(low.Requests()))
	}
}

// Within one tier, failover takes the heaviest supply every time; balance spreads by weight.
func TestSupplyModesWithinTier(t *testing.T) {
	for _, tc := range []struct {
		mode             string
		lightWeight      uint
		wantLightTraffic bool
	}{
		{model.SupplyModeFailover, 1, false},
		{model.SupplyModeBalance, 5, true},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			// Identical requests share a session; affinity would pin them all to the first pick.
			setAndRestore(t, &config.SessionAffinityEnabled, false)
			light := NewFakeProvider(t)
			light.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 11, 7)})
			heavy := NewFakeProvider(t)
			heavy.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 11, 7)})
			h := NewHarness(t, HarnessOptions{Channels: []ChannelSpec{
				{Name: "light", BaseURL: light.Server.URL},
				{Name: "heavy", BaseURL: heavy.Server.URL},
			}})
			useSupplies(t,
				[]model.ModelInfo{{Model: "gpt-4o-mini", Status: model.ModelStatusPublished, SupplyMode: tc.mode}},
				[]model.ModelSupply{
					{Model: "gpt-4o-mini", ChannelId: h.Channels[0].Id, UpstreamModel: "gpt-4o-mini", Weight: tc.lightWeight, Enabled: true},
					{Model: "gpt-4o-mini", ChannelId: h.Channels[1].Id, UpstreamModel: "gpt-4o-mini", Weight: 5, Enabled: true},
				})
			for i := 0; i < 20; i++ {
				if w := h.Post("/v1/chat/completions", chatFor("gpt-4o-mini")); w.Code != http.StatusOK {
					t.Fatalf("status %d: %s", w.Code, w.Body.String())
				}
			}
			if got := len(light.Requests()) > 0; got != tc.wantLightTraffic {
				t.Fatalf("light supply got %d of 20 requests, want traffic=%v", len(light.Requests()), tc.wantLightTraffic)
			}
			if len(heavy.Requests()) == 0 {
				t.Fatal("heavy supply got no requests")
			}
		})
	}
}

func listedModels(t *testing.T, h *Harness) map[string]bool {
	t.Helper()
	// The list names each model's vendor; the harness has no vendor registry, so use an empty one.
	setAndRestore(t, &model.ModelOwnedBysInstance, &model.ModelOwnedBys{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.Header.Set("Authorization", "Bearer "+h.Token.Key)
	h.Engine.ServeHTTP(w, req)
	var resp struct {
		Data []struct {
			Id string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("models list: %v (%s)", err, w.Body.String())
	}
	listed := map[string]bool{}
	for _, m := range resp.Data {
		listed[m.Id] = true
	}
	return listed
}

// Status decides what is listed and what can be called; aliases are callable, not listed.
func TestSupplyStatusGatesListingAndCalls(t *testing.T) {
	upstream := NewFakeProvider(t)
	upstream.SetResponse(FakeResponse{JSON: ChatCompletionJSON("x", "ok", 11, 7)})
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{{Name: "c", BaseURL: upstream.Server.URL}},
		Prices: []PriceSpec{
			{Model: "pub", Input: 1, Output: 1}, {Model: "drafted", Input: 1, Output: 1}, {Model: "old", Input: 1, Output: 1},
		},
	})
	useSupplies(t,
		[]model.ModelInfo{
			{Model: "pub", Status: model.ModelStatusPublished},
			{Model: "drafted", Status: model.ModelStatusDraft},
			{Model: "old", Status: model.ModelStatusRetired},
			{Model: "pub-alias", AliasOf: "pub"},
		},
		[]model.ModelSupply{
			{Model: "pub", ChannelId: h.Channel.Id, UpstreamModel: "pub-upstream", Enabled: true},
			{Model: "drafted", ChannelId: h.Channel.Id, UpstreamModel: "drafted", Enabled: true},
			{Model: "old", ChannelId: h.Channel.Id, UpstreamModel: "old", Enabled: true},
		})

	for name, wantOK := range map[string]bool{"pub": true, "pub-alias": true, "old": true, "drafted": false} {
		w := h.Post("/v1/chat/completions", chatFor(name))
		if (w.Code == http.StatusOK) != wantOK {
			t.Errorf("%s: status %d, want ok=%v (%s)", name, w.Code, wantOK, w.Body.String())
		}
		if !wantOK && w.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404 for a draft", name, w.Code)
		}
	}
	listed := listedModels(t, h)
	if !listed["pub"] || listed["drafted"] || listed["old"] || listed["pub-alias"] {
		t.Fatalf("listed = %v, want only the published model", listed)
	}
}

// A disabled supply is never used; the other supply serves.
func TestDisabledSupplyIsSkipped(t *testing.T) {
	off := NewFakeProvider(t)
	off.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 11, 7)})
	on := NewFakeProvider(t)
	on.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 11, 7)})
	h := NewHarness(t, HarnessOptions{Channels: []ChannelSpec{
		{Name: "off", BaseURL: off.Server.URL, Priority: 10},
		{Name: "on", BaseURL: on.Server.URL},
	}})
	useSupplies(t,
		[]model.ModelInfo{{Model: "gpt-4o-mini", Status: model.ModelStatusPublished}},
		[]model.ModelSupply{
			{Model: "gpt-4o-mini", ChannelId: h.Channels[0].Id, UpstreamModel: "gpt-4o-mini", Priority: 10, Enabled: false},
			{Model: "gpt-4o-mini", ChannelId: h.Channels[1].Id, UpstreamModel: "gpt-4o-mini", Enabled: true},
		})
	for i := 0; i < 3; i++ {
		if w := h.Post("/v1/chat/completions", chatFor("gpt-4o-mini")); w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
	}
	if len(off.Requests()) != 0 || len(on.Requests()) != 3 {
		t.Fatalf("requests: disabled supply %d, enabled supply %d", len(off.Requests()), len(on.Requests()))
	}
}

// With the switch off, supplies are ignored and channel model lists route as before.
func TestSuppliesIgnoredWhenSwitchOff(t *testing.T) {
	listed := NewFakeProvider(t)
	listed.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 11, 7)})
	supplied := NewFakeProvider(t)
	supplied.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 11, 7)})
	h := NewHarness(t, HarnessOptions{Channels: []ChannelSpec{
		{Name: "listed", BaseURL: listed.Server.URL},
		{Name: "supplied", BaseURL: supplied.Server.URL, Models: "something-else"},
	}})
	if err := model.DB.Create(&model.ModelSupply{Model: "gpt-4o-mini", ChannelId: h.Channels[1].Id, UpstreamModel: "x", Priority: 99, Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	model.ChannelGroup.Load()
	if w := h.Post("/v1/chat/completions", chatFor("gpt-4o-mini")); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(listed.Requests()) != 1 || len(supplied.Requests()) != 0 {
		t.Fatalf("requests: listed %d, supplied %d; want the channel model list to route", len(listed.Requests()), len(supplied.Requests()))
	}
}
