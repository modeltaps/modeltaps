package e2e

import (
	"net/http"
	"testing"
)

// TestResponseModelMismatchLogged: an upstream answering with another model than requested
// is flagged in the consume log; a dated variant of the same model is not.
func TestResponseModelMismatchLogged(t *testing.T) {
	cases := []struct {
		returned string
		mismatch bool
	}{
		{"gpt-4o-mini-2024-07-18", false},
		{"cheap-model-7b", true},
	}
	for _, tc := range cases {
		fake := NewFakeProvider(t)
		fake.SetResponse(FakeResponse{JSON: ChatCompletionJSON(tc.returned, "ok", 5, 2)})
		h := NewHarness(t, HarnessOptions{FakeBaseURL: fake.Server.URL})

		if w := h.Post("/v1/chat/completions", `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`); w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		logs := consumeLogs(t)
		if len(logs) != 1 {
			t.Fatalf("consume logs = %d", len(logs))
		}
		meta := logs[0].Metadata.Data()
		got, _ := meta["response_model_mismatch"].(bool)
		if got != tc.mismatch {
			t.Errorf("returned %q: response_model_mismatch=%v, want %v (meta %v)", tc.returned, got, tc.mismatch, meta)
		}
		if tc.mismatch && meta["response_model"] != tc.returned {
			t.Errorf("response_model = %v, want %q", meta["response_model"], tc.returned)
		}
	}
}
