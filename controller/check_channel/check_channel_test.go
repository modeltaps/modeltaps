package check_channel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/gemini"
	"github.com/modeltaps/modeltaps/providers/vertexai_express"
	"github.com/modeltaps/modeltaps/types"
)

var challengeQuestion = regexp.MustCompile(`Q: (\d+) ([+-]) (\d+) = \?\nA:$`)

func solve(t *testing.T, prompt string) string {
	t.Helper()
	m := challengeQuestion.FindStringSubmatch(prompt)
	if m == nil {
		t.Fatalf("challenge prompt does not end with a question: %q", prompt)
	}
	a, _ := strconv.Atoi(m[1])
	b, _ := strconv.Atoi(m[3])
	if m[2] == "+" {
		return strconv.Itoa(a + b)
	}
	return strconv.Itoa(a - b)
}

func TestGenerateChallenge(t *testing.T) {
	for range 500 {
		c := generateChallenge()
		want := solve(t, c.Prompt)
		if c.Expected != want {
			t.Fatalf("expected %q for %q, want %q", c.Expected, c.Prompt, want)
		}
		if n, _ := strconv.Atoi(c.Expected); n < 0 {
			t.Fatalf("negative answer %d for %q", n, c.Prompt)
		}
		if !validateChallenge(c.Expected, c.Expected) {
			t.Fatalf("the exact answer %q must pass", c.Expected)
		}
	}
}

func TestValidateChallenge(t *testing.T) {
	cases := []struct {
		answer, expected string
		want             bool
	}{
		{"42", "42", true},
		{" 42\n", "42", true},
		{"The answer is 42.", "42", true},
		{"A: 42", "42", true},
		{"41", "42", false},
		{"42 or maybe 7", "42", false},
		{"420", "42", false},
		// A relay echoing the prompt carries the few-shot answers and the operands.
		{"Q: 3 + 5 = ?\nA: 8\n\nQ: 40 + 2 = ?\nA: 42", "42", false},
		{"", "42", false},
		{"no digits", "42", false},
		{"0", "", false},
	}
	for _, tc := range cases {
		if got := validateChallenge(tc.answer, tc.expected); got != tc.want {
			t.Errorf("validateChallenge(%q, %q) = %v, want %v", tc.answer, tc.expected, got, tc.want)
		}
	}
}

func TestAnswerEquals(t *testing.T) {
	cases := []struct {
		answer string
		want   bool
	}{
		{"-3.75", true},
		{"x = -3.75", true},
		{"-15/4", true},
		{"x = -15/4 = -3.75", true},
		{"-3.750", true},
		{"3.75", false},
		{"-3.7", false},
		{"-15/0", false},
		{"", false},
		{"minus three point seven five", false},
	}
	for _, tc := range cases {
		if got := answerEquals(tc.answer, jsonExpectedAnswer); got != tc.want {
			t.Errorf("answerEquals(%q) = %v, want %v", tc.answer, got, tc.want)
		}
	}
}

func toolCall(name, args string) *types.ChatCompletionToolCalls {
	return &types.ChatCompletionToolCalls{Type: "function", Function: &types.ChatCompletionToolCallsFunction{Name: name, Arguments: args}}
}

