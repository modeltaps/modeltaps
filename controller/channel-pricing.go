package controller

import (
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
	syncOpenRouter := channelType == config.ChannelTypeOpenRouter && viper.GetBool("channel_pricing.auto_sync")
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
			n, err := SyncAllOpenRouterChannelsPricing()
			if err != nil {
				logger.SysError("auto sync OpenRouter pricing (" + reason + ") failed: " + err.Error())
			} else if n > 0 {
				logger.SysLog(fmt.Sprintf("auto sync OpenRouter pricing (%s) done: %d models", reason, n))
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

// SyncAllOpenRouterChannelsPricing 遍历所有 OpenRouter 渠道，拉取真实价格去重后 upsert。
// 无需 gin.Context（provider 在 nil context 下仅做无鉴权的 GET /v1/models），供 cron 调用。
func SyncAllOpenRouterChannelsPricing() (int, error) {
	channels, err := model.GetAllChannels()
	if err != nil {
		return 0, err
	}

	deduped := make(map[string]*model.Price)
	infoDedup := make(map[string]*model.ModelInfo)
	for _, ch := range channels {
		if ch.Type != config.ChannelTypeOpenRouter {
			continue
		}
		provider := providers.GetProvider(ch, nil)
		if provider == nil {
			continue
		}
		prices, unconvertible, err := getChannelPriceSync(provider)
		if err != nil {
			logger.SysError("sync OpenRouter pricing failed for channel " + ch.Name + ": " + err.Error())
			continue
		}
		if len(unconvertible) > 0 {
			logger.SysLog(fmt.Sprintf("sync OpenRouter pricing for channel %s: %d models with unconvertible prices",
				ch.Name, len(unconvertible)))
		}
		for _, p := range prices {
			deduped[p.Model] = p
		}
		if infoProvider, ok := provider.(providersBase.ModelInfoListInterface); ok {
			if infos, err := infoProvider.GetModelInfoList(); err == nil {
				for _, info := range infos {
					infoDedup[info.Model] = info
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
	if err := model.PricingInstance.UpsertPrices(list); err != nil {
		return 0, err
	}

	if len(infoDedup) > 0 {
		infos := make([]*model.ModelInfo, 0, len(infoDedup))
		for _, info := range infoDedup {
			infos = append(infos, info)
		}
		if err := model.UpsertModelInfos(infos, model.ModelInfoSourceOpenRouter); err != nil {
			logger.SysError("OpenRouter model info upsert failed: " + err.Error())
		}
	}

	return len(list), nil
}

// SyncChannelPricing 从渠道上游 API 拉取真实价格并落库。
// 仅支持「自带价格」的渠道（实现 ModelPriceListInterface，如 OpenRouter）。
// 落库语义为 upsert：新增缺失、更新未锁定、保留锁定，不影响其它渠道的价格。
func SyncChannelPricing(c *gin.Context) {
	channel := &model.Channel{}
	if err := c.ShouldBindJSON(channel); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}
	keys := strings.Split(channel.Key, "\n")
	channel.Key = keys[0]

	if channel.Key == "" {
		if channel.Id == 0 {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "key is required"})
			return
		}
		var err error
		channel, err = model.GetChannelById(channel.Id)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
			return
		}
	}

	provider := providers.GetProvider(channel, c)
	if provider == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "provider not found"})
		return
	}

	if _, ok := provider.(providersBase.ModelPriceListInterface); !ok {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "channel does not support pricing sync"})
		return
	}

	prices, unconvertible, err := getChannelPriceSync(provider)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	if err := model.PricingInstance.UpsertPrices(prices); err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": err.Error()})
		return
	}

	// 顺带同步模型元信息（名称/上下文/模态），使模型详情有内容（best-effort，不阻断价格同步）
	if infoProvider, ok := provider.(providersBase.ModelInfoListInterface); ok {
		if infos, err := infoProvider.GetModelInfoList(); err == nil {
			if err := model.UpsertModelInfos(infos, model.ModelInfoSourceOpenRouter); err != nil {
				logger.SysError("channel model info upsert failed: " + err.Error())
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"synced": len(prices),
			// 渠道里配置了、但上游没有精确同名价格的模型(多为 id 带版本/别名)，
			// 前端据此提示，避免用户误以为同步失败。
			"unmatched": findUnpricedChannelModels(channel.Models, prices),
			// 上游有、但计价口径换算不了（按次 / 按音频时长 / 按音频 token）的模型，未写价格。
			"unconvertible": unconvertible,
		},
	})
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
