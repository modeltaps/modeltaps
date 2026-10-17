package middleware

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"encoding/json"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/url"
	"time"
)

type turnstileCheckResponse struct {
	Success bool `json:"success"`
}

func TurnstileCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.TurnstileCheckEnabled {
			session := sessions.Default(c)
			// 会话内的校验标记带短 TTL：足以覆盖「发送验证码 → 注册」两步流程，
			// 又不会让一次校验长期免检。旧版本存布尔值，视为已过期需重新校验。
			if ts, ok := session.Get("turnstile").(int64); ok {
				if time.Since(time.Unix(ts, 0)) < config.TurnstileSessionTTL {
					c.Next()
					return
				}
			}
			response := c.Query("turnstile")
			if response == "" {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeTurnstileTokenMissing, "Turnstile token is empty", nil)
				c.Abort()
				return
			}
			rawRes, err := http.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", url.Values{
				"secret":   {config.TurnstileSecretKey},
				"response": {response},
				"remoteip": {c.ClientIP()},
			})
			if err != nil {
				logger.SysError(err.Error())
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeTurnstileUnavailable, err.Error(), nil)
				c.Abort()
				return
			}
			defer rawRes.Body.Close()
			var res turnstileCheckResponse
			err = json.NewDecoder(rawRes.Body).Decode(&res)
			if err != nil {
				logger.SysError(err.Error())
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeTurnstileUnavailable, err.Error(), nil)
				c.Abort()
				return
			}
			if !res.Success {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeTurnstileFailed, "Turnstile verification failed, please refresh and try again!", nil)
				c.Abort()
				return
			}
			session.Set("turnstile", time.Now().Unix())
			err = session.Save()
			if err != nil {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeSessionSaveFailed, "Unable to save session, please try again", nil)
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
