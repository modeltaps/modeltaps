package controller

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// alertListKinds are the alert kinds the admin list shows; the rest (as the pending price
// changes a sync keeps for the next one) are bookkeeping.
var alertListKinds = map[string]bool{
	"model_drift":       true,
	"price_change":      true,
	"credential":        true,
	"model_consistency": true,
	"cost_mismatch":     true,
}

// alertClosedDays is how far back the list of closed alerts goes.
const alertClosedDays = 30

// alertView is an alert as the admin list shows it, with the channel a "channel:N" subject
// names (0 and "" when the channel is gone or the subject is not a channel).
type alertView struct {
	*model.Alert
	ChannelId   int    `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name,omitempty"`
}

// GetAlerts lists the alerts (AdminAuth): ?state=open (default) the open ones an admin has not
// dismissed, ?state=closed the ones resolved or dismissed in the last alertClosedDays days.
func GetAlerts(c *gin.Context) {
	var (
		alerts []*model.Alert
		err    error
	)
	if c.Query("state") == "closed" {
		alerts, err = model.ClosedAlerts(utils.GetTimestamp() - int64((alertClosedDays*24*time.Hour)/time.Second))
	} else {
		alerts, err = model.OpenAlerts("")
	}
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	views := make([]alertView, 0, len(alerts))
	for _, a := range alerts {
		if !alertListKinds[a.Kind] {
			continue
		}
		v := alertView{Alert: a}
		if id, ok := strings.CutPrefix(a.Subject, "channel:"); ok {
			if n, err := strconv.Atoi(id); err == nil {
				v.ChannelId = n
				if ch, err := model.GetChannelById(n); err == nil {
					v.ChannelName = ch.Name
				}
			}
		}
		views = append(views, v)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": views})
}

// CountOpenAlerts is the number of open alerts an admin has not dismissed, for the sidebar
// badge (AdminAuth).
func CountOpenAlerts(c *gin.Context) {
	alerts, err := model.OpenAlerts("")
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	open := 0
	for _, a := range alerts {
		if alertListKinds[a.Kind] {
			open++
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"open": open}})
}

// DismissAlert closes an open alert by hand (AdminAuth): the same condition is not told again,
// and something new about it opens it again.
func DismissAlert(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.APIRespondWithError(c, http.StatusOK, fmt.Errorf("invalid alert id"))
		return
	}
	if err := model.DismissAlert(id); err != nil {
		if err == gorm.ErrRecordNotFound {
			err = fmt.Errorf("the alert is not open")
		}
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
