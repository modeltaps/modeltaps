package controller

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// 目录条目状态：别名优先，其次隐藏，其余按是否有启用渠道可路由判定。
const (
	modelInfoStateAlias    = "alias"
	modelInfoStateVisible  = "visible"
	modelInfoStateUnrouted = "unrouted"
	modelInfoStateHidden   = "hidden"
)

// boundChannel 是目录页展示的渠道摘要（仅管理员可见）。
type boundChannel struct {
	Id       int    `json:"id"`
	Name     string `json:"name"`
	Type     int    `json:"type"`
	Status   int    `json:"status"`
	Group    string `json:"group"`
	Priority int64  `json:"priority"`
	Weight   uint   `json:"weight"`
}

// modelInfoCatalogItem 在目录行上附加计算字段，不落库。
// VendorSlug 与公开接口的 vendor.slug 同源：厂商展示名可改，按厂商归类只能认 slug。
type modelInfoCatalogItem struct {
	*model.ModelInfo
	State         string         `json:"state"`
	VendorSlug    string         `json:"vendor_slug"`
	BoundChannels []boundChannel `json:"bound_channels"`
}

// boundChannelsByModel 建立「模型名 → 渠道」索引（含禁用渠道，按 status 区分），避免逐行查库。
func boundChannelsByModel() (map[string][]boundChannel, error) {
	channels, err := model.GetAllChannels()
	if err != nil {
		return nil, err
	}
	index := make(map[string][]boundChannel)
	for _, channel := range channels {
		summary := boundChannel{Id: channel.Id, Name: channel.Name, Type: channel.Type, Status: channel.Status, Group: channel.Group}
		if channel.Priority != nil {
			summary.Priority = *channel.Priority
		}
		if channel.Weight != nil {
			summary.Weight = *channel.Weight
		}
		for _, name := range strings.Split(channel.Models, ",") {
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			index[name] = append(index[name], summary)
		}
	}
	return index, nil
}