func TestMatchToolCalls(t *testing.T) {
	cases := []struct {
		name   string
		calls  []*types.ChatCompletionToolCalls
		status int
		code   string
		listed []string
	}{
		{"both", []*types.ChatCompletionToolCalls{toolCall("multiply", `{"a":3,"b":12}`), toolCall("add", `{"a":11,"b":49}`)},
			CheckStatusSuccess, "tool_passed", []string{"multiply(3, 12)", "add(11, 49)"}},
		{"commuted and float arguments", []*types.ChatCompletionToolCalls{toolCall("add", `{"a":49.0,"b":11.0}`), toolCall("multiply", `{"a":12,"b":3}`)},
			CheckStatusSuccess, "tool_passed", []string{"add(49, 11)", "multiply(12, 3)"}},
		{"only one", []*types.ChatCompletionToolCalls{toolCall("multiply", `{"a":3,"b":12}`)},
			CheckStatusWarning, "tool_partial", []string{"multiply(3, 12)"}},
		{"wrong arguments", []*types.ChatCompletionToolCalls{toolCall("add", `{"a":3,"b":12}`)},
			CheckStatusFailed, "tool_wrong", []string{"add(3, 12)"}},
		{"unparsable arguments", []*types.ChatCompletionToolCalls{toolCall("add", `{oops`), nil, {Type: "function"}},
			CheckStatusFailed, "tool_wrong", []string{"add(?)"}},
		{"none", nil, CheckStatusFailed, "tool_none", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := matchToolCalls(tc.calls)
			if r.Status != tc.status || r.Code != tc.code {
				t.Fatalf("got status %d code %q, want %d %q", r.Status, r.Code, tc.status, tc.code)
			}
			if tc.listed == nil {
				return
			}
			got, _ := r.Params["calls"].([]string)
			if len(got) != len(tc.listed) {
				t.Fatalf("calls = %v, want %v", got, tc.listed)
			}
			for i := range got {
				if got[i] != tc.listed[i] {
					t.Fatalf("calls = %v, want %v", got, tc.listed)
				}
			}
		})
	}
	if r := matchToolCalls([]*types.ChatCompletionToolCalls{toolCall("multiply", `{"a":3,"b":12}`)}); r.Params["matched"] != 1 || r.Params["total"] != 2 {
		t.Fatalf("partial params = %v", r.Params)
	}
}

func TestItemStatus(t *testing.T) {
	r := func(statuses ...int) []*CheckResult {
		out := make([]*CheckResult, len(statuses))
		for i, s := range statuses {
			out[i] = &CheckResult{Status: s}
		}
		return out
	}
	cases := []struct {
		results []*CheckResult
		want    int
	}{
		{r(CheckStatusSuccess, CheckStatusSuccess), CheckStatusSuccess},
		{r(CheckStatusSuccess, CheckStatusWarning), CheckStatusWarning},
		{r(CheckStatusWarning, CheckStatusFailed, CheckStatusSuccess), CheckStatusFailed},
		{r(CheckStatusSkipped), CheckStatusSkipped},
		{r(CheckStatusSkipped, CheckStatusSuccess), CheckStatusSuccess},
		{nil, CheckStatusSkipped},
	}
	for i, tc := range cases {
		if got := itemStatus(tc.results); got != tc.want {
			t.Errorf("case %d: itemStatus = %d, want %d", i, got, tc.want)
		}
	}
}

