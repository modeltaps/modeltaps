package controller

import (
	"errors"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetAllModelOwnedBy(c *gin.Context) {
	modelOwnedBies, err := model.GetAllModelOwnedBy()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    modelOwnedBies,
	})
}

func GetModelOwnedBy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	modelOwnedBy, err := model.GetModelOwnedBy(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    modelOwnedBy,
	})
}

func CreateModelOwnedBy(c *gin.Context) {
	var modelOwnedBy model.ModelOwnedBy
	if err := c.ShouldBindJSON(&modelOwnedBy); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if checkModelOwnedByReserveID(modelOwnedBy.Id) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid id"))
		return
	}

	icon, err := model.NormalizeVendorIcon(modelOwnedBy.Icon)
	if err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}
	modelOwnedBy.Icon = icon

	modelOwnedBy.Slug = normalizeVendorSlug(modelOwnedBy.Slug)
	if err := checkModelOwnedBySlugUnique(modelOwnedBy.Slug, modelOwnedBy.Id); err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}

	if err := model.CreateModelOwnedBy(&modelOwnedBy); err != nil {
		common.APIRespondWithError(c, slugConflictStatus(err), err)
		return
	}
	model.TriggerOwnedByBrandIconFetch(&modelOwnedBy)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func UpdateModelOwnedBy(c *gin.Context) {
	var modelOwnedBy model.ModelOwnedBy
	if err := c.ShouldBindJSON(&modelOwnedBy); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	icon, err := model.NormalizeVendorIcon(modelOwnedBy.Icon)
	if err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}
	modelOwnedBy.Icon = icon

	modelOwnedBy.Slug = normalizeVendorSlug(modelOwnedBy.Slug)
	if err := checkModelOwnedBySlugUnique(modelOwnedBy.Slug, modelOwnedBy.Id); err != nil {
		common.APIRespondWithError(c, http.StatusBadRequest, err)
		return
	}

	if err := model.UpdateModelOwnedBy(&modelOwnedBy); err != nil {
		common.APIRespondWithError(c, slugConflictStatus(err), err)
		return
	}
	model.TriggerOwnedByBrandIconFetch(&modelOwnedBy)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteModelOwnedBy(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if checkModelOwnedByReserveID(id) {
		common.APIRespondWithError(c, http.StatusOK, errors.New("invalid id"))
		return
	}

	if err := model.DeleteModelOwnedBy(id); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func checkModelOwnedByReserveID(id int) bool {
	return id <= model.ModelOwnedByReserveID
}

// normalizeVendorSlug 归一化厂商 slug：trim + 小写，空白串归为空（表示没有公开标识）。
func normalizeVendorSlug(slug model.NullableSlug) model.NullableSlug {
	return model.NullableSlug(strings.ToLower(strings.TrimSpace(string(slug))))
}

// checkModelOwnedBySlugUnique 用内存快照给出友好的早退；权威判据是库上的唯一索引，
// 并发写入漏过这一层时由 model.ErrVendorSlugTaken 兜住。空 slug 不参与唯一性。
func checkModelOwnedBySlugUnique(slug model.NullableSlug, id int) error {
	if slug == "" {
		return nil
	}
	if owner := model.ModelOwnedBysInstance.GetIdBySlug(string(slug)); owner != 0 && owner != id {
		return model.ErrVendorSlugTaken
	}
	return nil
}

// slugConflictStatus 把 slug 冲突映射为 400，其余写库错误沿用既有的 200 + success:false。
func slugConflictStatus(err error) int {
	if errors.Is(err, model.ErrVendorSlugTaken) {
		return http.StatusBadRequest
	}
	return http.StatusOK
}
