package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// While supplies route requests, the drift check of an opted-in channel adds a supply for a new
// upstream model that belongs to a public model, leaves the others for the models page instead
// of adding them to the channel's model list, and does not report a model a supply on the
// channel already sends.
func TestDriftWithSuppliesLeavesNewModelsPending(t *testing.T) {
	setupBalanceTestDB(t)
	if err := model.DB.AutoMigrate(&model.ModelSupply{}, &model.ModelInfo{}); err != nil {
		t.Fatal(err)
	}
	oldEnabled := config.ModelSupplyEnabled
	config.ModelSupplyEnabled = true
	t.Cleanup(func() { config.ModelSupplyEnabled = oldEnabled })

	upstream := []string{"m-old", "m-new", "m-supplied", "m-known"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.RawQuery != "" {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		ids := make([]string, 0, len(upstream))
		for _, m := range upstream {
			ids = append(ids, `{"id":"`+m+`","pricing":{"prompt":"0.000001","completion":"0.000001"}}`)
		}
		_, _ = w.Write([]byte(`{"data":[` + strings.Join(ids, ",") + `]}`))
	}))
	defer srv.Close()

	base := srv.URL + "/api"
	ch := model.Channel{Name: "or", Type: config.ChannelTypeOpenRouter, Key: "sk", BaseURL: &base, Models: "m-old",
		Group: "default", Status: config.ChannelStatusEnabled, DriftAutoAdd: true}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}
	if err := ch.UpdateModelDrift(model.ModelDriftResult{UpstreamSnapshot: []string{"m-old"}, OK: true}); err != nil {
		t.Fatal(err)
	}
	if err := model.DB.Create(&model.ModelSupply{Model: "m", ChannelId: ch.Id, UpstreamModel: "m-supplied", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.DB.Create(&model.ModelInfo{Model: "m-known", Status: model.ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}

	summary, err := CheckAllChannelsModelDrift()
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := model.GetChannelById(ch.Id)
	if stored.Models != "m-old" {
		t.Fatalf("models = %q; the model list must stay as it is while supplies route", stored.Models)
	}
	if summary.Added != 1 || len(summary.Channels) != 1 || strings.Join(summary.Channels[0].AddedModels, ",") != "m-known" {
		t.Fatalf("summary = %+v, want m-known supplied", summary)
	}
	var supply model.ModelSupply
	if err := model.DB.Where("model = ? AND channel_id = ?", "m-known", ch.Id).First(&supply).Error; err != nil {
		t.Fatalf("no supply for m-known: %v", err)
	}
	if !supply.Enabled || supply.UpstreamModel != "m-known" || supply.Source != model.ModelSupplySourceChannel {
		t.Fatalf("supply = %+v", supply)
	}
	drift := stored.GetModelDrift()
	if drift == nil || strings.Join(drift.NewModels, ",") != "m-new" {
		t.Fatalf("new models = %+v, want only m-new (m-supplied and m-known are supplied)", drift)
	}
	pending, err := model.GetPendingUpstreamModels()
	if err != nil || len(pending) != 1 || pending[0].UpstreamModel != "m-new" {
		t.Fatalf("pending = %+v, err %v", pending, err)
	}
}

func TestPendingUpstreamModelHandlers(t *testing.T) {
	setupSupplyHandlerDB(t)
	weight := uint(1)
	drift := model.ModelDriftResult{NewModels: []string{"up-x", "up-y"}, UpstreamSnapshot: []string{"up-x", "up-y"}, OK: true}
	ch := model.Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "default", Weight: &weight}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}
	if err := ch.UpdateModelDrift(drift); err != nil {
		t.Fatal(err)
	}

	_, resp := callGet(t, GetPendingUpstreamModels, "/api/model_supply/pending")
	if data, _ := resp["data"].([]any); resp["success"] != true || len(data) != 2 {
		t.Fatalf("list = %v", resp)
	}

	w, resp := callSupplyHandler(t, AttachPendingUpstreamModel, http.MethodPost, `{"upstream_model":"up-x","model":"pending-public"}`)
	if w.Code != http.StatusBadRequest || resp["code"] != "model_supply_model_not_found" {
		t.Fatalf("attach to a missing model: %d %v", w.Code, resp)
	}
	w, resp = callSupplyHandler(t, AttachPendingUpstreamModel, http.MethodPost, `{"upstream_model":"up-x","model":"pending-public","create":true,"status":"later"}`)
	if w.Code != http.StatusBadRequest || resp["code"] != "model_status_invalid" {
		t.Fatalf("attach with a bad status: %d %v", w.Code, resp)
	}
	_, resp = callSupplyHandler(t, AttachPendingUpstreamModel, http.MethodPost, `{"upstream_model":"up-x","model":"pending-public","create":true,"status":"published"}`)
	data, _ := resp["data"].(map[string]any)
	if resp["success"] != true || data["created_model"] != true || len(data["supplies"].([]any)) != 1 {
		t.Fatalf("attach with create = %v", resp)
	}
	if info, _ := model.GetModelInfoByModel("pending-public"); info == nil || info.Status != model.ModelStatusPublished {
		t.Fatalf("created row = %+v", info)
	}

	_, resp = callSupplyHandler(t, IgnorePendingUpstreamModel, http.MethodPost, `{"upstream_model":"up-y"}`)
	if data, _ := resp["data"].(map[string]any); resp["success"] != true || data["channels"] != float64(1) {
		t.Fatalf("ignore = %v", resp)
	}
	w, resp = callSupplyHandler(t, IgnorePendingUpstreamModel, http.MethodPost, `{"upstream_model":"up-y"}`)
	if w.Code != http.StatusBadRequest || resp["code"] != "model_supply_pending_not_found" {
		t.Fatalf("ignore again: %d %v", w.Code, resp)
	}
	_, resp = callGet(t, GetPendingUpstreamModels, "/api/model_supply/pending")
	if data, _ := resp["data"].([]any); len(data) != 0 {
		t.Fatalf("list after attaching and ignoring = %v", resp)
	}
}

