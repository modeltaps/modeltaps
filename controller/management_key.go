package controller

import (
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// managementKeyNameMaxLength matches the API key name limit.
const managementKeyNameMaxLength = 30

type createManagementKeyRequest struct {
	Name        string `json:"name"`
	Scope       string `json:"scope"`
	ExpiredTime int64  `json:"expired_time"`
}

// GetManagementKeys lists the signed-in user's management keys (masked), legacy access tokens
// included. The routes are session-only (middleware.SessionOnly).
func GetManagementKeys(c *gin.Context) {
	keys, err := model.ListManagementKeys(c.GetInt("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": keys})
}

// CreateManagementKey creates a read or write management key. The expiry is required (0 means
// the default of 90 days, at most one year); the key is returned once and only its hash is kept.
func CreateManagementKey(c *gin.Context) {
	var req createManagementKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "invalid parameters", nil)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeManagementKeyNameRequired, "a management key needs a name", nil)
		return
	}
	if utf8.RuneCountInString(name) > managementKeyNameMaxLength {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeTokenNameTooLong, "API key name is too long", nil)
		return
	}
	key, err := model.CreateManagementKey(c.GetInt("id"), name, req.Scope, req.ExpiredTime)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"id":           key.Id,
			"name":         key.Name,
			"scope":        key.Scope,
			"key":          key.Key,
			"key_hint":     key.KeyHint,
			"expired_time": key.ExpiredTime,
			"created_time": key.CreatedTime,
		},
	})
}

// DeleteManagementKey revokes one of the signed-in user's management keys at once.
func DeleteManagementKey(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeManagementKeyNotFound, "management key not found", nil)
		return
	}
	if err := model.DeleteManagementKey(id, c.GetInt("id")); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
