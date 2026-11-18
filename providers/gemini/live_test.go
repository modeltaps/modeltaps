package gemini

import "testing"

func TestParseLiveUsage(t *testing.T) {
	usage := ParseLiveUsage([]byte(`{"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":10,"responseTokenCount":40,"thoughtsTokenCount":5,"toolUsePromptTokenCount":3,"totalTokenCount":148,
		"promptTokensDetails":[{"modality":"AUDIO","tokenCount":60},{"modality":"TEXT","tokenCount":20},{"modality":"VIDEO","tokenCount":20}],
		"responseTokensDetails":[{"modality":"AUDIO","tokenCount":40}]}}`))
	if usage == nil {
		t.Fatal("expected usage")
	}
	if usage.InputTokens != 103 || usage.OutputTokens != 45 || usage.TotalTokens != 148 {
		t.Fatalf("tokens: %+v", usage)
	}
	if usage.InputTokenDetails.AudioTokens != 60 || usage.InputTokenDetails.TextTokens != 20 || usage.InputTokenDetails.ImageTokens != 20 || usage.InputTokenDetails.CachedTokens != 10 {
		t.Fatalf("input details: %+v", usage.InputTokenDetails)
	}
	if usage.OutputTokenDetails.AudioTokens != 40 || usage.OutputTokenDetails.ReasoningTokens != 5 {
		t.Fatalf("output details: %+v", usage.OutputTokenDetails)
	}
}

// Some Live models keep promptTokenCount small while the details carry the context read on the
// turn; the larger figure is billed.
func TestParseLiveUsageUsesDetailedContext(t *testing.T) {
	usage := ParseLiveUsage([]byte(`{"usageMetadata":{"promptTokenCount":25,"responseTokenCount":10,"promptTokensDetails":[{"modality":"TEXT","tokenCount":900}]}}`))
	if usage == nil || usage.InputTokens != 900 || usage.TotalTokens != 910 {
		t.Fatalf("usage: %+v", usage)
	}
}

func TestParseLiveUsageWithoutMetadata(t *testing.T) {
	for _, message := range []string{`{"setupComplete":{}}`, `{"serverContent":{"turnComplete":true}}`, `{"usageMetadata":{}}`, `x`} {
		if usage := ParseLiveUsage([]byte(message)); usage != nil {
			t.Fatalf("%s: unexpected usage %+v", message, usage)
		}
	}
}

func TestLiveURL(t *testing.T) {
	cases := map[string]string{
		"https://generativelanguage.googleapis.com": "wss://generativelanguage.googleapis.com/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent",
		"http://127.0.0.1:8080/":                    "ws://127.0.0.1:8080/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent",
	}
	for base, want := range cases {
		if got := liveURL(base, "v1beta"); got != want {
			t.Fatalf("%s: got %s", base, got)
		}
	}
}
