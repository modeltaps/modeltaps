package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// RelayBodyLimit caps relay request bodies at relay_max_body_mb (0 disables it). The relay reads
// the whole body into memory, so without a cap one valid key sending huge bodies in parallel can
// exhaust memory. A declared Content-Length over the limit is refused with 413 at once; a chunked
// body is wrapped in a MaxBytesReader that fails once the limit is reached.
func RelayBodyLimit() gin.HandlerFunc {
	return func(c *gin.Context) {
		limitMB := viper.GetInt64("relay_max_body_mb")
		if limitMB <= 0 {
			c.Next()
			return
		}
		if !capRequestBody(c, limitMB<<20) {
			abortWithMessage(c, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("request body exceeds the %d MB limit", limitMB))
			return
		}
		c.Next()
	}
}

// BodyLimit caps the request body at limitBytes (0 or less: no cap), the same way RelayBodyLimit
// does: a declared Content-Length over the limit gets 413 before anything is read, a chunked body
// fails once the limit is read. Nested caps do not raise each other (the smallest one wins), so a
// route that needs more than its group's cap is listed in RouteBodyLimit instead.
func BodyLimit(limitBytes int64) gin.HandlerFunc {
	return RouteBodyLimit(limitBytes, nil)
}

// RouteBodyLimit is BodyLimit for a whole route group: routes maps a registered route path
// (gin's FullPath, for example "/api/prices/sync") to its own cap, which replaces defaultBytes for
// every method of that path. It has to run before anything reads the body, so it goes first in
// the group.
func RouteBodyLimit(defaultBytes int64, routes map[string]int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := defaultBytes
		if routeLimit, ok := routes[c.FullPath()]; ok {
			limit = routeLimit
		}
		if !capRequestBody(c, limit) {
			abortBodyTooLarge(c, limit)
			return
		}
		c.Next()
	}
}

// capRequestBody applies limit to the request body and reports false when the declared
// Content-Length is already over it.
func capRequestBody(c *gin.Context, limit int64) bool {
	if limit <= 0 || c.Request.Body == nil || c.Request.Body == http.NoBody {
		return true
	}
	if c.Request.ContentLength > limit {
		return false
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	return true
}

// abortBodyTooLarge answers 413 in the same error shape as abortWithMessage. It does not write an
// error log line: these routes are reachable without signing in, so a flood of oversized requests
// would otherwise turn into a flood of log writes; the status is in the access log.
func abortBodyTooLarge(c *gin.Context, limit int64) {
	// The connection still holds the unread body; close it instead of draining it.
	c.Header("Connection", "close")
	c.JSON(http.StatusRequestEntityTooLarge, gin.H{
		"error": gin.H{
			"message": utils.MessageWithRequestId("request body exceeds the "+formatByteLimit(limit)+" limit",
				c.GetString(logger.RequestIdKey)),
			"type": "system_error",
		},
	})
	c.Abort()
}

func formatByteLimit(limit int64) string {
	switch {
	case limit >= 1<<20 && limit%(1<<20) == 0:
		return fmt.Sprintf("%d MB", limit>>20)
	case limit >= 1<<10 && limit%(1<<10) == 0:
		return fmt.Sprintf("%d KB", limit>>10)
	default:
		return fmt.Sprintf("%d bytes", limit)
	}
}

// BodyReadTimeout bounds how long a client may take to send the request body (0 or less: no
// bound), so a client trickling a body byte by byte cannot hold a connection and its goroutine
// open. The http.Server has no whole-request ReadTimeout because that would also cut off long
// relay uploads; this sets a read deadline on the connection of the one request instead.
//
// Requests without a body get no deadline: net/http already runs its background read on their
// connection (to notice a client that goes away), and a deadline expiring there would cancel the
// context of a handler that is still working, such as an MCP stream. For a request with a body,
// net/http clears the deadline itself once the body has been read to the end and it starts that
// background read; TestBodyReadTimeout holds it to that.
func BodyReadTimeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if timeout > 0 && c.Request.Body != nil && c.Request.Body != http.NoBody {
			// Fails only on a writer that cannot set deadlines (tests); the body is then unbounded.
			_ = http.NewResponseController(c.Writer).SetReadDeadline(time.Now().Add(timeout))
		}
		c.Next()
	}
}
