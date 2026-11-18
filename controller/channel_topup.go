package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// GetChannelTopups lists the recorded top-ups of channel :id and the fee they give.
func GetChannelTopups(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid channel id"))
		return
	}
	summary, err := model.GetChannelTopups(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": summary})
}

// AddChannelTopup records {"paid", "credited", "note"} (USD) for channel :id and sets the
// channel's fee from its latest top-ups.
func AddChannelTopup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid channel id"))
		return
	}
	var req struct {
		Paid     float64 `json:"paid"`
		Credited float64 `json:"credited"`
		Note     string  `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if len(req.Note) > 255 {
		req.Note = req.Note[:255]
	}
	summary, err := model.AddChannelTopup(&model.ChannelTopup{
		ChannelId: id, Paid: req.Paid, Credited: req.Credited, Note: req.Note, CreatedBy: c.GetInt("id"),
	})
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": summary})
}

// DeleteChannelTopup removes top-up :topup_id of channel :id.
func DeleteChannelTopup(c *gin.Context) {
	id, err1 := strconv.Atoi(c.Param("id"))
	topupId, err2 := strconv.Atoi(c.Param("topup_id"))
	if err1 != nil || err2 != nil {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid id"))
		return
	}
	summary, err := model.DeleteChannelTopup(id, topupId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": summary})
}
