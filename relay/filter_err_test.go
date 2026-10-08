package relay

// FilterOpenAIErr 错误坍缩特征化测试（真值表式）。
// 锁定安全敏感的坍缩行为：status/code/type 映射、429 保留、model-not-found 豁免、
// 脱敏开关、type 标签隐藏、bad_response_status_code 修补、request id 拼接与去重。
// 特征化测试：只记录现状，不修改生产代码。

import (
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/types"
)

// setWrap 临时覆盖坍缩总开关，测试结束自动恢复。
func setWrap(t *testing.T, enabled bool) {
	t.Helper()
	prev := config.ChannelFailErrorWrapEnabled
	config.ChannelFailErrorWrapEnabled = enabled
	t.Cleanup(func() { config.ChannelFailErrorWrapEnabled = prev })
}

// setFailMessage 临时覆盖统一坍缩文案，测试结束自动恢复。
func setFailMessage(t *testing.T, msg string) {
	t.Helper()
	prev := config.ChannelFailErrorMessage
	config.ChannelFailErrorMessage = msg
	t.Cleanup(func() { config.ChannelFailErrorMessage = prev })
}

// oaiErr 构造带 type/code/message 的 *types.OpenAIErrorWithStatusCode。
func oaiErr(status int, local bool, typ, code, msg string) *types.OpenAIErrorWithStatusCode {
	e := &types.OpenAIErrorWithStatusCode{StatusCode: status, LocalError: local}
	e.OpenAIError.Type = typ
	if code != "" {
		e.OpenAIError.Code = code
	}
	e.OpenAIError.Message = msg
	return e
}

func codeStr(e types.OpenAIErrorWithStatusCode) string {
	s, _ := e.OpenAIError.Code.(string)
	return s
}

// TestFilterOpenAIErr_CollapseMatrix 坍缩开关开启时的真值表：
//   - 非 LocalError：仅 400 透传原文，401/403/404/429/5xx 全部坍缩（429 保留 status，
//     401/403 坍缩为 upstream_auth_failed + 认证失败文案）
//   - LocalError：仅 type==upstream_unavailable 坍缩，其余走原路径
//   - upstream_seen_429 ctx flag / 最终 429 两个触发源都把坍缩结果拉回 429
func TestFilterOpenAIErr_CollapseMatrix(t *testing.T) {
	setWrap(t, true)

	tests := []struct {
		name          string
		err           *types.OpenAIErrorWithStatusCode
		seen429       bool
		wantStatus    int
		wantCode      string
		wantType      string
		wantCollapsed bool // true: 文案换为统一坍缩文案且隐藏上游原文
	}{
		{"upstream 400 passes through", oaiErr(400, false, "invalid_request_error", "", "temperature must be a number"), false, 400, "", "invalid_request_error", false},
		{"upstream 401 collapses to 503 auth failed", oaiErr(401, false, "authentication_error", "invalid_api_key", "Incorrect API key provided"), false, 503, "upstream_auth_failed", "system_error", true},
		{"upstream 403 collapses to 503 auth failed", oaiErr(403, false, "permission_error", "", "You are not allowed to use this endpoint"), false, 503, "upstream_auth_failed", "system_error", true},
		{"ctx seen429 flips upstream 401 to 429", oaiErr(401, false, "", "", "Missing Authentication header"), true, 429, "rate_limit_exceeded", "system_error", true},
		{"upstream 404 (non model) collapses to 503", oaiErr(404, false, "", "", "upstream route missing"), false, 503, "service_unavailable", "system_error", true},
		{"upstream 429 collapses but keeps 429", oaiErr(429, false, "", "", "You are being rate limited"), false, 429, "rate_limit_exceeded", "system_error", true},
		{"upstream 500 collapses to 503", oaiErr(500, false, "", "", "internal upstream failure"), false, 503, "service_unavailable", "system_error", true},
		{"upstream 502 collapses to 503", oaiErr(502, false, "", "", "upstream returned bad gateway"), false, 503, "service_unavailable", "system_error", true},
		{"local upstream_unavailable collapses to 503", oaiErr(503, true, "upstream_unavailable", "", "no channels available for group"), false, 503, "service_unavailable", "system_error", true},
		{"local 400 other type passes through", oaiErr(400, true, "invalid_request_error", "", "prompt is required"), false, 400, "", "invalid_request_error", false},
		{"local 402 billing passes through", oaiErr(402, true, "insufficient_quota", "", "user quota is not enough"), false, 402, "", "insufficient_quota", false},
		// SEC-14：LocalError 不再豁免 type 隐藏，modeltaps_error 改写为 system_error（message 仍走原路径透出）。
		{"local 500 modeltaps_error type hidden", oaiErr(500, true, "modeltaps_error", "", "internal local failure"), false, 500, "", "system_error", false},
		{"ctx seen429 flips collapsed 500 to 429", oaiErr(500, false, "", "", "internal upstream failure"), true, 429, "rate_limit_exceeded", "system_error", true},
		{"ctx seen429 flips local upstream_unavailable to 429", oaiErr(503, true, "upstream_unavailable", "", "retry budget exhausted"), true, 429, "rate_limit_exceeded", "system_error", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestContext()
			if tt.seen429 {
				c.Set("upstream_seen_429", true)
			}
			got := FilterOpenAIErr(c, tt.err)
			if got.StatusCode != tt.wantStatus {
				t.Fatalf("status: got %d, want %d", got.StatusCode, tt.wantStatus)
			}
			if codeStr(got) != tt.wantCode {
				t.Fatalf("code: got %v, want %q", got.OpenAIError.Code, tt.wantCode)
			}
			if got.OpenAIError.Type != tt.wantType {
				t.Fatalf("type: got %q, want %q", got.OpenAIError.Type, tt.wantType)
			}
			if tt.wantCollapsed {
				wantMsg := config.GetChannelFailErrorMessage()
				if tt.wantCode == "upstream_auth_failed" {
					wantMsg = config.UpstreamAuthFailedErrorMessage
				}
				if !strings.Contains(got.OpenAIError.Message, wantMsg) {
					t.Fatalf("collapsed message should contain %q: got %q", wantMsg, got.OpenAIError.Message)
				}
				if strings.Contains(got.OpenAIError.Message, tt.err.OpenAIError.Message) {
					t.Fatalf("collapsed message must hide upstream text: got %q", got.OpenAIError.Message)
				}
			} else if !strings.Contains(got.OpenAIError.Message, tt.err.OpenAIError.Message) {
				t.Fatalf("passthrough message should keep original text: got %q", got.OpenAIError.Message)
			}
		})
	}
}

