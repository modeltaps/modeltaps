package controller

import (
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/controller/check_channel"

	"github.com/gin-gonic/gin"
)

// CheckChannel runs every check against one model of a channel:
// GET /api/channel/check/:id?model=. One request checks one model; the web UI schedules several
// models concurrently, the same way the multi-model test does.
func CheckChannel(c *gin.Context) {
	channelId, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	result, err := check_channel.CheckModel(channelId, c.Query("model"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}
