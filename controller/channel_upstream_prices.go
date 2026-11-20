package controller

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providersBase "github.com/modeltaps/modeltaps/providers/base"

	"github.com/gin-gonic/gin"
)

// channelPricesResponse is a channel's upstream price list. Supported says whether the channel
// type's upstream publishes prices at all, so the panel can tell "not read yet" from "never".
type channelPricesResponse struct {
	Supported bool `json:"supported"`
	*model.ChannelPriceList
	// State is how the last reads of the list went; nil when it was never read.
	State *model.PriceSourceState `json:"state,omitempty"`
}

// refreshChannelPrices reads the channel's upstream price list and stores it as the channel's
// own. It returns how many models were stored; 0 with no error when the upstream lists none.
// A list that looks broken against the stored one (model.CheckChannelPriceList) is not stored
// unless force is set; it returns a *model.SuspiciousPriceListError. Every read is recorded as
// the channel's price source state, which ReviewPriceSources turns into alerts.
func refreshChannelPrices(ch *model.Channel, provider providersBase.ProviderInterface, force bool) (int, error) {
	lister, ok := provider.(providersBase.ChannelPriceListInterface)
	if !ok {
		return 0, nil
	}
	subject := model.ChannelPriceSubject(ch.Id)
	prices, err := lister.GetChannelPrices()
	if err != nil {
		model.RecordPriceSourceFailure(subject, model.PriceSourceReasonRead, err)
		return 0, err
	}
	if !force {
		if stored, err := model.GetChannelPrices(ch.Id); err == nil {
			if err := model.CheckChannelPriceList(stored.Prices, prices); err != nil {
				var suspicious *model.SuspiciousPriceListError
				if errors.As(err, &suspicious) {
					model.RecordPriceSourceFailure(subject, suspicious.Reason, err)
				}
				return 0, err
			}
		}
	}
	if err := model.ReplaceChannelPrices(ch.Id, prices); err != nil {
		return 0, err
	}
	if len(prices) > 0 {
		model.RecordPriceSourceSuccess(subject, len(prices))
	}
	return len(prices), nil
}

// channelKeepsPriceList says whether the channel's upstream publishes a price list it keeps: the
// catalog sources (ChannelTypeHasOwnPricing) and the preset platforms, priced from their own lists
// or models.dev.
func channelKeepsPriceList(ch *model.Channel) bool {
	if ChannelTypeHasOwnPricing(ch.Type) {
		return true
	}
	_, ok := providers.GetProvider(ch, nil).(providersBase.ChannelPriceListInterface)
	return ok
}

func channelForPrices(c *gin.Context) (*model.Channel, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, invalidChannelIDError())
		return nil, false
	}
	channel, err := model.GetChannelById(id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, channelLookupError(err))
		return nil, false
	}
	return channel, true
}

func respondChannelPrices(c *gin.Context, channel *model.Channel) {
	list, err := model.GetChannelPrices(channel.Id)
	if err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	model.DescribeChannelPrices(channel.Type, list.Prices...)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelPricesResponse{
		Supported:        channelKeepsPriceList(channel),
		ChannelPriceList: list,
		State:            model.GetPriceSourceState(model.ChannelPriceSubject(channel.Id)),
	}})
}

// GetChannelPrices returns the prices the channel's upstream charges, as stored at the last
// price sync. It never calls the upstream.
func GetChannelPrices(c *gin.Context) {
	if channel, ok := channelForPrices(c); ok {
		respondChannelPrices(c, channel)
	}
}

// RefreshChannelPrices reads the channel's upstream price list now and returns it. A list that
// looks broken is refused with channel_prices_suspicious; ?force=1 stores it anyway.
func RefreshChannelPrices(c *gin.Context) {
	channel, ok := channelForPrices(c)
	if !ok {
		return
	}
	if !channelKeepsPriceList(channel) {
		common.APIRespondWithError(c, http.StatusOK, common.NewAPIError(common.APICodeChannelPricesUnsupported, "this provider does not publish its prices", nil))
		return
	}
	provider := providers.GetProvider(channel, nil)
	if provider == nil {
		common.APIRespondWithError(c, http.StatusOK, common.NewAPIError(common.APICodeChannelNotFound, "channel provider not found", nil))
		return
	}
	if _, err := refreshChannelPrices(channel, provider, c.Query("force") == "1"); err != nil {
		var suspicious *model.SuspiciousPriceListError
		if errors.As(err, &suspicious) {
			err = common.NewAPIError(common.APICodeChannelPricesSuspicious, err.Error(), map[string]any{
				"reason": suspicious.Reason, "before": suspicious.Before, "after": suspicious.After,
			})
		}
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	respondChannelPrices(c, channel)
}
