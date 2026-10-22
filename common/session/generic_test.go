package session

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

func newGenericCtx(tokenID int, body string, headers map[string]string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	for k, v := range headers {
		c.Request.Header.Set(k, v)
	}
	c.Set("token_id", tokenID)
	c.Set(config.GinRequestBodyKey, []byte(body))
	return c
}

func TestGenerateGenericSessionHash(t *testing.T) {
	gin.SetMode(gin.TestMode)
	claudeTurn1 := `{"model":"claude-opus-5-5","metadata":{"user_id":{"device_id":"d","session_id":"0b6a0c1e-1111-4222-8333-944445555666"}},` +
		`"system":"You are Claude Code.","messages":[{"role":"user","content":"first"}]}`
	claudeTurn2 := `{"model":"claude-opus-5-5","metadata":{"user_id":{"device_id":"d","session_id":"0b6a0c1e-1111-4222-8333-944445555666"}},` +
		`"system":"You are Claude Code.","messages":[{"role":"user","content":"first"},{"role":"assistant","content":"ok"},{"role":"user","content":"second"}]}`
	otherSession := `{"model":"claude-opus-5-5","metadata":{"user_id":{"session_id":"99999999-1111-4222-8333-944445555666"}},"messages":[{"role":"user","content":"first"}]}`

	h := func(token int, model, body string, headers map[string]string) string {
		return GenerateGenericSessionHash(newGenericCtx(token, body, headers), model)
	}

	base := h(1, "claude-opus-5-5", claudeTurn1, nil)
	if base == "" {
		t.Fatal("Claude Code session not recognized")
	}
	if h(1, "claude-opus-5-5", claudeTurn2, nil) != base {
		t.Error("later turns of the same session must share the key")
	}
	if h(1, "claude-opus-5-5", otherSession, nil) == base {
		t.Error("different sessions must not share the key")
	}
	if h(2, "claude-opus-5-5", claudeTurn1, nil) == base {
		t.Error("the same session id under another API key must not share the key")
	}
	if h(1, "claude-haiku-5-5", claudeTurn1, nil) == base {
		t.Error("another model must not share the key")
	}

	// Codex: session_id header
	codex := `{"model":"gpt-6.1-sol","input":[{"role":"user","content":[{"type":"input_text","text":"x"}]}]}`
	a := h(1, "gpt-6.1-sol", codex, map[string]string{"session_id": "s-1"})
	b := h(1, "gpt-6.1-sol", `{"model":"gpt-6.1-sol","input":"different"}`, map[string]string{"session_id": "s-1"})
	if a == "" || a != b {
		t.Error("the session_id header should identify the session regardless of the body")
	}

	// OpenAI prompt_cache_key
	k1 := h(1, "gpt-6.1-sol", `{"model":"gpt-6.1-sol","prompt_cache_key":"k","messages":[{"role":"user","content":"a"}]}`, nil)
	k2 := h(1, "gpt-6.1-sol", `{"model":"gpt-6.1-sol","prompt_cache_key":"k","messages":[{"role":"user","content":"b"}]}`, nil)
	if k1 == "" || k1 != k2 {
		t.Error("prompt_cache_key should identify the session")
	}

	// Prompt prefix: same system prompt and first user turn, later turns differ.
	p1 := h(1, "gpt-6.1-sol", `{"messages":[{"role":"system","content":"S"},{"role":"user","content":"Q"}]}`, nil)
	p2 := h(1, "gpt-6.1-sol", `{"messages":[{"role":"system","content":"S"},{"role":"user","content":"Q"},{"role":"assistant","content":"A"},{"role":"user","content":"Q2"}]}`, nil)
	p3 := h(1, "gpt-6.1-sol", `{"messages":[{"role":"system","content":"other"},{"role":"user","content":"Q"}]}`, nil)
	if p1 == "" || p1 != p2 || p1 == p3 {
		t.Errorf("prompt prefix keys: p1=%q p2=%q p3=%q", p1, p2, p3)
	}

	if h(1, "m", ``, nil) != "" {
		t.Error("an empty request has no session")
	}
}
