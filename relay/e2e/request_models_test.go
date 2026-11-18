package e2e

// A request's own fallback models: a top-level "models" array, tried in order after the model
// when every line of the one before fails. The array never reaches an upstream, and only models
// the key could call by itself are used.

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func requestModelsBody(primary string, models ...string) string {
	list := ""
	for i, m := range models {
		if i > 0 {
			list += ","
		}
		list += `"` + m + `"`
	}
	return `{"model":"` + primary + `","models":[` + list + `],"messages":[{"role":"user","content":"hello"}]}`
}

func newRequestModelsHarness(t *testing.T, primaryURL, secondURL string) *Harness {
	t.Helper()
	h := NewHarness(t, HarnessOptions{
		Model: fallbackPrimary,
		Channels: []ChannelSpec{
			{Name: "fake-primary", BaseURL: primaryURL, Models: fallbackPrimary},
			{Name: "fake-second", BaseURL: secondURL, Models: fallbackSecond},
		},
		Prices: []PriceSpec{
			{Model: fallbackPrimary, Input: 10, Output: 10},
			{Model: fallbackSecond, Input: 1, Output: 1},
		},
	})
	setAndRestore(t, &config.RetryTimes, 2)
	return h
}

func TestRequestModelsFallBackInOrder(t *testing.T) {
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	second := NewFakeProvider(t)
	second.SetResponse(FakeResponse{JSON: ChatCompletionJSON(fallbackSecond, "ok", 100, 50)})
	h := newRequestModelsHarness(t, primary.Server.URL, second.Server.URL)

	w := h.Post("/v1/chat/completions", requestModelsBody(fallbackPrimary, fallbackSecond))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if _, ok := primary.LastRequest(t).JSONBody(t)["models"]; ok {
		t.Fatalf("the primary upstream received the models field")
	}
	body := second.LastRequest(t).JSONBody(t)
	if body["model"] != fallbackSecond {
		t.Fatalf("second upstream got model %v", body["model"])
	}
	if _, ok := body["models"]; ok {
		t.Fatalf("the second upstream received the models field")
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "model=second-model; step=1; reason=request_models" {
		t.Fatalf("X-Modeltaps-Route = %q", got)
	}
	log, ok := consumeLogsByModel(t)[fallbackSecond]
	if !ok {
		t.Fatalf("no consume log for the second model")
	}
	meta := log.Metadata.Data()
	if _, has := meta["routing_policy"]; has || meta["routing_step"] != float64(1) || meta["routing_reason"] != "request_models" {
		t.Fatalf("routing metadata = %v", meta)
	}
}

// A model the key may not call is dropped from the list: the request fails instead of reaching it.
func TestRequestModelsSkipModelsTheKeyCannotCall(t *testing.T) {
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	second := NewFakeProvider(t)
	second.SetResponse(FakeResponse{JSON: ChatCompletionJSON(fallbackSecond, "ok", 10, 5)})
	h := newRequestModelsHarness(t, primary.Server.URL, second.Server.URL)

	setting := model.TokenSetting{}
	setting.Limits.LimitModelSetting.Enabled = true
	setting.Limits.LimitModelSetting.Models = []string{fallbackPrimary}
	h.Token.Setting.Set(setting)
	if err := model.DB.Save(h.Token).Error; err != nil {
		t.Fatalf("save token: %v", err)
	}

	w := h.Post("/v1/chat/completions", requestModelsBody(fallbackPrimary, fallbackSecond))
	if w.Code == http.StatusOK {
		t.Fatalf("want an error, got 200")
	}
	if n := len(second.Requests()); n != 0 {
		t.Fatalf("a model outside the key's allow list got %d requests", n)
	}
}

// The model answers: the listed models are never asked, and the field is still stripped.
func TestRequestModelsUnusedWhenModelAnswers(t *testing.T) {
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{JSON: ChatCompletionJSON(fallbackPrimary, "ok", 10, 5)})
	second := NewFakeProvider(t)
	h := newRequestModelsHarness(t, primary.Server.URL, second.Server.URL)

	w := h.Post("/v1/chat/completions", requestModelsBody(fallbackPrimary, fallbackSecond))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(second.Requests()) != 0 {
		t.Fatalf("the listed model was asked although the model answered")
	}
	if _, ok := primary.LastRequest(t).JSONBody(t)["models"]; ok {
		t.Fatalf("the upstream received the models field")
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "" {
		t.Fatalf("no route header expected, got %q", got)
	}
}

// Claude Messages forwards the client's bytes to Anthropic channels; the models field is gone
// from them, and the listed model takes over when the first one fails.
func TestRequestModelsStrippedFromClaudePassthrough(t *testing.T) {
	primary := NewFakeProvider(t)
	primary.SetResponse(FakeResponse{Status: http.StatusInternalServerError, JSON: OpenAIErrorJSON("boom", "server_error", "internal_error")})
	second := NewFakeProvider(t)
	second.SetResponse(FakeResponse{JSON: anthropicMessageJSON(fallbackSecond, "ok", 3, 1)})
	h := NewHarness(t, HarnessOptions{
		Model: fallbackPrimary,
		Channels: []ChannelSpec{
			{Name: "a", Type: config.ChannelTypeAnthropic, BaseURL: primary.Server.URL, Models: fallbackPrimary},
			{Name: "b", Type: config.ChannelTypeAnthropic, BaseURL: second.Server.URL, Models: fallbackSecond},
		},
		Prices: []PriceSpec{{Model: fallbackPrimary, Input: 1, Output: 1}, {Model: fallbackSecond, Input: 1, Output: 1}},
	})
	setAndRestore(t, &config.RetryTimes, 2)
	body := `{"model":"` + fallbackPrimary + `","models":["` + fallbackSecond + `"],"max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`
	w := h.Post("/claude/v1/messages", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	for name, f := range map[string]*FakeProvider{"primary": primary, "second": second} {
		req := f.LastRequest(t).JSONBody(t)
		if _, ok := req["models"]; ok {
			t.Fatalf("the %s upstream received the models field", name)
		}
	}
	if got := second.LastRequest(t).JSONBody(t)["model"]; got != fallbackSecond {
		t.Fatalf("second upstream got model %v", got)
	}
}
