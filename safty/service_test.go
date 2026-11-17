package safty

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/safty/providers/keyword"
)

func mustJSON(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestExtractTextContentParts(t *testing.T) {
	cases := []struct {
		name string
		data any
		want string
	}{
		{"plain string", "hello", "hello"},
		{"nil", nil, ""},
		{
			name: "array parts use the text key, every part",
			data: mustJSON(t, `[{"type":"text","text":"first"},{"type":"text","text":"second"}]`),
			want: "first\nsecond",
		},
		{
			name: "image parts are skipped",
			data: mustJSON(t, `[{"type":"image_url","image_url":{"url":"https://x/forbidden.png"}},
				{"type":"image","source":{"type":"base64","media_type":"image/png","data":"Zm9yYmlkZGVu"}},
				{"type":"input_image","image_url":"data:image/png;base64,Zm9yYmlkZGVu"},
				{"type":"input_audio","input_audio":{"data":"Zm9yYmlkZGVu","format":"wav"}},
				{"type":"text","text":"caption"}]`),
			want: "caption",
		},
		{
			name: "roles, models and names are not text",
			data: mustJSON(t, `{"model":"forbidden-model","messages":[{"role":"user","name":"forbidden","content":"hi"}]}`),
			want: "hi",
		},
		{
			name: "nested tools: descriptions and parameter descriptions",
			data: mustJSON(t, `[{"type":"function","function":{"name":"lookup","description":"look things up",
				"parameters":{"type":"object","properties":{"q":{"type":"string","description":"the query","enum":["a"]}},"required":["q"]}}}]`),
			want: "look things up\nthe query",
		},
		{
			name: "Responses input items and instructions",
			data: []any{
				mustJSON(t, `[{"type":"message","role":"user","content":[{"type":"input_text","text":"question"}]},
					{"type":"function_call","name":"f","arguments":"{\"a\":1}"},
					{"type":"function_call_output","call_id":"c","output":"tool result"}]`),
				"be brief",
			},
			want: "question\n{\"a\":1}\ntool result\nbe brief",
		},
		{
			name: "string arrays under text keys",
			data: mustJSON(t, `{"query":"q","documents":["d1",{"text":"d2"}],"stop":["not text"]}`),
			want: "d1\nd2\nq",
		},
		{
			name: "raw JSON",
			data: json.RawMessage(`{"system":[{"type":"text","text":"sys"}]}`),
			want: "sys",
		},
		{
			name: "Gemini function and code parts: every string, media still skipped",
			data: json.RawMessage(`[{"functionCall":{"name":"f","args":{"q":"call arg"}}},
				{"functionResponse":{"name":"f","response":{"n":3,"out":{"summary":"tool said"},
					"parts":[{"inlineData":{"mimeType":"image/png","data":"Zm9yYmlkZGVu"}}]}}},
				{"executableCode":{"language":"PYTHON","code":"print(1)"}},
				{"codeExecutionResult":{"outcome":"OUTCOME_OK","output":"1"}}]`),
			want: "call arg\nf\nf\ntool said\nprint(1)\nPYTHON\nOUTCOME_OK\n1",
		},
		{
			name: "duplicate keys in raw JSON are all read",
			data: json.RawMessage(`{"messages":[{"role":"user","content":"first","content":"second"}]}`),
			want: "first\nsecond",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractText(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("extractText = %q, want %q", got, tc.want)
			}
		})
	}
}

// Structs are read through their JSON form, not fmt's %v rendering.
func TestExtractTextStruct(t *testing.T) {
	type part struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	}
	type message struct {
		Role    string `json:"role"`
		Content []part `json:"content"`
	}
	type request struct {
		Model  string    `json:"model"`
		System string    `json:"system"`
		Msgs   []message `json:"messages"`
	}
	got, err := extractText(request{
		Model:  "m",
		System: "sys",
		Msgs:   []message{{Role: "user", Content: []part{{Type: "text", Text: "a"}, {Type: "text", Text: "b"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != "a\nb\nsys" {
		t.Fatalf("extractText = %q", got)
	}
}

func enableKeywordCheck(t *testing.T, words ...string) {
	t.Helper()
	prevEnable, prevTool, prevWords := config.EnableSafe, config.SafeToolName, config.SafeKeyWords
	prevRegistered, hadRegistered := Tools["Keyword"]
	config.EnableSafe = true
	config.SafeToolName = "Keyword"
	config.SafeKeyWords = words
	Tools["Keyword"] = keyword.NewKeywordChecker()
	t.Cleanup(func() {
		config.EnableSafe, config.SafeToolName, config.SafeKeyWords = prevEnable, prevTool, prevWords
		if hadRegistered {
			Tools["Keyword"] = prevRegistered
		} else {
			delete(Tools, "Keyword")
		}
	})
}

// Text in the second content part was never checked before.
func TestCheckContentSeesEveryPart(t *testing.T) {
	enableKeywordCheck(t, "forbidden")

	content := mustJSON(t, `[{"type":"text","text":"hello"},{"type":"text","text":"something forbidden"}]`)
	result, err := CheckContent(content)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsSafe {
		t.Fatal("content with the keyword in a later part must be refused")
	}

	result, err = CheckContentByToolName("Keyword", content)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsSafe {
		t.Fatal("CheckContentByToolName must refuse it too")
	}

	result, _ = CheckContent(mustJSON(t, `[{"type":"image_url","image_url":{"url":"https://x/forbidden.png"}}]`))
	if !result.IsSafe {
		t.Fatal("content without text is safe")
	}
}

func TestCheckContentKeywordNotInNonText(t *testing.T) {
	enableKeywordCheck(t, "user")
	result, _ := CheckContent(mustJSON(t, `[{"role":"user","content":"hello"}]`))
	if !result.IsSafe {
		t.Fatalf("a role value must not match a keyword: %+v", result)
	}
	result, _ = CheckContent(mustJSON(t, `[{"role":"user","content":"USER says"}]`))
	if result.IsSafe {
		t.Fatal("the keyword in content must match")
	}
}
