package model

import (
	"net/http"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func usePauseTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := testDB.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&SupplyPause{}); err != nil {
		t.Fatal(err)
	}
	oldDB := DB
	DB = testDB
	pauseIndex.Lock()
	oldIndex := pauseIndex.byChannel
	pauseIndex.byChannel = map[int]map[string]*SupplyPause{}
	pauseIndex.Unlock()
	t.Cleanup(func() {
		DB = oldDB
		pauseIndex.Lock()
		pauseIndex.byChannel = oldIndex
		pauseIndex.Unlock()
	})
}

func firstMatch(t *testing.T, f PauseFailure) string {
	t.Helper()
	for _, rule := range EffectivePauseRules() {
		if rule.Matches(f) {
			return rule.Name
		}
	}
	return ""
}

func TestDefaultPauseRulesMatch(t *testing.T) {
	if err := SetCustomPauseRules(""); err != nil {
		t.Fatal(err)
	}
	config.SetRetryCooldownPerStatusMap(map[int]int{418: 30})
	t.Cleanup(func() { config.SetRetryCooldownPerStatusMap(map[int]int{}) })

	for _, tc := range []struct {
		name string
		f    PauseFailure
		want string
	}{
		{"402", PauseFailure{StatusCode: http.StatusPaymentRequired}, "quota"},
		{"openai insufficient_quota", PauseFailure{StatusCode: 429, ErrorType: "insufficient_quota", Unusable: true}, "quota"},
		{"credit balance as a 400", PauseFailure{StatusCode: 400, Message: "Your credit balance is too low to access the API.", Unusable: true}, "quota"},
		{"chinese balance message", PauseFailure{StatusCode: 403, Message: "账户余额不足"}, "quota"},
		{"bad key", PauseFailure{StatusCode: 401, Message: "Incorrect API key", Unusable: true}, "credential"},
		{"429 with reset", PauseFailure{StatusCode: 429, RateLimitReset: true}, "rate_limit_reset"},
		{"plain 429", PauseFailure{StatusCode: 429}, "rate_limit"},
		{"model not found", PauseFailure{StatusCode: 404, ModelNotFound: true}, "model_not_found"},
		{"configured status", PauseFailure{StatusCode: 418}, "per_status"},
		{"503", PauseFailure{StatusCode: 503}, "upstream_failure"},
		{"plain 400", PauseFailure{StatusCode: 400, Message: "messages: field required"}, ""},
	} {
		if got := firstMatch(t, tc.f); got != tc.want {
			t.Errorf("%s: first rule = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCustomPauseRules(t *testing.T) {
	t.Cleanup(func() { _ = SetCustomPauseRules("") })
	for _, bad := range []string{
		`not json`,
		`[{"name":"x","action":"explode"}]`,
		`[{"name":"x","action":"pause"}]`,
		`[{"name":"x","action":"pause","seconds":60,"scope":"galaxy"}]`,
		`[{"name":"x","action":"pause","seconds":60,"times":3}]`,
		`[{"name":"x","action":"pause","seconds":60,"keywords":["re:("]}]`,
	} {
		if err := SetCustomPauseRules(bad); err == nil {
			t.Errorf("accepted invalid rules %s", bad)
		}
	}
	if err := SetCustomPauseRules(`[{"name":"teapot","status_codes":[418],"keywords":["re:^brew"],"action":"pause","seconds":90,"scope":"channel"}]`); err != nil {
		t.Fatal(err)
	}
	rules := EffectivePauseRules()
	if len(rules) != 1 || rules[0].Name != "teapot" {
		t.Fatalf("effective rules = %+v, want the custom rule only", rules)
	}
	if !rules[0].Matches(PauseFailure{StatusCode: 500, Message: "Brewing failed"}) {
		t.Fatal("regex keyword did not match case-insensitively")
	}
	if rules[0].Matches(PauseFailure{StatusCode: 500, Message: "nothing"}) {
		t.Fatal("rule matched without any of its conditions")
	}
}

func TestPauseStore(t *testing.T) {
	usePauseTestDB(t)
	now := time.Now().Unix()

	started, err := SavePause(&SupplyPause{ChannelId: 1, Until: now + 600, Seconds: 600, Rule: "quota", Category: PauseCategoryQuota, Probe: true})
	if err != nil || !started {
		t.Fatalf("first save: started=%v err=%v", started, err)
	}
	if !IsPaused(1, "any-model") {
		t.Fatal("a whole-channel pause must block every model")
	}
	// Never shortened, only extended.
	if started, _ := SavePause(&SupplyPause{ChannelId: 1, Until: now + 60, Seconds: 60}); started {
		t.Fatal("a second save reported a new pause")
	}
	list, _ := ListPauses()
	if len(list) != 1 || list[0].Until != now+600 {
		t.Fatalf("pauses = %+v, want the original end kept", list)
	}

	if _, err := SavePause(&SupplyPause{ChannelId: 2, Model: "m", Until: now - 1, Seconds: 60}); err != nil {
		t.Fatal(err)
	}
	if IsPaused(2, "m") {
		t.Fatal("an expired pause without a probe must not block")
	}
	if _, err := SavePause(&SupplyPause{ChannelId: 3, Model: "m", Until: now - 1, Seconds: 60, Probe: true}); err != nil {
		t.Fatal(err)
	}
	if !IsPaused(3, "m") || IsPaused(3, "other") {
		t.Fatal("an expired pause with a probe must block its model only, until resumed")
	}

	due, _ := DuePauses(now)
	if len(due) != 2 {
		t.Fatalf("due pauses = %d, want 2", len(due))
	}
	var probed *SupplyPause
	for _, p := range due {
		if p.ChannelId == 3 {
			probed = p
		}
	}
	if !ExtendPause(probed, now) || probed.Seconds != 120 || probed.Until != now+120 || probed.Probes != 1 {
		t.Fatalf("extended pause = %+v, want doubled to 120s", probed)
	}
	stale := *probed
	stale.Until = now - 1
	if ResumePause(&stale) {
		t.Fatal("resumed a pause that another node had already extended")
	}
	if !ResumePause(probed) || IsPaused(3, "m") {
		t.Fatal("resume did not lift the pause")
	}

	capped := &SupplyPause{Id: 0, ChannelId: 4, Until: now + 10, Seconds: MaxPauseSeconds}
	if _, err := SavePause(capped); err != nil {
		t.Fatal(err)
	}
	ExtendPause(capped, now)
	if capped.Seconds != MaxPauseSeconds {
		t.Fatalf("extended seconds = %d, want capped at %d", capped.Seconds, MaxPauseSeconds)
	}

	resumed, err := ResumeChannelPausesInCategory(1, PauseCategoryQuota)
	if err != nil || len(resumed) != 1 || IsPaused(1, "any-model") {
		t.Fatalf("quota resume: %d resumed, err %v", len(resumed), err)
	}
}

// Pauses written by another node take effect after a reload.
func TestReloadPausesPicksUpOtherNodes(t *testing.T) {
	usePauseTestDB(t)
	if err := DB.Create(&SupplyPause{ChannelId: 9, Model: "m", Until: time.Now().Unix() + 300, Seconds: 300}).Error; err != nil {
		t.Fatal(err)
	}
	if IsPaused(9, "m") {
		t.Fatal("paused before the reload")
	}
	if err := ReloadPauses(); err != nil {
		t.Fatal(err)
	}
	if !IsPaused(9, "m") {
		t.Fatal("reload did not pick up the pause")
	}
}
