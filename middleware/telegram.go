package middleware

import (
	"crypto/subtle"

	"github.com/modeltaps/modeltaps/common/telegram"

	"github.com/gin-gonic/gin"
)

// telegramSecretHeader carries the webhook secret (tg.webhook_secret) on every update Telegram
// delivers; it is the only credential of the webhook route.
const telegramSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

// Telegram admits webhook deliveries only while the bot runs in webhook mode and the request
// carries the configured secret. Anything else gets the same 404 as an unknown route.
func Telegram() func(c *gin.Context) {
	return func(c *gin.Context) {
		secret := telegram.TGWebHookSecret
		got := c.GetHeader(telegramSecretHeader)
		if !telegram.TGEnabled || secret == "" || subtle.ConstantTimeCompare([]byte(got), []byte(secret)) != 1 {
			c.String(404, "Page not found")
			c.Abort()
			return
		}

		c.Next()
	}
}
