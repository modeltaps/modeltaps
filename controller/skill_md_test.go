package controller

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// AGENT-5: /skill.md is one file for the whole site, rendered from site settings only.

func fetchSkillMd(t *testing.T, req *http.Request) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/skill.md", SkillMd)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/markdown; charset=utf-8" {
		t.Fatalf("Content-Type %q, want text/markdown; charset=utf-8", ct)
	}
	return w.Body.String()
}

func keepSkillConfig(t *testing.T) {
	t.Helper()
	addr, name, docs, claude := config.ServerAddress, config.SystemName, config.DocsLink, config.ClaudeAPIEnabled
	secrets := []*string{&config.SessionSecret, &config.SMTPToken, &config.GitHubClientSecret, &config.TurnstileSecretKey, &config.OIDCClientSecret}
	saved := make([]string, len(secrets))
	for i, p := range secrets {
		saved[i] = *p
	}
	t.Cleanup(func() {
		config.ServerAddress, config.SystemName, config.DocsLink, config.ClaudeAPIEnabled = addr, name, docs, claude
		for i, p := range secrets {
			*p = saved[i]
		}
	})
}

func TestSkillMdRendersSiteSettings(t *testing.T) {
	keepSkillConfig(t)
	config.ServerAddress = "https://gw.example.com/"
	config.SystemName = "Acme AI: Gateway"
	config.DocsLink = "https://docs.example.com"
	config.ClaudeAPIEnabled = true

	body := fetchSkillMd(t, httptest.NewRequest(http.MethodGet, "/skill.md", nil))

	if !strings.HasPrefix(body, "---\nname: acme-ai-gateway\n") {
		t.Fatalf("frontmatter should open the file with the slug name, got:\n%s", body[:min(200, len(body))])
	}
	for _, want := range []string{
		`description: "Connect this agent to Acme AI: Gateway (https://gw.example.com)`,
		"compatibility: \"Needs an API key from https://gw.example.com",
		"# Acme AI: Gateway",
		"Register Acme AI: Gateway in your own model configuration",
		"`ANTHROPIC_BASE_URL=https://gw.example.com/claude-code`",
		"`base_url = \"https://gw.example.com/codex/v1\"`",
		"`https://gw.example.com/coding-agent/v1`",
		"## 1. What Acme AI: Gateway is",
		"OpenAI API and the Anthropic (Claude) API",
		"## 2. Base URLs",
		"`https://gw.example.com/v1`",
		"`https://gw.example.com/claude`",
		"## 3. Get an API key",
		"Never ask the user to paste a key into the chat",
		"POST https://gw.example.com/api/agent/pair",
		"POST https://gw.example.com/api/agent/device\n",
		"POST https://gw.example.com/api/agent/device/token",
		"authorization_pending", "slow_down", "access_denied", "expired_token",
		"## 4. Keep the key safe",
		"0600",
		"keychain",
		"## 5. Verify",
		"GET https://gw.example.com/v1/dashboard/billing/subscription",
		"GET https://gw.example.com/v1/models",
		"## 6. Configure the provider",
		"never hard-code them",
		"## 7. Report back to the user",
		"how to revoke it",
		"[Acme AI: Gateway documentation](https://docs.example.com)",
		"## Appendix: known agents",
		"`ANTHROPIC_HOST` set to `https://gw.example.com/coding-agent` (no `/v1`)",
		"`base_url` `https://gw.example.com/coding-agent/v1/chat/completions`",
		"`model_catalog_url = \"https://gw.example.com/codex/v1/models\"`",
		"## Appendix: reference commands",
		"security add-generic-password -U -s acme-ai-gateway",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("skill.md is missing %q", want)
		}
	}
	if strings.Contains(body, "gw.example.com//") {
		t.Error("the trailing slash of the server address must be dropped")
	}
}

func TestSkillMdLeavesOutDisabledAnthropic(t *testing.T) {
	keepSkillConfig(t)
	config.ServerAddress = "https://gw.example.com"
	config.ClaudeAPIEnabled = false

	body := fetchSkillMd(t, httptest.NewRequest(http.MethodGet, "/skill.md", nil))
	for _, unwanted := range []string{"/claude", "Anthropic"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("Claude API is off, skill.md still mentions %q", unwanted)
		}
	}
}

func TestSkillMdFallsBackToRequestHost(t *testing.T) {
	keepSkillConfig(t)
	config.ServerAddress = defaultServerAddress
	config.SystemName = ""

	req := httptest.NewRequest(http.MethodGet, "/skill.md", nil)
	req.Host = "gateway.example.org"
	req.Header.Set("X-Forwarded-Proto", "https")
	trustRequestPeer(t, req)
	body := fetchSkillMd(t, req)
	if !strings.Contains(body, "`https://gateway.example.org/v1`") || !strings.Contains(body, "name: modeltaps\n") {
		t.Errorf("want the request's origin and the default name, got:\n%s", body)
	}
}

// The file is the same for everyone and holds no secret: no configured secret, no key-shaped
// value, and nothing that tells an agent to drop safety measures.
func TestSkillMdHoldsNoSecrets(t *testing.T) {
	keepSkillConfig(t)
	config.ServerAddress = "https://gw.example.com"
	config.SessionSecret = "session-secret-value-123"
	config.SMTPToken = "smtp-token-value-456"
	config.GitHubClientSecret = "github-secret-value-789"
	config.TurnstileSecretKey = "turnstile-secret-value"
	config.OIDCClientSecret = "oidc-secret-value"

	body := fetchSkillMd(t, httptest.NewRequest(http.MethodGet, "/skill.md", nil))
	for _, secret := range []string{"session-secret-value-123", "smtp-token-value-456", "github-secret-value-789", "turnstile-secret-value", "oidc-secret-value"} {
		if strings.Contains(body, secret) {
			t.Errorf("skill.md leaks a configured secret %q", secret)
		}
	}
	if m := regexp.MustCompile(`(sk|mk)-[A-Za-z0-9]{16,}`).FindString(body); m != "" {
		t.Errorf("skill.md holds a key-shaped value %q", m)
	}
	if m := regexp.MustCompile(`pair-[A-Za-z0-9]{8,}`).FindString(body); m != "" {
		t.Errorf("skill.md holds a pairing-code-shaped value %q", m)
	}
	if strings.Contains(strings.ToLower(body), "moderation") {
		t.Error("skill.md must not say anything about content moderation")
	}
}
