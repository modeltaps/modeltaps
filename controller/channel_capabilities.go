package controller

import (
	"net/http"
	"sync"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providersBase "github.com/modeltaps/modeltaps/providers/base"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

// channelTypeCapabilities says, per channel type, what the panel's monitoring tab can offer.
type channelTypeCapabilities struct {
	// Balance lists the types whose upstream reports a money balance, which a low-balance
	// notice can watch. For OpenAI it depends on the address: the official API has no balance
	// endpoint, relays usually do.
	Balance []int `json:"balance"`
	// UsageWindows lists subscription types that report usage windows instead of a balance.
	UsageWindows []int `json:"usage_windows"`
}

var (
	capabilitiesOnce sync.Once
	capabilities     channelTypeCapabilities
)

// canQueryBalance reports whether a provider of the type has a balance it can read.
func canQueryBalance(provider providersBase.ProviderInterface) bool {
	if _, ok := provider.(providersBase.BalanceInterface); !ok {
		return false
	}
	if capable, ok := provider.(providersBase.BalanceCapability); ok {
		return capable.CanQueryBalance()
	}
	return true
}

// providerForType builds a provider for an empty channel of the type, only to look at what it
// implements. A factory that cannot cope with an empty channel counts as offering nothing.
func providerForType(channelType int) (provider providersBase.ProviderInterface) {
	defer func() {
		if recover() != nil {
			provider = nil
		}
	}()
	return providers.GetProvider(&model.Channel{Type: channelType}, nil)
}

func loadChannelTypeCapabilities() channelTypeCapabilities {
	capabilitiesOnce.Do(func() {
		capabilities = channelTypeCapabilities{Balance: []int{}, UsageWindows: []int{}}
		for _, t := range providers.ChannelTypes() {
			provider := providerForType(t)
			if provider == nil {
				continue
			}
			if _, ok := provider.(providersBase.UsageWindowsInterface); ok || t == config.ChannelTypeClaudeCode {
				capabilities.UsageWindows = append(capabilities.UsageWindows, t)
				continue
			}
			if canQueryBalance(provider) {
				capabilities.Balance = append(capabilities.Balance, t)
			}
		}
	})
	return capabilities
}

// lowBalanceNotice is how low-balance notices reach admins right now.
type lowBalanceNotice struct {
	// RefreshMinutes is how often balances are refreshed; 0 means only by hand, so no notice
	// fires on its own.
	RefreshMinutes int `json:"refresh_minutes"`
	// KindOn is false when low-balance notices are turned off in the notification settings.
	KindOn  bool            `json:"kind_on"`
	Targets []notify.Target `json:"targets"`
}

// GetChannelCapabilities returns which channel types report a balance or usage windows, and
// where low-balance notices go, so the monitoring tab only offers what works.
func GetChannelCapabilities(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
		"types": loadChannelTypeCapabilities(),
		"low_balance": lowBalanceNotice{
			RefreshMinutes: viper.GetInt("channel.update_frequency"),
			KindOn:         notify.KindEnabled(notify.KindLowBalance),
			Targets:        notify.Targets(),
		},
	}})
}
