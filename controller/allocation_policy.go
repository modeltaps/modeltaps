package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"
)

func respondAllocationError(c *gin.Context, err error) {
	if errors.Is(err, model.ErrAllocationEntry) || errors.Is(err, model.ErrAllocationDuplicate) || errors.Is(err, model.ErrAllocationValue) {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, err.Error(), nil)
		return
	}
	common.APIRespondWithError(c, http.StatusOK, err)
}

// GetAllocationPolicy returns the default allocation policy with the providers it can place.
func GetAllocationPolicy(c *gin.Context) {
	overview, err := model.GetAllocationOverview()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": overview})
}

// UpdateAllocationPolicy replaces the default policy's tiers. Body: {"tiers": [[{...}], ...]}.
func UpdateAllocationPolicy(c *gin.Context) {
	var req struct {
		Tiers [][]model.AllocationEntry `json:"tiers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, err.Error(), nil)
		return
	}
	if _, err := model.SaveDefaultAllocationPolicy(req.Tiers); err != nil {
		respondAllocationError(c, err)
		return
	}
	GetAllocationPolicy(c)
}

// SetModelAllocation switches public models between the policy and their own supply numbers.
// Body: {"ids": [1, 2], "allocation": "policy"|"manual"}.
func SetModelAllocation(c *gin.Context) {
	var req struct {
		Ids        []int  `json:"ids"`
		Allocation string `json:"allocation"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Ids) == 0 {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "ids is required", nil)
		return
	}
	affected, err := model.SetModelAllocation(req.Ids, req.Allocation)
	if err != nil {
		respondAllocationError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"affected": affected}})
}
