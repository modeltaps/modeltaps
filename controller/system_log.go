package controller

import (
	"errors"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"net/http"

	"github.com/gin-gonic/gin"
)

// SystemLog handles the request to get the latest system logs from the log file
// It accepts a POST request with a 'count' parameter specifying the number of logs to return
func SystemLog(c *gin.Context) {
	// Parse the count parameter from the request body
	var requestBody struct {
		Count int `json:"count" binding:"required"`
	}

	if err := c.ShouldBindJSON(&requestBody); err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}

	// Validate the count parameter
	if requestBody.Count <= 0 {
		common.APIRespondWithError(c, http.StatusBadRequest, errors.New("count must be greater than 0"))
		return
	}

	// Get the latest logs from the log file
	logs, err := logger.GetLatestLogs(requestBody.Count)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}

	// Return the logs
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    logs,
	})
}

// SystemLogQuery handles advanced log queries
func SystemLogQuery(c *gin.Context) {
	var params logger.LogQueryParams

	if err := c.ShouldBindJSON(&params); err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}

	// Validate parameters
	if params.Count <= 0 {
		params.Count = 50 // Default count
	}
	if params.Count > 1000 {
		params.Count = 1000 // Max count
	}

	if params.Level != "" {
		params.Level = logger.NormalizeLevel(params.Level)
	}

	result, err := logger.QueryLogs(params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}

// SystemLogContext returns the lines around one log entry, from memory or the log file.
func SystemLogContext(c *gin.Context) {
	var params logger.LogContextParams
	if err := c.ShouldBindJSON(&params); err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}
	if params.ContextLines <= 0 {
		params.ContextLines = 5
	}
	if params.ContextLines > 50 {
		params.ContextLines = 50
	}

	result, err := logger.LogContext(params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}
