package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupNotifyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&NotifyChannel{}, &Option{}); err != nil {
		t.Fatal(err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
	return testDB
}

func TestNotifyChannelSecretsKeptAndCleared(t *testing.T) {
	c := &NotifyChannel{Type: notify.TypeDingTalk}
	if err := c.SetValues(map[string]string{"token": "tok", "secret": "sec", "keyword": "kw", "junk": "x"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := c.PlainSettings(); got["keyword"] != "kw" || got["token"] != "" || got["junk"] != "" {
		t.Fatalf("plain = %v", got)
	}
	// Empty secrets keep the stored ones.
	if err := c.SetValues(map[string]string{"keyword": "kw2"}, nil); err != nil {
		t.Fatal(err)
	}
	if got := c.SecretSettings(); got["token"] != "tok" || got["secret"] != "sec" {
		t.Fatalf("secrets = %v", got)
	}
	// A cleared secret goes away; the keyword still builds the notifier.
	if err := c.SetValues(map[string]string{"keyword": "kw2"}, []string{"secret"}); err != nil {
		t.Fatal(err)
	}
	if got := c.SecretSettings(); got["secret"] != "" || got["token"] != "tok" {
		t.Fatalf("secrets after clear = %v", got)
	}
	if _, err := c.Notifier(); err != nil {
		t.Fatalf("notifier: %v", err)
	}
}

func TestLegacyNotifyChannels(t *testing.T) {
	cfg := map[string]string{
		"notify.email.smtp_to":        "ops@example.com",
		"notify.wecom.webhook":        "https://example.com/wecom",
		"notify.dingtalk.token":       "dt",
		"notify.lark.token":           "lark-without-secret-or-keyword",
		"notify.telegram.bot_api_key": "bot",
		"notify.telegram.chat_id":     "42",
	}
	disabled := map[string]bool{"notify.wecom.disable": true}
	got := legacyNotifyChannels(func(k string) string { return cfg[k] }, func(k string) bool { return disabled[k] })
	// WeCom is disabled, DingTalk and Lark lack a secret or keyword: only email and Telegram.
	if len(got) != 2 || got[0].Type != notify.TypeEmail || got[1].Type != notify.TypeTelegram {
		t.Fatalf("channels = %+v", got)
	}
	if got[0].PlainSettings()["to"] != "ops@example.com" || got[1].SecretSettings()["bot_token"] != "bot" {
		t.Fatalf("values = %v / %v", got[0].Values(), got[1].Values())
	}
}

func TestMigrateNotifyChannelsOnce(t *testing.T) {
	db := setupNotifyTestDB(t)
	viper.Set("notify.email.smtp_to", "ops@example.com")
	t.Cleanup(func() { viper.Set("notify.email.smtp_to", "") })

	for round := 0; round < 2; round++ {
		if err := migrateNotifyChannels().Migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	channels, err := GetNotifyChannels()
	if err != nil || len(channels) != 1 || channels[0].Type != notify.TypeEmail || !channels[0].Enabled {
		t.Fatalf("channels = %+v, %v", channels, err)
	}
	if got := len(NotifyDestinations()); got != 1 {
		t.Fatalf("destinations = %d", got)
	}
}

func TestFoldChannelDisableNotifyOption(t *testing.T) {
	db := setupNotifyTestDB(t)
	db.Create(&Option{Key: "AutomaticDisableChannelNotifyEnabled", Value: "false"})
	db.Create(&Option{Key: "NotifyDisabledKinds", Value: "price_change"})

	if err := foldChannelDisableNotifyOption().Migrate(db); err != nil {
		t.Fatal(err)
	}
	var kinds Option
	db.First(&kinds, "key = ?", "NotifyDisabledKinds")
	if kinds.Value != "price_change,channel_disabled" {
		t.Fatalf("NotifyDisabledKinds = %q", kinds.Value)
	}
	var count int64
	db.Model(&Option{}).Where("key = ?", "AutomaticDisableChannelNotifyEnabled").Count(&count)
	if count != 0 {
		t.Fatal("old option still stored")
	}
	if got := disabledKindsWith("channel_disabled", "channel_disabled"); got != "channel_disabled" {
		t.Fatalf("duplicate kind: %q", got)
	}
}
