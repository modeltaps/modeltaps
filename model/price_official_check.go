package model

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
)

// The built-in official prices (price_official.go) are kept by hand from the vendors' pricing
// pages. After each models.dev sync they are compared with the vendor's own entry there; a model
// whose input or output price differs by priceAlertThreshold or more is listed as a
// price_source alert under OfficialPriceSource, to be checked on the vendor's page. Nothing is
// changed: models.dev lags or errs too, and the official table only changes by a release.

// OfficialPriceSource is the alert subject of the built-in official prices.
const OfficialPriceSource = "source:official"

// officialCatalogProviders is the models.dev provider that is each official vendor itself.
var officialCatalogProviders = map[int]string{
	config.ChannelTypeOpenAI:    "openai",
	config.ChannelTypeAnthropic: "anthropic",
	config.ChannelTypeGemini:    "google",
	config.ChannelTypeDeepseek:  "deepseek",
	config.ChannelTypeMistral:   "mistral",
}

// officialOffPeakShares are the vendors whose models.dev entries may show the off-peak price
// instead of the list price: the share of the list price it is. DeepSeek charges 50% off peak
// hours, and models.dev lists that price (checked 2026-11-20 against
// https://api-docs.deepseek.com/quick_start/pricing).
var officialOffPeakShares = map[int]float64{
	config.ChannelTypeDeepseek: 0.5,
}

// officialPriceDiff is one model whose official price differs from the vendor's models.dev entry.
type officialPriceDiff struct {
	Model                         string
	OfficialInput, OfficialOutput float64
	CatalogInput, CatalogOutput   float64
}

func (d officialPriceDiff) item() string {
	return strings.Join([]string{"official", d.Model, num(d.OfficialInput), num(d.OfficialOutput), num(d.CatalogInput), num(d.CatalogOutput)}, "|")
}

// comparableOfficialPrice reports whether the official price has plain per-token input and
// output prices that models.dev states the same way. Free models and image-token models (whose
// base input is image tokens, text input an extra) are left out.
func comparableOfficialPrice(p officialTokenPrice) bool {
	if p.free {
		return false
	}
	_, imageTokens := p.extra[config.UsageExtraInputTextTokens]
	return !imageTokens
}

// officialPriceDiffs compares the official prices in effect at now (and those scheduled later,
// which models.dev may already show) with the vendors' models.dev entries.
func officialPriceDiffs(data map[string]catalogProvider, now time.Time) []officialPriceDiff {
	later := map[string][]officialTokenPrice{}
	for _, change := range scheduledOfficialPrices() {
		if now.Before(change.From) {
			later[change.Model] = append(later[change.Model], change.Price)
		}
	}
	var out []officialPriceDiff
	for name, official := range officialTokenPricesAt(now) {
		provider, ok := officialCatalogProviders[official.channelType]
		if !ok || !comparableOfficialPrice(official) {
			continue
		}
		entry, ok := data[provider].Models[name]
		if !ok || entry.Cost.Input == nil {
			continue
		}
		catalogIn := *entry.Cost.Input
		catalogOut, hasOut := official.output, entry.Cost.Output != nil
		if hasOut {
			catalogOut = *entry.Cost.Output
		}
		// differsAt compares p scaled by share; without an output price on models.dev only the
		// input is compared.
		differsAt := func(p officialTokenPrice, share float64) bool {
			in, out := p.input*share, p.output*share
			c := PriceChange{OldInput: in, OldOutput: out, NewInput: catalogIn, NewOutput: out}
			if hasOut {
				c.NewOutput = catalogOut
			}
			return significantPriceChange(c)
		}
		differs := func(p officialTokenPrice) bool {
			share, offPeak := officialOffPeakShares[p.channelType]
			return differsAt(p, 1) && (!offPeak || differsAt(p, share))
		}
		if !differs(official) {
			continue
		}
		matchesLater := false
		for _, p := range later[name] {
			matchesLater = matchesLater || !differs(p)
		}
		if matchesLater {
			continue
		}
		out = append(out, officialPriceDiff{Model: name, OfficialInput: official.input, OfficialOutput: official.output, CatalogInput: catalogIn, CatalogOutput: catalogOut})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Model < out[j].Model })
	return out
}

// CheckOfficialPrices compares the official prices with a models.dev download, keeps the
// differences as the official prices' alert and tells the new ones once.
func CheckOfficialPrices(data map[string]catalogProvider, now time.Time) {
	if DB == nil || len(data) == 0 {
		return
	}
	diffs := officialPriceDiffs(data, now)
	items := make([]string, len(diffs))
	for i, d := range diffs {
		items[i] = d.item()
	}
	fresh, err := ReportAlert(PriceSourceAlertKind, OfficialPriceSource, items)
	if err != nil {
		logger.SysError("official price check failed: " + err.Error())
		return
	}
	if len(fresh) == 0 {
		return
	}
	isFresh := make(map[string]bool, len(fresh))
	for _, item := range fresh {
		isFresh[item] = true
	}
	var lines []string
	for _, d := range diffs {
		if isFresh[d.item()] {
			lines = append(lines, fmt.Sprintf("- %s: built in $%s / $%s, models.dev $%s / $%s per 1M input / output tokens",
				d.Model, num(d.OfficialInput), num(d.OfficialOutput), num(d.CatalogInput), num(d.CatalogOutput)))
		}
	}
	logger.SysLog(fmt.Sprintf("official price check: %d model(s) differ from models.dev", len(diffs)))
	notify.SendKind(notify.KindPriceSource,
		fmt.Sprintf("Built-in official prices differ from models.dev for %d model(s)", len(lines)),
		"Nothing was changed. Check these models on the vendor's pricing page; if the vendor changed "+
			"the price, the built-in table needs an update.\n\n"+strings.Join(lines, "\n"))
}
