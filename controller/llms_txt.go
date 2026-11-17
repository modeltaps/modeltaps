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

// publicSiteURL 按 ServerAddress > 请求 Host 的优先级得出对外可访问的根地址（无尾部斜杠）。
func publicSiteURL(c *gin.Context) string {
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

// LlmsTxt serves /llms.txt (https://llmstxt.org): a short index for AI tools that points to
// /skill.md for setting an agent up, to the base URLs and to the public docs. Like skill.md it is
// rendered from site settings only and holds no user data. No sign-in needed.
func LlmsTxt(c *gin.Context) {
	base := publicSiteURL(c)
	name := siteDisplayName()

	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", name)
	fmt.Fprintf(&b, "> %s is a multi-provider AI gateway with OpenAI-compatible APIs", name)
	if config.ClaudeAPIEnabled {
		b.WriteString(" and Anthropic-compatible APIs")
	}
	b.WriteString(". Calls use an API key created in its console.\n\n")

	b.WriteString("## Agents\n\n")
	fmt.Fprintf(&b, "- [skill.md](%s/skill.md): how an agent connects itself: getting an API key without it passing through the chat "+
		"(pairing code or device authorization), keeping it, verifying it and configuring the provider\n\n", base)

	b.WriteString("## API\n\n")
	fmt.Fprintf(&b, "- OpenAI-compatible base URL: `%s/v1` (`Authorization: Bearer <API key>`)\n", base)
	if config.ClaudeAPIEnabled {
		fmt.Fprintf(&b, "- Anthropic-compatible base URL: `%s/claude` (`x-api-key: <API key>`)\n", base)
	}
	if config.GeminiAPIEnabled {
		fmt.Fprintf(&b, "- Gemini-compatible base URL: `%s/gemini`\n", base)
	}
	fmt.Fprintf(&b, "- Models: `GET %s/v1/models` lists the models the API key may call; never hard-code model IDs\n", base)
	fmt.Fprintf(&b, "- Pricing: `GET %s/api/prices`; status: `GET %s/api/status`\n", base, base)
	if config.MCP_ENABLE {
		fmt.Fprintf(&b, "- MCP (with a management key in the `Authorization` header): `POST %s/mcp`\n", base)
	}
	b.WriteString("\n")

	if docsLink := strings.TrimSpace(config.DocsLink); docsLink != "" {
		b.WriteString("## Docs\n\n")
		fmt.Fprintf(&b, "- [%s docs](%s)\n\n", name, docsLink)
	}

	fmt.Fprintf(&b, "Version: %s\n", config.Version)

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(b.String()))
}
