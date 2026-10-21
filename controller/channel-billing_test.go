package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// recordingNotifier 收集 notify.Send 发出的通知，供断言。注册后无法移除，所以用计数而不是精确列表。
type recordingNotifier struct {
	mu     sync.Mutex
	titles []string
}

func (r *recordingNotifier) Send(_ context.Context, title, _ string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.titles = append(r.titles, title)
	return nil
}

func (r *recordingNotifier) Name() string { return "test-recorder" }

func (r *recordingNotifier) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.titles)
}

var testNotifier = &recordingNotifier{}

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
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	notify.AddNotifiers(testNotifier)
}

func TestCheckLowBalanceAlertsOnceUntilRecovered(t *testing.T) {
	setupBalanceTestDB(t)
	channel := &model.Channel{Name: "or", Type: config.ChannelTypeOpenRouter, BalanceThreshold: 10}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	before := testNotifier.count()
	checkLowBalance(channel, 5)
	if got := testNotifier.count() - before; got != 1 {
		t.Fatalf("first drop below threshold sent %d notifications, want 1", got)
	}
	if channel.BalanceAlertedTime == 0 {
		t.Fatal("alerted time not recorded")
	}

	// 仍低于阈值：不重复提醒。
	checkLowBalance(channel, 4)
	if got := testNotifier.count() - before; got != 1 {
		t.Fatalf("second check sent %d notifications in total, want 1", got)
	}

	// 回到阈值之上：清零标记。
	checkLowBalance(channel, 50)
	if channel.BalanceAlertedTime != 0 {
		t.Fatal("alerted time should reset once balance recovers")
	}
	var stored model.Channel
	if err := model.DB.First(&stored, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.BalanceAlertedTime != 0 {
		t.Fatalf("stored alerted time = %d, want 0", stored.BalanceAlertedTime)
	}

	// 再次跌破：再提醒一次。
	checkLowBalance(channel, 1)
	if got := testNotifier.count() - before; got != 2 {
		t.Fatalf("re-drop sent %d notifications in total, want 2", got)
	}
}

func TestCheckLowBalanceDisabledWithoutThreshold(t *testing.T) {
	setupBalanceTestDB(t)
	channel := &model.Channel{Name: "or", Type: config.ChannelTypeOpenRouter}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	before := testNotifier.count()
	checkLowBalance(channel, 0.01)
	if got := testNotifier.count() - before; got != 0 {
		t.Fatalf("threshold 0 sent %d notifications, want 0", got)
	}
}

func TestUpdateChannelBalanceReportsUnsupported(t *testing.T) {
	setupBalanceTestDB(t)
	// Anthropic 没有余额接口，provider 不实现 BalanceInterface。
	channel := &model.Channel{Name: "claude", Type: config.ChannelTypeAnthropic, Key: "sk-ant"}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	body := callHandler(t, UpdateChannelBalance, http.MethodGet, gin.Params{{Key: "id", Value: "1"}}, "", nil)
	if body.Success {
		t.Fatal("expected failure for unsupported channel type")
	}
	if body.Message != "balance query is not supported for this channel type" {
		t.Errorf("message = %q", body.Message)
	}
}

func TestUpdateAllChannelsBalanceSkipsUnsupportedAndRefreshesOthers(t *testing.T) {
	setupBalanceTestDB(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balance":"3.25","total_used":"10"}`))
	}))
	defer upstream.Close()
	baseURL := upstream.URL

	vercel := &model.Channel{Name: "vercel", Type: config.ChannelTypeVercelAIGateway, Key: "vck", BaseURL: &baseURL, Status: config.ChannelStatusEnabled, BalanceThreshold: 5}
	claude := &model.Channel{Name: "claude", Type: config.ChannelTypeAnthropic, Key: "sk-ant", Status: config.ChannelStatusEnabled}
	for _, ch := range []*model.Channel{vercel, claude} {
		if err := model.DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}

	before := testNotifier.count()
	if err := updateAllChannelsBalance(); err != nil {
		t.Fatalf("updateAllChannelsBalance() error = %v", err)
	}

	var stored model.Channel
	if err := model.DB.First(&stored, vercel.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Balance != 3.25 || stored.Status != config.ChannelStatusEnabled {
		t.Errorf("vercel channel balance/status = %v/%d", stored.Balance, stored.Status)
	}
	if stored.BalanceAlertedTime == 0 {
		t.Error("low balance alert not recorded for vercel channel")
	}
	if got := testNotifier.count() - before; got != 1 {
		t.Errorf("sent %d notifications, want 1 (low balance)", got)
	}
	var untouched model.Channel
	if err := model.DB.First(&untouched, claude.Id).Error; err != nil {
		t.Fatal(err)
	}
	if untouched.Balance != 0 || untouched.BalanceUpdatedTime != 0 || untouched.Status != config.ChannelStatusEnabled {
		t.Errorf("unsupported channel should be untouched, got %+v", untouched)
	}
}
