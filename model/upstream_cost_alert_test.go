package model

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func useCostCheckTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "cost_check.db")), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&Channel{}, &Log{}, &AccountDailyCost{}, &Alert{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldSQLite := DB, common.UsingSQLite
	DB, common.UsingSQLite = testDB, true
	t.Cleanup(func() { DB, common.UsingSQLite = oldDB, oldSQLite })
}

func costLogs(t *testing.T, channel int, model string, n int, at time.Time, expected, upstream float64) {
	t.Helper()
	for i := 0; i < n; i++ {
		row := Log{Type: LogTypeConsume, ChannelId: channel, ModelName: model, CreatedAt: at.Unix(), ExpectedCost: expected, UpstreamCost: upstream}
		if err := DB.Create(&row).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func openCostAlert(t *testing.T, channel string) []string {
	t.Helper()
	alerts, err := OpenAlerts(CostMismatchAlertKind)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range alerts {
		if a.Subject == channel {
			return a.Items
		}
	}
	return nil
}

func TestCheckUpstreamCostsPerRequest(t *testing.T) {
	useCostCheckTestDB(t)
	now := time.Date(2026, 12, 2, 12, 0, 0, 0, time.UTC)
	hourAgo := now.Add(-time.Hour)
	for _, ch := range []*Channel{{Id: 1, Name: "or", Type: config.ChannelTypeOpenRouter, Key: "k"}} {
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	costLogs(t, 1, "gpt-5.6-sol", 12, hourAgo, 0.02, 0.03)           // billed 50% more: told
	costLogs(t, 1, "gemini-4-argon", 12, hourAgo, 0.02, 0.0205)      // 2.5%: within the noise
	costLogs(t, 1, "rare", 3, hourAgo, 0.1, 1)                       // too few requests
	costLogs(t, 1, "cheap", 20, hourAgo, 0.001, 0.01)                // too little money
	costLogs(t, 1, "fresh", 12, now.Add(-5*time.Minute), 0.02, 0.05) // generation lookup may be pending
	costLogs(t, 1, "old", 12, now.Add(-4*24*time.Hour), 0.02, 0.05)  // out of the window

	if err := CheckUpstreamCosts(now); err != nil {
		t.Fatal(err)
	}
	if got := openCostAlert(t, "channel:1"); !slices.Equal(got, []string{"cost|gpt-5.6-sol|higher"}) {
		t.Fatalf("only the billed-more model should be told, got %v", got)
	}

	// Once the price is corrected, newer requests match; when the old ones leave the window the
	// alert closes.
	if err := CheckUpstreamCosts(now.Add(4 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := openCostAlert(t, "channel:1"); got != nil {
		t.Fatalf("the alert should close, got %v", got)
	}
}

func TestCheckUpstreamCostsDaily(t *testing.T) {
	useCostCheckTestDB(t)
	now := time.Date(2026, 12, 2, 12, 0, 0, 0, time.UTC)
	key := "manage"
	ch := &Channel{Id: 7, Name: "openai", Type: config.ChannelTypeOpenAI, Key: "k", ManageKey: &key}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)
	costLogs(t, 7, "gpt-5.6-sol", 20, day, 0.1, 0) // OpenAI reports no per-request cost
	if err := SaveAccountDailyCosts(AccountOfChannel(ch), []AccountDailyCost{{Date: "2026-12-01", Cost: 1.5, Requests: 20}}); err != nil {
		t.Fatal(err)
	}
	if err := CheckUpstreamCosts(now); err != nil {
		t.Fatal(err)
	}
	if got := openCostAlert(t, "channel:7"); !slices.Equal(got, []string{"daily|lower"}) {
		t.Fatalf("billed $1.50 for $2 expected should be told as lower, got %v", got)
	}
}