func TestAggregateStatus(t *testing.T) {
	item := func(key string, status int) *CheckProcessResult { return &CheckProcessResult{Key: key, Status: status} }
	cases := []struct {
		name  string
		items []*CheckProcessResult
		want  int
	}{
		{"all passed", []*CheckProcessResult{item("base", 1), item("image", 1), item("json", 1), item("tool", 1), item("relay", 1)}, CheckStatusSuccess},
		{"base failed", []*CheckProcessResult{item("base", 0), item("image", 1)}, CheckStatusFailed},
		{"other failure is a warning", []*CheckProcessResult{item("base", 1), item("image", 0), item("json", 1)}, CheckStatusWarning},
		{"warning propagates", []*CheckProcessResult{item("base", 2), item("image", 1)}, CheckStatusWarning},
		{"relay is ignored", []*CheckProcessResult{item("base", 1), item("relay", 2), item("tool", 1)}, CheckStatusSuccess},
		{"skipped is ignored", []*CheckProcessResult{item("base", 1), item("json", 3), item("relay", 3)}, CheckStatusSuccess},
		{"base failure wins over order", []*CheckProcessResult{item("image", 2), item("base", 0)}, CheckStatusFailed},
		{"nil item", []*CheckProcessResult{item("base", 1), nil}, CheckStatusSuccess},
	}
	for _, tc := range cases {
		if got := aggregateStatus(tc.items); got != tc.want {
			t.Errorf("%s: aggregateStatus = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestConversionApplicability(t *testing.T) {
	cases := []struct {
		name            string
		channelType     int
		model           string
		geminiOpenAI    bool
		claude, rewrite bool
	}{
		{"anthropic", config.ChannelTypeAnthropic, "claude-sonnet-4-5", false, true, true},
		{"claude code", config.ChannelTypeClaudeCode, "claude-opus-4-1", false, true, true},
		{"bedrock alias", config.ChannelTypeBedrock, "claude-3-5-haiku-20241022", false, true, true},
		{"bedrock id", config.ChannelTypeBedrock, "us.anthropic.claude-3-7-sonnet-20250219-v1:0", false, true, true},
		{"bedrock other", config.ChannelTypeBedrock, "amazon.nova-pro-v1:0", false, false, false},
		{"vertex claude", config.ChannelTypeVertexAI, "claude-sonnet-4@20250514", false, true, true},
		{"vertex gemini", config.ChannelTypeVertexAI, "gemini-2.5-pro", false, false, true},
		{"vertex imagen", config.ChannelTypeVertexAI, "imagen-4.0-generate-001", false, false, true},
		{"vertex other", config.ChannelTypeVertexAI, "llama-4", false, false, false},
		{"gemini native", config.ChannelTypeGemini, "gemini-2.5-flash", false, false, true},
		{"gemini openai api", config.ChannelTypeGemini, "gemini-2.5-flash", true, false, false},
		{"vertex express", config.ChannelTypeVertexAIExpress, "gemini-2.5-flash", false, false, true},
		{"gemini cli", config.ChannelTypeGeminiCli, "gemini-2.5-pro", false, false, true},
		{"antigravity", config.ChannelTypeAntigravity, "claude-sonnet-4-5", false, false, true},
		{"openai selling claude", config.ChannelTypeOpenAI, "claude-sonnet-4-5", false, false, false},
		{"openrouter", config.ChannelTypeOpenRouter, "anthropic/claude-sonnet-4.5", false, false, false},
		{"custom", config.ChannelTypeCustom, "gemini-2.5-flash", false, false, false},
		{"codex", config.ChannelTypeCodex, "gpt-5-codex", false, false, false},
	}
	for _, tc := range cases {
		if got := usesClaudeConversion(tc.channelType, tc.model); got != tc.claude {
			t.Errorf("%s: usesClaudeConversion = %v, want %v", tc.name, got, tc.claude)
		}
		if got := rewritesUnknownRole(tc.channelType, tc.model, tc.geminiOpenAI); got != tc.rewrite {
			t.Errorf("%s: rewritesUnknownRole = %v, want %v", tc.name, got, tc.rewrite)
		}
	}
}

func TestUsesGeminiOpenAIAPI(t *testing.T) {
	if !usesGeminiOpenAIAPI(&gemini.GeminiProvider{UseOpenaiAPI: true}) {
		t.Error("gemini provider with use_openai_api must report true")
	}
	if usesGeminiOpenAIAPI(&gemini.GeminiProvider{}) {
		t.Error("gemini provider without use_openai_api must report false")
	}
	if usesGeminiOpenAIAPI(&vertexai_express.VertexAIExpressProvider{}) {
		t.Error("vertex ai express never uses the OpenAI endpoint")
	}
	if usesGeminiOpenAIAPI(nil) {
		t.Error("other providers must report false")
	}
}

func TestClassifyError(t *testing.T) {
	cases := []struct {
		status       int
		typ, message string
		want         string
	}{
		{401, "", "whatever", "auth"},
		{400, "invalid_request_error", "Incorrect API key provided", "auth"},
		{402, "", "insufficient_quota: You exceeded your current quota", "quota"},
		{200, "", "当前余额不足", "quota"},
		{400, "", "Your request was rejected by the safety system", "content_policy"},
		{404, "", "The model `gpt-9` does not exist", "model_unavailable"},
		{400, "", "This model's maximum context length is 8192 tokens", "context_length"},
		{429, "", "slow down", "rate_limit"},
		{529, "overloaded_error", "Overloaded", "rate_limit"},
		{504, "", "upstream", "timeout"},
		{0, "", "dial tcp: lookup api.example.com: no such host", "network"},
		{403, "", "forbidden", "forbidden"},
		{404, "", "page", "not_found"},
		{400, "", "bad", "bad_request"},
		{0, "", "unsupported parameter: tools", "bad_request"},
		{502, "", "bad gateway", "server_error"},
		{0, "", "something odd", ""},
	}
	for _, tc := range cases {
		if got := classifyError(tc.status, tc.typ, tc.message); got != tc.want {
			t.Errorf("classifyError(%d, %q, %q) = %q, want %q", tc.status, tc.typ, tc.message, got, tc.want)
		}
	}
}

func chatResponse(model, id, content string, usage *types.Usage) *types.ChatCompletionResponse {
	return &types.ChatCompletionResponse{
		ID:      id,
		Model:   model,
		Usage:   usage,
		Choices: []types.ChatCompletionChoice{{Message: types.ChatCompletionMessage{Role: "assistant", Content: content}}},
	}
}

func resultByName(results []*CheckResult, name string) *CheckResult {
	for _, r := range results {
		if r.Name == name {
			return r
		}
	}
	return nil
}

func TestBaseCheck(t *testing.T) {
	p := CreateCheckBaseProcess("gpt-4o")
	req := p.GetRequest()
	usage := &types.Usage{PromptTokens: 40, CompletionTokens: 1}

	results := p.Check(CheckContext{}, req, chatResponse("gpt-4o-2024-08-06", "chatcmpl-1", p.challenge.Expected, usage), nil)
	for _, name := range []string{resultChallenge, resultUsage, resultResponseModel, resultResponseID} {
		if r := resultByName(results, name); r == nil || r.Status != CheckStatusSuccess {
			t.Fatalf("%s = %+v, want success", name, r)
		}
	}

	results = p.Check(CheckContext{}, req, chatResponse("gpt-4o-mini", "abc", "nope", nil), nil)
	want := map[string]struct {
		status int
		code   string
	}{
		resultChallenge:     {CheckStatusFailed, "challenge_mismatch"},
		resultUsage:         {CheckStatusWarning, "usage_missing"},
		resultResponseModel: {CheckStatusWarning, "response_model_mismatch"},
		resultResponseID:    {CheckStatusWarning, "response_id_unofficial"},
	}
	for name, w := range want {
		r := resultByName(results, name)
		if r == nil || r.Status != w.status || r.Code != w.code {
			t.Fatalf("%s = %+v, want %d %s", name, r, w.status, w.code)
		}
	}
	if itemStatus(results) != CheckStatusFailed {
		t.Fatal("a wrong challenge answer must fail the base check")
	}

	failed := p.Check(CheckContext{}, req, nil, &types.OpenAIError{Message: "bad key for https://api.example.com/v1"})
	if len(failed) != 1 || failed[0].Code != "upstream_error" || failed[0].Params["message"] == "bad key for https://api.example.com/v1" {
		t.Fatalf("upstream errors must be reported masked, got %+v", failed)
	}
}

func TestJSONCheckClaudeConversion(t *testing.T) {
	p := CreateCheckJsonFormatProcess("claude-sonnet-4-5")
	valid := `{"steps":[{"explanation":"subtract 7","output":"8x = -30"}],"final_answer":"-3.75"}`
	cases := []struct {
		name    string
		ctx     CheckContext
		content string
		status  int
		code    string
	}{
		{"native", CheckContext{}, valid, CheckStatusSuccess, "json_passed"},
		{"fenced", CheckContext{}, "```json\n" + valid + "\n```", CheckStatusWarning, "json_passed_fenced"},
		{"wrong answer", CheckContext{}, `{"steps":[{"explanation":"","output":""}],"final_answer":"-3"}`, CheckStatusWarning, "json_answer_mismatch"},
		{"missing fields", CheckContext{}, `{"answer":"-3.75"}`, CheckStatusFailed, "json_missing_fields"},
		{"not json", CheckContext{}, "x = -3.75", CheckStatusFailed, "json_invalid"},
		{"claude prose", CheckContext{ClaudeConversion: true}, "x = -3.75", CheckStatusSkipped, "json_skipped_claude"},
		{"claude json", CheckContext{ClaudeConversion: true}, valid, CheckStatusWarning, "json_passed_unconstrained"},
	}
	for _, tc := range cases {
		r := p.Check(tc.ctx, p.GetRequest(), chatResponse("m", "id", tc.content, nil), nil)
		if len(r) != 1 || r[0].Status != tc.status || r[0].Code != tc.code {
			t.Errorf("%s: got %+v, want %d %s", tc.name, r[0], tc.status, tc.code)
		}
	}
}

func TestRelayCheck(t *testing.T) {
	p := CreateCheckErrorProcess("m")
	cases := []struct {
		name   string
		ctx    CheckContext
		err    *types.OpenAIError
		status int
		code   string
	}{
		{"role rewritten", CheckContext{RewritesUnknownRole: true}, nil, CheckStatusSkipped, "request_id_role_rewritten"},
		{"no error", CheckContext{}, nil, CheckStatusWarning, "request_id_no_error"},
		{"direct", CheckContext{}, &types.OpenAIError{Message: "invalid role user11"}, CheckStatusSuccess, "request_id_none"},
		{"two relays", CheckContext{}, &types.OpenAIError{Message: "invalid role (request id: a) (Request ID: b)"}, CheckStatusWarning, "request_id_found"},
	}
	for _, tc := range cases {
		r := p.Check(tc.ctx, p.GetRequest(), nil, tc.err)
		if len(r) != 1 || r[0].Status != tc.status || r[0].Code != tc.code {
			t.Errorf("%s: got %+v, want %d %s", tc.name, r[0], tc.status, tc.code)
		}
	}
	r := p.Check(CheckContext{}, p.GetRequest(), nil, &types.OpenAIError{Message: "(request id: a) (Request ID: b)"})
	if r[0].Params["count"] != 2 {
		t.Fatalf("count = %v, want 2", r[0].Params["count"])
	}
}

func TestRenderDigitsPNG(t *testing.T) {
	p := CreateCheckImgProcess("gpt-4o")
	if p.renderErr != nil {
		t.Fatal(p.renderErr)
	}
	if len(p.expected) != 6 || !regexp.MustCompile(`^data:image/png;base64,[A-Za-z0-9+/=]+$`).MatchString(p.imageURL) {
		t.Fatalf("unexpected image probe: %q %q", p.expected, p.imageURL[:40])
	}
	r := p.Check(CheckContext{}, p.GetRequest(), chatResponse("m", "id", p.expected, nil), nil)
	if r[0].Status != CheckStatusSuccess {
		t.Fatalf("reading the digits back must pass, got %+v", r[0])
	}
}

type fakeProcess struct{ key string }

func (f fakeProcess) GetKey() string                           { return f.key }
func (f fakeProcess) GetRequest() *types.ChatCompletionRequest { return &types.ChatCompletionRequest{} }
func (f fakeProcess) Check(CheckContext, *types.ChatCompletionRequest, *types.ChatCompletionResponse, *types.OpenAIError) []*CheckResult {
	return nil
}

// The base check must finish before any other check starts, and the others run in parallel.
func TestRunChecksRunsBaseFirst(t *testing.T) {
	processes := []CheckProcess{fakeProcess{"base"}, fakeProcess{"image"}, fakeProcess{"json"}, fakeProcess{"tool"}, fakeProcess{"relay"}}
	var baseDone atomic.Bool
	var inFlight, maxInFlight atomic.Int32
	var mu sync.Mutex
	startedEarly := []string{}

	result := runChecks("m", processes, func(p CheckProcess) *CheckProcessResult {
		if p.GetKey() == "base" {
			time.Sleep(20 * time.Millisecond)
			baseDone.Store(true)
			return &CheckProcessResult{Key: "base", Status: CheckStatusSuccess}
		}
		if !baseDone.Load() {
			mu.Lock()
			startedEarly = append(startedEarly, p.GetKey())
			mu.Unlock()
		}
		n := inFlight.Add(1)
		for {
			m := maxInFlight.Load()
			if n <= m || maxInFlight.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		inFlight.Add(-1)
		return &CheckProcessResult{Key: p.GetKey(), Status: CheckStatusWarning}
	})

	if len(startedEarly) > 0 {
		t.Fatalf("checks started before the base check finished: %v", startedEarly)
	}
	if maxInFlight.Load() < 2 {
		t.Fatalf("the other checks did not run in parallel (max in flight %d)", maxInFlight.Load())
	}
	for i, p := range processes {
		if result.Items[i] == nil || result.Items[i].Key != p.GetKey() {
			t.Fatalf("item %d = %+v, want key %s", i, result.Items[i], p.GetKey())
		}
	}
	if result.Status != CheckStatusWarning || result.Model != "m" {
		t.Fatalf("result = %+v", result)
	}
}

func TestGetProcessStartsWithBase(t *testing.T) {
	ps := getProcess("m")
	if ps[0].GetKey() != baseProbeKey {
		t.Fatalf("first process = %s, want base", ps[0].GetKey())
	}
	keys := map[string]bool{}
	for _, p := range ps {
		keys[p.GetKey()] = true
	}
	for _, k := range []string{"base", "image", "json", "tool", "relay"} {
		if !keys[k] {
			t.Fatalf("missing check %s", k)
		}
	}
}

type panicProcess struct{ fakeProcess }

func (panicProcess) GetRequest() *types.ChatCompletionRequest { panic("boom") }

func TestRunProcessRecoversPanic(t *testing.T) {
	baseURL := "http://127.0.0.1:1"
	channel := &model.Channel{Type: config.ChannelTypeOpenAI, Key: "sk-test", BaseURL: &baseURL}
	item := runProcess(channel, "gpt-4o", panicProcess{fakeProcess{"image"}})
	if item.Key != "image" || item.Status != CheckStatusFailed || item.Results[0].Code != "check_panicked" {
		t.Fatalf("item = %+v", item)
	}
}

func TestRunProcessReportsProviderError(t *testing.T) {
	// No provider factory and no base URL: GetProvider returns nil.
	channel := &model.Channel{Type: 9999}
	item := runProcess(channel, "gpt-4o", CreateCheckBaseProcess("gpt-4o"))
	if item.Status != CheckStatusFailed || item.Results[0].Code != "provider_error" || item.Results[0].Params["message"] == "" {
		t.Fatalf("item = %+v", item.Results[0])
	}
}

// Every code, result name, probe key and error category the checks can return must have an
// English translation in the web UI; the other languages are kept in step by the locale tests.
func TestEveryKeyIsTranslated(t *testing.T) {
	raw, err := os.ReadFile("../../web/src/i18n/locales/modeltaps/en_US.json")
	if err != nil {
		t.Fatal(err)
	}
	var locale struct {
		ChannelCheck struct {
			Code     map[string]string `json:"code"`
			Result   map[string]string `json:"result"`
			Probe    map[string]string `json:"probe"`
			Category map[string]string `json:"category"`
		} `json:"channelCheck"`
	}
	if err := json.Unmarshal(raw, &locale); err != nil {
		t.Fatal(err)
	}

	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	codePattern := regexp.MustCompile(`Code:\s+"([a-z_]+)"`)
	resultPattern := regexp.MustCompile(`result\w+\s+=\s+"([a-z_]+)"`)
	categoryPattern := regexp.MustCompile(`return "([a-z_]+)"`)
	found := 0
	for _, file := range sources {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range codePattern.FindAllStringSubmatch(string(src), -1) {
			found++
			if locale.ChannelCheck.Code[m[1]] == "" {
				t.Errorf("code %q (%s) has no translation", m[1], file)
			}
		}
		for _, m := range resultPattern.FindAllStringSubmatch(string(src), -1) {
			if locale.ChannelCheck.Result[m[1]] == "" {
				t.Errorf("result name %q has no translation", m[1])
			}
		}
		if file == "error_category.go" {
			for _, m := range categoryPattern.FindAllStringSubmatch(string(src), -1) {
				if locale.ChannelCheck.Category[m[1]] == "" {
					t.Errorf("category %q has no translation", m[1])
				}
			}
		}
	}
	if found < 30 {
		t.Fatalf("found only %d codes; the pattern no longer matches the sources", found)
	}
	for _, p := range getProcess("m") {
		if locale.ChannelCheck.Probe[p.GetKey()] == "" {
			t.Errorf("probe %q has no translation", p.GetKey())
		}
	}
}
