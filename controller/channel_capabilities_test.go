package controller

import (
	"slices"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestChannelTypeCapabilities(t *testing.T) {
	caps := loadChannelTypeCapabilities()
	t.Logf("balance: %v, usage windows: %v", caps.Balance, caps.UsageWindows)

	// Providers with a balance endpoint of their own, including ones that embed the OpenAI
	// provider with its billing endpoints turned off.
	for _, typ := range []int{config.ChannelTypeOpenRouter, config.ChannelTypeVercelAIGateway, config.ChannelTypeDeepseek,
		config.ChannelTypeMoonshot, config.ChannelTypeSiliconflow, config.ChannelTypeOpenAI} {
		if !slices.Contains(caps.Balance, typ) {
			t.Errorf("type %d should report a balance", typ)
		}
	}
	// Providers that only embed the OpenAI provider, and subscription channels, do not.
	for _, typ := range []int{config.ChannelTypeAzure, config.ChannelTypeAnthropic, config.ChannelTypeClaudeCode} {
		if slices.Contains(caps.Balance, typ) {
			t.Errorf("type %d should not report a balance", typ)
		}
	}
	if !slices.Contains(caps.UsageWindows, config.ChannelTypeClaudeCode) {
		t.Error("Claude Code reports usage windows")
	}
}
