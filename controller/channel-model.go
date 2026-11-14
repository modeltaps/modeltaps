package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetModelList(c *gin.Context) {
	channel := &model.Channel{}
	err := c.ShouldBindJSON(channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	keys := strings.Split(channel.Key, "\n")
	channel.Key = keys[0]

	if channel.Key == "" {
		if channel.Id == 0 {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelKeyRequired, "key is required", nil)
			return
		}

		channel, err = model.GetChannelById(channel.Id)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, channelLookupError(err))
			return
		}
	}

	provider := providers.GetProvider(channel, c)
	if provider == nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelProviderNotFound, "provider not found", nil)
		return
	}

	modelProvider, ok := provider.(providersBase.ModelListInterface)
	if !ok {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelModelListUnsupported, "channel not implemented", nil)
		return
	}

	modelList, err := modelProvider.GetModelList()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 去除重复的模型名称
	uniqueModels := removeDuplicates(modelList)

	// ?detail=1：渠道模型挑选用，按家族拆解并补目录元数据、售价与家族热度。
	if c.Query("detail") == "1" {
		familyRequests, err := model.FamilyRequestCounts(model.ModelPickerHeatDays)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		channelCounts, err := model.ModelChannelCounts(channel.Id)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		picker, err := model.BuildModelPicker(uniqueModels, familyRequests, channelCounts)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data":    picker,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    uniqueModels,
	})
}

// 辅助函数：去除切片中的重复元素
func removeDuplicates(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// channelModelDetailsLimit bounds the list one request may describe.
const channelModelDetailsLimit = 5000

// GetChannelModelDetails describes a channel's model list (the one being edited, so it may not
// be saved yet) for the channel panel: catalog metadata, the usual order and the public models
// each serves. It never calls the upstream.
func GetChannelModelDetails(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, invalidChannelIDError())
		return
	}
	var body struct {
		Models  []string          `json:"models"`
		Mapping map[string]string `json:"mapping"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	models := make([]string, 0, len(body.Models))
	for _, m := range removeDuplicates(body.Models) {
		if m = strings.TrimSpace(m); m != "" {
			models = append(models, m)
		}
	}
	if len(models) > channelModelDetailsLimit {
		models = models[:channelModelDetailsLimit]
	}
	details, err := model.BuildChannelModelDetails(id, models, body.Mapping)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": details})
}
