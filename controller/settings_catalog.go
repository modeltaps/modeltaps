package controller

import (
	"errors"
	"net/http"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// GetSettingOverrides tells, per site setting objects can override, how many objects do.
func GetSettingOverrides(c *gin.Context) {
	overrides, err := model.SettingOverrides()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": overrides})
}

// GetInheritedSiteSetting returns ?key=, a site setting an object field follows, to admins who
// edit those objects.
func GetInheritedSiteSetting(c *gin.Context) {
	value, ok := model.InheritedSiteSetting(c.Query("key"))
	if !ok {
		common.APIRespondWithError(c, http.StatusOK, errors.New("not a setting objects follow"))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"key": c.Query("key"), "value": value}})
}
