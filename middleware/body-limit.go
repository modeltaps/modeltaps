package middleware

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// RelayBodyLimit 限制中继请求体大小（relay_max_body_mb，0 表示不限）。中继会把整个请求体读进内存，
// 不设上限时一个有效令牌并发发送超大请求体即可耗尽内存。声明了 Content-Length 的请求超限直接 413；
// 分块传输的请求包上 MaxBytesReader，读到上限即报错。
func RelayBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		limitMB := viper.GetInt64("relay_max_body_mb")
		if limitMB <= 0 || c.Request.Body == nil {
			c.Next()
			return
		}
		limit := limitMB << 20
		if c.Request.ContentLength > limit {
			abortWithMessage(c, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("request body exceeds the %d MB limit", limitMB))
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		c.Next()
	}
}
