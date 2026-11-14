package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// fakeModelList serves a fixed model list on every path (query-string lists get an empty one).
func fakeModelList(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.RawQuery != "" {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setupAggregatorSyncDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&model.Price{}, &model.Channel{}, &model.ModelInfo{}, &model.ModelSupply{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldSQLite, oldPricing, oldApply := model.DB, common.UsingSQLite, model.PricingInstance, config.PriceSyncAutoApply
	model.DB, common.UsingSQLite = testDB, true
	config.PriceSyncAutoApply = model.PriceSyncAutoApplyAll
	t.Cleanup(func() {
		model.DB, common.UsingSQLite, model.PricingInstance, config.PriceSyncAutoApply = oldDB, oldSQLite, oldPricing, oldApply
	})
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	model.PricingInstance = &model.Pricing{Prices: map[string]*model.Price{}, Match: []string{}}
}

// The OpenRouter and Vercel lists use platform spellings. The sync writes catalog rows under the
// public names (one per model across both lists, OpenRouter first), keeps the prices under the
// spellings and copies them to the public names; the channels' supplies then land on those rows.
func TestSyncAllChannelsOwnPricingUsesPublicNames(t *testing.T) {
	setupAggregatorSyncDB(t)
	openrouter := fakeModelList(t, `{"data":[
		{"id":"anthropic/claude-opus-5","name":"OpenRouter Opus","pricing":{"prompt":"0.000005","completion":"0.000025"}},
		{"id":"foo/bar-1","name":"OpenRouter Bar","pricing":{"prompt":"0.000001","completion":"0.000001"}}
	]}`)
	vercel := fakeModelList(t, `{"data":[
		{"id":"anthropic/claude-opus-5","name":"Vercel Opus","type":"language","pricing":{"input":"0.000006","output":"0.00003"}},
		{"id":"openai/gpt-4.1-mini","name":"Vercel Mini","type":"language","pricing":{"input":"0.0000004","output":"0.0000016"}},
		{"id":"other/bar-1","name":"Vercel Bar","type":"language","pricing":{"input":"0.000002","output":"0.000002"}}
	]}`)
	orURL, vURL := openrouter.URL+"/api", vercel.URL
	orChannel := &model.Channel{Type: config.ChannelTypeOpenRouter, Name: "or", Key: "sk-or", BaseURL: &orURL,
		Status: config.ChannelStatusEnabled, Group: "default", Models: "anthropic/claude-opus-5,foo/bar-1"}
	vChannel := &model.Channel{Type: config.ChannelTypeVercelAIGateway, Name: "vercel", Key: "sk-v", BaseURL: &vURL,
		Status: config.ChannelStatusEnabled, Group: "default", Models: "anthropic/claude-opus-5,openai/gpt-4.1-mini,other/bar-1"}
	for _, ch := range []*model.Channel{orChannel, vChannel} {
		if err := model.DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}

	n, err := SyncAllChannelsOwnPricing()
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 {
		t.Errorf("synced %d upstream prices, want 4", n)
	}

	var rows []*model.ModelInfo
	if err := model.DB.Order("model").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range rows {
		got = append(got, r.Model+"|"+r.Name+"|"+r.Source)
	}
	want := "bar-1|OpenRouter Bar|openrouter,claude-opus-5|OpenRouter Opus|openrouter,gpt-4.1-mini|Vercel Mini|vercel"
	if strings.Join(got, ",") != want {
		t.Fatalf("catalog rows\n got %v\nwant %s", got, want)
	}

	var prices []string
	for name, p := range model.PricingInstance.GetAllPrices() {
		prices = append(prices, fmt.Sprintf("%s:%d", name, p.ChannelType))
	}
	sort.Strings(prices)
	wantPrices := fmt.Sprintf("anthropic/claude-opus-5:%d,bar-1:%d,claude-opus-5:%d,foo/bar-1:%d,gpt-4.1-mini:%d,openai/gpt-4.1-mini:%d,other/bar-1:%d",
		config.ChannelTypeOpenRouter, config.ChannelTypeOpenRouter, config.ChannelTypeOpenRouter, config.ChannelTypeOpenRouter,
		config.ChannelTypeVercelAIGateway, config.ChannelTypeVercelAIGateway, config.ChannelTypeVercelAIGateway)
	if strings.Join(prices, ",") != wantPrices {
		t.Fatalf("prices\n got %v\nwant %s", prices, wantPrices)
	}

	var supplies []string
	for _, ch := range []*model.Channel{orChannel, vChannel} {
		created, err := model.AutoSupplyChannel(ch, model.SplitChannelModels(ch.Models))
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range created {
			supplies = append(supplies, fmt.Sprintf("%s@%s<-%s", s.Model, ch.Name, s.UpstreamModel))
		}
	}
	sort.Strings(supplies)
	wantSupplies := "bar-1@or<-foo/bar-1,bar-1@vercel<-other/bar-1,claude-opus-5@or<-anthropic/claude-opus-5," +
		"claude-opus-5@vercel<-anthropic/claude-opus-5,gpt-4.1-mini@vercel<-openai/gpt-4.1-mini"
	if strings.Join(supplies, ",") != wantSupplies {
		t.Fatalf("supplies\n got %v\nwant %s", supplies, wantSupplies)
	}

	// An official price under the public name is not replaced by an aggregator's on the next run.
	official := &model.Price{Model: "gpt-4.1-mini", Type: model.TokensPriceType, ChannelType: config.ChannelTypeOpenAI, Input: 0.2, Output: 0.8}
	if err := model.DB.Where("model = ?", "gpt-4.1-mini").Delete(&model.Price{}).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.DB.Create(official).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.PricingInstance.Init(); err != nil {
		t.Fatal(err)
	}
	if _, err := SyncAllChannelsOwnPricing(); err != nil {
		t.Fatal(err)
	}
	if p := model.PricingInstance.GetAllPrices()["gpt-4.1-mini"]; p == nil || p.ChannelType != config.ChannelTypeOpenAI || p.Input != 0.2 {
		t.Errorf("official gpt-4.1-mini price replaced: %+v", p)
	}
	var count int64
	model.DB.Model(&model.ModelInfo{}).Count(&count)
	if count != 3 {
		t.Errorf("catalog has %d rows after the second run, want 3", count)
	}
}

// The manual price sync of one channel writes the same way: catalog rows under public names,
// prices under the spellings plus copies under the public names.
func TestSyncChannelPricingUsesPublicNames(t *testing.T) {
	setupAggregatorSyncDB(t)
	srv := fakeModelList(t, `{"data":[
		{"id":"anthropic/claude-opus-5","name":"Opus","pricing":{"prompt":"0.000005","completion":"0.000025"}},
		{"id":"foo/bar-1","name":"Bar","pricing":{"prompt":"0.000001","completion":"0.000001"}}
	]}`)
	resp := postSyncPricing(t, map[string]any{
		"type": config.ChannelTypeOpenRouter, "key": "sk-or", "base_url": srv.URL + "/api",
		"models": "anthropic/claude-opus-5,foo/bar-1", "group": "default",
	})
	if synced := resp["data"].(map[string]any)["synced"].(float64); synced != 2 {
		t.Fatalf("synced = %v", synced)
	}
	var names []string
	if err := model.DB.Model(&model.ModelInfo{}).Order("model").Pluck("model", &names).Error; err != nil {
		t.Fatal(err)
	}
	if strings.Join(names, ",") != "bar-1,claude-opus-5" {
		t.Fatalf("catalog rows = %v", names)
	}
	var priced []string
	for name := range model.PricingInstance.GetAllPrices() {
		priced = append(priced, name)
	}
	sort.Strings(priced)
	if strings.Join(priced, ",") != "anthropic/claude-opus-5,bar-1,claude-opus-5,foo/bar-1" {
		t.Fatalf("prices = %v", priced)
	}
}
