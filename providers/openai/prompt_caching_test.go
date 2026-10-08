package openai

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
	"testing"

	"gorm.io/datatypes"
)

func channelOn() *model.Channel {
	data := datatypes.NewJSONType(model.PluginType{"prompt_caching": map[string]interface{}{"enabled": "on"}})
	ch := &model.Channel{}
	ch.Plugin = &data
	return ch
}

// RemapAnthropicCacheUsage 仅对 Anthropic 模型名生效，其它供应商原生 cached_tokens 不受影响。
func TestRemapAnthropicCacheUsage(t *testing.T) {
	t.Run("anthropic模型remap到read桶", func(t *testing.T) {
		u := &types.Usage{}
		u.PromptTokensDetails.CachedTokens = 100
		RemapAnthropicCacheUsage("claude-3-5-sonnet", u)
		if u.PromptTokensDetails.CachedTokens != 0 || u.PromptTokensDetails.CachedReadTokens != 100 {
			t.Fatalf("unexpected remap: %#v", u.PromptTokensDetails)
		}
	})
	t.Run("非anthropic模型不动", func(t *testing.T) {
		u := &types.Usage{}
		u.PromptTokensDetails.CachedTokens = 100
		RemapAnthropicCacheUsage("gpt-4o", u)
		if u.PromptTokensDetails.CachedTokens != 100 || u.PromptTokensDetails.CachedReadTokens != 0 {
			t.Fatalf("non-anthropic cached_tokens must stay: %#v", u.PromptTokensDetails)
		}
	})
	t.Run("nil安全", func(t *testing.T) {
		RemapAnthropicCacheUsage("claude-3-5-sonnet", nil)
	})
}

func TestInjectAnthropicPromptCaching(t *testing.T) {
	t.Run("anthropic模型注入system", func(t *testing.T) {
		req := &types.ChatCompletionRequest{
			Model: "claude-3-5-sonnet",
			Messages: []types.ChatCompletionMessage{
				{Role: types.ChatMessageRoleSystem, Content: "you are helpful"},
				{Role: types.ChatMessageRoleUser, Content: "hi"},
			},
		}
		InjectAnthropicPromptCaching(channelOn(), req)
		sys, ok := req.Messages[0].Content.([]map[string]any)
		if !ok || len(sys) != 1 || sys[0]["cache_control"] == nil {
			t.Fatalf("expected system cache_control injection, got %#v", req.Messages[0].Content)
		}
	})

	t.Run("已有cache_control时整体跳过", func(t *testing.T) {
		req := &types.ChatCompletionRequest{
			Model: "claude-3-5-sonnet",
			Messages: []types.ChatCompletionMessage{
				{Role: types.ChatMessageRoleSystem, Content: "you are helpful"},
				{Role: types.ChatMessageRoleUser, Content: []any{
					map[string]any{"type": "text", "text": "hi", "cache_control": map[string]any{"type": "ephemeral"}},
				}},
			},
		}
		InjectAnthropicPromptCaching(channelOn(), req)
		if _, ok := req.Messages[0].Content.(string); !ok {
			t.Fatalf("system should be untouched when cache_control already present: %#v", req.Messages[0].Content)
		}
	})

	t.Run("非anthropic模型不注入", func(t *testing.T) {
		req := &types.ChatCompletionRequest{
			Model: "gpt-4o",
			Messages: []types.ChatCompletionMessage{
				{Role: types.ChatMessageRoleSystem, Content: "you are helpful"},
				{Role: types.ChatMessageRoleUser, Content: "hi"},
			},
		}
		InjectAnthropicPromptCaching(channelOn(), req)
		if _, ok := req.Messages[0].Content.(string); !ok {
			t.Fatalf("non-anthropic model should not be injected: %#v", req.Messages[0].Content)
		}
	})
}

// vercelChannel 模拟一个通用 OpenAI 兼容渠道（如 Vercel AI Gateway，ChannelTypeCustom）：
// 不显式配置 prompt_caching plugin，因此 enabled=inherit、model_match_mode=auto，
// 注入完全由全局 ClaudePromptCachingEnabled + 上游模型命名驱动。
func vercelChannel() *model.Channel {
	return &model.Channel{Type: config.ChannelTypeCustom}
}

// systemInjected 判断首条 system 消息是否被注入了 cache_control。
func systemInjected(req *types.ChatCompletionRequest) bool {
	sys, ok := req.Messages[0].Content.([]map[string]any)
	return ok && len(sys) == 1 && sys[0]["cache_control"] != nil
}

