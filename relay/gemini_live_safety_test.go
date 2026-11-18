package relay

import "testing"

func TestGeminiLiveClientMessageSafety(t *testing.T) {
	enableTestModeration(t)
	refused := []string{
		`{"setup":{"model":"models/m","systemInstruction":{"parts":[{"text":"` + blockedWord + `"}]}}}`,
		`{"setup":{"model":"models/m","system_instruction":"` + blockedWord + `"}}`,
		`{"setup":{"model":"models/m","tools":[{"functionDeclarations":[{"name":"f","description":"` + blockedWord + `"}]}]}}`,
		`{"clientContent":{"turns":[{"role":"user","parts":[{"text":"` + blockedWord + `"}]}],"turnComplete":true}}`,
		`{"client_content":{"turns":[{"role":"user","parts":[{"text":"` + blockedWord + `"}]}]}}`,
		`{"realtimeInput":{"text":"` + blockedWord + `"}}`,
		`{"toolResponse":{"functionResponses":[{"id":"1","name":"f","response":{"output":"` + blockedWord + `"}}]}}`,
	}
	for _, message := range refused {
		assertRefused(t, geminiLiveClientMessageSafety([]byte(message)), true)
	}
	passed := []string{
		`{"realtimeInput":{"audio":{"data":"` + blockedWord + `","mimeType":"audio/pcm;rate=16000"}}}`,
		`{"realtimeInput":{"video":{"data":"` + blockedWord + `","mimeType":"image/jpeg"}}}`,
		`{"setup":{"model":"models/` + blockedWord + `"}}`,
		`{"clientContent":{"turns":[{"role":"user","parts":[{"text":"hello"}]}]}}`,
		`not json`,
	}
	for _, message := range passed {
		if err := geminiLiveClientMessageSafety([]byte(message)); err != nil {
			t.Fatalf("message %s refused: %v", message, err)
		}
	}
}

func TestParseGeminiLiveSetup(t *testing.T) {
	cases := map[string]string{
		`{"setup":{"model":"models/gemini-live"}}`: "gemini-live",
		`{"setup":{"model":"gemini-live"}}`:        "gemini-live",
	}
	for message, want := range cases {
		got, reason := parseGeminiLiveSetup([]byte(message))
		if got != want || reason != "" {
			t.Fatalf("%s: got %q (%s), want %q", message, got, reason, want)
		}
	}
	for _, message := range []string{`{"setup":{}}`, `{"clientContent":{}}`, `{`, `{"setup":"x"}`} {
		if _, reason := parseGeminiLiveSetup([]byte(message)); reason == "" {
			t.Fatalf("%s: expected a refusal", message)
		}
	}
}

func TestTruncateCloseReason(t *testing.T) {
	long := ""
	for len(long) < 200 {
		long += "限额"
	}
	got := truncateCloseReason(long)
	if len(got) > 123 || len(got) == 0 {
		t.Fatalf("length %d", len(got))
	}
	if truncateCloseReason("short") != "short" {
		t.Fatal("short reason changed")
	}
}
