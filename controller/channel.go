package controller

import (
	"errors"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
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

// GetChannelUpstreamModels lists what the channel's upstream returned at the last model check,
// with the public model each name belongs to and whether this channel supplies it.
func GetChannelUpstreamModels(c *gin.Context) {
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
	list, err := model.GetChannelUpstreamModels(channel)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": list})
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
	if rejectInvalidPauseRules(c, channel.PauseRules) {
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
	ids := make([]int, 0, len(channels))
	supplied := 0
	for i := range channels {
		model.TriggerChannelBrandIconFetch(&channels[i])
		ids = append(ids, channels[i].Id)
		supplied += autoSupplyNewChannel(&channels[i])
	}
	// ids lets the admin UI open the new channel's page (one id per key line in batch mode);
	// supplies is how many public models the new channels were attached to.
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"ids": ids, "supplies": supplied},
	})
}

// autoSupplyNewChannel attaches a new channel to the public models it lists while supplies
// route requests, so it serves them without a visit to each model's page. It returns how many
// supplies it created; a failure is logged and leaves the channel as it is.
func autoSupplyNewChannel(channel *model.Channel) int {
	if !config.ModelSupplyEnabled {
		return 0
	}
	supplies, err := model.AutoSupplyChannel(channel, model.SplitChannelModels(channel.Models))
	if err != nil {
		logger.SysError("auto supply failed for channel " + channel.Name + ": " + err.Error())
		return 0
	}
	return len(supplies)
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
	if rejectInvalidPauseRules(c, channel.PauseRules) {
		return
	}
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
	sync := model.ChannelSupplySync{}
	if getErr == nil && channel.Models != "" {
		sync = syncChannelSupplies(oldChannel, channel.Id)
	}
	channel.HideKey()
	// supplies counts the public models the edit attached the channel to; supplies_updated and
	// supplies_removed the supplies made from the model list that followed the edit.
	c.JSON(http.StatusOK, gin.H{
		"success":          true,
		"message":          "",
		"data":             channel,
		"supplies":         sync.Attached,
		"supplies_updated": sync.Updated,
		"supplies_removed": sync.Removed,
	})
}

// syncChannelSupplies carries an edit of the channel's model list or mapping to its supplies
// while supplies route requests (see model.SyncChannelSupplies). The channel is read back so the
// supplies take its stored list, mapping, priority and weight.
func syncChannelSupplies(before *model.Channel, channelId int) model.ChannelSupplySync {
	if !config.ModelSupplyEnabled {
		return model.ChannelSupplySync{}
	}
	stored, err := model.GetChannelById(channelId)
	if err != nil {
		logger.SysError("supply sync failed for channel #" + strconv.Itoa(channelId) + ": " + err.Error())
		return model.ChannelSupplySync{}
	}
	sync, err := model.SyncChannelSupplies(before, stored)
	if err != nil {
		logger.SysError("supply sync failed for channel " + stored.Name + ": " + err.Error())
	}
	return sync
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
