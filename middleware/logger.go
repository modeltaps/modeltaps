package middleware

import (
	"path"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/metrics"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gin-gonic/gin"
)

func SetUpLogger(server *gin.Engine) {

	server.Use(GinzapWithConfig())
}

// staticExt lists the file types the embedded front end serves.
var staticExt = map[string]bool{
	".js": true, ".css": true, ".map": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".svg": true, ".ico": true, ".webp": true, ".woff": true, ".woff2": true, ".ttf": true,
	".webmanifest": true, ".txt": true,
}

// skipAccessLog leaves out successful requests nobody looks for in an access log: the front
// end's static files and the system log page polling itself. Failed requests are always kept.
func skipAccessLog(method, p string, status int) bool {
	if status >= 400 {
		return false
	}
	if strings.HasPrefix(p, "/api/system_info/") {
		return true
	}
	return (method == "GET" || method == "HEAD") && staticExt[strings.ToLower(path.Ext(p))]
}

func GinzapWithConfig() gin.HandlerFunc {
	// logs.access_log=false turns the per-request line off; errors are still logged.
	accessLog := utils.GetOrDefault("logs.access_log", true)
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		// 如果query的值包含key=sk-fsdfsdfsdf 则把sk-fsdfsdfsdf脱敏处理
		if query != "" {
			if i := strings.Index(query, "key=sk-"); i >= 0 {
				start := i + 4 // "key=" length
				end := strings.Index(query[start:], "&")
				if end == -1 {
					end = len(query)
				} else {
					end += start
				}
				query = query[:start] + "sk-***" + query[end:]
			}
		}
		c.Next()
		end := time.Now()
		latency := end.Sub(start)
		requestID := c.GetString(logger.RequestIdKey)
		userID := c.GetInt("id")

		fields := []zapcore.Field{
			zap.Int("status", c.Writer.Status()),
			zap.String("request_id", requestID),
			zap.String("method", c.Request.Method),
			zap.String("path", path),
			zap.String("query", query),
			zap.String("ip", c.ClientIP()),
			zap.String("user-agent", c.Request.UserAgent()),
			zap.Duration("latency", latency),
			zap.Int("user_id", userID),
			zap.String("original_model", c.GetString("original_model")),
			// requested_model 仅在请求名是目录别名、被改写为主名时非空，保留用户原始请求名
			zap.String("requested_model", c.GetString("requested_model")),
			zap.String("new_model", c.GetString("new_model")),
			zap.Int("token_id", c.GetInt("token_id")),
			zap.String("token_name", c.GetString("token_name")),
			zap.Int("channel_id", c.GetInt("channel_id")),
		}

		if len(c.Errors) == 0 && (!accessLog || skipAccessLog(c.Request.Method, path, c.Writer.Status())) {
			metrics.RecordHttp(c, latency)
			return
		}

		if len(c.Errors) > 0 {
			// Append error field if this is an erroneous request.
			for _, e := range c.Errors.Errors() {
				logger.Logger.Error(e, fields...)
			}
		} else {
			logger.Logger.Info("GIN request", fields...)
		}
		metrics.RecordHttp(c, latency)
	}
}
