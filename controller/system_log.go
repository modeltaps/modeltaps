package controller

import (
	"errors"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"net/http"
	"os"
	"path/filepath"

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

// SystemLogContext returns the in-memory lines around one log entry.
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

// SystemLogProblems lists warnings and errors folded by message template since the last start.
func SystemLogProblems(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    logger.ListProblems(),
	})
}

// SystemLogResolveProblems removes the given problem groups, or all of them when keys is empty.
func SystemLogResolveProblems(c *gin.Context) {
	var body struct {
		Keys []string `json:"keys"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"resolved": logger.ResolveProblems(body.Keys)},
	})
}

// SystemLogDownload sends the active log file. Full-history search belongs to a log collector
// (docker logs, journald, Loki ...), not to this server.
func SystemLogDownload(c *gin.Context) {
	path, err := logger.LogFilePath()
	if err != nil {
		common.APIRespondWithError(c, http.StatusInternalServerError, err)
		return
	}
	if _, err := os.Stat(path); err != nil {
		common.APIRespondWithError(c, http.StatusNotFound, errors.New("log file not found"))
		return
	}
	c.FileAttachment(path, filepath.Base(path))
}
