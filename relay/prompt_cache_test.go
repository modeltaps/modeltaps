package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func promptCacheContext(keyMode, header, body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	if header != "" {
		c.Request.Header.Set(config.PromptCacheHeader, header)
	}
	c.Set("token_setting", &model.TokenSetting{PromptCache: keyMode})
	c.Set(config.GinRequestBodyKey, []byte(body))
	return c
}

func TestPromptCacheModeResolution(t *testing.T) {
	cases := []struct {
		keyMode, header, wantMode, wantSource string
	}{
		{"", "", "auto", ""},
		{"auto", "", "auto", ""},
		{"on", "", "on", "key"},
		{"off", "", "off", "key"},
		{"off", "on", "on", "header"},
		{"on", "AUTO", "auto", "header"},
		{"on", "bogus", "on", "key"},
		{"", "off", "off", "header"},
	}
	for _, tc := range cases {
		c := promptCacheContext(tc.keyMode, tc.header, `{}`)
		applyPromptCacheMode(c)
		if got := c.GetString(config.GinPromptCacheModeKey); got != tc.wantMode {
			t.Errorf("key=%q header=%q: mode %q, want %q", tc.keyMode, tc.header, got, tc.wantMode)
		}
		if got := c.GetString(config.GinPromptCacheSourceKey); got != tc.wantSource {
			t.Errorf("key=%q header=%q: source %q, want %q", tc.keyMode, tc.header, got, tc.wantSource)
		}
	}
}

func TestPromptCacheOffStripsMarkers(t *testing.T) {
	body := `{"model":"claude-sonnet-4-5","cache_control":{"type":"ephemeral"},
"tools":[{"name":"t","input_schema":{"type":"object","properties":{"cache_control":{"type":"string"}}},"cache_control":{"type":"ephemeral"}}],
"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral","ttl":"1h"}}],
"messages":[{"role":"user","content":[{"type":"text","text":"hi","cache_control":{"type":"ephemeral"}},
{"type":"tool_result","tool_use_id":"x","content":[{"type":"text","text":"r","cache_control":{"type":"ephemeral"}}]}]}]}`
	c := promptCacheContext("off", "", body)
	applyPromptCacheMode(c)
	out := c.MustGet(config.GinRequestBodyKey).([]byte)
	if !c.GetBool(config.GinPromptCacheStrippedKey) {
		t.Fatal("stripped flag not set")
	}
	for _, path := range []string{"cache_control", "tools.0.cache_control", "system.0.cache_control", "messages.0.content.0.cache_control", "messages.0.content.1.content.0.cache_control"} {
		if gjson.GetBytes(out, path).Exists() {
			t.Errorf("%s still present: %s", path, out)
		}
	}
	// A tool's own schema property named cache_control is data, not a marker.
	if !gjson.GetBytes(out, "tools.0.input_schema.properties.cache_control").Exists() {
		t.Errorf("schema property removed: %s", out)
	}
	if gjson.GetBytes(out, "messages.0.content.0.text").String() != "hi" || gjson.GetBytes(out, "system.0.text").String() != "sys" {
		t.Errorf("content changed: %s", out)
	}
}

func TestPromptCacheOffLeavesNonJSONBodiesAlone(t *testing.T) {
	c := promptCacheContext("off", "", `{"cache_control":{"type":"ephemeral"}}`)
	c.Request.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	applyPromptCacheMode(c)
	if c.GetBool(config.GinPromptCacheStrippedKey) {
		t.Fatal("a multipart body was edited")
	}
}

func TestPromptCacheOffStripsChatAndResponsesMarkers(t *testing.T) {
	chat := `{"model":"anthropic/claude-sonnet-4-5","messages":[{"role":"system","content":[{"type":"text","text":"s","cache_control":{"type":"ephemeral"}}]}]}`
	out, changed := StripCacheControl([]byte(chat))
	if !changed || gjson.GetBytes(out, "messages.0.content.0.cache_control").Exists() {
		t.Errorf("chat markers kept: %s", out)
	}
	responses := `{"model":"m","input":[{"role":"user","content":[{"type":"input_text","text":"x","cache_control":{"type":"ephemeral"}}]}]}`
	out, changed = StripCacheControl([]byte(responses))
	if !changed || gjson.GetBytes(out, "input.0.content.0.cache_control").Exists() {
		t.Errorf("responses markers kept: %s", out)
	}
}

func TestPromptCacheLeavesBodyAloneUnlessOff(t *testing.T) {
	body := `{"system":[{"type":"text","text":"s","cache_control":{"type":"ephemeral"}}]}`
	for _, mode := range []string{"", "on"} {
		c := promptCacheContext(mode, "", body)
		applyPromptCacheMode(c)
		if got := string(c.MustGet(config.GinRequestBodyKey).([]byte)); got != body {
			t.Errorf("mode %q changed the body: %s", mode, got)
		}
	}
	if out, changed := StripCacheControl([]byte(`{"messages":[{"role":"user","content":"hi"}]}`)); changed || string(out) != `{"messages":[{"role":"user","content":"hi"}]}` {
		t.Errorf("unmarked body changed: %s", out)
	}
}
