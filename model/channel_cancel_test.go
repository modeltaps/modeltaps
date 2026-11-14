package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestCancelOnDisconnectDefault(t *testing.T) {
	cases := []struct {
		name        string
		channelType int
		baseURL     string
		want        bool
	}{
		{"openai official empty", config.ChannelTypeOpenAI, "", true},
		{"openai official host", config.ChannelTypeOpenAI, "https://api.openai.com", true},
		{"openai third-party relay", config.ChannelTypeOpenAI, "https://relay.example.com/v1", false},
		{"anthropic official", config.ChannelTypeAnthropic, "https://API.anthropic.com/", true},
		{"anthropic relay", config.ChannelTypeAnthropic, "https://claude-proxy.example.com", false},
		{"openrouter", config.ChannelTypeOpenRouter, "https://openrouter.ai/api", true},
		{"vercel", config.ChannelTypeVercelAIGateway, "", true},
		{"gemini", config.ChannelTypeGemini, "https://generativelanguage.googleapis.com", true},
		{"azure resource url", config.ChannelTypeAzure, "https://my-res.openai.azure.com", true},
		{"bedrock", config.ChannelTypeBedrock, "", true},
		{"vertex", config.ChannelTypeVertexAI, "", true},
		{"claude code", config.ChannelTypeClaudeCode, "", true},
		{"codex", config.ChannelTypeCodex, "", true},
		{"gemini cli", config.ChannelTypeGeminiCli, "", true},
		{"antigravity", config.ChannelTypeAntigravity, "", true},
		{"custom", config.ChannelTypeCustom, "https://x.example.com", false},
		{"deepseek", config.ChannelTypeDeepseek, "", false},
		{"bad url", config.ChannelTypeOpenAI, "://bad", false},
	}
	for _, tc := range cases {
		if got := CancelOnDisconnectDefault(tc.channelType, tc.baseURL); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestChannelCancelsOnDisconnectOverride(t *testing.T) {
	on, off := true, false
	relay := "https://relay.example.com"

	auto := &Channel{Type: config.ChannelTypeOpenAI, BaseURL: &relay}
	if auto.CancelsOnDisconnect() {
		t.Error("auto on a relay base URL should be off")
	}
	forced := &Channel{Type: config.ChannelTypeOpenAI, BaseURL: &relay, CancelOnDisconnect: &on}
	if !forced.CancelsOnDisconnect() {
		t.Error("explicit on should win")
	}
	disabled := &Channel{Type: config.ChannelTypeClaudeCode, CancelOnDisconnect: &off}
	if disabled.CancelsOnDisconnect() {
		t.Error("explicit off should win")
	}
	var nilChannel *Channel
	if nilChannel.CancelsOnDisconnect() {
		t.Error("nil channel should be off")
	}
}
