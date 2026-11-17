package model

import (
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// CostSourceOfficial is a channel cost taken from the built-in official price list: the channel is
// the model's own vendor at its own address (an OpenAI channel serving an OpenAI model with no base
// URL of its own), so it charges the vendor's list price.
const CostSourceOfficial = "official"

// Official prices are read on every billed request with the cost floor on; they only change with a
// release or an announced price change, so the index is rebuilt every few minutes.
const officialCostIndexTTL = 10 * time.Minute

var officialCostIndex struct {
	sync.Mutex
	builtAt time.Time
	prices  map[string]*Price
}

func officialPriceFor(modelName string) *Price {
	officialCostIndex.Lock()
	defer officialCostIndex.Unlock()
	if officialCostIndex.prices == nil || time.Since(officialCostIndex.builtAt) > officialCostIndexTTL {
		index := map[string]*Price{}
		for _, p := range GetOfficialPrices() {
			index[p.Model] = p
		}
		officialCostIndex.prices, officialCostIndex.builtAt = index, time.Now()
	}
	return officialCostIndex.prices[modelName]
}

// isVendorChannel reports a channel that sends to its type's own address: no base URL of its own.
// A relay of the same type (an OpenAI-compatible gateway on an OpenAI channel) sets one.
func isVendorChannel(ch *Channel) bool {
	return ch != nil && strings.TrimSpace(ch.GetBaseURL()) == ""
}

// OfficialChannelCost is the official list price of upstreamModel when ch is that model's vendor at
// its own address, in the same price type as the catalog price; nil otherwise.
func OfficialChannelCost(catalog *Price, ch *Channel, upstreamModel string) *Price {
	if catalog == nil || !isVendorChannel(ch) || upstreamModel == "" {
		return nil
	}
	official := officialPriceFor(upstreamModel)
	if official == nil || official.ChannelType != ch.Type || official.Type != catalog.Type {
		return nil
	}
	return clonePrice(official)
}

// OfficialChannelPrice is OfficialChannelCost as a channel price for the console, in USD per
// million tokens (or per request), for channels that have no price list of their own.
func OfficialChannelPrice(ch *Channel, upstreamModel string) *ChannelPrice {
	if !isVendorChannel(ch) || upstreamModel == "" {
		return nil
	}
	official := officialPriceFor(upstreamModel)
	if official == nil || official.ChannelType != ch.Type {
		return nil
	}
	cp := &ChannelPrice{Model: upstreamModel, Official: true}
	switch official.Type {
	case TokensPriceType:
		cp.Input = official.Input * modelsDevCostPerMillionBase
		cp.Output = official.Output * modelsDevCostPerMillionBase
		if official.ExtraRatios != nil {
			extra := official.ExtraRatios.Data()
			for _, key := range []string{config.UsageExtraCache, config.UsageExtraCachedRead} {
				if r, ok := extra[key]; ok {
					cp.CacheRead = cp.Input * r
				}
			}
			if r, ok := extra[config.UsageExtraCachedWrite]; ok {
				cp.CacheWrite = cp.Input * r
			}
		}
	case TimesPriceType:
		cp.Request = official.Input * DollarRate
	default:
		return nil
	}
	return cp
}
