package controller

import (
	"net/http"
	"strconv"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// GetChannelCacheStats returns per-channel prompt cache and session affinity counters
// from the consume logs. Query: start_timestamp / end_timestamp (unix seconds, default the
// last 24 hours) and an optional channel_id.
func GetChannelCacheStats(c *gin.Context) {
	end, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	start, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	if end <= 0 {
		end = time.Now().Unix()
	}
	if start <= 0 {
		start = end - 24*3600
	}
	channelId, _ := strconv.Atoi(c.Query("channel_id"))

	stats, err := model.GetChannelCacheStats(start, end, channelId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    stats,
	})
}
