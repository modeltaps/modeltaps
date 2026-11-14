package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// GetModelLibrary lists the model library for the add-models dialog
// (GET /api/model_info/library).
func GetModelLibrary(c *gin.Context) {
	items, err := model.ListLibraryItems()
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": items})
}

// AddModelsFromLibrary adds library models to the catalog (POST /api/model_info/library/add).
func AddModelsFromLibrary(c *gin.Context) {
	var req model.LibraryAddRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	result, err := model.AddLibraryModels(req)
	if err != nil {
		if errors.Is(err, model.ErrLibraryAddEmpty) || errors.Is(err, model.ErrModelStatus) {
			common.APIRespondWithError(c, http.StatusBadRequest, err)
			return
		}
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	for _, name := range result.Added {
		model.RecordModelCatalogAudit(c.GetInt("id"), model.ModelCatalogActionAddFromLibrary, name, nil, req.Status)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": result})
}

type libraryRulesView struct {
	Rules    []model.LibraryRule `json:"rules"`
	Custom   bool                `json:"custom"`
	Defaults []model.LibraryRule `json:"defaults"`
	Enabled  bool                `json:"enabled"`
	Apply    bool                `json:"apply"`
}

func currentLibraryRules() libraryRulesView {
	return libraryRulesView{
		Rules:    model.EffectiveLibraryRules(),
		Custom:   strings.TrimSpace(config.ModelLibraryRules) != "",
		Defaults: model.DefaultLibraryRules(),
		Enabled:  config.ModelLibraryRulesEnabled,
		Apply:    config.ModelLibraryRulesApply,
	}
}

// GetLibraryRules returns the inclusion rules and their switches (GET /api/model_info/library/rules).
func GetLibraryRules(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": currentLibraryRules()})
}

type libraryRulesUpdate struct {
	// Rules replaces the saved rules; Reset goes back to the defaults instead.
	Rules   *[]model.LibraryRule `json:"rules"`
	Reset   bool                 `json:"reset"`
	Enabled *bool                `json:"enabled"`
	Apply   *bool                `json:"apply"`
}

// UpdateLibraryRules saves the inclusion rules and switches (PUT /api/model_info/library/rules).
func UpdateLibraryRules(c *gin.Context) {
	var req libraryRulesUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if req.Reset {
		if err := model.UpdateOption("ModelLibraryRules", ""); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	} else if req.Rules != nil {
		raw, err := json.Marshal(*req.Rules)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		if _, err := model.ParseLibraryRules(string(raw)); err != nil {
			common.APIRespondWithError(c, http.StatusBadRequest, err)
			return
		}
		if err := model.UpdateOption("ModelLibraryRules", string(raw)); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	for key, v := range map[string]*bool{"ModelLibraryRulesEnabled": req.Enabled, "ModelLibraryRulesApply": req.Apply} {
		if v == nil {
			continue
		}
		if err := model.UpdateOption(key, strconv.FormatBool(*v)); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": currentLibraryRules()})
}

// PreviewLibraryRules shows what the given rules (or the saved ones) would do now, changing
// nothing (POST /api/model_info/library/rules/preview).
func PreviewLibraryRules(c *gin.Context) {
	var req struct {
		Rules *[]model.LibraryRule `json:"rules"`
	}
	_ = c.ShouldBindJSON(&req)
	rules := model.EffectiveLibraryRules()
	if req.Rules != nil {
		raw, _ := json.Marshal(*req.Rules)
		parsed, err := model.ParseLibraryRules(string(raw))
		if err != nil {
			common.APIRespondWithError(c, http.StatusBadRequest, err)
			return
		}
		rules = parsed
	}
	outcomes, err := model.EvaluateLibraryRules(rules, time.Now().Unix())
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if outcomes == nil {
		outcomes = []model.LibraryRuleOutcome{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": outcomes})
}

// RunLibraryRules runs the saved rules now, in the saved mode (POST /api/model_info/library/rules/run).
func RunLibraryRules(c *gin.Context) {
	run, err := model.RunLibraryRules(config.ModelLibraryRulesApply)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": run})
}
