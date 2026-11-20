package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func GetUserGroups(c *gin.Context) {
	var params model.SearchUserGroupParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	userGroups, err := model.GetUserGroupsList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    userGroups,
	})
}

func GetUserGroupById(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	userGroup, err := model.GetUserGroupsById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    userGroup,
	})
}

func AddUserGroup(c *gin.Context) {
	userGroup := model.UserGroup{}
	if err := c.ShouldBindJSON(&userGroup); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if err := userGroup.Create(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateUserGroup(c *gin.Context) {
	userGroup := model.UserGroup{}
	err := c.ShouldBindJSON(&userGroup)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if err := userGroup.Update(); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteUserGroup(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))

	userGroup, err := model.GetUserGroupsById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if userGroup.Symbol == "default" {
		common.APIRespondWithError(c, http.StatusOK, errors.New("the default user group cannot be deleted"))
		return
	}

	// What still names the group moves to the successor (?successor=), which is required then.
	successor := c.Query("successor")
	if successor == "" {
		refs, err := model.CountUserGroupReferences(userGroup.Symbol)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		if refs.Any() {
			common.APIRespondWithError(c, http.StatusOK, common.NewAPIError(common.APICodeUserGroupInUse,
				"the group is still in use: choose a successor group for its accounts, API keys and channels", userGroupReferenceParams(refs)))
			return
		}
	}
	if err := model.DeleteUserGroupMoving(userGroup, successor); err != nil {
		if errors.Is(err, model.ErrUserGroupSuccessor) {
			err = common.NewAPIError(common.APICodeUserGroupSuccessorInvalid, err.Error(), nil)
		}
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})

}

func userGroupReferenceParams(r model.UserGroupReferences) map[string]any {
	return map[string]any{
		"users": r.Users, "shadow_accounts": r.ShadowAccounts, "tokens": r.Tokens,
		"backup_tokens": r.BackupTokens, "channels": r.Channels,
	}
}

// GetUserGroupReferences counts what names the group: the delete dialog lists it and asks for a
// successor when there is any.
func GetUserGroupReferences(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	userGroup, err := model.GetUserGroupsById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	refs, err := model.CountUserGroupReferences(userGroup.Symbol)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": refs})
}

func ChangeUserGroupEnable(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	userGroup, err := model.GetUserGroupsById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	if *userGroup.Enable && userGroup.Symbol == "default" {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "Cannot disable the default user group. Set another default group first.",
		})
		return
	}

	err = model.ChangeUserGroupEnable(id, !*userGroup.Enable)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
