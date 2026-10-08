package openai

import (
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"strings"
)

// InjectAnthropicPromptCaching 在 OpenAI chat 形态请求上自动注入 cache_control 断点，
// 复用 claude.GetPromptCachingConfig 的全局开关 + 渠道 Plugin["prompt_caching"] + 模型命中判定，
// 使任意 OpenAI 兼容渠道在服务 Anthropic 家族模型时都能受益。调用方已显式带 cache_control 时整体跳过。
func InjectAnthropicPromptCaching(channel *model.Channel, request *types.ChatCompletionRequest) {
	if request == nil {
		return
	}
	cfg := claude.GetPromptCachingConfig(channel, request.Model)
	if cfg == nil {
		return
	}
	if messagesHaveCacheControl(request.Messages) {
		return
	}

	cc := map[string]any{"type": "ephemeral"}
	if cfg.TTL == "1h" {
		cc["ttl"] = "1h"
	}

	injectSystemCacheControl(request.Messages, cc)
	if cfg.Strategy == "system+last_user" {
		injectLastUserCacheControl(request.Messages, cc)
	}
}

// RemapAnthropicCacheUsage 把 OpenAI 兼容网关为 Anthropic 模型返回的缓存读取 token
// (prompt_tokens_details.cached_tokens) 重新归入 cached_read_tokens 桶(0.1x)，而非通用
// cached 桶(1.0x)。严格按 model_utils.IsAnthropicModelName 门控，避免影响其它供应商的原生计费。
func RemapAnthropicCacheUsage(modelName string, usage *types.Usage) {
	if usage == nil || !model_utils.IsAnthropicModelName(modelName) {
		return
	}
	if usage.PromptTokensDetails.CachedTokens > 0 {
		usage.PromptTokensDetails.CachedReadTokens += usage.PromptTokensDetails.CachedTokens
		usage.PromptTokensDetails.CachedTokens = 0
	}
}

// messagesHaveCacheControl 检测是否已有调用方显式提供的 cache_control。
func messagesHaveCacheControl(messages []types.ChatCompletionMessage) bool {
	for i := range messages {
		if contentHasCacheControl(messages[i].Content) {
			return true
		}
	}
	return false
}

func contentHasCacheControl(content any) bool {
	arr, ok := content.([]any)
	if !ok {
		return false
	}
	for _, item := range arr {
		if mp, ok := item.(map[string]any); ok {
			if _, exists := mp["cache_control"]; exists {
				return true
			}
		}
	}
	return false
}

func injectSystemCacheControl(messages []types.ChatCompletionMessage, cc map[string]any) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].IsSystemRole() {
			return injectCacheControlOnMessage(&messages[i], cc)
		}
	}
	return false
}

func injectLastUserCacheControl(messages []types.ChatCompletionMessage, cc map[string]any) bool {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == types.ChatMessageRoleUser {
			return injectCacheControlOnMessage(&messages[i], cc)
		}
	}
	return false
}

// injectCacheControlOnMessage 在消息内容的最后一个分片上写入 cache_control。
// ChatMessagePart 无 cache_control 字段，故统一转成 []map[string]any 形态携带。
func injectCacheControlOnMessage(msg *types.ChatCompletionMessage, cc map[string]any) bool {
	switch v := msg.Content.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return false
		}
		msg.Content = []map[string]any{{"type": "text", "text": v, "cache_control": cc}}
		return true
	case []any:
		if len(v) == 0 {
			return false
		}
		if mp, ok := v[len(v)-1].(map[string]any); ok {
			mp["cache_control"] = cc
			return true
		}
		return false
	case []types.ChatMessagePart:
		if len(v) == 0 {
			return false
		}
		b, err := json.Marshal(v)
		if err != nil {
			return false
		}
		var parts []map[string]any
		if err := json.Unmarshal(b, &parts); err != nil || len(parts) == 0 {
			return false
		}
		parts[len(parts)-1]["cache_control"] = cc
		msg.Content = parts
		return true
	}
	return false
}
