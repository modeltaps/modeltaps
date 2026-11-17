package relay_util

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// Cost floor billing (SupplyCostFloorBilling): a request is charged the higher of the quoted
// amount (catalog price × group ratio) and what the channel that served it charges (its price
// with its fee, no group ratio). The group ratio still sets the price of most requests; it only
// gives way when the channel costs more than that.

// How a request was charged, written to the log as billing.basis.
const (
	billingBasisQuote       = "quote"        // the quoted amount; the channel cost was lower
	billingBasisChannelCost = "channel_cost" // the channel cost, which was higher than the quote
	billingBasisCostUnknown = "cost_unknown" // the quoted amount; the channel cost is not known
)

type costFloorResult struct {
	quote, cost, charged int
	costKnown            bool
	basis                string
}

// EnableCostFloor turns on the cost floor for this request when the site setting is on. It reads
// the channel and upstream model the current attempt goes to, so it must be called after the
// channel is chosen; relay retries build a new Quota per attempt, so the floor follows the
// channel that served the request. Unpriced models are left alone: their policy decides.
func (q *Quota) EnableCostFloor(c *gin.Context) {
	if !config.SupplyCostFloorBilling || q.price.Unconfigured {
		return
	}
	channel := model.ChannelGroup.GetChannel(q.channelId)
	if channel == nil {
		return
	}
	q.costFloor = true
	q.feeRate = channel.GetFeeRate()
	q.feeNote = channel.FeeNote
	upstream := c.GetString("new_model")
	if upstream == "" {
		upstream = q.modelName
	}
	cp := model.CachedChannelPrice(channel.Id, upstream)
	if price, source, ok := model.SupplyCostPrice(&q.price, channel, cp, upstream); ok {
		q.costPrice, q.costSource = price, source
	}
}

// billingLogMeta is the billing breakdown of a request charged with the cost floor. Keys that
// start with "_admin_" and the channel cost of a request charged the quote are removed from the
// logs users see (model.Log.RedactForMember).
func (q *Quota) billingLogMeta() map[string]any {
	b := q.billing
	if b == nil {
		return nil
	}
	meta := map[string]any{
		"basis":         b.basis,
		"quote_quota":   b.quote,
		"charged_quota": b.charged,
		"group_ratio":   q.groupRatio,
	}
	if b.costKnown {
		meta["channel_cost_quota"] = b.cost
		meta["fee_rate"] = q.feeRate
		meta["_admin_cost_source"] = q.costSource
		if q.feeNote != "" {
			meta["_admin_fee_note"] = q.feeNote
		}
	}
	return meta
}