// TestFilterOpenAIErr_UpstreamAuthFailed 上游 401/403 不再落到「负载已饱和」文案，
// 且不透出上游原文中的 key；上游 429 仍是限流文案。
func TestFilterOpenAIErr_UpstreamAuthFailed(t *testing.T) {
	setWrap(t, true)
	setFailMessage(t, config.DefaultChannelFailErrorMessage)
	const fakeKey = "sk-test-FAKE-KEY-PLACEHOLDER"

	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		c := newTestContext()
		c.Set(logger.RequestIdKey, "rid-auth")
		got := FilterOpenAIErr(c, oaiErr(status, false, "", "", "Missing Authentication header, key "+fakeKey))
		if codeStr(got) != "upstream_auth_failed" || got.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("status %d: got %d/%v, want 503/upstream_auth_failed", status, got.StatusCode, got.OpenAIError.Code)
		}
		msg := got.OpenAIError.Message
		if !strings.Contains(msg, config.UpstreamAuthFailedErrorMessage) || !strings.Contains(msg, "(request id: rid-auth)") {
			t.Fatalf("status %d: unexpected message %q", status, msg)
		}
		if strings.Contains(msg, config.DefaultChannelFailErrorMessage) || strings.Contains(msg, fakeKey) || strings.Contains(msg, "Missing Authentication") {
			t.Fatalf("status %d: message must not be saturation copy or leak upstream text: %q", status, msg)
		}
	}

	c := newTestContext()
	got := FilterOpenAIErr(c, oaiErr(http.StatusTooManyRequests, false, "", "", "rate limited"))
	if got.StatusCode != http.StatusTooManyRequests || !strings.Contains(got.OpenAIError.Message, config.DefaultChannelFailErrorMessage) {
		t.Fatalf("429 should keep rate-limit copy: got %d %q", got.StatusCode, got.OpenAIError.Message)
	}
}

