package relay_util

// buildLogIODetail 的请求体留存策略测试:JSON/文本类原样留存,multipart 等二进制载荷
// 只写占位说明(修复 STT 上传把音频字节灌进 log_details 的问题)。

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// newLogIOContext 构造已开启 LogIO 闸门、带指定 Content-Type 与缓存请求体的 gin.Context。
func newLogIOContext(t *testing.T, contentType string, body []byte) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)
	if contentType != "" {
		c.Request.Header.Set("Content-Type", contentType)
	}
	c.Set("token_log_io", true)
	c.Set(config.GinRequestBodyKey, body)
	return c
}

func TestIsTextualLogIOContentType(t *testing.T) {
	cases := map[string]bool{
		"application/json":                    true,
		"application/json; charset=utf-8":     true,
		"APPLICATION/JSON":                    true,
		"text/plain":                          true,
		"text/event-stream":                   true,
		"application/x-ndjson":                true,
		"application/xml":                     true,
		"application/x-www-form-urlencoded":   true,
		"application/vnd.api+json":            true,
		"multipart/form-data":                 false,
		"multipart/form-data; boundary=xxxxx": false,
		"application/octet-stream":            false,
		"audio/mpeg":                          false,
		"image/png":                           false,
		// 缺省 Content-Type 与 common.UnmarshalBodyReusable 同口径:按 JSON 放行
		"":    true,
		"   ": true,
	}
	for in, want := range cases {
		if got := isTextualLogIOContentType(in); got != want {
			t.Errorf("isTextualLogIOContentType(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestBuildLogIODetailSkipsMultipartBody(t *testing.T) {
	prev := config.LogIOEnabled
	config.LogIOEnabled = true
	defer func() { config.LogIOEnabled = prev }()

	// 模拟 multipart 上传:请求体含非法 UTF-8 二进制字节
	binary := append([]byte("--boundary\r\n\r\n"), 0xff, 0xfe, 0x00, 0x80)
	c := newLogIOContext(t, "multipart/form-data; boundary=boundary", binary)

	q := &Quota{tokenId: 7, createdBy: 9}
	detail := q.buildLogIODetail(c, nil, false)
	if detail == nil {
		t.Fatal("detail 不应为 nil")
	}
	if strings.Contains(detail.RequestBody, "\xff") || strings.Contains(detail.RequestBody, "\xfe") {
		t.Errorf("request_body 不应含二进制字节: %q", detail.RequestBody)
	}
	if !strings.Contains(detail.RequestBody, "request body omitted") ||
		!strings.Contains(detail.RequestBody, "multipart/form-data") {
		t.Errorf("request_body 应为占位说明: %q", detail.RequestBody)
	}
	if detail.RequestTruncated {
		t.Error("主动省略不算截断,RequestTruncated 应为 false")
	}
}

func TestBuildLogIODetailKeepsJSONBody(t *testing.T) {
	prev := config.LogIOEnabled
	config.LogIOEnabled = true
	defer func() { config.LogIOEnabled = prev }()

	body := []byte(`{"model":"gpt-4o","messages":[{"role":"user","content":"你好"}]}`)
	c := newLogIOContext(t, "application/json; charset=utf-8", body)

	q := &Quota{tokenId: 7, createdBy: 9}
	detail := q.buildLogIODetail(c, nil, false)
	if detail == nil {
		t.Fatal("detail 不应为 nil")
	}
	if detail.RequestBody != string(body) {
		t.Errorf("request_body = %q, want %q", detail.RequestBody, string(body))
	}
	if detail.RequestTruncated {
		t.Error("未超限不应标记截断")
	}
}

func TestBuildLogIODetailKeepsBodyWithoutContentType(t *testing.T) {
	prev := config.LogIOEnabled
	config.LogIOEnabled = true
	defer func() { config.LogIOEnabled = prev }()

	// 缺省 Content-Type:common.UnmarshalBodyReusable 按 JSON 处理,明细应原样留存
	body := []byte(`{"model":"gpt-4o","messages":[]}`)
	c := newLogIOContext(t, "", body)

	q := &Quota{tokenId: 7, createdBy: 9}
	detail := q.buildLogIODetail(c, nil, false)
	if detail == nil {
		t.Fatal("detail 不应为 nil")
	}
	if detail.RequestBody != string(body) {
		t.Errorf("request_body = %q, want %q", detail.RequestBody, string(body))
	}
	if detail.RequestTruncated {
		t.Error("未超限不应标记截断")
	}
}

func TestBuildLogIODetailGateOff(t *testing.T) {
	prev := config.LogIOEnabled
	config.LogIOEnabled = false
	defer func() { config.LogIOEnabled = prev }()

	c := newLogIOContext(t, "application/json", []byte(`{"a":1}`))
	q := &Quota{}
	if detail := q.buildLogIODetail(c, nil, false); detail != nil {
		t.Error("闸门为假时应返回 nil")
	}
}
