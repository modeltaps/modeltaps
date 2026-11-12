package model

import (
	"net/url"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/gorm"
)

// cancelOnDisconnectHosts lists the channel types whose upstream stops generating and bills only
// the tokens produced so far once the connection is closed. The value is the official API host;
// the automatic setting applies only when the channel's base URL is empty or points at that host,
// since the same type is often pointed at a third-party relay whose behaviour is unknown.
// An empty host means the type has no single official host (regional or per-resource URLs) and
// any base URL counts.
var cancelOnDisconnectHosts = map[int]string{
	config.ChannelTypeOpenAI:          "api.openai.com",
	config.ChannelTypeAnthropic:       "api.anthropic.com",
	config.ChannelTypeGemini:          "generativelanguage.googleapis.com",
	config.ChannelTypeOpenRouter:      "openrouter.ai",
	config.ChannelTypeVercelAIGateway: "ai-gateway.vercel.sh",
	config.ChannelTypeAzure:           "",
	config.ChannelTypeBedrock:         "",
	config.ChannelTypeVertexAI:        "",
	config.ChannelTypeVertexAIExpress: "",
	// Subscription accounts: stopping saves the account's usage window.
	config.ChannelTypeClaudeCode:  "",
	config.ChannelTypeCodex:       "",
	config.ChannelTypeGeminiCli:   "",
	config.ChannelTypeAntigravity: "",
}

// CancelOnDisconnectDefault reports the automatic setting for a channel type and base URL.
func CancelOnDisconnectDefault(channelType int, baseURL string) bool {
	host, ok := cancelOnDisconnectHosts[channelType]
	if !ok {
		return false
	}
	baseURL = strings.TrimSpace(baseURL)
	if host == "" || baseURL == "" {
		return true
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	return strings.EqualFold(parsed.Hostname(), host)
}

// CancelsOnDisconnect reports whether a client disconnect should close this channel's upstream
// stream: the admin's explicit choice, or the automatic default when unset.
func (channel *Channel) CancelsOnDisconnect() bool {
	if channel == nil {
		return false
	}
	if channel.CancelOnDisconnect != nil {
		return *channel.CancelOnDisconnect
	}
	return CancelOnDisconnectDefault(channel.Type, channel.GetBaseURL())
}

// AfterFind fills CancelOnDisconnectAuto for the console.
func (channel *Channel) AfterFind(_ *gorm.DB) error {
	channel.CancelOnDisconnectAuto = CancelOnDisconnectDefault(channel.Type, channel.GetBaseURL())
	return nil
}
