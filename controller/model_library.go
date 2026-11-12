package controller

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
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