// TestFilterOpenAIErr_ModelNotFoundExemption model-not-found 豁免：
// 坍缩路径上命中 isModelNotFoundErr → 404 + model_not_available，曾见 429 时豁免让位。
func TestFilterOpenAIErr_ModelNotFoundExemption(t *testing.T) {
	setWrap(t, true)

	t.Run("generic wording without original_model", func(t *testing.T) {
		c := newTestContext()
		c.Set(logger.RequestIdKey, "rid-mnf")
		got := FilterOpenAIErr(c, oaiErr(502, false, "", "", "No endpoints found for foo/bar"))
		if got.StatusCode != http.StatusNotFound {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusNotFound)
		}
		if codeStr(got) != "model_not_available" {
			t.Fatalf("code: got %v, want model_not_available", got.OpenAIError.Code)
		}
		if got.OpenAIError.Type != "invalid_request_error" {
			t.Fatalf("type: got %q, want invalid_request_error", got.OpenAIError.Type)
		}
		if !strings.Contains(got.OpenAIError.Message, "The requested model does not exist") {
			t.Fatalf("message should use generic wording: got %q", got.OpenAIError.Message)
		}
		if !strings.Contains(got.OpenAIError.Message, "(request id: rid-mnf)") {
			t.Fatalf("message should append request id: got %q", got.OpenAIError.Message)
		}
	})

	t.Run("original_model interpolated and skips masking", func(t *testing.T) {
		c := newTestContext()
		// 域名形模型名：若走了 MaskSensitiveInfo 会被打成 ***.turbo，据此证明 skipMask。
		c.Set("original_model", "gpt-4.turbo")
		got := FilterOpenAIErr(c, oaiErr(404, false, "", "", "model not found"))
		if got.StatusCode != http.StatusNotFound {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusNotFound)
		}
		if !strings.Contains(got.OpenAIError.Message, "The model `gpt-4.turbo` does not exist") {
			t.Fatalf("message should interpolate original_model unmasked: got %q", got.OpenAIError.Message)
		}
	})

	t.Run("final status 429 wins over model-not-found", func(t *testing.T) {
		c := newTestContext()
		got := FilterOpenAIErr(c, oaiErr(429, false, "", "", "model not found"))
		if got.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusTooManyRequests)
		}
		if codeStr(got) != "rate_limit_exceeded" {
			t.Fatalf("code: got %v, want rate_limit_exceeded", got.OpenAIError.Code)
		}
	})

	t.Run("ctx seen429 wins over model-not-found", func(t *testing.T) {
		c := newTestContext()
		c.Set("upstream_seen_429", true)
		got := FilterOpenAIErr(c, oaiErr(404, false, "", "", "No endpoints found for foo"))
		if got.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusTooManyRequests)
		}
	})
}

// TestFilterOpenAIErr_Masking 脱敏边界：坍缩文案（受信管理员配置）跳过脱敏；
// 非坍缩路径的 Message 一律过 MaskSensitiveInfo。
func TestFilterOpenAIErr_Masking(t *testing.T) {
	t.Run("collapse message skips masking (trusted admin copy)", func(t *testing.T) {
		setWrap(t, true)
		setFailMessage(t, "上游繁忙，请联系 support.example.com")
		c := newTestContext()
		got := FilterOpenAIErr(c, oaiErr(500, false, "", "", "boom"))
		if !strings.Contains(got.OpenAIError.Message, "support.example.com") {
			t.Fatalf("admin copy must not be masked: got %q", got.OpenAIError.Message)
		}
	})

	t.Run("passthrough upstream 400 message is masked", func(t *testing.T) {
		setWrap(t, true)
		c := newTestContext()
		got := FilterOpenAIErr(c, oaiErr(400, false, "invalid_request_error", "",
			"request to https://api.openai.com/v1/chat failed, api_key: sk-secret123"))
		if strings.Contains(got.OpenAIError.Message, "api.openai.com") {
			t.Fatalf("upstream host must be masked: got %q", got.OpenAIError.Message)
		}
		if strings.Contains(got.OpenAIError.Message, "sk-secret123") {
			t.Fatalf("api key must be masked: got %q", got.OpenAIError.Message)
		}
	})

	t.Run("wrap disabled passthrough is still masked", func(t *testing.T) {
		setWrap(t, false)
		c := newTestContext()
		got := FilterOpenAIErr(c, oaiErr(500, false, "", "", "upstream api.internal.example.com returned 500"))
		if strings.Contains(got.OpenAIError.Message, "api.internal.example.com") {
			t.Fatalf("domain must be masked even with wrap disabled: got %q", got.OpenAIError.Message)
		}
	})
}

