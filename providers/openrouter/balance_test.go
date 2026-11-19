package openrouter

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
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
	// Every connection to ":memory:" opens a database of its own: keep one, so goroutines the
	// test starts (the generation lookup) see the tables it created.
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("migrate channels: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() {
		common.WaitTrackedGoroutines(10 * time.Second)
		model.DB = oldDB
	})
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

// newAccountBalanceTestProvider serves the key info and the account credits, and records which
// key asked for the credits.
func newAccountBalanceTestProvider(t *testing.T, keyBody string, creditsStatus int, creditsBody string) (*OpenRouterProvider, *string) {
	t.Helper()
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var creditsAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/credits" {
			creditsAuth = r.Header.Get("Authorization")
			w.WriteHeader(creditsStatus)
			_, _ = w.Write([]byte(creditsBody))
			return
		}
		_, _ = w.Write([]byte(keyBody))
	}))
	t.Cleanup(upstream.Close)
	baseURL := upstream.URL + "/api"
	manageKey := "sk-or-manage"
	channel := &model.Channel{Type: config.ChannelTypeOpenRouter, Key: "sk-or-test", ManageKey: &manageKey, BaseURL: &baseURL}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	return OpenRouterProviderFactory{}.Create(channel).(*OpenRouterProvider), &creditsAuth
}

const (
	keyWithoutLimit = `{"data":{"label":"gw","limit":null,"limit_remaining":null,"limit_reset":null,"usage":12}}`
	keyWithLimit    = `{"data":{"label":"gw","limit":100,"limit_remaining":42.5,"usage":57.5}}`
)

func TestOpenRouterAccountBalanceWithManageKey(t *testing.T) {
	setupBalanceTestDB(t)
	p, creditsAuth := newAccountBalanceTestProvider(t, keyWithoutLimit, http.StatusOK, `{"data":{"total_credits":100.5,"total_usage":25.75}}`)

	balance, err := p.Balance()
	if err != nil || balance != 74.75 {
		t.Fatalf("Balance() = %v, %v, want the account balance 74.75", balance, err)
	}
	if *creditsAuth != "Bearer sk-or-manage" {
		t.Errorf("credits were asked with %q, want the management key", *creditsAuth)
	}
	var stored model.Channel
	if err := model.DB.First(&stored, p.Channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	snap := stored.GetQuotaSnapshot()
	if stored.Balance != 74.75 || snap == nil || snap.AccountBalance == nil || *snap.AccountBalance != 74.75 {
		t.Errorf("stored balance = %v, snapshot = %+v", stored.Balance, snap)
	}
}

func TestOpenRouterBalanceIsTheSmallerOfKeyLimitAndAccount(t *testing.T) {
	setupBalanceTestDB(t)
	// The key may still spend 42.5 but the account only holds 10.
	p, _ := newAccountBalanceTestProvider(t, keyWithLimit, http.StatusOK, `{"data":{"total_credits":110,"total_usage":100}}`)
	if balance, err := p.Balance(); err != nil || balance != 10 {
		t.Fatalf("Balance() = %v, %v, want 10", balance, err)
	}
	// The account holds plenty: the key limit decides.
	p, _ = newAccountBalanceTestProvider(t, keyWithLimit, http.StatusOK, `{"data":{"total_credits":1000,"total_usage":100}}`)
	if balance, err := p.Balance(); err != nil || balance != 42.5 {
		t.Fatalf("Balance() = %v, %v, want 42.5", balance, err)
	}
}

func TestOpenRouterRejectedManageKey(t *testing.T) {
	setupBalanceTestDB(t)
	forbidden := `{"error":{"code":403,"message":"Only management keys can perform this operation"}}`
	// With a key limit the balance still comes from the key.
	p, _ := newAccountBalanceTestProvider(t, keyWithLimit, http.StatusForbidden, forbidden)
	if balance, err := p.Balance(); err != nil || balance != 42.5 {
		t.Fatalf("Balance() = %v, %v, want the key limit 42.5", balance, err)
	}
	// Without one there is nothing to show, and the error names the management key.
	p, _ = newAccountBalanceTestProvider(t, keyWithoutLimit, http.StatusForbidden, forbidden)
	if _, err := p.Balance(); err == nil || errors.Is(err, ErrNoKeyLimit) {
		t.Fatalf("Balance() error = %v, want the management key error", err)
	}
}
