package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// A chat request's prompt cache fields survive decoding and encoding, so they reach the upstream.
func TestChatRequestKeepsPromptCacheKey(t *testing.T) {
	var req ChatCompletionRequest
	body := `{"model":"gpt-4.1-mini","messages":[{"role":"user","content":"hi"}],` +
		`"prompt_cache_key":"tenant-1","prompt_cache_retention":"24h"}`
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatal(err)
	}
	out, err := json.Marshal(&req)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"prompt_cache_key":"tenant-1"`, `"prompt_cache_retention":"24h"`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("encoded request %s lacks %s", out, want)
		}
	}

	// Absent fields are not sent, so upstreams that do not know them never see them.
	var plain ChatCompletionRequest
	if err := json.Unmarshal([]byte(`{"model":"m","messages":[]}`), &plain); err != nil {
		t.Fatal(err)
	}
	if out, _ := json.Marshal(&plain); strings.Contains(string(out), "prompt_cache") {
		t.Fatalf("request without cache fields encodes them: %s", out)
	}
}

func TestPromptCacheKeyCarriedAcrossProtocols(t *testing.T) {
	chat := &ChatCompletionRequest{Model: "m", PromptCacheKey: "k", PromptCacheRetention: "24h"}
	res := chat.ToResponsesRequest()
	if res.PromptCacheKey != "k" || res.PromptCacheRetention != "24h" {
		t.Fatalf("chat -> responses lost cache fields: %+v", res)
	}
	back, err := (&OpenAIResponsesRequest{Model: "m", PromptCacheKey: "k2", PromptCacheRetention: "in_memory"}).ToChatCompletionRequest()
	if err != nil {
		t.Fatal(err)
	}
	if back.PromptCacheKey != "k2" || back.PromptCacheRetention != "in_memory" {
		t.Fatalf("responses -> chat lost cache fields: %+v", back)
	}
}
