package relay_util

import (
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestNormalizeFinishReason(t *testing.T) {
	cases := map[string]string{
		// OpenAI 兼容常见值
		"stop":           "stop",
		"STOP":           "stop",
		"  stop  ":       "stop",
		"length":         "length",
		"MAX_TOKENS":     "length",
		"content_filter": "content_filter",
		"tool_calls":     "tool_calls",
		"function_call":  "tool_calls",
		"error":          "error",
		// 别名兜底(Anthropic/Gemini 常见)
		"end_turn":      "stop",
		"stop_sequence": "stop",
		"tool_use":      "tool_calls",
		"safety":        "content_filter",
		"recitation":    "content_filter",
		// 未知值保守归一为 error
		"something_unknown": "error",
		"":                  "error",
	}

	for in, want := range cases {
		if got := normalizeFinishReason(in); got != want {
			t.Errorf("normalizeFinishReason(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetLogMetaFinishReason(t *testing.T) {
	q := &Quota{}

	// 有 finish_reason：native 存原文，finish_reason 存归一值
	meta := q.GetLogMeta(&types.Usage{FinishReason: "end_turn"})
	if got := meta["native_finish_reason"]; got != "end_turn" {
		t.Errorf("native_finish_reason = %v, want end_turn", got)
	}
	if got := meta["finish_reason"]; got != "stop" {
		t.Errorf("finish_reason = %v, want stop", got)
	}

	// 未知值：native 原样，归一为 error
	meta = q.GetLogMeta(&types.Usage{FinishReason: "weird_reason"})
	if got := meta["native_finish_reason"]; got != "weird_reason" {
		t.Errorf("native_finish_reason = %v, want weird_reason", got)
	}
	if got := meta["finish_reason"]; got != "error" {
		t.Errorf("finish_reason = %v, want error", got)
	}

	// 无法取得(空串)：两个 key 都不写，不得写空串
	meta = q.GetLogMeta(&types.Usage{})
	if _, ok := meta["finish_reason"]; ok {
		t.Error("finish_reason should be absent when FinishReason is empty")
	}
	if _, ok := meta["native_finish_reason"]; ok {
		t.Error("native_finish_reason should be absent when FinishReason is empty")
	}
}

func TestSanitizeHeaderValue(t *testing.T) {
	if got := sanitizeHeaderValue("  DemoApp  ", 256); got != "DemoApp" {
		t.Errorf("trim failed: got %q", got)
	}
	// 控制字符被剔除(含 CR/LF/Tab,防止日志注入)
	if got := sanitizeHeaderValue("Demo\r\n\tApp", 256); got != "DemoApp" {
		t.Errorf("control chars not stripped: got %q", got)
	}
	// 按 rune 截断
	if got := sanitizeHeaderValue("abcdef", 3); got != "abc" {
		t.Errorf("truncate failed: got %q", got)
	}
	// 空/纯空白 -> 空串
	if got := sanitizeHeaderValue("   ", 256); got != "" {
		t.Errorf("blank should be empty: got %q", got)
	}
}

func TestExtractRefererDomain(t *testing.T) {
	cases := map[string]string{
		"https://demo.app":          "demo.app",
		"https://demo.app/chat?x=1": "demo.app",
		"http://demo.app:8080/path": "demo.app",
		"demo.app/path":             "demo.app",
		"demo.app":                  "demo.app",
		"":                          "",
		"   ":                       "",
		"https://sub.demo.app/x":    "sub.demo.app",
	}
	for in, want := range cases {
		if got := extractRefererDomain(in); got != want {
			t.Errorf("extractRefererDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetLogMetaAppAttribution(t *testing.T) {
	// X-Title 优先作为 app_name,Referer 域名作为 app_domain,User-Agent 截断写入
	q := &Quota{appName: "DemoApp", appDomain: "demo.app", userAgent: "curl/8.0"}
	meta := q.GetLogMeta(&types.Usage{})
	if meta["app_name"] != "DemoApp" {
		t.Errorf("app_name = %v, want DemoApp", meta["app_name"])
	}
	if meta["app_domain"] != "demo.app" {
		t.Errorf("app_domain = %v, want demo.app", meta["app_domain"])
	}
	if meta["user_agent"] != "curl/8.0" {
		t.Errorf("user_agent = %v, want curl/8.0", meta["user_agent"])
	}

	// 三者皆缺省:不写任何 App 归因字段
	q = &Quota{}
	meta = q.GetLogMeta(&types.Usage{})
	for _, key := range []string{"app_name", "app_domain", "user_agent"} {
		if _, ok := meta[key]; ok {
			t.Errorf("%s should be absent when app attribution is empty", key)
		}
	}
}

func TestRelayModeFromPath(t *testing.T) {
	cases := map[string]string{
		"/v1/chat/completions":             "chat_completions",
		"/v1/completions":                  "completions",
		"/v1/embeddings":                   "embeddings",
		"/v1/moderations":                  "moderations",
		"/v1/images/generations":           "image_generations",
		"/v1/images/edits":                 "image_edits",
		"/v1/images/variations":            "image_variations",
		"/v1/audio/speech":                 "audio_speech",
		"/v1/audio/transcriptions":         "audio_transcription",
		"/v1/audio/translations":           "audio_translation",
		"/v1/rerank":                       "rerank",
		"/v1/realtime":                     "realtime",
		"/v1/responses":                    "responses",
		"/v1/responses/compact":            "responses",
		"/claude/v1/messages":              "chat_completions",
		"/recraftAI/v1/images/generations": "image_generations",
		"/gemini/v1beta/models/gemini-pro:generateContent": "chat_completions",
		"/gemini/v1beta/models/imagen:predict":             "image_generations",
		"/gemini/v1beta/models/veo:predictLongRunning":     "video",
		// 无法判定的路径不写 metadata(空串)
		"/v1/files": "",
		"/anything": "",
		"":          "",
	}
	for in, want := range cases {
		if got := relayModeFromPath(in); got != want {
			t.Errorf("relayModeFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetLogMetaRelayMode(t *testing.T) {
	q := &Quota{relayMode: "image_generations"}
	if got := q.GetLogMeta(&types.Usage{})["relay_mode"]; got != "image_generations" {
		t.Errorf("relay_mode = %v, want image_generations", got)
	}

	// 判不出模态时不写该键(旧日志同形态,前端显示 "-")
	q = &Quota{}
	if _, ok := q.GetLogMeta(&types.Usage{})["relay_mode"]; ok {
		t.Error("relay_mode should be absent when unresolved")
	}
}
