package relay

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/providers/openai"

	"github.com/gin-gonic/gin"
)

// The upstream protocol follows the model, not the entry point: the entry protocol only decides
// the shape of the reply. Claude-family models go to the upstream over the Anthropic Messages
// protocol whenever a channel can speak it, because cache breakpoints, cache usage and thinking
// signatures only survive a round trip in that protocol.

// ginKeyPreferChannel holds a func(*model.Channel) bool: channel selection first tries the
// channels it accepts and falls back to every channel only when none of them is available.
const ginKeyPreferChannel = "prefer_channel"

// claudeFirstPartyChannelTypes reach Claude models through the model maker's or a cloud's own
// Claude API (or a first-party subscription), so Claude requests prefer them on every entry.
var claudeFirstPartyChannelTypes = []int{
	config.ChannelTypeAnthropic,
	config.ChannelTypeClaudeCode,
	config.ChannelTypeBedrock,
	config.ChannelTypeVertexAI,
	config.ChannelTypeAntigravity,
}

// claudeSourcePreferred reports whether a channel is a preferred source for a Claude-family
// model: a first-party Claude channel, or a custom channel that speaks Anthropic for it.
func claudeSourcePreferred(modelName string) func(*model.Channel) bool {
	return func(channel *model.Channel) bool {
		if channel == nil {
			return false
		}
		if utils.Contains(channel.Type, claudeFirstPartyChannelTypes) {
			return true
		}
		return channel.Type == config.ChannelTypeCustom && openai.CustomUsesAnthropic(channel, modelName)
	}
}

// preferClaudeSources makes channel selection try the preferred Claude sources first when the
// requested model is a Claude-family model. Other models are left alone.
func preferClaudeSources(c *gin.Context, modelName string) {
	if model_utils.IsAnthropicModelName(modelName) {
		c.Set(ginKeyPreferChannel, claudeSourcePreferred(modelName))
	}
}

// customClaudeProvider returns the Anthropic delegate of a custom channel that sends modelName
// over the Anthropic Messages protocol, or nil.
func customClaudeProvider(provider providersBase.ProviderInterface, modelName string) *claude.ClaudeProvider {
	if delegating, ok := provider.(interface {
		CustomClaudeProvider(modelName string) *claude.ClaudeProvider
	}); ok {
		return delegating.CustomClaudeProvider(modelName)
	}
	return nil
}
