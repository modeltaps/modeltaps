package controller

import (
	"errors"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func GetChannelsList(c *gin.Context) {
	var params model.SearchChannelsParams
	if err := c.ShouldBindQuery(&params); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	channels, err := model.GetChannelsList(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channels,
	})
}

func GetChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, invalidChannelIDError())
		return
	}
	channel, err := model.GetChannelById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, channelLookupError(err))
		return
	}
	channel.HideKey()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    channel,
	})
}

func AddChannel(c *gin.Context) {
	channel := model.Channel{}
	err := c.ShouldBindJSON(&channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if err = model.CheckTagTypeConsistency(channel.Tag, channel.Type, 0); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	if rejectInvalidDriftRules(c, channel.DriftIgnoreRules) {
		return
	}
	// 复制渠道时前端拿不到完整 key，由后端从源渠道取
	if channel.Key == "" && c.Query("copy_key_from") != "" {
		sourceId, convErr := strconv.Atoi(c.Query("copy_key_from"))
		if convErr != nil {
			common.APIRespondWithError(c, http.StatusOK, convErr)
			return
		}
		source, getErr := model.GetChannelById(sourceId)
		if getErr != nil {
			common.APIRespondWithError(c, http.StatusOK, getErr)
			return
		}
		channel.Key = source.Key
	}
	canonicalized := canonicalizeChannelModels(&channel, "")
	channel.CreatedTime = utils.GetTimestamp()
	keys := strings.Split(channel.Key, "\n")

	baseUrls := []string{}
	if channel.BaseURL != nil && *channel.BaseURL != "" {
		baseUrls = strings.Split(*channel.BaseURL, "\n")
	}
	channels := make([]model.Channel, 0, len(keys))
	for index, key := range keys {
		if key == "" {
			continue
		}
		localChannel := channel
		localChannel.Key = key
		if index > 0 {
			localChannel.Name = localChannel.Name + "_" + strconv.Itoa(index+1)
		}

		if len(baseUrls) > index && baseUrls[index] != "" {
			localChannel.BaseURL = &baseUrls[index]
		} else if len(baseUrls) > 0 {
			localChannel.BaseURL = &baseUrls[0]
		}

		channels = append(channels, localChannel)
	}
	err = model.BatchInsertChannels(channels)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 保存后异步为渠道模型补齐元信息与价格（OpenRouter 自带列表优先，其余走 models.dev），免去手动同步。
	AutoSyncChannelCatalog(channel.Type, channel.Models, "channel add")
	for i := range channels {
		model.TriggerChannelBrandIconFetch(&channels[i])
	}
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "",
		"canonicalized": canonicalized,
	})
}

func DeleteChannel(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	channel := model.Channel{Id: id}
	err := channel.Delete()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteChannelTag(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	err := model.DeleteChannelTag(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func DeleteDisabledChannel(c *gin.Context) {
	rows, err := model.DeleteDisabledChannel()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    rows,
	})
}

func UpdateChannel(c *gin.Context) {
	channel := model.Channel{}
	err := c.ShouldBindJSON(&channel)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 校验类型一致性：标签变化（加入/切换标签）时校验；标签未变但「在组内单独编辑改了类型」时
	// 也要校验，防止单渠道编辑把同一标签分组变成混合类型。仅编辑已有成员且类型未变则放行，
	// 避免阻断历史遗留的混合分组成员的正常编辑。
	if rejectInvalidDriftRules(c, channel.DriftIgnoreRules) {
		return
	}
	oldChannel, getErr := model.GetChannelById(channel.Id)
	if getErr != nil || oldChannel.Tag != channel.Tag || (channel.Tag != "" && oldChannel.Type != channel.Type) {
		if err = model.CheckTagTypeConsistency(channel.Tag, channel.Type, channel.Id); err != nil {
			common.APIRespondWithError(c, http.StatusOK, err)
			return
		}
	}
	canonicalized := map[string]string{}
	if channel.Models != "" {
		fallbackMapping := ""
		if getErr == nil && oldChannel != nil {
			fallbackMapping = oldChannel.GetModelMapping()
		}
		canonicalized = canonicalizeChannelModels(&channel, fallbackMapping)
	}
	if channel.Models == "" {
		err = channel.Update(false)
	} else {
		err = channel.Update(true)
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 保存后异步为渠道模型补齐元信息与价格（OpenRouter 自带列表优先，其余走 models.dev），免去手动同步。
	AutoSyncChannelCatalog(channel.Type, channel.Models, "channel update")
	model.TriggerChannelBrandIconFetch(&channel)
	channel.HideKey()
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"message":       "",
		"data":          channel,
		"canonicalized": canonicalized,
	})
}

