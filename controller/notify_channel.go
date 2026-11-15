package controller

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
)

// notifyChannelTestTimeout bounds a test notice, so a dead webhook does not hang the request.
const notifyChannelTestTimeout = 15 * time.Second

// NotifyChannelResponse is a notification channel as the console sees it: the plain settings,
// and for each secret only that it is set and a masked echo.
type NotifyChannelResponse struct {
	*model.NotifyChannel
	Settings map[string]string `json:"settings"`
	Secrets  map[string]string `json:"secrets"`
	// Ready and Detail are what the notifier reports (an email channel without SMTP is not
	// ready); Problem says why the settings do not build.
	Ready   bool   `json:"ready"`
	Detail  string `json:"detail,omitempty"`
	Problem string `json:"problem,omitempty"`
}

func newNotifyChannelResponse(c *model.NotifyChannel) *NotifyChannelResponse {
	resp := &NotifyChannelResponse{NotifyChannel: c, Settings: c.PlainSettings(), Secrets: map[string]string{}}
	for k, v := range c.SecretSettings() {
		resp.Secrets[k] = utils.MaskSecret(v)
	}
	n, err := c.Notifier()
	if err != nil {
		resp.Problem = err.Error()
		return resp
	}
	target := notify.TargetOf(c.Name, c.Type, n)
	resp.Ready, resp.Detail = target.Ready, target.Detail
	return resp
}

// NotifyChannelRequest creates, updates or tests a channel. Settings holds every field of the
// type, plain and secret; an empty secret keeps the stored one unless it is in ClearSecrets.
type NotifyChannelRequest struct {
	Id           int               `json:"id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	Settings     map[string]string `json:"settings"`
	ClearSecrets []string          `json:"clear_secrets"`
	Enabled      bool              `json:"enabled"`
	// Kinds limits the channel to these notice kinds; empty takes every kind.
	Kinds []string `json:"kinds"`
	Sort  int      `json:"sort"`
}

func notifyChannelError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"success": false, "message": message})
}

// apply writes the request onto channel (a stored one, or a new one with only the type set)
// and checks that the settings build.
func (req *NotifyChannelRequest) apply(channel *model.NotifyChannel) error {
	channel.Name = strings.TrimSpace(req.Name)
	channel.Enabled, channel.Sort = req.Enabled, req.Sort
	channel.SetKinds(req.Kinds)
	if err := channel.SetValues(req.Settings, req.ClearSecrets); err != nil {
		return err
	}
	_, err := channel.Notifier()
	return err
}

// GetNotifyChannelTypes lists the channel types and their settings.
func GetNotifyChannelTypes(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": notify.Types})
}

func GetNotifyChannels(c *gin.Context) {
	channels, err := model.GetNotifyChannels()
	if err != nil {
		notifyChannelError(c, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]*NotifyChannelResponse, 0, len(channels))
	for _, ch := range channels {
		out = append(out, newNotifyChannelResponse(ch))
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

func CreateNotifyChannel(c *gin.Context) {
	var req NotifyChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		notifyChannelError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	channel := &model.NotifyChannel{Type: req.Type}
	if err := req.apply(channel); err != nil {
		notifyChannelError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := channel.Insert(); err != nil {
		notifyChannelError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": newNotifyChannelResponse(channel)})
}

func storedNotifyChannel(c *gin.Context, id int) (*model.NotifyChannel, bool) {
	channel, err := model.GetNotifyChannelById(id)
	if err != nil {
		notifyChannelError(c, http.StatusNotFound, "notification channel not found")
		return nil, false
	}
	return channel, true
}

// UpdateNotifyChannel updates a channel. Its type cannot change: the stored secrets belong to it.
func UpdateNotifyChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	channel, ok := storedNotifyChannel(c, id)
	if !ok {
		return
	}
	var req NotifyChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		notifyChannelError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := req.apply(channel); err != nil {
		notifyChannelError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := channel.Update(); err != nil {
		notifyChannelError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": newNotifyChannelResponse(channel)})
}

func UpdateNotifyChannelStatus(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	channel, ok := storedNotifyChannel(c, id)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		notifyChannelError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	if err := channel.UpdateEnabled(req.Enabled); err != nil {
		notifyChannelError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func DeleteNotifyChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.DeleteNotifyChannelById(id); err != nil {
		notifyChannelError(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// TestNotifyChannel sends a test notice with the settings in the request, which may not be saved
// yet. With an id, the stored secrets fill the ones left empty, as saving would.
func TestNotifyChannel(c *gin.Context) {
	var req NotifyChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		notifyChannelError(c, http.StatusBadRequest, "invalid request body")
		return
	}
	channel := &model.NotifyChannel{Type: req.Type}
	if req.Id != 0 {
		stored, ok := storedNotifyChannel(c, req.Id)
		if !ok {
			return
		}
		channel = stored
	}
	if err := req.apply(channel); err != nil {
		notifyChannelError(c, http.StatusBadRequest, err.Error())
		return
	}
	n, _ := channel.Notifier()
	ctx, cancel := context.WithTimeout(c.Request.Context(), notifyChannelTestTimeout)
	defer cancel()
	title := fmt.Sprintf("%s test notice", config.SystemName)
	message := "This is a test notice. Admin notices such as disabled channels and low balances will arrive here."
	err := n.Send(notify.WithKind(ctx, "test"), title, message)
	notify.Record(notify.Delivery{Kind: "test", Type: channel.Type, Name: channel.Name, Title: title, Err: err})
	if err != nil {
		notifyChannelError(c, http.StatusBadGateway, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
