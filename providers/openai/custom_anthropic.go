package openai

import (
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
)

// Upstream protocol modes of a custom channel (plugin "upstream_protocol", param "mode").
const (
	// UpstreamProtocolOpenAI sends everything to the OpenAI-compatible endpoints (the default).
	UpstreamProtocolOpenAI = "openai"
	// UpstreamProtocolAnthropicClaude sends Claude-family models to the Anthropic Messages
	// endpoint and every other model to the OpenAI-compatible endpoints.
	UpstreamProtocolAnthropicClaude = "anthropic_claude"
	// UpstreamProtocolAnthropic sends every chat model to the Anthropic Messages endpoint.
	UpstreamProtocolAnthropic = "anthropic"
)

const defaultMessagesPath = "/v1/messages"

// CustomAnthropicSettings is the parsed "upstream_protocol" plugin of a custom channel.
type CustomAnthropicSettings struct {
	Mode         string
	MessagesPath string
	BearerAuth   bool
}

// GetCustomAnthropicSettings reads the upstream protocol settings of a custom channel.
// Any other channel type, or a missing plugin, reports the OpenAI mode.
func GetCustomAnthropicSettings(channel *model.Channel) CustomAnthropicSettings {
	settings := CustomAnthropicSettings{Mode: UpstreamProtocolOpenAI, MessagesPath: defaultMessagesPath}
	if channel == nil || channel.Type != config.ChannelTypeCustom || channel.Plugin == nil {
		return settings
	}
	plugin, ok := channel.Plugin.Data()["upstream_protocol"]
	if !ok {
		return settings
	}
	if mode, ok := plugin["mode"].(string); ok {
		switch strings.TrimSpace(mode) {
		case UpstreamProtocolAnthropicClaude:
			settings.Mode = UpstreamProtocolAnthropicClaude
		case UpstreamProtocolAnthropic:
			settings.Mode = UpstreamProtocolAnthropic
		}
	}
	if path, ok := plugin["messages_path"].(string); ok && strings.TrimSpace(path) != "" {
		path = strings.TrimSpace(path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		settings.MessagesPath = path
	}
	if auth, ok := plugin["auth"].(string); ok && strings.EqualFold(strings.TrimSpace(auth), "bearer") {
		settings.BearerAuth = true
	}
	return settings
}

// CustomUsesAnthropic reports whether a custom channel sends this model over the Anthropic
// Messages protocol. modelName is the upstream name (after model mapping) when known.
func CustomUsesAnthropic(channel *model.Channel, modelName string) bool {
	switch GetCustomAnthropicSettings(channel).Mode {
	case UpstreamProtocolAnthropic:
		return true
	case UpstreamProtocolAnthropicClaude:
		return model_utils.IsAnthropicModelName(modelName)
	}
	return false
}

// CustomClaudeProvider returns a Claude provider bound to this custom channel when the channel
// sends modelName over the Anthropic Messages protocol, and nil otherwise. It shares the
// channel, context and usage of p, so billing and logs see one request.
//
// Every provider that embeds OpenAIProvider inherits this method; it only answers for the
// custom channel type, so those providers keep their own behaviour.
func (p *OpenAIProvider) CustomClaudeProvider(modelName string) *claude.ClaudeProvider {
	if !CustomUsesAnthropic(p.Channel, modelName) {
		return nil
	}
	settings := GetCustomAnthropicSettings(p.Channel)
	provider := &claude.ClaudeProvider{BaseProvider: p.BaseProvider, BearerAuth: settings.BearerAuth}
	provider.Config = base.ProviderConfig{
		BaseURL:         p.Config.BaseURL,
		ChatCompletions: settings.MessagesPath,
		ModelList:       "/v1/models",
	}
	provider.Requester = requester.NewHTTPRequester(p.Channel.GetProxy(), claude.RequestErrorHandle)
	return provider
}
