package openrouter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupBalanceTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("migrate channels: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
}

func newBalanceTestProvider(t *testing.T, body string) (*OpenRouterProvider, *string, *string) {
	t.Helper()
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var gotPath, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(upstream.Close)
	baseURL := upstream.URL + "/api"
	channel := &model.Channel{Type: config.ChannelTypeOpenRouter, Key: "sk-or-test", BaseURL: &baseURL}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	p := OpenRouterProviderFactory{}.Create(channel).(*OpenRouterProvider)
	return p, &gotPath, &gotAuth
}

func TestOpenRouterBalanceWithKeyLimit(t *testing.T) {
	setupBalanceTestDB(t)
	p, gotPath, gotAuth := newBalanceTestProvider(t, `{"data":{"label":"gw","limit":100,"limit_remaining":42.5,"limit_reset":"monthly","usage":57.5,"usage_daily":1.25,"usage_weekly":10,"usage_monthly":57.5,"is_free_tier":false}}`)

	balance, err := p.Balance()
	if err != nil {
		t.Fatalf("Balance() error = %v", err)
	}
	if *gotPath != "/api/v1/key" {
		t.Errorf("path = %q, want /api/v1/key", *gotPath)
	}
	if *gotAuth != "Bearer sk-or-test" {
		t.Errorf("auth = %q", *gotAuth)
	}
	if balance != 42.5 {
		t.Errorf("balance = %v, want 42.5", balance)
	}

	var stored model.Channel
	if err := model.DB.First(&stored, p.Channel.Id).Error; err != nil {
		t.Fatalf("load channel: %v", err)
	}
	if stored.Balance != 42.5 || stored.BalanceUpdatedTime == 0 {
		t.Errorf("stored balance = %v (updated %d)", stored.Balance, stored.BalanceUpdatedTime)
	}
	snap := stored.GetQuotaSnapshot()
	if snap == nil {
		t.Fatal("quota snapshot not stored")
	}
	if snap.Limit == nil || *snap.Limit != 100 || snap.Remaining == nil || *snap.Remaining != 42.5 {
		t.Errorf("snapshot limit/remaining = %v/%v", snap.Limit, snap.Remaining)
	}
	if snap.ResetInterval != "monthly" || snap.UsageDaily != 1.25 || snap.UsageMonthly != 57.5 || snap.FetchedAt == 0 {
		t.Errorf("snapshot = %+v", snap)
	}
}

func TestOpenRouterBalanceWithoutKeyLimit(t *testing.T) {
	setupBalanceTestDB(t)
	p, _, _ := newBalanceTestProvider(t, `{"data":{"label":"gw","limit":null,"limit_remaining":null,"limit_reset":null,"usage":12,"usage_daily":2,"usage_weekly":5,"usage_monthly":12,"is_free_tier":true}}`)

	_, err := p.Balance()
	if !errors.Is(err, ErrNoKeyLimit) {
		t.Fatalf("Balance() error = %v, want ErrNoKeyLimit", err)
	}

	// 没有上限时余额查不到，但用量快照仍应写入，详情页才有东西看。
	var stored model.Channel
	if err := model.DB.First(&stored, p.Channel.Id).Error; err != nil {
		t.Fatalf("load channel: %v", err)
	}
	if stored.BalanceUpdatedTime != 0 {
		t.Errorf("balance should stay untouched, updated at %d", stored.BalanceUpdatedTime)
	}
	snap := stored.GetQuotaSnapshot()
	if snap == nil {
		t.Fatal("quota snapshot not stored")
	}
	if snap.Limit != nil || snap.Remaining != nil || snap.Usage != 12 || !snap.IsFreeTier || snap.ResetInterval != "" {
		t.Errorf("snapshot = %+v", snap)
	}
}
