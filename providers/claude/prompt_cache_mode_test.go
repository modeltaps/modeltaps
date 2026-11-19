package claude

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/common/config"
)

func modeContext(mode string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(config.GinPromptCacheModeKey, mode)
	return c
}

// The request's mode (API key or header) beats the channel and the site-wide switch; the model
// rule still applies, so "on" never marks a model that is not Claude.
func TestRequestPromptCacheModeOverridesChannel(t *testing.T) {
	withGlobalCaching(t, false)
	offChannel := channelWithPromptCaching(map[string]interface{}{"enabled": "off", "ttl": "1h"})
	got := GetPromptCachingConfig(modeContext("on"), offChannel, "claude-sonnet-4-5")
	if got == nil || got.TTL != "1h" {
		t.Fatalf("on: cfg = %v, want injection with the channel's ttl", got)
	}
	if got := GetPromptCachingConfig(modeContext("on"), offChannel, "gpt-5"); got != nil {
		t.Fatalf("on for a non-Claude model: cfg = %v, want none", got)
	}

	withGlobalCaching(t, true)
	onChannel := channelWithPromptCaching(map[string]interface{}{"enabled": "on"})
	if got := GetPromptCachingConfig(modeContext("off"), onChannel, "claude-sonnet-4-5"); got != nil {
		t.Fatalf("off: cfg = %v, want none", got)
	}
	if got := GetPromptCachingConfig(modeContext("auto"), onChannel, "claude-sonnet-4-5"); got == nil {
		t.Fatal("auto: want the channel's injection")
	}
	if got := RequestPromptCacheMode(nil); got != "auto" {
		t.Fatalf("no context: mode %q, want auto", got)
	}
}

// Out of the box (site switch on, channel inherit) a Claude request gets breakpoints, and a key
// set to Off still gets none.
func TestPromptCachingOnByDefault(t *testing.T) {
	if !config.ClaudePromptCachingEnabled {
		t.Fatal("the site-wide prompt cache switch should be on by default")
	}
	if GetPromptCachingConfig(modeContext("auto"), channelWithPromptCaching(nil), "claude-sonnet-4-5") == nil {
		t.Fatal("auto on a default channel: want breakpoints")
	}
	if GetPromptCachingConfig(modeContext("off"), channelWithPromptCaching(nil), "claude-sonnet-4-5") != nil {
		t.Fatal("off: want none")
	}
}
