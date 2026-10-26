package gemini

import (
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"strings"
	"testing"
)

// The native non-stream generateContent relay decodes the upstream response into GeminiChatResponse
// and re-encodes it, so a missing struct field drops it. Long Decode needs
// candidates[].continuationToken to survive the round trip.
func TestGeminiChatResponse_ContinuationTokenRoundTrip(t *testing.T) {
	upstream := `{
		"candidates": [{
			"content": {"role": "model", "parts": [{"text": "partial"}]},
			"finishReason": "CONTINUATION",
			"index": 0,
			"continuationToken": "CiQ4ZmQ3-token"
		}],
		"usageMetadata": {"promptTokenCount": 10, "candidatesTokenCount": 32768, "totalTokenCount": 32778}
	}`

	var resp GeminiChatResponse
	if err := json.Unmarshal([]byte(upstream), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := resp.Candidates[0].ContinuationToken; got != "CiQ4ZmQ3-token" {
		t.Fatalf("ContinuationToken = %q, want %q", got, "CiQ4ZmQ3-token")
	}

	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `"continuationToken":"CiQ4ZmQ3-token"`) {
		t.Fatalf("continuationToken dropped after round trip: %s", out)
	}
	if !strings.Contains(string(out), `"finishReason":"CONTINUATION"`) {
		t.Fatalf("finishReason dropped after round trip: %s", out)
	}
}

// Without a continuation no empty continuationToken field is added.
func TestGeminiChatResponse_ContinuationTokenOmittedWhenEmpty(t *testing.T) {
	resp := GeminiChatResponse{Candidates: []GeminiChatCandidate{{Content: GeminiChatContent{Role: "model"}}}}
	out, err := json.Marshal(&resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "continuationToken") {
		t.Fatalf("empty continuationToken should be omitted: %s", out)
	}
}

func TestConvertFinishReason_Continuation(t *testing.T) {
	if got := ConvertFinishReason("CONTINUATION"); got != types.FinishReasonLength {
		t.Fatalf("CONTINUATION -> %q, want %q", got, types.FinishReasonLength)
	}
	if got := ConvertFinishReason("SAFETY"); got != types.FinishReasonContentFilter {
		t.Fatalf("SAFETY -> %q, want %q", got, types.FinishReasonContentFilter)
	}
}
