package controller

import (
	"github.com/modeltaps/modeltaps/common"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providersBase "github.com/modeltaps/modeltaps/providers/base"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// channelPriceSyncProvider 由能列出「价格换算不了的模型」的渠道实现（如 OpenRouter）。
type channelPriceSyncProvider interface {
	GetModelPriceSync() ([]*model.Price, []model.UnconvertiblePrice, error)
}

// AutoSyncChannelCatalog 在渠道保存后异步为渠道模型补齐元信息与价格，无需手动同步或定价：
// ① OpenRouter 渠道先重拉一次自带的价格与元信息（channel_pricing.auto_sync，默认开）；
// ② 其余仍缺价格或目录行的模型再从 models.dev 全量匹配补齐（catalog_pricing.auto_sync，默认开）。
// best-effort，失败仅日志，不阻断保存。
func AutoSyncChannelCatalog(channelType int, models string, reason string) {
	syncOpenRouter := ChannelTypeHasOwnPricing(channelType) && viper.GetBool("channel_pricing.auto_sync")
	syncCatalog := viper.GetBool("catalog_pricing.auto_sync")
	if !syncOpenRouter && !syncCatalog {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SysError(fmt.Sprintf("AutoSyncChannelCatalog panic: %v\n%s", r, debug.Stack()))
			}
		}()
		if syncOpenRouter {
			n, err := SyncAllChannelsOwnPricing()
			if err != nil {
				logger.SysError("auto sync channel pricing (" + reason + ") failed: " + err.Error())
			} else if n > 0 {
				logger.SysLog(fmt.Sprintf("auto sync channel pricing (%s) done: %d models", reason, n))
			}
		}
		if !syncCatalog {
			return
		}
		missing := model.CatalogMissingModels(model.SplitChannelModels(models))
		if len(missing) == 0 {
			return
		}
		result, err := model.SyncCatalogForModels(viper.GetString("catalog_pricing.url"), missing)
		if err != nil {
			logger.SysError("auto sync models.dev catalog (" + reason + ") failed: " + err.Error())
			return
		}
		logger.SysLog(fmt.Sprintf("auto sync models.dev catalog (%s) done: %d prices, unconvertible %v, unmatched %v",
			reason, result.Synced, result.Unconvertible, result.Unmatched))
	}()
}

// ownPricingChannelTypes 列出「上游 API 自带价格与元数据」的渠道类型及其元数据来源标记。
// 顺序即优先级：同一模型多个渠道都给价时，排在前面的类型胜出（OpenRouter 覆盖面最广，放首位）。
var ownPricingChannelTypes = []struct {
	channelType int
	infoSource  string
}{
	{config.ChannelTypeOpenRouter, model.ModelInfoSourceOpenRouter},
	{config.ChannelTypeVercelAIGateway, model.ModelInfoSourceVercel},
}

// ChannelTypeHasOwnPricing 判断渠道类型是否自带价格表（可自动同步）。
func ChannelTypeHasOwnPricing(channelType int) bool {
	return modelInfoSourceFor(channelType) != ""
}

func modelInfoSourceFor(channelType int) string {
	for _, t := range ownPricingChannelTypes {
		if t.channelType == channelType {
			return t.infoSource
		}
	}
	return ""
}

// SyncAllChannelsOwnPricing 遍历所有自带价格表的渠道（OpenRouter、Vercel AI Gateway），
// 拉取真实价格去重后 upsert；元数据按来源分别落库。
// 无需 gin.Context（provider 在 nil context 下仅做 GET /v1/models），供启动与 cron 调用。
func SyncAllChannelsOwnPricing() (int, error) {
	channels, err := model.GetAllChannels()
	if err != nil {
		return 0, err
	}

	deduped := make(map[string]*model.Price)
	infoDedup := make(map[string]map[string]*model.ModelInfo) // source -> model -> info
	// 按类型优先级遍历，先到先得：后面的类型不覆盖已有价格。
	for _, t := range ownPricingChannelTypes {
		for _, ch := range channels {
			if ch.Type != t.channelType {
				continue
			}
			provider := providers.GetProvider(ch, nil)
			if provider == nil {
				continue
			}
			prices, unconvertible, err := getChannelPriceSync(provider)
			if err != nil {
				logger.SysError("sync channel pricing failed for channel " + ch.Name + ": " + err.Error())
				continue
			}
			if len(unconvertible) > 0 {
				logger.SysLog(fmt.Sprintf("sync channel pricing for channel %s: %d models with unconvertible prices",
					ch.Name, len(unconvertible)))
			}
			for _, p := range prices {
				if _, exists := deduped[p.Model]; !exists {
					deduped[p.Model] = p
				}
			}
			if infoProvider, ok := provider.(providersBase.ModelInfoListInterface); ok {
				if infos, err := infoProvider.GetModelInfoList(); err == nil {
					if infoDedup[t.infoSource] == nil {
						infoDedup[t.infoSource] = make(map[string]*model.ModelInfo)
					}
					for _, info := range infos {
						infoDedup[t.infoSource][info.Model] = info
					}
				}
			}
		}
	}

	if len(deduped) == 0 {
		return 0, nil
	}

	list := make([]*model.Price, 0, len(deduped))
	for _, p := range deduped {
		list = append(list, p)
	}
	// 启动、每日定时与保存渠道都走这里：已有价格的变动按 PriceSyncAutoApply 处理并通知。
	if _, err := model.PricingInstance.ApplyAutoPriceSync(list, "channel price lists"); err != nil {
		return 0, err
	}

	// 元数据同样先到先得：OpenRouter 已写过的模型，Vercel 不再覆盖。
	seenInfo := make(map[string]bool)
	for _, t := range ownPricingChannelTypes {
		byModel := infoDedup[t.infoSource]
		if len(byModel) == 0 {
			continue
		}
		infos := make([]*model.ModelInfo, 0, len(byModel))
		for name, info := range byModel {
			if seenInfo[name] {
				continue
			}
			seenInfo[name] = true
			infos = append(infos, info)
		}
		if err := model.UpsertModelInfos(infos, t.infoSource); err != nil {
			logger.SysError(t.infoSource + " model info upsert failed: " + err.Error())
		}
	}

	return len(list), nil
}

