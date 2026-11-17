package middleware

import (
	"context"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestId tags the request with an id. It runs first on every route, so it is also where
// the temp files of multipart uploads are removed once the handlers are done: the form is
// parsed on the shallow copy of the request a middleware made, which net/http never cleans.
func RequestId() func(c *gin.Context) {
	return func(c *gin.Context) {
		defer common.RemoveMultipartForms(c)
		id := utils.GetTimeString() + utils.GetRandomString(8)
		c.Set(logger.RequestIdKey, id)
		c.Set("requestStartTime", time.Now())
		ctx := context.WithValue(c.Request.Context(), logger.RequestIdKey, id)
		c.Request = c.Request.WithContext(ctx)
		c.Header(logger.RequestIdKey, id)
		c.Next()
	}
}