// A channel created while supplies route requests is attached to the public models it lists,
// and the response says how many.
func TestAddChannelAutoSupplies(t *testing.T) {
	setupSupplyHandlerDB(t)
	for _, info := range []*model.ModelInfo{
		{Model: "pub-a", Status: model.ModelStatusPublished},
		{Model: "pub-b", Status: model.ModelStatusDraft},
	} {
		if err := model.DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	body := `{"name":"new","type":1,"key":"sk-test","group":"default","models":"pub-a,pub-b,not-public","priority":4,"weight":2}`

	oldEnabled := config.ModelSupplyEnabled
	t.Cleanup(func() { config.ModelSupplyEnabled = oldEnabled })

	config.ModelSupplyEnabled = false
	_, resp := callSupplyHandler(t, AddChannel, http.MethodPost, body)
	data, _ := resp["data"].(map[string]any)
	if resp["success"] != true || data["supplies"] != float64(0) {
		t.Fatalf("switch off: %v", resp)
	}

	config.ModelSupplyEnabled = true
	_, resp = callSupplyHandler(t, AddChannel, http.MethodPost, body)
	data, _ = resp["data"].(map[string]any)
	if resp["success"] != true || data["supplies"] != float64(2) {
		t.Fatalf("switch on: %v", resp)
	}
	ids, _ := data["ids"].([]any)
	if len(ids) != 1 {
		t.Fatalf("ids = %v", data["ids"])
	}
	var supplies []*model.ModelSupply
	if err := model.DB.Where("channel_id = ?", int(ids[0].(float64))).Order("model").Find(&supplies).Error; err != nil {
		t.Fatal(err)
	}
	if len(supplies) != 2 || supplies[0].Model != "pub-a" || supplies[1].Model != "pub-b" ||
		supplies[0].Priority != 4 || supplies[0].Weight != 2 || !supplies[0].Enabled {
		t.Fatalf("supplies = %+v", supplies)
	}
}