// channelPriceSyncRequest 是渠道表单提交的渠道，外加价格同步选项：
// sync_preview 只返回与价格表的差异不写入；sync_models 只写入勾选的模型（两者都缺省时全部写入）。
type channelPriceSyncRequest struct {
	model.Channel
	SyncPreview bool     `json:"sync_preview"`
	SyncModels  []string `json:"sync_models"`
}

// SyncChannelPricing 从渠道上游 API 拉取真实价格。
// 仅支持「自带价格」的渠道（实现 ModelPriceListInterface，如 OpenRouter）。
// 预览返回逐模型差异（新增 / 涨价 / 降价）与亏本分组；写入语义为 upsert：新增缺失、更新未锁定、
// 保留锁定，不影响其它渠道的价格。
func SyncChannelPricing(c *gin.Context) {
	req := &channelPriceSyncRequest{}
	if err := c.ShouldBindJSON(req); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	channel := &req.Channel
	keys := strings.Split(channel.Key, "\n")
	channel.Key = keys[0]

	if channel.Key == "" {
		if channel.Id == 0 {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelKeyRequired, "key is required", nil)
			return
		}
		stored, err := model.GetChannelById(channel.Id)
		if err != nil {
			common.APIRespondWithError(c, http.StatusOK, channelLookupError(err))
			return
		}
		// 表单里带了分组 / 模型时优先用正在编辑的值，亏本提示与预览范围反映未保存的修改。
		if channel.Group != "" {
			stored.Group = channel.Group
		}
		if channel.Models != "" {
			stored.Models = channel.Models
		}
		if channel.CostRatio != nil {
			stored.CostRatio = channel.CostRatio
		}
		channel = stored
	}

	provider := providers.GetProvider(channel, c)
	if provider == nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelProviderNotFound, "provider not found", nil)
		return
	}

	if _, ok := provider.(providersBase.ModelPriceListInterface); !ok {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelPricingSyncUnsupported, "channel does not support pricing sync", nil)
		return
	}

	prices, unconvertible, err := getChannelPriceSync(provider)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if req.SyncPreview {
		changes := model.PricingInstance.DiffPrices(onlyChannelModels(prices, channel.Models))
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "",
			"data": gin.H{
				"changes":       changes,
				"losses":        model.ChannelGroupLosses(channel),
				"unmatched":     findUnpricedChannelModels(channel.Models, prices),
				"unconvertible": unconvertible,
			},
		})
		return
	}

	synced := len(prices)
	if req.SyncModels != nil {
		synced, err = model.PricingInstance.UpsertSelectedPrices(prices, req.SyncModels)
	} else {
		err = model.PricingInstance.UpsertPrices(prices)
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 顺带同步模型元信息（名称/上下文/模态），使模型详情有内容（best-effort，不阻断价格同步）
	if infoProvider, ok := provider.(providersBase.ModelInfoListInterface); ok {
		if infos, err := infoProvider.GetModelInfoList(); err == nil {
			source := modelInfoSourceFor(channel.Type)
			if source == "" {
				source = model.ModelInfoSourceOpenRouter
			}
			if err := model.UpsertModelInfos(infos, source); err != nil {
				logger.SysError("channel model info upsert failed: " + err.Error())
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"synced": synced,
			// 渠道里配置了、但上游没有精确同名价格的模型(多为 id 带版本/别名)，
			// 前端据此提示，避免用户误以为同步失败。
			"unmatched": findUnpricedChannelModels(channel.Models, prices),
			// 上游有、但计价口径换算不了（按次 / 按音频时长 / 按音频 token）的模型，未写价格。
			"unconvertible": unconvertible,
		},
	})
}

// onlyChannelModels 预览只列渠道配置了的模型（自带价格表的网关有数百个模型）；
// 渠道未配置模型时列出全部。
func onlyChannelModels(prices []*model.Price, modelsCSV string) []*model.Price {
	names := model.SplitChannelModels(modelsCSV)
	if len(names) == 0 {
		return prices
	}
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	out := make([]*model.Price, 0, len(names))
	for _, p := range prices {
		if want[p.Model] {
			out = append(out, p)
		}
	}
	return out
}

// getChannelPriceSync 拉取渠道自带价格；渠道能列出换算不了的模型时一并返回。
func getChannelPriceSync(provider providersBase.ProviderInterface) ([]*model.Price, []model.UnconvertiblePrice, error) {
	if syncProvider, ok := provider.(channelPriceSyncProvider); ok {
		return syncProvider.GetModelPriceSync()
	}
	priceProvider, ok := provider.(providersBase.ModelPriceListInterface)
	if !ok {
		return nil, nil, fmt.Errorf("channel does not support pricing sync")
	}
	prices, err := priceProvider.GetModelPriceList()
	return prices, nil, err
}

// findUnpricedChannelModels 返回 modelsCSV 中未出现在 prices 里的模型名(去空白)。
func findUnpricedChannelModels(modelsCSV string, prices []*model.Price) []string {
	priced := make(map[string]bool, len(prices))
	for _, p := range prices {
		priced[p.Model] = true
	}
	var unmatched []string
	for _, m := range strings.Split(modelsCSV, ",") {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if !priced[m] {
			unmatched = append(unmatched, m)
		}
	}
	return unmatched
}
