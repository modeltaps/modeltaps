package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// A decision model generates no text: chat, Responses and Claude requests for it are refused
// with a 400 before any upstream is called, so no line is tried or cooled down.
func TestDecisionModelRefusedOnTextEntries(t *testing.T) {
	upstream := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{{Name: "c", BaseURL: upstream.Server.URL, Models: "unrelated"}},
	})
	useSupplies(t,
		[]model.ModelInfo{{Model: "heygen-decision-1", Status: model.ModelStatusPublished, Endpoints: `["decisions"]`}},
		[]model.ModelSupply{{Model: "heygen-decision-1", ChannelId: h.Channel.Id, UpstreamModel: "heygen/heygen-decision-1", Enabled: true}})

	cases := map[string]string{
		"/v1/chat/completions": `{"model":"heygen-decision-1","messages":[{"role":"user","content":"hi"}]}`,
		"/v1/responses":        `{"model":"heygen-decision-1","input":"hi"}`,
		"/claude/v1/messages":  `{"model":"heygen-decision-1","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`,
	}
	for path, body := range cases {
		w := h.Post(path, body)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "decision model") {
			t.Errorf("%s: status %d body %s", path, w.Code, w.Body.String())
		}
	}
	if n := len(upstream.Requests()); n != 0 {
		t.Errorf("upstream called %d times, want 0", n)
	}
}

const decisionResponse = `{"model":"heygen/heygen-decision-1","answers":[
 {"type":"predicate","name":"damaged","probability":0.95},
 {"type":"choice","name":"queue","choice":"shipping","confidence":0.81,
  "probabilities":[{"value":"billing","probability":0.19},{"value":"shipping","probability":0.81}]}],
 "usage":{"input_tokens":96,"output_tokens":0,"total_tokens":96},
 "provider_metadata":{"gateway":{"cost":"0.0000096"}}}`

const decisionRequest = `{"model":"heygen-decision-1","input":"The package arrived broken.","safety_identifier":"u-1",
 "questions":[{"type":"predicate","name":"damaged","instructions":"Damaged?"},
 {"type":"choice","name":"queue","instructions":"Team?","choices":[{"value":"billing"},{"value":"shipping"}]}]}`

// POST /v1/decisions relays the client's body with only the model renamed, answers with the
// model the client asked for and without the upstream's metadata, and bills the input tokens
// the upstream reports (decision models have no output).
func TestDecisionsRelayAndBilling(t *testing.T) {
	upstream := NewFakeProvider(t)
	upstream.SetResponse(FakeResponse{Raw: decisionResponse})
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{{Name: "c", BaseURL: upstream.Server.URL, Models: "unrelated"}},
		Prices:   []PriceSpec{{Model: "heygen-decision-1", Input: billingInputRatio, Output: 0}},
	})
	useSupplies(t,
		[]model.ModelInfo{{Model: "heygen-decision-1", Status: model.ModelStatusPublished, Endpoints: `["decisions"]`}},
		[]model.ModelSupply{{Model: "heygen-decision-1", ChannelId: h.Channel.Id, UpstreamModel: "heygen/heygen-decision-1", Enabled: true}})

	before := h.UserQuota(t)
	w := h.Post("/v1/decisions", decisionRequest)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}

	sent := upstream.LastRequest(t)
	if sent.Path != "/v1/decisions" {
		t.Errorf("upstream path = %s", sent.Path)
	}
	body := sent.JSONBody(t)
	if body["model"] != "heygen/heygen-decision-1" || body["safety_identifier"] != "u-1" || body["questions"] == nil {
		t.Errorf("upstream body = %v", body)
	}

	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["model"] != "heygen-decision-1" || resp["provider_metadata"] != nil {
		t.Errorf("response model / metadata = %v / %v", resp["model"], resp["provider_metadata"])
	}
	if answers, _ := resp["answers"].([]any); len(answers) != 2 {
		t.Errorf("answers = %v", resp["answers"])
	}

	want := expectQuota(96, 0, 1)
	if got := before - h.UserQuota(t); got != want {
		t.Errorf("charged %d, want %d", got, want)
	}
	log := soleConsumeLog(t)
	if log.PromptTokens != 96 || log.CompletionTokens != 0 || log.ModelName != "heygen-decision-1" {
		t.Errorf("log = prompt %d completion %d model %s", log.PromptTokens, log.CompletionTokens, log.ModelName)
	}
	if mode := log.Metadata.Data()["relay_mode"]; mode != "decisions" {
		t.Errorf("relay_mode = %v", mode)
	}
}

