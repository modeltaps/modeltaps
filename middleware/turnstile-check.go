package middleware

import (
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
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Turnstile token is empty",
				})
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
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				c.Abort()
				return
			}
			defer rawRes.Body.Close()
			var res turnstileCheckResponse
			err = json.NewDecoder(rawRes.Body).Decode(&res)
			if err != nil {
				logger.SysError(err.Error())
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": err.Error(),
				})
				c.Abort()
				return
			}
			if !res.Success {
				c.JSON(http.StatusOK, gin.H{
					"success": false,
					"message": "Turnstile verification failed, please refresh and try again!",
				})
				c.Abort()
				return
			}
			session.Set("turnstile", time.Now().Unix())
			err = session.Save()
			if err != nil {
				c.JSON(http.StatusOK, gin.H{
					"message": "Unable to save session, please try again",
					"success": false,
				})
				return
			}
		}
		c.Next()
	}
}
