package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/telegram"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func serveTelegramWebhook(t *testing.T, path, secret string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST(telegram.WebhookPath, Telegram(), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodPost, path, nil)
	if secret != "" {
		req.Header.Set(telegramSecretHeader, secret)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w.Code
}

// SEC-84: the webhook is authenticated by the secret header, and its path never holds the bot token.
func TestTelegramWebhookAuthenticatesBySecretHeader(t *testing.T) {
	oldEnabled, oldSecret := telegram.TGEnabled, telegram.TGWebHookSecret
	telegram.TGEnabled, telegram.TGWebHookSecret = true, "hook-secret"
	viper.Set("tg.bot_api_key", "123456:bot-token")
	t.Cleanup(func() {
		telegram.TGEnabled, telegram.TGWebHookSecret = oldEnabled, oldSecret
		viper.Set("tg.bot_api_key", "")
	})

	if telegram.WebhookPath != "/api/telegram/webhook" {
		t.Fatalf("WebhookPath = %q, want a fixed path without the bot token", telegram.WebhookPath)
	}
	if got := serveTelegramWebhook(t, telegram.WebhookPath, "hook-secret"); got != http.StatusOK {
		t.Fatalf("valid secret: status %d, want 200", got)
	}
	if got := serveTelegramWebhook(t, telegram.WebhookPath, "wrong"); got != http.StatusNotFound {
		t.Fatalf("wrong secret: status %d, want 404", got)
	}
	if got := serveTelegramWebhook(t, telegram.WebhookPath, ""); got != http.StatusNotFound {
		t.Fatalf("missing secret: status %d, want 404", got)
	}

	telegram.TGWebHookSecret = ""
	if got := serveTelegramWebhook(t, telegram.WebhookPath, ""); got != http.StatusNotFound {
		t.Fatalf("polling mode (no webhook secret): status %d, want 404", got)
	}
}

func TestRedactLogPathTelegram(t *testing.T) {
	cases := map[string]string{
		"/api/telegram/123456:bot-token": "/api/telegram/***",
		"/api/telegram/123456:tok/extra": "/api/telegram/***",
		telegram.WebhookPath:             telegram.WebhookPath,
		"/api/telegram/":                 "/api/telegram/",
		"/api/option/telegram/reload":    "/api/option/telegram/reload",
	}
	for in, want := range cases {
		if got := redactLogPath(in); got != want {
			t.Errorf("redactLogPath(%q) = %q, want %q", in, got, want)
		}
	}
}
