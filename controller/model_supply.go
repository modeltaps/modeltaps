package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
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
		{model.ErrPendingNotFound, common.APICodeSupplyPendingNotFound},
		{model.ErrModelStatus, common.APICodeModelStatusInvalid},
		{model.ErrSupplyBatchAction, common.APICodeInvalidParams},
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
		recordSupplyMigration(c.GetInt("id"), report)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": report})
}

// recordSupplyMigration writes an applied migration to the catalog audit log.
func recordSupplyMigration(userId int, report *model.SupplyMigrationReport) {
	model.RecordModelCatalogAudit(userId, model.ModelSupplyActionMigrate, "", nil, gin.H{
		"supplies": report.SuppliesPlanned, "new_models": len(report.NewModels),
		"status_changes": len(report.StatusChanges), "price_copies": len(report.PriceCopies),
	})
}

// PreviewModelSupply shows in which order a request for ?model= (optionally from ?group=)
// would try its supplies, with each supply's state.
func PreviewModelSupply(c *gin.Context) {
	preview, err := model.PreviewModelSupply(c.Query("model"), c.Query("group"))
	if err != nil {
		respondSupplyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": preview})
}

// PreviewPricingRisk shows, for ?ratio= (the default group's ratio) and ?payment= (percent; the
// saved values when left out), what each channel needs to break even, which supplies would lose
// money and what each user group brings in. Read only.
func PreviewPricingRisk(c *gin.Context) {
	ratio := model.DefaultGroupRatio()
	payment := config.PaymentCostPercent
	if v, err := strconv.ParseFloat(c.Query("ratio"), 64); err == nil {
		ratio = v
	}
	if v, err := strconv.ParseFloat(c.Query("payment"), 64); err == nil {
		payment = v
	}
	if ratio <= 0 || ratio > 100 || payment < 0 || payment >= 100 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("ratio must be above 0 and at most 100, payment cost 0 to under 100 percent"))
		return
	}
	risk, err := model.PreviewPricingRisk(ratio, payment)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": risk})
}

// ConfirmPendingSupplies confirms every supply of {"channel_id"} waiting for review.
func ConfirmPendingSupplies(c *gin.Context) {
	var req struct {
		ChannelId int `json:"channel_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.ChannelId <= 0 {
		common.APIRespondWithError(c, http.StatusOK, errors.New("channel_id is required"))
		return
	}
	n, err := model.ConfirmPendingSupplies(req.ChannelId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"confirmed": n}})
}

// GetPendingUpstreamModels lists new upstream models waiting on channels for an admin: each
// with the channels it waits on and the public model it most likely belongs to.
func GetPendingUpstreamModels(c *gin.Context) {
	pending, err := model.GetPendingUpstreamModels()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": pending})
}

// AttachPendingUpstreamModel supplies a public model with a waiting upstream model on its
// channels, creating the public model first when asked to.
func AttachPendingUpstreamModel(c *gin.Context) {
	var req model.PendingAttach
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	result, err := model.AttachPendingUpstreamModel(req)
	if err != nil {
		respondSupplyError(c, err)
		return
	}
	for _, supply := range result.Supplies {
		model.RecordModelCatalogAudit(c.GetInt("id"), model.ModelSupplyActionCreate, supply.Model, nil, supply)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

// IgnorePendingUpstreamModel stops listing a waiting upstream model on the given channels (all
// when none are given).
func IgnorePendingUpstreamModel(c *gin.Context) {
	var req struct {
		UpstreamModel string `json:"upstream_model"`
		ChannelIds    []int  `json:"channel_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	changed, err := model.IgnorePendingUpstreamModel(req.UpstreamModel, req.ChannelIds)
	if err != nil {
		respondSupplyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"channels": changed}})
}

// SuggestModelSupplyUpstream tells the add-supply form how a channel names a public model
// upstream (GET /api/model_supply/upstream_suggestion?model=&channel_id=).
func SuggestModelSupplyUpstream(c *gin.Context) {
	channelId, _ := strconv.Atoi(c.Query("channel_id"))
	channel, err := model.GetChannelById(channelId)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": model.SuggestSupplyUpstream(c.Query("model"), channel)})
}

// BatchModelSupplies adds or removes one channel as a supply of several public models.
// Body: {"action": "add"|"remove", "channel_id": 1, "models": ["..."]}.
func BatchModelSupplies(c *gin.Context) {
	var req model.SupplyBatch
	if err := c.ShouldBindJSON(&req); err != nil || len(req.Models) == 0 {
		common.APIRespondWithCode(c, http.StatusBadRequest, common.APICodeInvalidParams, "models is required", nil)
		return
	}
	result, err := model.ApplySupplyBatch(req)
	if err != nil {
		respondSupplyError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}