// TestFilterOpenAIErr_TypeHiding 残余路径的 type 标签隐藏（SEC-14，LocalError 不豁免）：
// modeltaps_error / *_api_error / upstream_unavailable → system_error；
// Code 槽位携带 modeltaps_error 时同步隐藏（Claude 原生协议会把 Code 映射为顶层 type）。
func TestFilterOpenAIErr_TypeHiding(t *testing.T) {
	tests := []struct {
		name     string
		wrap     bool
		err      *types.OpenAIErrorWithStatusCode
		wantType string
	}{
		{"upstream 400 modeltaps_error hidden", true, oaiErr(400, false, "modeltaps_error", "", "boom"), "system_error"},
		{"upstream 400 anthropic_api_error hidden", true, oaiErr(400, false, "anthropic_api_error", "", "boom"), "system_error"},
		{"upstream 400 invalid_request_error kept", true, oaiErr(400, false, "invalid_request_error", "", "boom"), "invalid_request_error"},
		{"local 400 modeltaps_error hidden", true, oaiErr(400, true, "modeltaps_error", "", "boom"), "system_error"},
		{"local 400 anthropic_api_error hidden", true, oaiErr(400, true, "anthropic_api_error", "", "boom"), "system_error"},
		{"wrap off: local upstream_unavailable sentinel hidden", false, oaiErr(503, true, "upstream_unavailable", "", "no channels"), "system_error"},
		{"wrap off: upstream openai_api_error hidden", false, oaiErr(500, false, "openai_api_error", "", "boom"), "system_error"},
		{"wrap off: local modeltaps_error hidden", false, oaiErr(500, true, "modeltaps_error", "", "boom"), "system_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setWrap(t, tt.wrap)
			c := newTestContext()
			got := FilterOpenAIErr(c, tt.err)
			if got.OpenAIError.Type != tt.wantType {
				t.Fatalf("type: got %q, want %q", got.OpenAIError.Type, tt.wantType)
			}
		})
	}

	t.Run("code slot modeltaps_error hidden", func(t *testing.T) {
		setWrap(t, true)
		c := newTestContext()
		// relay/main.go setRequest 失败路径：code 参数传 modeltaps_error。
		got := FilterOpenAIErr(c, oaiErr(400, true, "modeltaps_error", "modeltaps_error", "invalid request body"))
		if codeStr(got) != "system_error" {
			t.Fatalf("code: got %v, want system_error", got.OpenAIError.Code)
		}
		if got.OpenAIError.Type != "system_error" {
			t.Fatalf("type: got %q, want system_error", got.OpenAIError.Type)
		}
	})
}

// TestFilterOpenAIErr_BadResponseStatusCode bad_response_status_code 文案修补。
func TestFilterOpenAIErr_BadResponseStatusCode(t *testing.T) {
	setWrap(t, true)

	t.Run("opaque message repaired from Param and keeps request id", func(t *testing.T) {
		c := newTestContext()
		c.Set(logger.RequestIdKey, "rid-brc")
		e := oaiErr(400, false, "", "bad_response_status_code", "unexpected upstream failure")
		e.OpenAIError.Param = "502"
		got := FilterOpenAIErr(c, e)
		// UX-16：修补先于 request id 拼接，覆盖后仍带 request id。
		if !strings.Contains(got.OpenAIError.Message, "Provider API error: bad response status code 502") {
			t.Fatalf("message: got %q, want repaired copy", got.OpenAIError.Message)
		}
		if !strings.Contains(got.OpenAIError.Message, "(request id: rid-brc)") {
			t.Fatalf("repaired message should keep request id: got %q", got.OpenAIError.Message)
		}
	})

	t.Run("repair with stale request id: stripped then current appended", func(t *testing.T) {
		c := newTestContext()
		c.Set(logger.RequestIdKey, "rid-cur")
		e := oaiErr(400, false, "", "bad_response_status_code", "unexpected upstream failure (request id: rid-old)")
		e.OpenAIError.Param = "503"
		got := FilterOpenAIErr(c, e)
		if !strings.Contains(got.OpenAIError.Message, "Provider API error: bad response status code 503") {
			t.Fatalf("message: got %q, want repaired copy", got.OpenAIError.Message)
		}
		if strings.Contains(got.OpenAIError.Message, "rid-old") {
			t.Fatalf("stale request id should not survive repair: got %q", got.OpenAIError.Message)
		}
		if !strings.Contains(got.OpenAIError.Message, "(request id: rid-cur)") {
			t.Fatalf("current request id should be appended: got %q", got.OpenAIError.Message)
		}
		if n := strings.Count(got.OpenAIError.Message, "(request id:"); n != 1 {
			t.Fatalf("expected exactly one request id, got %d: %q", n, got.OpenAIError.Message)
		}
	})

	t.Run("message already mentioning bad response status code kept", func(t *testing.T) {
		c := newTestContext()
		e := oaiErr(400, false, "", "bad_response_status_code", "bad response status code 502 from provider")
		e.OpenAIError.Param = "502"
		got := FilterOpenAIErr(c, e)
		if !strings.Contains(got.OpenAIError.Message, "bad response status code 502 from provider") {
			t.Fatalf("message should be kept: got %q", got.OpenAIError.Message)
		}
		if !strings.Contains(got.OpenAIError.Message, "(request id:") {
			t.Fatalf("message should keep request id suffix: got %q", got.OpenAIError.Message)
		}
	})
}