func BatchUpdateChannelsAzureApi(c *gin.Context) {
	var params model.BatchChannelsParams
	err := c.ShouldBindJSON(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if params.Ids == nil || len(params.Ids) == 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelIDsRequired, "ids must not be empty", nil)
		return
	}
	var count int64
	count, err = model.BatchUpdateChannelsAzureApi(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":    count,
		"success": true,
		"message": "Updated successfully",
	})
}

func BatchDelModelChannels(c *gin.Context) {
	var params model.BatchChannelsParams
	err := c.ShouldBindJSON(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if params.Ids == nil || len(params.Ids) == 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelIDsRequired, "ids must not be empty", nil)
		return
	}

	var count int64
	count, err = model.BatchDelModelChannels(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":    count,
		"success": true,
		"message": "Updated successfully",
	})
}

func BatchAddUserGroupToChannels(c *gin.Context) {
	var params model.BatchChannelsParams
	err := c.ShouldBindJSON(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if params.Ids == nil || len(params.Ids) == 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelIDsRequired, "ids must not be empty", nil)
		return
	}

	if params.Value == "" {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelUserGroupRequired, "user group must not be empty", nil)
		return
	}

	var count int64
	count, err = model.BatchAddUserGroupToChannels(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":    count,
		"success": true,
		"message": "User groups added successfully",
	})
}

func BatchAddModelToChannels(c *gin.Context) {
	var params model.BatchChannelsParams
	err := c.ShouldBindJSON(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if params.Ids == nil || len(params.Ids) == 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelIDsRequired, "ids must not be empty", nil)
		return
	}

	if params.Value == "" {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelModelRequired, "model must not be empty", nil)
		return
	}

	var count int64
	count, err = model.BatchAddModelToChannels(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data":    count,
		"success": true,
		"message": "Models added successfully",
	})
}

func BatchDeleteChannel(c *gin.Context) {
	var params model.BatchChannelsParams
	err := c.ShouldBindJSON(&params)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}

	if params.Ids == nil || len(params.Ids) == 0 {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelIDsRequired, "ids must not be empty", nil)
		return
	}

	count, err := model.BatchDeleteChannel(params.Ids)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    count,
	})
}

// invalidChannelIDError replaces the strconv message for a malformed :id.
func invalidChannelIDError() error {
	return common.NewAPIError(common.APICodeInvalidChannelID, "invalid channel id", nil)
}

// channelLookupError gives a missing channel a stable code; other lookup errors pass through.
func channelLookupError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return common.NewAPIError(common.APICodeChannelNotFound, "channel not found", nil)
	}
	return err
}

// canonicalizeChannelModels rewrites catalog aliases in channel.Models to their canonical names and
// maps them back to the upstream name (see model.CanonicalizeChannelModels). fallbackMapping is the
// stored model_mapping, used when the request leaves model_mapping out. It never blocks a save: on
// any error the channel is kept as sent. Returns alias -> canonical for each rewritten model.
func canonicalizeChannelModels(channel *model.Channel, fallbackMapping string) map[string]string {
	changes := map[string]string{}
	if channel.Models == "" {
		return changes
	}
	mapping := fallbackMapping
	if channel.ModelMapping != nil {
		mapping = *channel.ModelMapping
	}
	resolver, err := model.NewCanonicalResolver()
	if err != nil {
		logger.SysError("canonicalize channel models: load catalog failed: " + err.Error())
		return changes
	}
	models, newMapping, changes, err := model.CanonicalizeChannelModels(resolver, channel.Models, mapping)
	if err != nil {
		logger.SysError("canonicalize channel models: " + err.Error())
		return map[string]string{}
	}
	if len(changes) == 0 {
		return changes
	}
	channel.Models = models
	channel.ModelMapping = &newMapping
	return changes
}