// A model the catalog does not list for /v1/decisions is refused there before any upstream call,
// and so is a request without questions.
func TestDecisionsRefusesNonDecisionModels(t *testing.T) {
	upstream := NewFakeProvider(t)
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{{Name: "c", BaseURL: upstream.Server.URL, Models: "unrelated"}},
	})
	useSupplies(t,
		[]model.ModelInfo{
			{Model: "gpt-4o-mini", Status: model.ModelStatusPublished, Endpoints: `["chat","responses"]`},
			{Model: "heygen-decision-1", Status: model.ModelStatusPublished, Endpoints: `["decisions"]`},
		},
		[]model.ModelSupply{
			{Model: "gpt-4o-mini", ChannelId: h.Channel.Id, UpstreamModel: "gpt-4o-mini", Enabled: true},
			{Model: "heygen-decision-1", ChannelId: h.Channel.Id, UpstreamModel: "heygen/heygen-decision-1", Enabled: true},
		})

	w := h.Post("/v1/decisions", strings.Replace(decisionRequest, "heygen-decision-1", "gpt-4o-mini", 1))
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "not a decision model") {
		t.Errorf("chat model: status %d body %s", w.Code, w.Body.String())
	}
	w = h.Post("/v1/decisions", `{"model":"heygen-decision-1","input":"x","questions":[]}`)
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "questions") {
		t.Errorf("no questions: status %d body %s", w.Code, w.Body.String())
	}
	if n := len(upstream.Requests()); n != 0 {
		t.Errorf("upstream called %d times, want 0", n)
	}
}

// An upstream failure on one line moves the request to the next line, as for chat.
func TestDecisionsRetriesOnAnotherLine(t *testing.T) {
	down := NewFakeProvider(t)
	down.SetResponse(FakeResponse{Status: http.StatusInternalServerError, Raw: `{"error":{"message":"down","type":"server_error"}}`})
	up := NewFakeProvider(t)
	up.SetResponse(FakeResponse{Raw: decisionResponse})
	h := NewHarness(t, HarnessOptions{
		Channels: []ChannelSpec{
			{Name: "down", BaseURL: down.Server.URL, Models: "unrelated", Priority: 10},
			{Name: "up", BaseURL: up.Server.URL, Models: "unrelated"},
		},
		Prices: []PriceSpec{{Model: "heygen-decision-1", Input: billingInputRatio, Output: 0}},
	})
	useSupplies(t,
		[]model.ModelInfo{{Model: "heygen-decision-1", Status: model.ModelStatusPublished, Endpoints: `["decisions"]`}},
		[]model.ModelSupply{
			{Model: "heygen-decision-1", ChannelId: h.Channels[0].Id, UpstreamModel: "heygen/heygen-decision-1", Enabled: true, Priority: 10},
			{Model: "heygen-decision-1", ChannelId: h.Channels[1].Id, UpstreamModel: "heygen/heygen-decision-1", Enabled: true},
		})
	setAndRestore(t, &config.RetryTimes, 3)

	w := h.Post("/v1/decisions", decisionRequest)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	if len(down.Requests()) != 1 || len(up.Requests()) != 1 {
		t.Errorf("requests: down %d, up %d", len(down.Requests()), len(up.Requests()))
	}
}
