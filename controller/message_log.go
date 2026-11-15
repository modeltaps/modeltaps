package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/model"
)

// GetMessageLogs lists the newest send records of one source (mail or notify), page by page.
func GetMessageLogs(c *gin.Context) {
	source := c.Query("source")
	if source != model.MessageSourceMail && source != model.MessageSourceNotify {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "source must be mail or notify"})
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	logs, total, err := model.GetMessageLogs(source, c.Query("failed") == "true", (page-1)*size, size)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": logs, "total": total, "page": page, "size": size}})
}