// TestFilterOpenAIErr_RequestId 残余路径的 request id 拼接与去重。
func TestFilterOpenAIErr_RequestId(t *testing.T) {
	setWrap(t, true)

	t.Run("stale request id stripped and current one appended", func(t *testing.T) {
		c := newTestContext()
		c.Set(logger.RequestIdKey, "rid-new")
		got := FilterOpenAIErr(c, oaiErr(400, false, "invalid_request_error", "", "boom (request id: rid-old)"))
		if strings.Contains(got.OpenAIError.Message, "rid-old") {
			t.Fatalf("stale request id should be stripped: got %q", got.OpenAIError.Message)
		}
		if !strings.Contains(got.OpenAIError.Message, "(request id: rid-new)") {
			t.Fatalf("current request id should be appended: got %q", got.OpenAIError.Message)
		}
		if n := strings.Count(got.OpenAIError.Message, "(request id:"); n != 1 {
			t.Fatalf("expected exactly one request id, got %d: %q", n, got.OpenAIError.Message)
		}
	})

	t.Run("request id appended to fresh message", func(t *testing.T) {
		c := newTestContext()
		c.Set(logger.RequestIdKey, "rid-fresh")
		got := FilterOpenAIErr(c, oaiErr(400, false, "invalid_request_error", "", "boom"))
		if !strings.Contains(got.OpenAIError.Message, "(request id: rid-fresh)") {
			t.Fatalf("request id should be appended: got %q", got.OpenAIError.Message)
		}
	})

	t.Run("collapse message also carries request id", func(t *testing.T) {
		c := newTestContext()
		c.Set(logger.RequestIdKey, "rid-collapse")
		got := FilterOpenAIErr(c, oaiErr(500, false, "", "", "boom"))
		if !strings.Contains(got.OpenAIError.Message, "(request id: rid-collapse)") {
			t.Fatalf("collapsed message should carry request id: got %q", got.OpenAIError.Message)
		}
	})
}

// TestFilterOpenAIErr_NilInput err=nil 不 panic；零值按"非 LocalError 非 400"坍缩。
func TestFilterOpenAIErr_NilInput(t *testing.T) {
	t.Run("wrap enabled: nil collapses to 503", func(t *testing.T) {
		setWrap(t, true)
		c := newTestContext()
		got := FilterOpenAIErr(c, nil)
		if got.StatusCode != http.StatusServiceUnavailable {
			t.Fatalf("status: got %d, want %d", got.StatusCode, http.StatusServiceUnavailable)
		}
		if codeStr(got) != "service_unavailable" {
			t.Fatalf("code: got %v, want service_unavailable", got.OpenAIError.Code)
		}
	})

	t.Run("wrap disabled: nil passes through zero value", func(t *testing.T) {
		setWrap(t, false)
		c := newTestContext()
		got := FilterOpenAIErr(c, nil)
		if got.StatusCode != 0 {
			t.Fatalf("status: got %d, want 0", got.StatusCode)
		}
	})
}
