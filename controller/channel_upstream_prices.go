package controller

import (
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
}

// refreshChannelPrices reads the channel's upstream price list and stores it as the channel's
// own. It returns how many models were stored; 0 with no error when the upstream lists none.
func refreshChannelPrices(ch *model.Channel, provider providersBase.ProviderInterface) (int, error) {
	lister, ok := provider.(providersBase.ChannelPriceListInterface)
	if !ok {
		return 0, nil
	}
	prices, err := lister.GetChannelPrices()
	if err != nil {
		return 0, err
	}
	if err := model.ReplaceChannelPrices(ch.Id, prices); err != nil {
		return 0, err
	}
	return len(prices), nil
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
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": channelPricesResponse{
		Supported:        ChannelTypeHasOwnPricing(channel.Type),
		ChannelPriceList: list,
	}})
}

// GetChannelPrices returns the prices the channel's upstream charges, as stored at the last
// price sync. It never calls the upstream.
func GetChannelPrices(c *gin.Context) {
	if channel, ok := channelForPrices(c); ok {
		respondChannelPrices(c, channel)
	}
}

// RefreshChannelPrices reads the channel's upstream price list now and returns it.
func RefreshChannelPrices(c *gin.Context) {
	channel, ok := channelForPrices(c)
	if !ok {
		return
	}
	if !ChannelTypeHasOwnPricing(channel.Type) {
		common.APIRespondWithError(c, http.StatusOK, common.NewAPIError(common.APICodeChannelPricesUnsupported, "this provider does not publish its prices", nil))
		return
	}
	provider := providers.GetProvider(channel, nil)
	if provider == nil {
		common.APIRespondWithError(c, http.StatusOK, common.NewAPIError(common.APICodeChannelNotFound, "channel provider not found", nil))
		return
	}
	if _, err := refreshChannelPrices(channel, provider); err != nil {
		common.APIRespondWithError(c, http.StatusOK, err)
		return
	}
	respondChannelPrices(c, channel)
}
