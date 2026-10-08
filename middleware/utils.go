package middleware

import (
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"net/http"

	"github.com/gin-gonic/gin"
)

func abortWithMessage(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, gin.H{
		"error": gin.H{
			"message": utils.MessageWithRequestId(message, c.GetString(logger.RequestIdKey)),
			"type":    "system_error",
		},
	})
	c.Abort()
	logger.LogError(c.Request.Context(), message)
}

func midjourneyAbortWithMessage(c *gin.Context, code int, description string) {
	c.JSON(http.StatusBadRequest, gin.H{
		"description": description,
		"type":        "system_error",
		"code":        code,
	})

	c.Abort()
	logger.LogError(c.Request.Context(), description)
}
