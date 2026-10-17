package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// validateInviteCodeFormat 验证邀请码格式
func validateInviteCodeFormat(code string) error {
	if code == "" {
		return nil // 空邀请码允许，会自动生成
	}

	// 去除前后空格
	trimmedCode := strings.TrimSpace(code)
	if trimmedCode != code {
		return common.NewAPIError(common.APICodeInviteCodeHasSpaces, "invite code must not have leading or trailing spaces", nil)
	}

	// 检查长度
	if len(code) < 3 || len(code) > 32 {
		return common.NewAPIError(common.APICodeInviteCodeLength, "invite code must be 3-32 characters long", nil)
	}

	// 检查字符格式：只允许字母、数字、下划线和短横线
	matched, _ := regexp.MatchString("^[a-zA-Z0-9_-]+$", code)
	if !matched {
		return common.NewAPIError(common.APICodeInviteCodeCharset, "invite code may only contain letters, digits, underscores and hyphens", nil)
	}

	return nil
}

func GetInviteCodesList(c *gin.Context) {
	params := model.InviteCodeSearchParams{}
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	inviteCodes, err := model.GetInviteCodesList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    inviteCodes,
	})
}

func GetInviteCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "Invalid ID parameter", nil)
		return
	}

	inviteCode, err := model.GetInviteCodeById(id)
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInviteCodeLoadFailed, "Failed to get invite code: "+err.Error(), map[string]any{"reason": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    inviteCode,
	})
}

type CreateInviteCodeRequest struct {
	Name      string `json:"name"`
	Code      string `json:"code"` // 手动指定邀请码
	MaxUses   int    `json:"max_uses"`
	StartsAt  int64  `json:"starts_at"`  // 生效开始时间
	ExpiresAt int64  `json:"expires_at"` // 生效结束时间
	Count     int    `json:"count"`      // 批量创建数量
}

func CreateInviteCode(c *gin.Context) {
	var req CreateInviteCodeRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 邀请码格式验证
	if err := validateInviteCodeFormat(req.Code); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 参数验证（MaxUses = 0 表示无限使用，负数不允许，强制改为0）
	if req.MaxUses < 0 {
		req.MaxUses = 0
	}
	if req.Count <= 0 {
		req.Count = 1
	}
	if req.Count > 100 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeBatchTooLarge, "Cannot create more than 100 at a time", nil)
		return
	}

	// 时间验证
	if req.StartsAt > 0 && req.ExpiresAt > 0 && req.StartsAt >= req.ExpiresAt {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeStartAfterExpiry, "Expiration time must be later than the start time", nil)
		return
	}

	userId := c.GetInt("id")
	var createdCodes []string

	for i := 0; i < req.Count; i++ {
		var code string
		var err error

		// 如果手动指定了邀请码且只创建一个
		if req.Code != "" && req.Count == 1 {
			code = req.Code
			// 检查邀请码是否已存在
			var count int64
			model.DB.Model(&model.InviteCode{}).Where("code = ?", code).Count(&count)
			if count > 0 {
				common.APIRespondWithCode(c, http.StatusOK, common.APICodeInviteCodeExists, "Invite code already exists", nil)
				return
			}
		} else {
			// 自动生成邀请码
			code, err = model.GenerateInviteCode()
			if err != nil {
				common.APIRespondWithError(c, http.StatusOK, err)
				return
			}
		}

		inviteCode := model.InviteCode{
			Code:      code,
			Name:      req.Name,
			MaxUses:   req.MaxUses,
			StartsAt:  req.StartsAt,
			ExpiresAt: req.ExpiresAt,
			Status:    model.InviteCodeStatusEnabled,
			CreatedBy: userId,
		}

		if req.Count > 1 {
			inviteCode.Name = req.Name + "_" + strconv.Itoa(i+1)
		}

		err = inviteCode.Insert()
		if err != nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeInviteCodeCreateFailed, "Failed to create invite code", nil)
			return
		}
		createdCodes = append(createdCodes, inviteCode.Code)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"codes": createdCodes,
			"count": len(createdCodes),
		},
	})
}

type UpdateInviteCodeRequest struct {
	Name      string `json:"name"`
	MaxUses   int    `json:"max_uses"`
	Status    int    `json:"status"`
	StartsAt  int64  `json:"starts_at"`
	ExpiresAt int64  `json:"expires_at"`
}

func UpdateInviteCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "Invalid ID parameter", nil)
		return
	}

	var req UpdateInviteCodeRequest
	err = c.ShouldBindJSON(&req)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	inviteCode, err := model.GetInviteCodeById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	// 参数验证（MaxUses = 0 表示无限使用）
	if req.MaxUses < 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInviteCodeMaxUsesNegative, "Max uses must not be less than 0", nil)
		return
	}

	// 只有当MaxUses发生变化时才检查使用次数限制
	if req.MaxUses != inviteCode.MaxUses && req.MaxUses > 0 && req.MaxUses < inviteCode.UsedCount {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInviteCodeMaxUsesBelowUsed, "Max uses must not be less than the current use count", nil)
		return
	}

	if req.Status != model.InviteCodeStatusEnabled && req.Status != model.InviteCodeStatusDisabled {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeStatusInvalid, "Invalid status value", nil)
		return
	}

	// 时间验证
	if req.StartsAt > 0 && req.ExpiresAt > 0 && req.StartsAt >= req.ExpiresAt {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeStartAfterExpiry, "Expiration time must be later than the start time", nil)
		return
	}

	// 如果要启用邀请码，检查使用次数是否已达上限（MaxUses = 0 表示无限使用）
	// 使用数据库中的MaxUses值，而不是请求中的值（状态切换时请求中的MaxUses为0）
	if req.Status == model.InviteCodeStatusEnabled && inviteCode.MaxUses > 0 && inviteCode.UsedCount >= inviteCode.MaxUses {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInviteCodeUsedUp, "Invite code has reached its usage limit and cannot be enabled", nil)
		return
	}

	// 只更新非零值字段，避免状态切换时重置其他字段
	if req.Name != "" {
		inviteCode.Name = req.Name
	}
	if req.MaxUses != 0 || (req.MaxUses == 0 && req.Name != "") { // 如果传了name说明是完整更新
		inviteCode.MaxUses = req.MaxUses
	}
	inviteCode.Status = req.Status           // 状态总是更新
	if req.StartsAt != 0 || req.Name != "" { // 如果传了name说明是完整更新
		inviteCode.StartsAt = req.StartsAt
	}
	if req.ExpiresAt != 0 || req.Name != "" { // 如果传了name说明是完整更新
		inviteCode.ExpiresAt = req.ExpiresAt
	}

	err = inviteCode.Update()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// GenerateRandomInviteCode 生成随机邀请码
func GenerateRandomInviteCode(c *gin.Context) {
	code, err := model.GenerateInviteCode()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"code": code},
	})
}

func DeleteInviteCode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "Invalid ID parameter", nil)
		return
	}

	err = model.DeleteInviteCodeById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// BatchDeleteInviteCodes 批量删除邀请码
type BatchDeleteRequest struct {
	Ids []int `json:"ids" binding:"required"`
}

func BatchDeleteInviteCodes(c *gin.Context) {
	var req BatchDeleteRequest
	err := c.ShouldBindJSON(&req)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if len(req.Ids) == 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeDeleteSelectionRequired, "Please select invite codes to delete", nil)
		return
	}

	err = model.BatchDeleteInviteCodes(req.Ids)
	if err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeBulkDeleteFailed, "Bulk delete failed: "+err.Error(), map[string]any{"reason": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}
