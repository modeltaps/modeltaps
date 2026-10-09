package base

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
)

func newUnifiedModelContext(originalModel string, channelEnabled bool) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	if originalModel != "" {
		c.Set("original_model", originalModel)
	}
	c.Set("channel_unified_request_response_model", channelEnabled)
	return c
}

func TestGetResponseModelNameFromContext_Sources(t *testing.T) {
	orig := config.UnifiedRequestResponseModelEnabled
	t.Cleanup(func() { config.UnifiedRequestResponseModelEnabled = orig })

	cases := []struct {
		name           string
		global         bool
		channelEnabled bool
		originalModel  string
		want           string
	}{
		{"both off keeps upstream name", false, false, "gpt-4o", "gpt-4o-2024-08-06"},
		{"global on replaces", true, false, "gpt-4o", "gpt-4o"},
		{"channel on replaces", false, true, "gpt-4o", "gpt-4o"},
		{"both on replaces", true, true, "gpt-4o", "gpt-4o"},
		{"channel on without original model keeps upstream name", false, true, "", "gpt-4o-2024-08-06"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config.UnifiedRequestResponseModelEnabled = tc.global
			c := newUnifiedModelContext(tc.originalModel, tc.channelEnabled)
			if got := GetResponseModelNameFromContext(c, "gpt-4o-2024-08-06"); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGetResponseModelNameFromContext_NilContext(t *testing.T) {
	orig := config.UnifiedRequestResponseModelEnabled
	t.Cleanup(func() { config.UnifiedRequestResponseModelEnabled = orig })
	config.UnifiedRequestResponseModelEnabled = true

	if got := GetResponseModelNameFromContext(nil, "upstream"); got != "upstream" {
		t.Fatalf("got %q, want %q", got, "upstream")
	}
}

func TestUnifyModelInJSONBytes_ChannelLevel(t *testing.T) {
	orig := config.UnifiedRequestResponseModelEnabled
	t.Cleanup(func() { config.UnifiedRequestResponseModelEnabled = orig })
	config.UnifiedRequestResponseModelEnabled = false

	raw := []byte(`{"type":"message_start","message":{"id":"x","model":"claude-upstream"}}`)

	off := newUnifiedModelContext("claude-requested", false)
	if out, changed := UnifyModelInJSONBytes(off, raw, "message.model"); changed || string(out) != string(raw) {
		t.Fatalf("expected no change when both switches are off, got changed=%v out=%s", changed, out)
	}

	on := newUnifiedModelContext("claude-requested", true)
	out, changed := UnifyModelInJSONBytes(on, raw, "message.model")
	want := `{"type":"message_start","message":{"id":"x","model":"claude-requested"}}`
	if !changed || string(out) != want {
		t.Fatalf("expected channel switch to replace model, got changed=%v out=%s", changed, out)
	}
}