// countCacheControl 统计整个请求里出现的 cache_control 标记数量（兼容 string / []map / []any 形态）。
func countCacheControl(req *types.ChatCompletionRequest) int {
	n := 0
	for i := range req.Messages {
		switch c := req.Messages[i].Content.(type) {
		case []map[string]any:
			for _, item := range c {
				if item["cache_control"] != nil {
					n++
				}
			}
		case []any:
			for _, item := range c {
				if mp, ok := item.(map[string]any); ok && mp["cache_control"] != nil {
					n++
				}
			}
		}
	}
	return n
}

// TestInjectAnthropicPromptCaching_VercelSimulation 证明：在全局 ClaudePromptCachingEnabled=on
// 且渠道走默认 auto 模式（模拟 Vercel AI Gateway 这类通用 OpenAI 兼容渠道）时，
// cache_control 仅对 Anthropic 家族模型自动注入，其它供应商模型保持原样。
func TestInjectAnthropicPromptCaching_VercelSimulation(t *testing.T) {
	old := config.ClaudePromptCachingEnabled
	config.ClaudePromptCachingEnabled = true
	t.Cleanup(func() { config.ClaudePromptCachingEnabled = old })

	newReq := func(modelName string) *types.ChatCompletionRequest {
		return &types.ChatCompletionRequest{
			Model: modelName,
			Messages: []types.ChatCompletionMessage{
				{Role: types.ChatMessageRoleSystem, Content: "you are helpful"},
				{Role: types.ChatMessageRoleUser, Content: "hi"},
			},
		}
	}

	injectCases := []string{"anthropic/claude-3.5-sonnet", "claude-opus-4-1", "my-opus-prod"}
	for _, m := range injectCases {
		t.Run("注入_"+m, func(t *testing.T) {
			req := newReq(m)
			InjectAnthropicPromptCaching(vercelChannel(), req)
			if !systemInjected(req) {
				t.Fatalf("expected cache_control injected for %q, got %#v", m, req.Messages[0].Content)
			}
		})
	}

	t.Run("非anthropic_gpt-4o不注入", func(t *testing.T) {
		req := newReq("gpt-4o")
		InjectAnthropicPromptCaching(vercelChannel(), req)
		if _, ok := req.Messages[0].Content.(string); !ok {
			t.Fatalf("gpt-4o must not be injected: %#v", req.Messages[0].Content)
		}
		if countCacheControl(req) != 0 {
			t.Fatalf("gpt-4o must carry no cache_control, got %d", countCacheControl(req))
		}
	})

	t.Run("已带用户cache_control不重复注入", func(t *testing.T) {
		req := &types.ChatCompletionRequest{
			Model: "anthropic/claude-3.5-sonnet",
			Messages: []types.ChatCompletionMessage{
				{Role: types.ChatMessageRoleSystem, Content: "you are helpful"},
				{Role: types.ChatMessageRoleUser, Content: []any{
					map[string]any{"type": "text", "text": "hi", "cache_control": map[string]any{"type": "ephemeral"}},
				}},
			},
		}
		InjectAnthropicPromptCaching(vercelChannel(), req)
		if _, ok := req.Messages[0].Content.(string); !ok {
			t.Fatalf("system should stay untouched when user supplied cache_control: %#v", req.Messages[0].Content)
		}
		if got := countCacheControl(req); got != 1 {
			t.Fatalf("explicit user cache_control must not be doubled, want 1 got %d", got)
		}
	})

	t.Run("remap_anthropic迁移到read桶", func(t *testing.T) {
		u := &types.Usage{}
		u.PromptTokensDetails.CachedTokens = 100
		RemapAnthropicCacheUsage("anthropic/claude-3.5-sonnet", u)
		if u.PromptTokensDetails.CachedTokens != 0 || u.PromptTokensDetails.CachedReadTokens != 100 {
			t.Fatalf("unexpected remap: %#v", u.PromptTokensDetails)
		}
	})

	t.Run("remap_gpt-4o保持不变", func(t *testing.T) {
		u := &types.Usage{}
		u.PromptTokensDetails.CachedTokens = 100
		RemapAnthropicCacheUsage("gpt-4o", u)
		if u.PromptTokensDetails.CachedTokens != 100 || u.PromptTokensDetails.CachedReadTokens != 0 {
			t.Fatalf("non-anthropic cached_tokens must stay: %#v", u.PromptTokensDetails)
		}
	})
}
