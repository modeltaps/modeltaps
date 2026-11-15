package controller

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func useDailyCostTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "daily_costs.db")
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.Channel{}, &model.Log{}, &model.AccountDailyCost{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldSQLite := model.DB, common.UsingSQLite
	model.DB, common.UsingSQLite = testDB, true
	t.Cleanup(func() { model.DB, common.UsingSQLite = oldDB, oldSQLite })
}

// The upstream bills the account, so the comparison sums the channels that share a management
// key and leaves the others out, per UTC day.
func TestBuildChannelDailyCosts(t *testing.T) {
	useDailyCostTestDB(t)
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	at := func(date string, hour int) int64 {
		day, _ := time.Parse(time.DateOnly, date)
		return day.Add(time.Duration(hour) * time.Hour).Unix()
	}
	shared, other := "manage-shared", "manage-other"
	channels := []*model.Channel{
		{Id: 1, Name: "or-a", Type: config.ChannelTypeOpenRouter, Key: "k1", ManageKey: &shared},
		{Id: 2, Name: "or-b", Type: config.ChannelTypeOpenRouter, Key: "k2", ManageKey: &shared},
		{Id: 3, Name: "or-other", Type: config.ChannelTypeOpenRouter, Key: "k3", ManageKey: &other},
		{Id: 4, Name: "or-none", Type: config.ChannelTypeOpenRouter, Key: "k4"},
		// The same key on another type is another upstream's account.
		{Id: 5, Name: "anthropic", Type: config.ChannelTypeAnthropic, Key: "k5", ManageKey: &shared},
	}
	for _, channel := range channels {
		if err := model.DB.Create(channel).Error; err != nil {
			t.Fatal(err)
		}
	}
	logs := []model.Log{
		{ChannelId: 1, Type: model.LogTypeConsume, CreatedAt: at("2026-11-14", 1), ExpectedCost: 1},
		{ChannelId: 2, Type: model.LogTypeConsume, CreatedAt: at("2026-11-14", 23), ExpectedCost: 0.5},
		{ChannelId: 3, Type: model.LogTypeConsume, CreatedAt: at("2026-11-14", 5), ExpectedCost: 100}, // another account
		{ChannelId: 1, Type: model.LogTypeConsume, CreatedAt: at("2026-11-13", 12), ExpectedCost: 2},
		{ChannelId: 1, Type: model.LogTypeConsume, CreatedAt: at("2026-11-15", 3), ExpectedCost: 50}, // today, not complete
		{ChannelId: 1, Type: model.LogTypeConsume, CreatedAt: at("2026-10-01", 3), ExpectedCost: 50}, // out of range
	}
	if err := model.DB.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}
	account := model.AccountOfChannel(channels[0])
	if account == "" || account != model.AccountOfChannel(channels[1]) || account == model.AccountOfChannel(channels[2]) || account == model.AccountOfChannel(channels[4]) {
		t.Fatalf("accounts: %q %q %q %q", account, model.AccountOfChannel(channels[1]), model.AccountOfChannel(channels[2]), model.AccountOfChannel(channels[4]))
	}
	if err := model.SaveAccountDailyCosts(account, []model.AccountDailyCost{
		{Date: "2026-11-14", Cost: 1.8, Requests: 2},
		{Date: "2026-11-12", Cost: 0.4, Requests: -1},
	}); err != nil {
		t.Fatal(err)
	}
	// A second read of the report replaces the day.
	if err := model.SaveAccountDailyCosts(account, []model.AccountDailyCost{{Date: "2026-11-14", Cost: 1.9, Requests: 2}}); err != nil {
		t.Fatal(err)
	}

	got, err := buildChannelDailyCosts(channels[1], now)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Supported || !got.HasManageKey || len(got.Channels) != 2 || got.Channels[0].Name != "or-a" || got.UpdatedAt == 0 {
		t.Fatalf("header = %+v", got)
	}
	if len(got.Days) != 3 {
		t.Fatalf("days = %+v", got.Days)
	}
	// Newest first. 2026-11-14: both channels of the account, 1 + 0.5, against 1.9 upstream.
	d := got.Days[0]
	if d.Date != "2026-11-14" || d.ExpectedCost != 1.5 || d.RequestCount != 2 || d.UpstreamCost == nil || *d.UpstreamCost != 1.9 || *d.UpstreamRequests != 2 {
		t.Errorf("2026-11-14 = %+v", d)
	}
	// 2026-11-13: requests here, no upstream row yet.
	if d := got.Days[1]; d.Date != "2026-11-13" || d.ExpectedCost != 2 || d.UpstreamCost != nil {
		t.Errorf("2026-11-13 = %+v", d)
	}
	// 2026-11-12: billed upstream, nothing sent from here; the report has no request count.
	if d := got.Days[2]; d.Date != "2026-11-12" || d.ExpectedCost != 0 || d.UpstreamCost == nil || *d.UpstreamCost != 0.4 || d.UpstreamRequests != nil {
		t.Errorf("2026-11-12 = %+v", d)
	}

	// No management key, or a type without a report: nothing to compare.
	if none, err := buildChannelDailyCosts(channels[3], now); err != nil || !none.Supported || none.HasManageKey || len(none.Days) != 0 {
		t.Errorf("no management key = %+v, %v", none, err)
	}
	relay := "https://relay.example.com"
	if out, err := buildChannelDailyCosts(&model.Channel{Type: config.ChannelTypeOpenAI, BaseURL: &relay, ManageKey: &shared}, now); err != nil || out.Supported {
		t.Errorf("a relay = %+v, %v", out, err)
	}
}
