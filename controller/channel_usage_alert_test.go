package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

func TestChannelSaveRejectsInvalidUsageAlertTiers(t *testing.T) {
	setupBalanceTestDB(t)
	got := callHandler(t, UpdateChannel, http.MethodPut, nil, "", map[string]any{"id": 1, "usage_alert_tiers": "50,120"})
	if got.Success || got.Code != "usage_alert_tiers_invalid" {
		t.Fatalf("reply = %+v, want code usage_alert_tiers_invalid", got)
	}
}

func TestCapabilitiesReportTheSiteUsageAlertTiers(t *testing.T) {
	setupUsageAlertTest(t, time.Now())
	config.UsageAlertTiers = "60,90"
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/channel/capabilities", nil)
	GetChannelCapabilities(c)
	var body struct {
		Data struct {
			UsageAlert struct {
				Tiers  []float64 `json:"tiers"`
				KindOn bool      `json:"kind_on"`
			} `json:"usage_alert"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if got := body.Data.UsageAlert; len(got.Tiers) != 2 || got.Tiers[0] != 60 || got.Tiers[1] != 90 || !got.KindOn {
		t.Fatalf("usage_alert = %+v", got)
	}
}

// setupUsageAlertTest gives the usage alerts an in-memory database, a fixed clock, the site's
// default tiers and no hourly notice limit.
func setupUsageAlertTest(t *testing.T, now time.Time) {
	t.Helper()
	setupBalanceTestDB(t)
	oldNow, oldTiers := usageAlertNow, config.UsageAlertTiers
	oldDisabled, oldLimit := config.NotifyDisabledKinds, config.NotifyKindLimitPerHour
	usageAlertNow = func() time.Time { return now }
	config.UsageAlertTiers = ""
	config.NotifyDisabledKinds = ""
	config.NotifyKindLimitPerHour = 0
	t.Cleanup(func() {
		usageAlertNow, config.UsageAlertTiers = oldNow, oldTiers
		config.NotifyDisabledKinds, config.NotifyKindLimitPerHour = oldDisabled, oldLimit
	})
}

func createUsageChannel(t *testing.T, name string) *model.Channel {
	t.Helper()
	channel := &model.Channel{Name: name, Type: config.ChannelTypeCodex}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	return channel
}

func usageWindows(fiveHour, weekly float64, fiveHourReset, weeklyReset int64) *model.ChannelUsageWindows {
	return &model.ChannelUsageWindows{PlanType: "plus", Windows: []model.UsageWindow{
		{Name: "5h", UsedPercent: fiveHour, ResetAt: fiveHourReset, WindowSeconds: 18000},
		{Name: "7d", UsedPercent: weekly, ResetAt: weeklyReset, WindowSeconds: 604800},
	}}
}

func TestUsageAlertsFireEachTierOncePerWindowPeriod(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	setupUsageAlertTest(t, now)
	channel := createUsageChannel(t, "codex-a")
	reset5h, reset7d := now.Unix()+3600, now.Unix()+5*86400

	sent := func(step string, five, weekly float64, want ...string) {
		t.Helper()
		before := testNotifier.count()
		checkUsageAlerts(channel, usageWindows(five, weekly, reset5h, reset7d))
		titles, _ := testNotifier.since(before)
		if len(titles) != len(want) {
			t.Fatalf("%s: sent %q, want %d notices", step, titles, len(want))
		}
		for i, part := range want {
			if !strings.Contains(titles[i], part) {
				t.Fatalf("%s: notice %q does not contain %q", step, titles[i], part)
			}
		}
	}

	sent("below every tier", 10, 5)
	sent("crosses 50%", 55, 5, "5-hour window usage reached 50%")
	sent("still between 50% and 75%", 60, 5)
	sent("crosses 75%", 76, 20, "5-hour window usage reached 75%")
	sent("same tier again", 80, 20)
	sent("weekly jumps past two tiers: one notice for the highest", 80, 77, "7-day window usage reached 75%")
	sent("5h reaches 100%", 100, 77, "5-hour window usage limit reached")
	sent("100% again", 100, 77)

	stored, err := model.GetChannelById(channel.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got := stored.UsageAlertStates()["5h"]; got.Tier != 100 || got.ResetAt != reset5h {
		t.Fatalf("stored 5h state = %+v", got)
	}
}

func TestUsageAlertsStartOverAfterTheWindowResets(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	setupUsageAlertTest(t, now)
	channel := createUsageChannel(t, "codex-b")
	reset := now.Unix() + 600

	before := testNotifier.count()
	checkUsageAlerts(channel, usageWindows(80, 0, reset, 0))
	if got := testNotifier.count() - before; got != 1 {
		t.Fatalf("first period sent %d notices, want 1", got)
	}

	// The period ended but the stored usage is from before the reset (Claude Code only updates
	// it on traffic): no notice for the stale figure, and the state is cleared.
	usageAlertNow = func() time.Time { return now.Add(20 * time.Minute) }
	checkUsageAlerts(channel, usageWindows(80, 0, reset, 0))
	if got := testNotifier.count() - before; got != 1 {
		t.Fatalf("stale usage after the reset sent %d notices in total, want 1", got)
	}
	if _, ok := channel.UsageAlertStates()["5h"]; ok {
		t.Fatal("alert state should be cleared once the window has reset")
	}

	// The new period reaches 50% and then 75%: both fire again.
	next := now.Unix() + 20*60 + 18000
	checkUsageAlerts(channel, usageWindows(52, 0, next, 0))
	checkUsageAlerts(channel, usageWindows(75, 0, next, 0))
	titles, _ := testNotifier.since(before)
	if len(titles) != 3 || !strings.Contains(titles[1], "50%") || !strings.Contains(titles[2], "75%") {
		t.Fatalf("new period notices = %q", titles)
	}
}

func TestUsageAlertsStartOverWhenUsageFallsBackWithoutResetTime(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	setupUsageAlertTest(t, now)
	channel := createUsageChannel(t, "codex-c")

	before := testNotifier.count()
	checkUsageAlerts(channel, usageWindows(60, 0, 0, 0))
	checkUsageAlerts(channel, usageWindows(65, 0, 0, 0))
	checkUsageAlerts(channel, usageWindows(3, 0, 0, 0))
	checkUsageAlerts(channel, usageWindows(51, 0, 0, 0))
	if got := testNotifier.count() - before; got != 2 {
		t.Fatalf("sent %d notices, want 2 (one per period)", got)
	}
}

func TestUsageAlertsOff(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	setupUsageAlertTest(t, now)
	windows := usageWindows(100, 100, now.Unix()+600, now.Unix()+86400)

	before := testNotifier.count()
	off := "0"
	channel := createUsageChannel(t, "codex-off")
	channel.UsageAlertTiers = &off
	checkUsageAlerts(channel, windows)
	if got := testNotifier.count() - before; got != 0 {
		t.Fatalf("channel tiers 0 sent %d notices", got)
	}

	config.UsageAlertTiers = "0"
	checkUsageAlerts(createUsageChannel(t, "codex-site-off"), windows)
	if got := testNotifier.count() - before; got != 0 {
		t.Fatalf("site tiers 0 sent %d notices", got)
	}

	config.UsageAlertTiers = ""
	config.NotifyDisabledKinds = "subscription_usage"
	checkUsageAlerts(createUsageChannel(t, "codex-kind-off"), windows)
	if got := testNotifier.count() - before; got != 0 {
		t.Fatalf("notice kind turned off sent %d notices", got)
	}
}

func TestUsageAlertsUseTheChannelTiersOverTheSite(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	setupUsageAlertTest(t, now)
	config.UsageAlertTiers = "90"
	own := "30,60"
	channel := createUsageChannel(t, "codex-own")
	channel.UsageAlertTiers = &own

	before := testNotifier.count()
	checkUsageAlerts(channel, usageWindows(35, 0, now.Unix()+600, 0))
	titles, _ := testNotifier.since(before)
	if len(titles) != 1 || !strings.Contains(titles[0], "30%") {
		t.Fatalf("channel tiers notices = %q", titles)
	}
	checkUsageAlerts(createUsageChannel(t, "codex-site"), usageWindows(35, 0, now.Unix()+600, 0))
	if got := testNotifier.count() - before; got != 1 {
		t.Fatalf("site tier 90 should not fire at 35%%, sent %d in total", got)
	}
}

func TestUsageAlertNoticeNamesTheChannelAndWindow(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	setupUsageAlertTest(t, now)
	channel := createUsageChannel(t, "team-codex")
	reset := now.Unix() + 3*86400

	before := testNotifier.count()
	checkUsageAlerts(channel, usageWindows(0, 82.34, 0, reset))
	titles, messages := testNotifier.since(before)
	if len(titles) != 1 {
		t.Fatalf("sent %q, want one notice", titles)
	}
	if titles[0] != "Channel team-codex: 7-day window usage reached 75%" {
		t.Errorf("title = %q", titles[0])
	}
	for _, part := range []string{
		"Channel: team-codex (ID ",
		"Window: 7-day window (7d)",
		"Used: 82.3% (alert tier 75%)",
		"Resets at: " + time.Unix(reset, 0).Format(time.RFC3339),
		"Plan: plus",
	} {
		if !strings.Contains(messages[0], part) {
			t.Errorf("message %q does not contain %q", messages[0], part)
		}
	}
}

func TestParseUsageAlertTiers(t *testing.T) {
	for in, want := range map[string]string{
		"":           "",
		"0":          "0",
		"off":        "0",
		"100, 50,75": "50,75,100",
		"50%，80%":    "50,80",
		"90 90 99.5": "90,99.5",
		" 75 ":       "75",
	} {
		got, err := model.NormalizeUsageAlertTiers(in)
		if err != nil || got != want {
			t.Errorf("NormalizeUsageAlertTiers(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"abc", "0,50", "150", "-5", ","} {
		if _, err := model.NormalizeUsageAlertTiers(bad); err == nil {
			t.Errorf("NormalizeUsageAlertTiers(%q) should fail", bad)
		}
	}
}

// The scheduled refresh covers subscription channels: a Codex channel's usage is queried on
// each run and its tiers are checked, and a Claude Code channel's stored usage is checked too.
func TestScheduledRefreshChecksSubscriptionUsage(t *testing.T) {
	now := time.Now()
	setupUsageAlertTest(t, now)
	resetAt := now.Unix() + 3600
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/backend-api/wham/usage" {
			http.NotFound(w, r)
			return
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"plan_type":"pro","rate_limit":{"allowed":true,"primary_window":{"used_percent":77,` +
			`"limit_window_seconds":18000,"reset_at":` + strconv.FormatInt(resetAt, 10) + `},"secondary_window":{"used_percent":12,"limit_window_seconds":604800}}}`))
	}))
	defer upstream.Close()
	baseURL := upstream.URL

	codex := &model.Channel{Name: "codex-sched", Type: config.ChannelTypeCodex, Key: "access-token", BaseURL: &baseURL, Status: config.ChannelStatusEnabled}
	claude := &model.Channel{Name: "claude-sched", Type: config.ChannelTypeClaudeCode, Key: "sk-ant-oat", Status: config.ChannelStatusEnabled}
	for _, ch := range []*model.Channel{codex, claude} {
		if err := model.DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := claude.UpdateUsageWindows(*usageWindows(51, 0, resetAt, 0)); err != nil {
		t.Fatal(err)
	}

	before := testNotifier.count()
	if err := updateAllChannelsBalance(); err != nil {
		t.Fatalf("updateAllChannelsBalance() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("usage endpoint called %d times, want 1", calls)
	}
	titles, _ := testNotifier.since(before)
	var codexNotice, claudeNotice bool
	for _, title := range titles {
		codexNotice = codexNotice || title == "Channel codex-sched: 5-hour window usage reached 75%"
		claudeNotice = claudeNotice || title == "Channel claude-sched: 5-hour window usage reached 50%"
	}
	if !codexNotice || !claudeNotice || len(titles) != 2 {
		t.Fatalf("notices = %q", titles)
	}

	// The next run finds the same usage: nothing new to send.
	if err := updateAllChannelsBalance(); err != nil {
		t.Fatal(err)
	}
	if got := testNotifier.count() - before; got != 2 {
		t.Fatalf("second run sent %d notices in total, want 2", got)
	}
	stored, err := model.GetChannelById(codex.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.UsageWindows == nil || len(stored.UsageWindows.Data().Windows) != 2 {
		t.Fatalf("codex usage windows not stored: %+v", stored.UsageWindows)
	}
}
