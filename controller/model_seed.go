package controller

import (
	"net/http"

	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

type applySeedRequest struct {
	DryRun bool `json:"dry_run"`
}

// ApplyModelInfoSeed 应用内置策展白名单：dry_run 只返回变更摘要，不落库。
func ApplyModelInfoSeed(c *gin.Context) {
	req := applySeedRequest{}
	// 空请求体视为实跑，不报错。
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
	}

	result, err := model.ApplyCatalogSeed(req.DryRun)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// dry_run 什么都没改，不记审计；实跑按整体动作记一条，摘要即改后快照。
	if !req.DryRun {
		model.RecordModelCatalogAudit(c.GetInt("id"), model.ModelCatalogActionApplySeed, "", nil, result)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    result,
	})
}
