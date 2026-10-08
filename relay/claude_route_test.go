package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
)

// newRouteProvider 用真实工厂构造 provider，保证路由断言反映各渠道的实际接口实现。
func newRouteProvider(t *testing.T, channelType int) providersBase.ProviderInterface {
	t.Helper()
	baseURL := "https://example.com"
	channel := &model.Channel{
		Id:      1,
		Type:    channelType,
		Key:     "test-key",
		BaseURL: &baseURL,
	}
	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", nil)
	provider := providers.GetProvider(channel, c)
	if provider == nil {
		t.Fatalf("channelType=%d: GetProvider returned nil", channelType)
	}
	return provider
}

// TestResolveClaudeRoute_DecisionMatrix 覆盖 send() 的路由决策矩阵：
// 每种渠道类型 → 预期路由归宿。新纳入白名单的对话渠道必须落到转换链而非 unsupported。
func TestResolveClaudeRoute_DecisionMatrix(t *testing.T) {
	tests := []struct {
		name        string
		channelType int
		modelName   string
		want        claudeSendRoute
	}{
		// 专用分支
		{"custom", config.ChannelTypeCustom, "claude-3-5-sonnet", claudeRouteOpenAIConvert},
		{"openai", config.ChannelTypeOpenAI, "gpt-4o", claudeRouteOpenAIConvert},
		{"openrouter", config.ChannelTypeOpenRouter, "anthropic/claude-3.5-sonnet", claudeRouteOpenAIConvert},
		{"vertexai gemini model", config.ChannelTypeVertexAI, "gemini-2.0-flash", claudeRouteVertexAIGemini},
		{"vertexai claude-3-5-haiku", config.ChannelTypeVertexAI, "claude-3-5-haiku-20241022", claudeRouteVertexAIGemini},
		{"gemini", config.ChannelTypeGemini, "gemini-2.0-flash", claudeRouteGemini},
		{"antigravity", config.ChannelTypeAntigravity, "claude-sonnet-4", claudeRouteAntigravity},

		// 原生 Claude 实现优先于兜底
		{"anthropic native", config.ChannelTypeAnthropic, "claude-3-5-sonnet", claudeRouteNative},
		{"claudecode native", config.ChannelTypeClaudeCode, "claude-3-5-sonnet", claudeRouteNative},
		{"deepseek native not hijacked by fallback", config.ChannelTypeDeepseek, "deepseek-chat", claudeRouteNative},
		{"bedrock native not hijacked by fallback", config.ChannelTypeBedrock, "claude-3-5-sonnet", claudeRouteNative},

		// ChatInterface 兜底：新纳入的对话渠道
		{"azure", config.ChannelTypeAzure, "gpt-4o", claudeRouteOpenAIConvert},
		{"azure v1", config.ChannelTypeAzureV1, "gpt-4o", claudeRouteOpenAIConvert},
		{"azure databricks", config.ChannelTypeAzureDatabricks, "databricks-claude", claudeRouteOpenAIConvert},
		{"palm", config.ChannelTypePaLM, "chat-bison", claudeRouteOpenAIConvert},
		{"baidu", config.ChannelTypeBaidu, "ernie-4.0", claudeRouteOpenAIConvert},
		{"zhipu", config.ChannelTypeZhipu, "glm-4", claudeRouteOpenAIConvert},
		{"ali", config.ChannelTypeAli, "qwen-max", claudeRouteOpenAIConvert},
		{"xunfei", config.ChannelTypeXunfei, "spark", claudeRouteOpenAIConvert},
		{"360", config.ChannelType360, "360gpt", claudeRouteOpenAIConvert},
		{"tencent", config.ChannelTypeTencent, "hunyuan", claudeRouteOpenAIConvert},
		{"baichuan", config.ChannelTypeBaichuan, "baichuan2", claudeRouteOpenAIConvert},
		{"minimax", config.ChannelTypeMiniMax, "abab6", claudeRouteOpenAIConvert},
		{"moonshot", config.ChannelTypeMoonshot, "moonshot-v1-8k", claudeRouteOpenAIConvert},
		{"mistral", config.ChannelTypeMistral, "mistral-large", claudeRouteOpenAIConvert},
		{"groq", config.ChannelTypeGroq, "llama-3.1-70b", claudeRouteOpenAIConvert},
		{"lingyi", config.ChannelTypeLingyi, "yi-large", claudeRouteOpenAIConvert},
		{"cloudflare ai", config.ChannelTypeCloudflareAI, "@cf/meta/llama-3", claudeRouteOpenAIConvert},
		{"cohere", config.ChannelTypeCohere, "command-r", claudeRouteOpenAIConvert},
		{"coze", config.ChannelTypeCoze, "bot", claudeRouteOpenAIConvert},
		{"ollama", config.ChannelTypeOllama, "llama3", claudeRouteOpenAIConvert},
		{"hunyuan", config.ChannelTypeHunyuan, "hunyuan-pro", claudeRouteOpenAIConvert},
		{"siliconflow", config.ChannelTypeSiliconflow, "Qwen/Qwen2.5-72B", claudeRouteOpenAIConvert},
		{"github", config.ChannelTypeGithub, "gpt-4o", claudeRouteOpenAIConvert},
		{"replicate", config.ChannelTypeReplicate, "meta/llama-3", claudeRouteOpenAIConvert},
		{"xai", config.ChannelTypeXAI, "grok-3", claudeRouteOpenAIConvert},
		{"geminicli", config.ChannelTypeGeminiCli, "gemini-2.0-flash", claudeRouteOpenAIConvert},
		{"codex", config.ChannelTypeCodex, "gpt-5-codex", claudeRouteOpenAIConvert},
		{"vertexai express", config.ChannelTypeVertexAIExpress, "gemini-2.0-flash", claudeRouteOpenAIConvert},
		// VertexAI 无原生 Claude 实现，claude 模型经 category 走转换链
		{"vertexai claude model", config.ChannelTypeVertexAI, "claude-sonnet-4", claudeRouteOpenAIConvert},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := newRouteProvider(t, tt.channelType)
			if got := resolveClaudeRoute(tt.channelType, tt.modelName, provider); got != tt.want {
				t.Fatalf("channelType=%d model=%q: got route %d, want %d", tt.channelType, tt.modelName, got, tt.want)
			}
		})
	}
}

