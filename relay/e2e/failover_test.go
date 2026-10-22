package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

type loggedAttempt struct {
	ChannelID int  `json:"channel_id"`
	Status    int  `json:"status"`
	Cooldown  bool `json:"cooldown"`
}

func logAttempts(t *testing.T, log model.Log) []loggedAttempt {
	t.Helper()
	raw, ok := log.Metadata.Data()["retry_attempts"]
	if !ok {
		return nil
	}
	b, _ := json.Marshal(raw)
	var attempts []loggedAttempt
	if err := json.Unmarshal(b, &attempts); err != nil {
		t.Fatalf("decode retry_attempts: %v (%s)", err, b)
	}
	return attempts
}

func errorLogs(t *testing.T) []model.Log {
	t.Helper()
	var logs []model.Log
	if err := model.DB.Where("type = ?", model.LogTypeError).Find(&logs).Error; err != nil {
		t.Fatalf("read error logs: %v", err)
	}
	return logs
}

// TestRetryChainRecordedOnSuccess: the consume log of a request that succeeded after a
// retry lists the failed attempt, and no error log is written.
func TestRetryChainRecordedOnSuccess(t *testing.T) {
	failing := NewFakeProvider(t)
	failing.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	healthy := NewFakeProvider(t)
	healthy.SetResponse(FakeResponse{JSON: ChatCompletionJSON("gpt-4o-mini", "ok", 11, 7)})

	h := NewHarness(t, HarnessOptions{Channels: []ChannelSpec{
		{Name: "failing", BaseURL: failing.Server.URL, Priority: 10},
		{Name: "healthy", BaseURL: healthy.Server.URL, Priority: 1},
	}})
	setAndRestore(t, &config.RetryTimes, 3)
	setAndRestore(t, &config.LogRelayErrorEnabled, true)

	if w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`); w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	logs := consumeLogs(t)
	if len(logs) != 1 {
		t.Fatalf("consume logs = %d", len(logs))
	}
	attempts := logAttempts(t, logs[0])
	if len(attempts) != 1 || attempts[0].ChannelID != h.Channels[0].Id || attempts[0].Status != http.StatusInternalServerError {
		t.Fatalf("retry_attempts = %+v, want one 500 on channel %d", attempts, h.Channels[0].Id)
	}
	if n := len(errorLogs(t)); n != 0 {
		t.Fatalf("a successful request wrote %d error logs", n)
	}
}

// TestFinalFailureWritesErrorLog: when every channel fails, an error log carries the
// status and message the client received and the whole retry chain; no quota is charged.
func TestFinalFailureWritesErrorLog(t *testing.T) {
	first := NewFakeProvider(t)
	first.SetResponse(FakeResponse{Status: http.StatusBadGateway, JSON: OpenAIErrorJSON("bad gateway", "server_error", "bad_gateway")})
	second := NewFakeProvider(t)
	second.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("secret upstream detail", "server_error", "internal_error")})

	h := NewHarness(t, HarnessOptions{Channels: []ChannelSpec{
		{Name: "first", BaseURL: first.Server.URL, Priority: 10},
		{Name: "second", BaseURL: second.Server.URL, Priority: 1},
	}})
	setAndRestore(t, &config.RetryTimes, 3)
	setAndRestore(t, &config.LogRelayErrorEnabled, true)

	w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	logs := errorLogs(t)
	if len(logs) != 1 {
		t.Fatalf("error logs = %d", len(logs))
	}
	log := logs[0]
	if !strings.HasPrefix(log.Content, "503 ") || strings.Contains(log.Content, "secret upstream detail") {
		t.Fatalf("content = %q, want the collapsed 503 the client saw", log.Content)
	}
	if log.Quota != 0 || log.UserId != h.User.Id || log.ModelName != "gpt-4o-mini" || log.ChannelId != h.Channels[1].Id {
		t.Fatalf("log = %+v", log)
	}
	attempts := logAttempts(t, log)
	if len(attempts) != 2 || attempts[0].ChannelID != h.Channels[0].Id || attempts[0].Status != 502 ||
		attempts[1].ChannelID != h.Channels[1].Id || attempts[1].Status != 500 {
		t.Fatalf("retry_attempts = %+v", attempts)
	}
	if n := len(consumeLogs(t)); n != 0 {
		t.Fatalf("a failed request wrote %d consume logs", n)
	}
}

// TestErrorLogSwitchOff: LogRelayErrorEnabled=false writes nothing.
func TestErrorLogSwitchOff(t *testing.T) {
	fake := NewFakeProvider(t)
	fake.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})
	setAndRestore(t, &config.LogRelayErrorEnabled, false)

	h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`)
	if n := len(errorLogs(t)); n != 0 {
		t.Fatalf("error logs written with the switch off: %d", n)
	}
}
