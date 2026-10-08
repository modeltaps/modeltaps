package model

import (
	"math"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/datatypes"
)

// roundRatio 消除换算引入的浮点尾噪（如 1.2500000000000002），保留 6 位小数。
// 6 位对应 $0.000002/1M 的粒度，远超任何真实定价精度需求。
func roundRatio(x float64) float64 {
	return math.Round(x*1e6) / 1e6
}

// RatioFromUSDPerToken 把 OpenRouter 的「USD/token」换算为内部倍率。
// 基准：1 倍率 = $0.002 / 1K tokens = $0.000002 / token（DollarRate=0.002）。
// 故 倍率 = USD每token ÷ (DollarRate/1000)。保留全精度，不做截断。
func RatioFromUSDPerToken(usdPerToken float64) float64 {
	if usdPerToken <= 0 {
		return 0
	}
	return usdPerToken / (DollarRate / 1000)
}

// RatioFromUSDPerMillion 把「USD/1M tokens」换算为内部倍率（models.dev 用此单位）。
func RatioFromUSDPerMillion(usdPerMillion float64) float64 {
	return RatioFromUSDPerToken(usdPerMillion / 1_000_000)
}

// newRatioPrice 构造一条 token 计费 Price；cacheReadRatio>0 时写入 ExtraRatios（相对 input 的倍数）。
// 供 OpenRouter 直连与 models.dev 目录两条路径共用。
func newRatioPrice(modelID string, channelType int, inputRatio, outputRatio, cacheReadRatio float64) *Price {
	p := &Price{
		Model:       modelID,
		Type:        TokensPriceType,
		ChannelType: channelType,
		Input:       roundRatio(inputRatio),
		Output:      roundRatio(outputRatio),
	}
	if cacheReadRatio > 0 {
		jt := datatypes.NewJSONType(map[string]float64{
			config.UsageExtraCachedRead: roundRatio(cacheReadRatio),
		})
		p.ExtraRatios = &jt
	}
	return p
}

// PriceFromOpenRouter 由 OpenRouter /v1/models 的 pricing 字段构造一条 Price。
// promptUSD/completionUSD/cacheReadUSD 均为 USD/token。
// ChannelType 固定为 OpenRouter，使前端「供应商」列显示 OpenRouter 而非「未知」。
func PriceFromOpenRouter(modelID string, promptUSD, completionUSD, cacheReadUSD float64) *Price {
	cacheReadRatio := 0.0
	if promptUSD > 0 && cacheReadUSD > 0 {
		cacheReadRatio = cacheReadUSD / promptUSD
	}
	return newRatioPrice(modelID, config.ChannelTypeOpenRouter,
		RatioFromUSDPerToken(promptUSD), RatioFromUSDPerToken(completionUSD), cacheReadRatio)
}

// UpsertPrices 按渠道同步语义落库：不存在的插入、已存在且未锁定的更新、锁定的保留，
// 且**只动传入列表里的模型**——不会像 overwrite 那样误删其它渠道的价格。
func (p *Pricing) UpsertPrices(pricing []*Price) error {
	var toInsert []*Price
	var toUpdate []*Price
	var updateNames []string

	for _, price := range pricing {
		existing, ok := p.Prices[price.Model]
		if !ok {
			toInsert = append(toInsert, price)
		} else if !existing.Locked {
			toUpdate = append(toUpdate, price)
			updateNames = append(updateNames, price.Model)
		}
	}

	if len(toInsert) == 0 && len(toUpdate) == 0 {
		return nil
	}

	tx := DB.Begin()
	if len(toUpdate) > 0 {
		if err := DeletePricesByModelNameAndNotLock(tx, updateNames); err != nil {
			tx.Rollback()
			return err
		}
		if err := InsertPrices(tx, toUpdate); err != nil {
			tx.Rollback()
			return err
		}
	}
	if len(toInsert) > 0 {
		if err := InsertPrices(tx, toInsert); err != nil {
			tx.Rollback()
			return err
		}
	}
	tx.Commit()

	return p.Init()
}
