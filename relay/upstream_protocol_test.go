package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

func withUpstreamProtocol(channel *model.Channel, mode string) *model.Channel {
	data := datatypes.NewJSONType(model.PluginType{"upstream_protocol": {"mode": mode}})
	channel.Plugin = &data
	return channel
}

// A custom channel goes native on the Anthropic entry only for the models it declares the
// Anthropic protocol for; everything else keeps the OpenAI conversion chain.
func TestResolveClaudeRoute_CustomUpstreamProtocol(t *testing.T) {
	baseURL := "https://example.com"
	tests := []struct {
		name  string
		mode  string
		model string
		want  claudeSendRoute
	}{
		{"openai mode converts", "openai", "claude-sonnet-4-5", claudeRouteOpenAIConvert},
		{"claude mode, claude model", "anthropic_claude", "claude-sonnet-4-5", claudeRouteCustomAnthropic},
		{"claude mode, other model", "anthropic_claude", "gpt-5.5", claudeRouteOpenAIConvert},
		{"anthropic mode, any model", "anthropic", "glm-4.6", claudeRouteCustomAnthropic},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := withUpstreamProtocol(&model.Channel{
				Id: 1, Type: config.ChannelTypeCustom, Key: "k", BaseURL: &baseURL}, tt.mode)
			c := newTestContext()
			c.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", nil)
			provider := providers.GetProvider(channel, c)
			if got := resolveClaudeRoute(channel.Type, tt.model, provider); got != tt.want {
				t.Fatalf("route = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaudeSourcePreferred(t *testing.T) {
	prefer := claudeSourcePreferred("claude-sonnet-4-5")
	cases := []struct {
		name    string
		channel *model.Channel
		want    bool
	}{
		{"anthropic", &model.Channel{Type: config.ChannelTypeAnthropic}, true},
		{"claude code", &model.Channel{Type: config.ChannelTypeClaudeCode}, true},
		{"bedrock", &model.Channel{Type: config.ChannelTypeBedrock}, true},
		{"vertex", &model.Channel{Type: config.ChannelTypeVertexAI}, true},
		{"antigravity", &model.Channel{Type: config.ChannelTypeAntigravity}, true},
		{"custom over anthropic", withUpstreamProtocol(&model.Channel{Type: config.ChannelTypeCustom}, "anthropic_claude"), true},
		{"custom over openai", &model.Channel{Type: config.ChannelTypeCustom}, false},
		{"openrouter", &model.Channel{Type: config.ChannelTypeOpenRouter}, false},
		{"openai", &model.Channel{Type: config.ChannelTypeOpenAI}, false},
		{"nil", nil, false},
	}
	for _, tc := range cases {
		if got := prefer(tc.channel); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// Claude models prefer Claude sources on every entry: an OpenAI-protocol custom reseller listed
// first loses to the first-party channel, an Anthropic-protocol custom channel counts as a
// Claude source, and the reseller is still used when it is the only channel.
func TestFetchChannelByModel_ClaudeModelsPreferClaudeSources(t *testing.T) {
	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels
	defer func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
	}()

	weight := uint(1)
	resellerOpenAI := &model.Channel{Id: 30, Type: config.ChannelTypeCustom, Weight: &weight, Status: config.ChannelStatusEnabled}
	anthropicCh := &model.Channel{Id: 31, Type: config.ChannelTypeAnthropic, Weight: &weight, Status: config.ChannelStatusEnabled}
	resellerAnthropic := withUpstreamProtocol(&model.Channel{
		Id: 32, Type: config.ChannelTypeCustom, Weight: &weight, Status: config.ChannelStatusEnabled}, "anthropic_claude")
	openrouterCh := &model.Channel{Id: 33, Type: config.ChannelTypeOpenRouter, Weight: &weight, Status: config.ChannelStatusEnabled}

	model.ChannelGroup.Rule = map[string]map[string][][]int{
		"g": {
			"claude-sonnet-4-5": {{30, 31}},
			"claude-opus-5":     {{30, 32}},
			"claude-haiku-4-5":  {{30}},
			"gpt-5.5":           {{33, 30}},
		},
	}
	model.ChannelGroup.Channels = map[int]*model.ChannelChoice{
		30: {Channel: resellerOpenAI},
		31: {Channel: anthropicCh},
		32: {Channel: resellerAnthropic},
		33: {Channel: openrouterCh},
	}

	// Each entry sets the preference the same way from the requested model.
	entries := map[string]func(c *gin.Context){
		"chat":   func(c *gin.Context) {},
		"claude": func(c *gin.Context) { c.Set("allow_channel_type", AllowChannelType); c.Set("prefer_channel_type", nativeClaudeChannelType) },
	}
	for entry, setup := range entries {
		newCtx := func(modelName string) *gin.Context {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("token_group", "g")
			setup(c)
			preferClaudeSources(c, modelName)
			return c
		}
		for _, tc := range []struct {
			model string
			want  int
		}{
			{"claude-sonnet-4-5", 31},
			{"claude-opus-5", 32},
			{"claude-haiku-4-5", 30},
		} {
			// Repeat to rule out a lucky draw between equal-weight channels.
			for i := 0; i < 20; i++ {
				ch, err := fetchChannelByModel(newCtx(tc.model), tc.model)
				if err != nil {
					t.Fatalf("%s %s: unexpected err %v", entry, tc.model, err)
				}
				if ch.Id != tc.want {
					t.Fatalf("%s %s: got channel %d, want %d", entry, tc.model, ch.Id, tc.want)
				}
			}
		}
	}

	// A non-Claude model on the chat entry sets no preference.
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	preferClaudeSources(c, "gpt-5.5")
	if _, exists := c.Get(ginKeyPreferChannel); exists {
		t.Fatal("gpt-5.5 must not get a Claude source preference")
	}
}