// TestAllowChannelType_CoversRoutableChannels 保证白名单与路由能力一致：
// 白名单里的每个渠道类型都必须有可用归宿，不能落到 unsupported。
func TestAllowChannelType_CoversRoutableChannels(t *testing.T) {
	for _, channelType := range AllowChannelType {
		provider := newRouteProvider(t, channelType)
		if got := resolveClaudeRoute(channelType, "test-model", provider); got == claudeRouteUnsupported {
			t.Fatalf("channelType=%d in AllowChannelType resolved to unsupported", channelType)
		}
	}
}

// TestAllowChannelType_ExcludesNonChatChannels 保证纯图像/音频/任务型渠道未混入白名单。
func TestAllowChannelType_ExcludesNonChatChannels(t *testing.T) {
	excluded := []int{
		config.ChannelTypeAzureSpeech,
		config.ChannelTypeMidjourney,
		config.ChannelTypeStabilityAI,
		config.ChannelTypeSuno,
		config.ChannelTypeIdeogram,
		config.ChannelTypeFlux,
		config.ChannelTypeJina,
		config.ChannelTypeRerank,
		config.ChannelTypeRecraft,
		config.ChannelTypeKling,
		config.ChannelTypeLLAMA,
	}
	allowed := make(map[int]bool, len(AllowChannelType))
	for _, ct := range AllowChannelType {
		allowed[ct] = true
	}
	for _, channelType := range excluded {
		if allowed[channelType] {
			t.Fatalf("channelType=%d should not be in AllowChannelType", channelType)
		}
	}
}
