package controller

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// balanceUpstream is a DeepSeek-shaped balance endpoint answering with the status in *status.
func balanceUpstream(t *testing.T, status *atomic.Int32, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		code := int(status.Load())
		w.WriteHeader(code)
		if code == http.StatusOK {
			_, _ = w.Write([]byte(`{"is_available":true,"balance_infos":[{"currency":"CNY","total_balance":"12.5"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"error":{"message":"nope","type":"invalid_request_error"}}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func setupBackgroundReadTest(t *testing.T) (*model.Channel, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	setupBalanceTestDB(t)
	oldBackoffs, oldNow, oldInterval := backgroundBackoffs, backgroundNow, config.RequestInterval
	backgroundBackoffs = map[string]*backgroundBackoff{}
	config.RequestInterval = 0
	t.Cleanup(func() { backgroundBackoffs, backgroundNow, config.RequestInterval = oldBackoffs, oldNow, oldInterval })

	var status, hits atomic.Int32
	srv := balanceUpstream(t, &status, &hits)
	base := srv.URL
	channel := &model.Channel{Name: "ds", Type: config.ChannelTypeDeepseek, Key: "sk-old", BaseURL: &base, Status: config.ChannelStatusEnabled}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	return channel, &status, &hits
}

func TestScheduledBalanceStopsAfterKeyRejected(t *testing.T) {
	channel, status, hits := setupBackgroundReadTest(t)
	status.Store(http.StatusUnauthorized)

	before := testNotifier.count()
	for range 3 {
		if err := updateAllChannelsBalance(); err != nil {
			t.Fatal(err)
		}
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("upstream asked %d times after refusing the key, want 1", got)
	}
	if got := testNotifier.count() - before; got != 1 {
		t.Fatalf("sent %d notices, want 1", got)
	}
	if !backgroundReadStopped(channel, backgroundReadBalance) {
		t.Fatal("balance read should be stopped")
	}

	// A manual refresh is an admin's choice and still asks; a working key clears the stop.
	status.Store(http.StatusOK)
	if _, err := updateChannelBalance(channel); err != nil {
		t.Fatal(err)
	}
	noteBackgroundRead(channel, backgroundReadBalance, nil)
	if backgroundReadStopped(channel, backgroundReadBalance) {
		t.Fatal("a working read should clear the stop")
	}
	if err := updateAllChannelsBalance(); err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 3 {
		t.Fatalf("upstream asked %d times, want 3", got)
	}
}

func TestKeyChangeRestartsStoppedRead(t *testing.T) {
	channel, status, hits := setupBackgroundReadTest(t)
	status.Store(http.StatusForbidden)
	if err := updateAllChannelsBalance(); err != nil {
		t.Fatal(err)
	}
	old, err := model.GetChannelById(channel.Id)
	if err != nil {
		t.Fatal(err)
	}

	// Saving the channel with the same key keeps the stop.
	clearKeyRejectedOnKeyChange(old, channel.Id)
	if !backgroundReadStopped(channel, backgroundReadBalance) {
		t.Fatal("an edit that kept the key should keep the stop")
	}

	if err := model.DB.Model(&model.Channel{}).Where("id = ?", channel.Id).Update("key", "sk-new").Error; err != nil {
		t.Fatal(err)
	}
	clearKeyRejectedOnKeyChange(old, channel.Id)
	if backgroundReadStopped(channel, backgroundReadBalance) {
		t.Fatal("a new key should clear the stop")
	}
	status.Store(http.StatusOK)
	if err := updateAllChannelsBalance(); err != nil {
		t.Fatal(err)
	}
	if got := hits.Load(); got != 2 {
		t.Fatalf("upstream asked %d times, want 2", got)
	}
}

func TestScheduledBalanceBacksOffOnOtherFailures(t *testing.T) {
	_, status, hits := setupBackgroundReadTest(t)
	status.Store(http.StatusInternalServerError)
	now := time.Unix(1_800_000_000, 0)
	backgroundNow = func() time.Time { return now }

	run := func() {
		t.Helper()
		if err := updateAllChannelsBalance(); err != nil {
			t.Fatal(err)
		}
	}
	run()
	run() // within the first hour: skipped
	if got := hits.Load(); got != 1 {
		t.Fatalf("asked %d times within the backoff, want 1", got)
	}
	now = now.Add(time.Hour)
	run() // second failure: next wait is two hours
	now = now.Add(time.Hour)
	run()
	if got := hits.Load(); got != 2 {
		t.Fatalf("asked %d times, want 2 (second wait is two hours)", got)
	}
	now = now.Add(time.Hour)
	status.Store(http.StatusOK)
	run()
	run() // success resets the backoff
	if got := hits.Load(); got != 4 {
		t.Fatalf("asked %d times, want 4", got)
	}
}

func TestBackgroundReadBackoff(t *testing.T) {
	cases := map[int]time.Duration{0: 0, 1: time.Hour, 2: 2 * time.Hour, 3: 4 * time.Hour, 5: 16 * time.Hour, 6: 24 * time.Hour, 40: 24 * time.Hour}
	for n, want := range cases {
		if got := backgroundReadBackoff(n); got != want {
			t.Errorf("backgroundReadBackoff(%d) = %v, want %v", n, got, want)
		}
	}
}