// publicModelInfo 是公开列表的响应投影：保留价格页展示所需字段，
// 去掉纯管理字段（locked / vendor_id / alias_of）。
type publicModelInfo struct {
	Id               int    `json:"id"`
	Model            string `json:"model"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	ContextLength    int    `json:"context_length"`
	MaxTokens        int    `json:"max_tokens"`
	InputModalities  string `json:"input_modalities"`
	OutputModalities string `json:"output_modalities"`
	Tags             string `json:"tags"`
	SupportUrl       string `json:"support_url"`
	Capabilities     string `json:"capabilities"`
	Mode             string `json:"mode"`
	Endpoints        string `json:"endpoints"`
	Source           string `json:"source"`
	SyncedAt         int64  `json:"synced_at"`
	CreatedAt        int64  `json:"created_at"`
	UpdatedAt        int64  `json:"updated_at"`
}

// visibleModelInfos 收敛公开列表：只保留有启用渠道可路由、未隐藏的主名，隐藏行、无渠道行与
// 别名行不对外暴露。这里刻意不走 model.IsCatalogVisible——catalog.enforce_hidden 是 relay 侧的
// 应急回滚阀门，关闭它不应让公开价格页泄露被隐藏的目录行。
func visibleModelInfos(infos []*model.ModelInfo, routable map[string]map[string]bool) []*publicModelInfo {
	visible := make([]*publicModelInfo, 0, len(infos))
	for _, info := range infos {
		if info.Hidden || info.AliasOf != "" {
			continue
		}
		if _, ok := routable[info.Model]; !ok {
			continue
		}
		visible = append(visible, &publicModelInfo{
			Id:               info.Id,
			Model:            info.Model,
			Name:             info.Name,
			Description:      info.Description,
			ContextLength:    info.ContextLength,
			MaxTokens:        info.MaxTokens,
			InputModalities:  info.InputModalities,
			OutputModalities: info.OutputModalities,
			Tags:             info.Tags,
			SupportUrl:       info.SupportUrl,
			Capabilities:     info.Capabilities,
			Mode:             info.Mode,
			Endpoints:        info.Endpoints,
			Source:           info.Source,
			SyncedAt:         info.SyncedAt,
			CreatedAt:        info.CreatedAt,
			UpdatedAt:        info.UpdatedAt,
		})
	}
	return visible
}

// GetAllModelInfo 是公开的模型元信息列表：匿名的公开价格页与用户面板按模型名取
// 描述 / 模态 / 标签 / 能力，故不能收进 AdminAuth。隐藏行、无渠道行与别名行一律不暴露；
// 管理端要看整张目录请用 AdminAuth 下的 GET /api/model_info/catalog。
func GetAllModelInfo(c *gin.Context) {
	modelInfos, err := model.GetAllModelInfo()
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
		"data":    visibleModelInfos(modelInfos, model.ChannelGroup.GetModelsGroups()),
	})
}

// GetModelInfoCatalog 返回带计算字段的目录列表；渠道绑定与隐藏条目属管理员信息，
// 因此单独走 AdminAuth 路由。
func GetModelInfoCatalog(c *gin.Context) {
	modelInfos, err := model.GetAllModelInfo()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channelsByModel, err := boundChannelsByModel()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	items := make([]*modelInfoCatalogItem, 0, len(modelInfos))
	for _, info := range modelInfos {
		channels := channelsByModel[info.Model]
		if channels == nil {
			channels = []boundChannel{}
		}
		items = append(items, &modelInfoCatalogItem{
			ModelInfo:     info,
			State:         modelInfoState(info, countEnabledChannels(channels)),
			VendorSlug:    model.ModelOwnedBysInstance.GetSlug(info.VendorID),
			BoundChannels: channels,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    items,
	})
}

func countEnabledChannels(channels []boundChannel) int {
	count := 0
	for _, channel := range channels {
		if channel.Status == config.ChannelStatusEnabled {
			count++
		}
	}
	return count
}

func modelInfoState(info *model.ModelInfo, channelCount int) string {
	if info.AliasOf != "" {
		return modelInfoStateAlias
	}
	if info.Hidden {
		return modelInfoStateHidden
	}
	if channelCount == 0 {
		return modelInfoStateUnrouted
	}
	return modelInfoStateVisible
}

type hideModelInfoRequest struct {
	Ids    []int `json:"ids"`
	Hidden bool  `json:"hidden"`
}

// HideModelInfo 批量隐藏 / 取消隐藏目录条目。
func HideModelInfo(c *gin.Context) {
	req := hideModelInfoRequest{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if len(req.Ids) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "ids is required",
		})
		return
	}
	// 先取快照：隐藏状态决定模型对外是否可见，审计要能回答改前是什么。
	before, err := model.GetModelInfosByIds(req.Ids)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	affected, err := model.SetModelInfoHidden(req.Ids, req.Hidden)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	recordHideAudit(c.GetInt("id"), req.Hidden, before)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"affected": affected},
	})
}

// recordHideAudit 为每个真正改变了隐藏状态的条目写一条审计；状态未变的条目不记。
func recordHideAudit(actorId int, hidden bool, before []*model.ModelInfo) {
	action := model.ModelCatalogActionUnhide
	if hidden {
		action = model.ModelCatalogActionHide
	}
	for _, info := range before {
		if info.Hidden == hidden {
			continue
		}
		model.RecordModelCatalogAudit(actorId, action, info.Model,
			gin.H{"hidden": info.Hidden}, gin.H{"hidden": hidden})
	}
}

func GetModelInfo(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	modelInfo, err := model.GetModelInfo(id)
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
		"data":    modelInfo,
	})
}

func CreateModelInfo(c *gin.Context) {
	modelInfo := model.ModelInfo{}
	err := c.ShouldBindJSON(&modelInfo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !model.IsValidModelMode(modelInfo.Mode) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid mode",
		})
		return
	}
	if !model.IsValidModelCapabilities(modelInfo.Capabilities) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid capabilities",
		})
		return
	}
	if !model.IsValidModelEndpoints(modelInfo.Endpoints) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid endpoints",
		})
		return
	}
	if err := model.ValidateAliasTarget(modelInfo.Model, modelInfo.AliasOf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	existingModel, _ := model.GetModelInfoByModel(modelInfo.Model)
	if existingModel != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "model identifier already exists",
		})
		return
	}
	modelInfo.Source = model.ModelInfoSourceManual
	err = model.CreateModelInfo(&modelInfo)
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

func UpdateModelInfo(c *gin.Context) {
	modelInfo := model.ModelInfo{}
	err := c.ShouldBindJSON(&modelInfo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	if !model.IsValidModelMode(modelInfo.Mode) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid mode",
		})
		return
	}
	if !model.IsValidModelCapabilities(modelInfo.Capabilities) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid capabilities",
		})
		return
	}
	if !model.IsValidModelEndpoints(modelInfo.Endpoints) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "invalid endpoints",
		})
		return
	}
	if err := model.ValidateAliasTarget(modelInfo.Model, modelInfo.AliasOf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	existingModel, _ := model.GetModelInfoByModel(modelInfo.Model)
	if existingModel != nil && existingModel.Id != modelInfo.Id {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": "model identifier already exists",
		})
		return
	}
	// 改名会让指向旧主名的别名悬空：拒绝并列出引用方，由管理员先处理别名。
	if current, err := model.GetModelInfo(modelInfo.Id); err == nil && current.Model != modelInfo.Model {
		if refs, err := model.ModelAliasReferences(current.Model); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		} else if len(refs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "model is referenced by aliases: " + strings.Join(refs, ", "),
				"data":    refs,
			})
			return
		}
	}
	modelInfo.Source = model.ModelInfoSourceManual
	err = model.UpdateModelInfo(&modelInfo)
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

func DeleteModelInfo(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	// 删除主名同样会让别名悬空：拒绝并列出引用方。
	if current, err := model.GetModelInfo(id); err == nil {
		refs, err := model.ModelAliasReferences(current.Model)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}
		if len(refs) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "model is referenced by aliases: " + strings.Join(refs, ", "),
				"data":    refs,
			})
			return
		}
	}
	err = model.DeleteModelInfo(id)
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
