package relay_util

import (
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// LoadSupplyCost reads what the channel that serves this attempt charges: its list price (the
// expected upstream charge in the log) and its cost with its fee (the request's cost). Neither
// is ever charged to the user, who pays the catalog price times the group ratio whatever the
// channel. It must be called after the channel is chosen; relay retries build a new Quota per
// attempt, so the cost follows the channel that served the request. Unpriced models are left
// alone: their policy decides.
func (q *Quota) LoadSupplyCost(c *gin.Context) {
	if q.price.Unconfigured {
		return
	}
	channel := model.ChannelGroup.GetChannel(q.channelId)
	if channel == nil {
		return
	}
	q.feeRate = channel.GetFeeRate()
	upstream := c.GetString("new_model")
	if upstream == "" {
		upstream = q.modelName
	}
	cp := model.CachedChannelPrice(channel.Id, upstream)
	if list, _, ok := model.SupplyListPrice(&q.price, channel, cp, upstream); ok {
		q.listPrice = list
		cost := *list
		cost.Input = list.Input * (1 + q.feeRate)
		cost.Output = list.Output * (1 + q.feeRate)
		q.costPrice = &cost
	}
}
