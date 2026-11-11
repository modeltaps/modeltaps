package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// ---------- Model access (ORG-15) ----------

type orgModelAccessListView struct {
	*model.ModelAccessList
	Usage *model.ModelAccessListUsage `json:"usage,omitempty"`
}

func isOrgAdmin(role string) bool {
	return model.OrgRoleWeight(role) >= model.OrgRoleWeight(model.OrgRoleAdmin)
}

// GetOrgModelAccess returns the approved models, the organization default, the member layer of one
// member (user_id; members can only read their own) and the named lists. Members see the lists they
// can attach to their keys (not restricted); admins see every list with its attachment counts.
func GetOrgModelAccess(c *gin.Context) {
	org, role := getOrgFromContext(c)
	userId := c.GetInt("id")
	targetUserId := utils.String2Int(c.Query("user_id"))
	if targetUserId == 0 {
		targetUserId = userId
	}
	if targetUserId != userId && !isOrgAdmin(role) {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeOrgPermissionDenied, "Permission denied for this organization", nil)
		return
	}
	scope, err := model.GetOrgModelAccessScope(org, targetUserId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	lists, err := model.GetOrgModelAccessLists(org.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var usage map[int]*model.ModelAccessListUsage
	if isOrgAdmin(role) {
		if usage, err = model.GetOrgModelAccessListUsage(org); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	views := make([]orgModelAccessListView, 0, len(lists))
	for _, list := range lists {
		if list.Restricted && !isOrgAdmin(role) {
			continue
		}
		view := orgModelAccessListView{ModelAccessList: list}
		if usage != nil {
			view.Usage = usage[list.Id]
			if view.Usage == nil {
				view.Usage = &model.ModelAccessListUsage{}
			}
		}
		views = append(views, view)
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"scope": scope,
			"lists": views,
		},
	})
}

type orgModelAccessDefaultRequest struct {
	ModelAccessListId int      `json:"model_access_list_id"`
	Models            []string `json:"models"`
}

// UpdateOrgModelAccessDefault saves the organization default layer (Admin+).
func UpdateOrgModelAccessDefault(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	var req orgModelAccessDefaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "invalid parameters", nil)
		return
	}
	if err := model.UpdateOrgDefaultModelAccess(org, req.ModelAccessListId, req.Models); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	setting := org.Setting.Data()
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.model_access_default",
		fmt.Sprintf("Updated the organization default model access: list=%d, models=%v", setting.ModelAccessListId, setting.ModelWhitelist))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

type orgModelAccessListRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Restricted  bool     `json:"restricted"`
	Models      []string `json:"models"`
}

func (r orgModelAccessListRequest) toList() *model.ModelAccessList {
	return &model.ModelAccessList{
		Name:        r.Name,
		Description: r.Description,
		Restricted:  r.Restricted,
		Models:      r.Models,
	}
}

// CreateOrgModelAccessList creates a named list (Admin+).
func CreateOrgModelAccessList(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	var req orgModelAccessListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "invalid parameters", nil)
		return
	}
	list := req.toList()
	if err := model.CreateOrgModelAccessList(org, list, c.GetInt("id")); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.model_access_list_create",
		fmt.Sprintf("Created model access list %s (ID: %d, restricted: %t, %d models)", list.Name, list.Id, list.Restricted, len(list.Models)))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": list})
}

// UpdateOrgModelAccessList edits a named list (Admin+); the change applies to every attachment.
func UpdateOrgModelAccessList(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	var req orgModelAccessListRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "invalid parameters", nil)
		return
	}
	list := req.toList()
	list.Id = utils.String2Int(c.Param("list_id"))
	if err := model.UpdateOrgModelAccessList(org, list); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.model_access_list_update",
		fmt.Sprintf("Updated model access list %s (ID: %d, restricted: %t, %d models)", list.Name, list.Id, list.Restricted, len(list.Models)))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": list})
}

// DeleteOrgModelAccessList deletes a named list (Admin+); attachments fall back to the next layer.
func DeleteOrgModelAccessList(c *gin.Context) {
	org, _ := getOrgFromContext(c)
	id := utils.String2Int(c.Param("list_id"))
	list, err := model.GetOrgModelAccessList(org.Id, id)
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeModelAccessListNotFound, "the model access list does not exist", nil)
		return
	}
	if err := model.DeleteOrgModelAccessList(org, id); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.model_access_list_delete",
		fmt.Sprintf("Deleted model access list %s (ID: %d)", list.Name, list.Id))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// ---------- Site admin: approved models of an organization ----------

// AdminGetOrganizationApprovedModels returns the approved models of an organization (root).
func AdminGetOrganizationApprovedModels(c *gin.Context) {
	org, err := getOrgByIdParam(c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	models := []string(org.ApprovedModels)
	if models == nil {
		models = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"models": models}})
}

type adminOrgApprovedModelsRequest struct {
	Models []string `json:"models"`
}

// AdminSetOrganizationApprovedModels saves the approved models of an organization (root). It is
// refused, with the conflicting lists in data.conflicts, while a lower org-level list still uses an
// entry the new list does not cover.
func AdminSetOrganizationApprovedModels(c *gin.Context) {
	org, err := getOrgByIdParam(c)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	var req adminOrgApprovedModelsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "Invalid parameters", nil)
		return
	}
	conflicts, err := model.SetOrganizationApprovedModels(org, req.Models)
	if err != nil {
		var apiErr *common.APIError
		if len(conflicts) > 0 && errors.As(err, &apiErr) {
			body := common.APIErrorBody(err)
			body["data"] = gin.H{"conflicts": conflicts}
			c.JSON(http.StatusOK, body)
			return
		}
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	detail := "Site admin removed the approved models limit"
	if len(org.ApprovedModels) > 0 {
		detail = fmt.Sprintf("Site admin set the approved models: %s", strings.Join(org.ApprovedModels, ", "))
	}
	model.RecordOrgAudit(org.Id, c.GetInt("id"), "org.admin_approved_models", detail)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"models": []string(org.ApprovedModels)}})
}
