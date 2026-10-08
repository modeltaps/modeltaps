package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// defaultServerAddress 是 ServerAddress 未被管理员配置时的出厂值，
// 此时用请求自身的 scheme+Host 拼 base URL 更贴近真实部署域名。
const defaultServerAddress = "http://localhost:3000"

// llmsTxtBaseURL 按 ServerAddress > 请求 Host 的优先级得出对外可访问的根地址（无尾部斜杠）。
func llmsTxtBaseURL(c *gin.Context) string {
	addr := strings.TrimSpace(config.ServerAddress)
	if addr != "" && strings.TrimRight(addr, "/") != defaultServerAddress {
		return strings.TrimRight(addr, "/")
	}

	scheme := "http"
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	} else if c.Request != nil && c.Request.TLS != nil {
		scheme = "https"
	}

	host := ""
	if c.Request != nil {
		host = c.Request.Host
	}
	if host == "" {
		return strings.TrimRight(defaultServerAddress, "/")
	}
	return scheme + "://" + host
}

// LlmsTxt 按 https://llmstxt.org 格式输出本网关的机器可读说明，无需认证。
func LlmsTxt(c *gin.Context) {
	base := llmsTxtBaseURL(c)
	systemName := strings.TrimSpace(config.SystemName)
	if systemName == "" {
		systemName = "Modeltaps"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", systemName)
	b.WriteString("> Multi-protocol AI gateway, compatible with the OpenAI / Claude / Gemini APIs.\n\n")

	b.WriteString("## API Endpoints\n\n")
	fmt.Fprintf(&b, "- OpenAI-compatible (base URL: `%s/v1`)\n", base)
	fmt.Fprintf(&b, "  - `POST %s/v1/chat/completions`\n", base)
	fmt.Fprintf(&b, "  - `POST %s/v1/completions`\n", base)
	fmt.Fprintf(&b, "  - `POST %s/v1/responses`\n", base)
	fmt.Fprintf(&b, "  - `POST %s/v1/embeddings`\n", base)
	fmt.Fprintf(&b, "  - `POST %s/v1/images/generations`\n", base)
	fmt.Fprintf(&b, "  - `POST %s/v1/audio/speech`\n", base)
	fmt.Fprintf(&b, "  - `POST %s/v1/rerank`\n", base)
	if config.ClaudeAPIEnabled {
		fmt.Fprintf(&b, "- Claude native (base URL: `%s/claude`)\n", base)
		fmt.Fprintf(&b, "  - `POST %s/claude/v1/messages`\n", base)
		fmt.Fprintf(&b, "  - `GET %s/claude/v1/models`\n", base)
	}
	if config.GeminiAPIEnabled {
		fmt.Fprintf(&b, "- Gemini native (base URL: `%s/gemini`)\n", base)
		fmt.Fprintf(&b, "  - `POST %s/gemini/v1beta/models/{model}:generateContent`\n", base)
		fmt.Fprintf(&b, "  - `GET %s/gemini/v1beta/models`\n", base)
	}
	if config.MCP_ENABLE {
		fmt.Fprintf(&b, "- MCP (Model Context Protocol)\n")
		fmt.Fprintf(&b, "  - Streamable HTTP: `POST %s/mcp/{accessToken}`\n", base)
		fmt.Fprintf(&b, "  - SSE: `GET %s/mcp/sse/{accessToken}`\n", base)
	}
	b.WriteString("\n")

	b.WriteString("## Authentication\n\n")
	b.WriteString("- All endpoints use the `Authorization: Bearer <API key>` header.\n")
	if config.ClaudeAPIEnabled {
		b.WriteString("- Claude endpoints also accept `x-api-key: <API key>`.\n")
	}
	fmt.Fprintf(&b, "- Create API keys in the console: sign in to %s and open the \"API Keys\" page.\n\n", base)

	b.WriteString("## Discovery\n\n")
	fmt.Fprintf(&b, "- `GET %s/v1/models`: list the models available to the current API key\n", base)
	fmt.Fprintf(&b, "- `GET %s/api/prices`: model pricing\n", base)
	fmt.Fprintf(&b, "- `GET %s/api/status`: service status and version\n\n", base)

	b.WriteString("## Client Configuration\n\n")
	b.WriteString("Any client that supports an \"OpenAI-compatible / custom base URL\" setting (OpenAI SDK, LangChain, Cline, LobeChat, etc.) ")
	b.WriteString("can connect with just two settings:\n\n")
	fmt.Fprintf(&b, "- `base_url` / `OPENAI_BASE_URL` = `%s/v1`\n", base)
	b.WriteString("- `api_key` / `OPENAI_API_KEY` = an API key created in the console\n")
	if config.ClaudeAPIEnabled {
		fmt.Fprintf(&b, "- Anthropic SDK clients: `ANTHROPIC_BASE_URL` = `%s/claude`\n", base)
	}
	if config.GeminiAPIEnabled {
		fmt.Fprintf(&b, "- Gemini SDK clients: base URL = `%s/gemini`\n", base)
	}
	b.WriteString("\n")

	if docsLink := strings.TrimSpace(config.DocsLink); docsLink != "" {
		b.WriteString("## Docs\n\n")
		fmt.Fprintf(&b, "- [%s docs](%s)\n\n", systemName, docsLink)
	}

	fmt.Fprintf(&b, "Version: %s\n", config.Version)

	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(b.String()))
}
