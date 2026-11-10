package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// respondSupplyError maps supply validation errors to API codes; anything else is reported as is.
func respondSupplyError(c *gin.Context, err error) {
	codes := []struct {
		err  error
		code string
	}{
		{model.ErrSupplyModelNotFound, common.APICodeModelSupplyModelNotFound},
		{model.ErrSupplyModelNotPublic, common.APICodeModelSupplyModelNotPublic},
		{model.ErrSupplyChannelNotFound, common.APICodeModelSupplyChannelNotFound},
		{model.ErrSupplyUpstreamMissing, common.APICodeModelSupplyUpstreamMissing},
		{model.ErrSupplyExists, common.APICodeModelSupplyExists},
	}
	for _, entry := range codes {
		if errors.Is(err, entry.err) {
			common.APIRespondWithCode(c, http.StatusBadRequest, entry.code, err.Error(), nil)
			return
		}
	}
	common.APIRespondWithError(c, http.StatusOK, err)
}

// GetModelSupplies lists supplies, optionally for one model (?model=) or one channel
// (?channel_id=), each with its channel's name, status, type and tag.
func GetModelSupplies(c *gin.Context) {
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	supplies, err := model.GetModelSupplyViews(c.Query("model"), channelId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": supplies})
}

type modelSupplyRequest struct {
	Id            int    `json:"id"`
	Model         string `json:"model"`
	ChannelId     int    `json:"channel_id"`
	UpstreamModel string `json:"upstream_model"`
	Priority      int64  `json:"priority"`
	Weight        uint   `json:"weight"`
	// Enabled is a pointer so that leaving it out means "enabled" on create and "unchanged" on update.
	Enabled *bool  `json:"enabled"`
	Note    string `json:"note"`
}

func CreateModelSupply(c *gin.Context) {
	var req modelSupplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	supply := &model.ModelSupply{
		Model: req.Model, ChannelId: req.ChannelId, UpstreamModel: req.UpstreamModel,
		Priority: req.Priority, Weight: req.Weight, Enabled: req.Enabled == nil || *req.Enabled, Note: req.Note,
	}
	if err := model.CreateModelSupply(supply); err != nil {
		respondSupplyError(c, err)
		return
	}
	model.RecordModelCatalogAudit(c.GetInt("id"), model.ModelSupplyActionCreate, supply.Model, nil, supply)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": supply})
}

func UpdateModelSupply(c *gin.Context) {
	var req modelSupplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	before, err := model.GetModelSupply(req.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	enabled := before.Enabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	after := &model.ModelSupply{
		Id: req.Id, UpstreamModel: req.UpstreamModel, Priority: req.Priority, Weight: req.Weight,
		Enabled: enabled, Note: req.Note,
	}
	if err := model.UpdateModelSupply(after); err != nil {
		respondSupplyError(c, err)
		return
	}
	updated, _ := model.GetModelSupply(req.Id)
	model.RecordModelCatalogAudit(c.GetInt("id"), model.ModelSupplyActionUpdate, before.Model, before, updated)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": updated})
}

func DeleteModelSupply(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	before, err := model.GetModelSupply(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if err := model.DeleteModelSupply(id); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.RecordModelCatalogAudit(c.GetInt("id"), model.ModelSupplyActionDelete, before.Model, before, nil)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

// MigrateModelSupplies turns the channels' models and model_mapping into supplies. It is a dry
// run unless the body says {"dry_run": false}; either way it returns the report.
func MigrateModelSupplies(c *gin.Context) {
	var req struct {
		DryRun *bool `json:"dry_run"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	dryRun := req.DryRun == nil || *req.DryRun
	report, err := model.MigrateChannelsToSupplies(dryRun)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if !dryRun {
		model.RecordModelCatalogAudit(c.GetInt("id"), model.ModelSupplyActionMigrate, "", nil, gin.H{
			"supplies": report.SuppliesPlanned, "new_models": len(report.NewModels),
			"status_changes": len(report.StatusChanges), "price_copies": len(report.PriceCopies),
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": report})
}
