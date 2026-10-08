package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

func setupLlmsTxtRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/llms.txt", LlmsTxt)

	oldAddr, oldName, oldDocs := config.ServerAddress, config.SystemName, config.DocsLink
	oldClaude, oldGemini, oldMCP := config.ClaudeAPIEnabled, config.GeminiAPIEnabled, config.MCP_ENABLE
	t.Cleanup(func() {
		config.ServerAddress, config.SystemName, config.DocsLink = oldAddr, oldName, oldDocs
		config.ClaudeAPIEnabled, config.GeminiAPIEnabled, config.MCP_ENABLE = oldClaude, oldGemini, oldMCP
	})
	return r
}

func fetchLlmsTxt(t *testing.T, r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
		t.Fatalf("Content-Type 应为 text/markdown; charset=utf-8，实际 %q", ct)
	}
	return w
}

// 默认配置（三协议开关全开）应输出完整的三协议 URL、认证与发现段落。
func TestLlmsTxtFullOutput(t *testing.T) {
	r := setupLlmsTxtRouter(t)
	config.ServerAddress = "https://api.example.com"
	config.SystemName = "Modeltaps Test"
	config.ClaudeAPIEnabled = true
	config.GeminiAPIEnabled = true
	config.MCP_ENABLE = true
	config.DocsLink = "https://docs.example.com"

	body := fetchLlmsTxt(t, r, httptest.NewRequest("GET", "/llms.txt", nil)).Body.String()

	for _, want := range []string{
		"# Modeltaps Test",
		"## Docs",
		"- [Modeltaps Test docs](https://docs.example.com)",
		"https://api.example.com/v1/chat/completions",
		"https://api.example.com/claude/v1/messages",
		"https://api.example.com/gemini/v1beta/models",
		"https://api.example.com/mcp/",
		"Authorization: Bearer <API key>",
		"x-api-key",
		"https://api.example.com/v1/models",
		"https://api.example.com/api/prices",
		"https://api.example.com/api/status",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("输出缺少 %q", want)
		}
	}
}

// 协议开关关闭时对应段落应被裁剪。
func TestLlmsTxtTrimsDisabledProtocols(t *testing.T) {
	r := setupLlmsTxtRouter(t)
	config.ServerAddress = "https://api.example.com"
	config.ClaudeAPIEnabled = false
	config.GeminiAPIEnabled = false
	config.MCP_ENABLE = false

	body := fetchLlmsTxt(t, r, httptest.NewRequest("GET", "/llms.txt", nil)).Body.String()

	for _, unwanted := range []string{"/claude", "/gemini", "/mcp/", "x-api-key"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("开关关闭后输出仍含 %q", unwanted)
		}
	}
	if !strings.Contains(body, "https://api.example.com/v1/chat/completions") {
		t.Error("OpenAI 段落不应被裁剪")
	}
}

// ServerAddress 为默认值时应回退用请求 scheme+Host 拼 base URL。
func TestLlmsTxtFallsBackToRequestHost(t *testing.T) {
	r := setupLlmsTxtRouter(t)
	config.ServerAddress = defaultServerAddress

	req := httptest.NewRequest("GET", "/llms.txt", nil)
	req.Host = "gateway.example.org"
	req.Header.Set("X-Forwarded-Proto", "https")
	body := fetchLlmsTxt(t, r, req).Body.String()

	if !strings.Contains(body, "https://gateway.example.org/v1/chat/completions") {
		t.Errorf("应使用请求 Host 拼 base URL，实际输出：%s", body)
	}
	if strings.Contains(body, defaultServerAddress) {
		t.Error("不应回落到默认 localhost 地址")
	}
}

// ServerAddress 为空且无 X-Forwarded-Proto 时按 http + 请求 Host 拼接。
func TestLlmsTxtEmptyServerAddressUsesHTTPHost(t *testing.T) {
	r := setupLlmsTxtRouter(t)
	config.ServerAddress = ""

	req := httptest.NewRequest("GET", "/llms.txt", nil)
	req.Host = "127.0.0.1:8080"
	body := fetchLlmsTxt(t, r, req).Body.String()

	if !strings.Contains(body, "http://127.0.0.1:8080/v1/chat/completions") {
		t.Errorf("应使用 http + 请求 Host，实际输出：%s", body)
	}
}

// DocsLink 置空时应整段省略 Docs。
func TestLlmsTxtOmitsDocsWhenDocsLinkEmpty(t *testing.T) {
	r := setupLlmsTxtRouter(t)
	config.ServerAddress = "https://api.example.com"
	config.DocsLink = "  "

	body := fetchLlmsTxt(t, r, httptest.NewRequest("GET", "/llms.txt", nil)).Body.String()

	if strings.Contains(body, "## Docs") {
		t.Errorf("DocsLink 为空时不应输出 Docs 段，实际输出：%s", body)
	}
}
