package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// fakeOpenRouterModels serves /api/v1/models with prices in USD per token.
func fakeOpenRouterModels(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.RawQuery != "" { // audio-only lists
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(`{"data":[
			{"id":"vendor/up","name":"Up","pricing":{"prompt":"0.000003","completion":"0.000004"}},
			{"id":"vendor/down","name":"Down","pricing":{"prompt":"0.000001","completion":"0.000004"}},
			{"id":"vendor/unused","name":"Unused","pricing":{"prompt":"0.000001","completion":"0.000001"}}
		]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setupPricePreviewDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.Price{}, &model.Channel{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldSQLite, oldPricing := model.DB, common.UsingSQLite, model.PricingInstance
	model.DB, common.UsingSQLite = testDB, true
	t.Cleanup(func() { model.DB, common.UsingSQLite, model.PricingInstance = oldDB, oldSQLite, oldPricing })
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	// $2 / $4 per 1M tokens for up, $4 / $8 for down
	for _, p := range []model.Price{
		{Model: "vendor/up", Type: model.TokensPriceType, Input: 1, Output: 2},
		{Model: "vendor/down", Type: model.TokensPriceType, Input: 2, Output: 4},
	} {
		p := p
		if err := testDB.Create(&p).Error; err != nil {
			t.Fatal(err)
		}
	}
	model.PricingInstance = &model.Pricing{Prices: map[string]*model.Price{}, Match: []string{}}
	if err := model.PricingInstance.Init(); err != nil {
		t.Fatal(err)
	}
}

func postSyncPricing(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/channel/sync_pricing", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	SyncChannelPricing(c)
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %s: %v", w.Body.String(), err)
	}
	if resp["success"] != true {
		t.Fatalf("response %s", w.Body.String())
	}
	return resp
}

// TestSyncChannelPricingPreviewAndSelect: the preview lists the channel's models with their
// change and writes nothing; applying a selection writes only the selected model.
func TestSyncChannelPricingPreviewAndSelect(t *testing.T) {
	setupPricePreviewDB(t)
	srv := fakeOpenRouterModels(t)
	channel := map[string]any{
		"type": config.ChannelTypeOpenRouter, "key": "sk-or", "base_url": srv.URL + "/api",
		"models": "vendor/up,vendor/down", "group": "default",
	}

	preview := map[string]any{"sync_preview": true}
	for k, v := range channel {
		preview[k] = v
	}
	resp := postSyncPricing(t, preview)
	changes := resp["data"].(map[string]any)["changes"].([]any)
	kinds := map[string]string{}
	for _, c := range changes {
		m := c.(map[string]any)
		kinds[m["model"].(string)] = m["kind"].(string)
	}
	if len(kinds) != 2 || kinds["vendor/up"] != model.PriceChangeRaised || kinds["vendor/down"] != model.PriceChangeLowered {
		t.Fatalf("changes = %v (unused models must not be listed)", kinds)
	}
	if model.PricingInstance.Prices["vendor/down"].Input != 2 {
		t.Fatal("preview wrote prices")
	}

	apply := map[string]any{"sync_models": []string{"vendor/down"}}
	for k, v := range channel {
		apply[k] = v
	}
	resp = postSyncPricing(t, apply)
	if synced := resp["data"].(map[string]any)["synced"].(float64); synced != 1 {
		t.Fatalf("synced = %v", synced)
	}
	if got := model.PricingInstance.Prices["vendor/down"].Input; got != 0.5 {
		t.Fatalf("down input ratio = %v, want 0.5", got)
	}
	if got := model.PricingInstance.Prices["vendor/up"].Input; got != 1 {
		t.Fatalf("unselected up changed to %v", got)
	}
}
