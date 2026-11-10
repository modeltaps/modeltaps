package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"go.uber.org/zap"
)

func setupPauseRecovery(t *testing.T, upstreamStatus int) *model.Channel {
	t.Helper()
	setModelInfoModeSwitch(t, false)
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()
	testDB := setupChannelTestDB(t)
	if err := testDB.AutoMigrate(&model.SupplyPause{}); err != nil {
		t.Fatal(err)
	}
	if err := model.ReloadPauses(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = model.ReloadPauses() })
	notify.AddNotifiers(testNotifier)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(upstreamStatus)
		if upstreamStatus == http.StatusOK {
			fmt.Fprint(w, `{"id":"c1","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
			return
		}
		fmt.Fprint(w, `{"error":{"message":"still broken","type":"server_error"}}`)
	}))
	t.Cleanup(upstream.Close)
	baseURL := upstream.URL
	channel := &model.Channel{Type: config.ChannelTypeOpenAI, Key: "sk-x", Name: "paused-channel", Status: config.ChannelStatusEnabled,
		BaseURL: &baseURL, Models: "gpt-4o-mini", TestModel: "gpt-4o-mini"}
	if err := testDB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	return channel
}

func duePause(t *testing.T, p *model.SupplyPause) *model.SupplyPause {
	t.Helper()
	p.Until = time.Now().Unix() - 1
	if _, err := model.SavePause(p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRecoverDuePauses(t *testing.T) {
	t.Run("probe passes", func(t *testing.T) {
		channel := setupPauseRecovery(t, http.StatusOK)
		before := testNotifier.countContaining("resumed")
		duePause(t, &model.SupplyPause{ChannelId: channel.Id, Seconds: 1800, Rule: "quota", Probe: true, Notify: true})
		recoverDuePauses(time.Now().Unix())
		if model.IsPaused(channel.Id, "gpt-4o-mini") {
			t.Fatal("still paused after a passing probe")
		}
		if got := testNotifier.countContaining("resumed") - before; got != 1 {
			t.Fatalf("resumed notices = %d, want 1", got)
		}
	})

	t.Run("probe fails", func(t *testing.T) {
		channel := setupPauseRecovery(t, http.StatusInternalServerError)
		before := testNotifier.countContaining("still paused")
		duePause(t, &model.SupplyPause{ChannelId: channel.Id, Model: "gpt-4o-mini", Seconds: 120, Rule: "upstream_failure", Probe: true, Notify: true})
		recoverDuePauses(time.Now().Unix())
		pauses, _ := model.ListPauses()
		if len(pauses) != 1 || pauses[0].Seconds != 240 || pauses[0].Probes != 1 || pauses[0].Until <= time.Now().Unix() {
			t.Fatalf("pauses = %+v, want the pause doubled to 240s", pauses)
		}
		if !model.IsPaused(channel.Id, "gpt-4o-mini") {
			t.Fatal("not paused after a failing probe")
		}
		if got := testNotifier.countContaining("still paused") - before; got != 1 {
			t.Fatalf("still-paused notices = %d, want 1", got)
		}
	})

	t.Run("no probe", func(t *testing.T) {
		channel := setupPauseRecovery(t, http.StatusInternalServerError)
		duePause(t, &model.SupplyPause{ChannelId: channel.Id, Model: "gpt-4o-mini", Seconds: 600, Rule: "manual"})
		recoverDuePauses(time.Now().Unix())
		if pauses, _ := model.ListPauses(); len(pauses) != 0 {
			t.Fatalf("pauses = %+v, want the expired pause removed without a test", pauses)
		}
	})

	t.Run("channel disabled meanwhile", func(t *testing.T) {
		channel := setupPauseRecovery(t, http.StatusOK)
		duePause(t, &model.SupplyPause{ChannelId: channel.Id, Seconds: 600, Rule: "quota", Probe: true, Notify: true})
		model.DB.Model(channel).Update("status", config.ChannelStatusManuallyDisabled)
		before := testNotifier.countContaining("resumed")
		recoverDuePauses(time.Now().Unix())
		if pauses, _ := model.ListPauses(); len(pauses) != 0 {
			t.Fatalf("pauses = %+v, want the pause dropped", pauses)
		}
		if got := testNotifier.countContaining("resumed") - before; got != 0 {
			t.Fatalf("announced the resume of a disabled channel")
		}
	})
}

// An empty balance pauses the channel like an exhausted quota; money ends the pause early.
func TestBalanceRefreshPausesAndResumes(t *testing.T) {
	channel := setupPauseRecovery(t, http.StatusOK)
	if !handleBalanceForPauses(channel, 0) {
		t.Fatal("balance not handled with pause rules on")
	}
	pauses, _ := model.ListPauses()
	if len(pauses) != 1 || pauses[0].Model != "" || pauses[0].Category != model.PauseCategoryQuota || pauses[0].Seconds != 1800 {
		t.Fatalf("pauses = %+v, want a 30-minute quota pause of the channel", pauses)
	}
	before := testNotifier.countContaining("resumed")
	handleBalanceForPauses(channel, 12.5)
	if model.IsPaused(channel.Id, "gpt-4o-mini") {
		t.Fatal("still paused after the balance refresh found money")
	}
	if got := testNotifier.countContaining("resumed") - before; got != 1 {
		t.Fatalf("resumed notices = %d, want 1", got)
	}

	old := config.PauseRulesEnabled
	config.PauseRulesEnabled = false
	t.Cleanup(func() { config.PauseRulesEnabled = old })
	if handleBalanceForPauses(channel, 0) {
		t.Fatal("balance handled with pause rules off")
	}
}
