package claude

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/model"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// extendedCacheTTLBeta 是 1h TTL 缓存所需的 anthropic-beta 标记。
const extendedCacheTTLBeta = "extended-cache-ttl-2025-04-11"

// gin.Context 中记录 Prompt Caching 注入结果的键，供结算/日志层
// (relay_util.Quota.GetLogMeta) 读取。键名与 quota.go 中保持一致。
const (
	GinKeyPromptCachingInjected = "prompt_caching_injected"
	GinKeyPromptCachingTTL      = "prompt_caching_ttl"
	GinKeyPromptCachingStrategy = "prompt_caching_strategy"
)

// RecordPromptCachingInjection 在注入成功时把结果写入 gin.Context，
// 供结算/日志层标记本请求实际注入了 cache_control 及对应 ttl/strategy。
// 仅在 injected 为 true 时写入，未注入则不留痕(保持消费日志 additive)。
func RecordPromptCachingInjection(c *gin.Context, cfg *PromptCachingConfig, injected bool) {
	if c == nil || cfg == nil || !injected {
		return
	}
	c.Set(GinKeyPromptCachingInjected, true)
	c.Set(GinKeyPromptCachingTTL, cfg.TTL)
	c.Set(GinKeyPromptCachingStrategy, cfg.Strategy)
}

// PromptCachingConfig 是解析渠道 Plugin + 全局兜底后的最终注入配置。
type PromptCachingConfig struct {
	TTL      string // "5m" 或 "1h"
	Strategy string // "system" 或 "system+last_user"
}

// GetPromptCachingConfig 解析 Prompt Caching 自动注入配置。
// 返回 nil 表示对该请求不注入。
//
// 解析顺序：
//  1. 渠道 Plugin["prompt_caching"].enabled：on 强制开 / off 强制关 / inherit(默认) 跟随全局开关
//  2. 模型命中判定：auto(默认) 走内置 Anthropic 命名规则；custom 走用户通配符列表；
//     regex 走用户正则；auto+custom 取内置规则与用户规则(通配符/正则)的并集。
//     modelName 必须是 model_mapping 之后发往上游的模型名。
func GetPromptCachingConfig(channel *model.Channel, modelName string) *PromptCachingConfig {
	enabled, ttl, strategy, mode, patterns, regex := "inherit", "5m", "system", "auto", "", ""
	if channel != nil && channel.Plugin != nil {
		if pc, ok := channel.Plugin.Data()["prompt_caching"]; ok {
			enabled = pluginString(pc, "enabled", enabled)
			ttl = pluginString(pc, "ttl", ttl)
			strategy = pluginString(pc, "strategy", strategy)
			mode = pluginString(pc, "model_match_mode", mode)
			patterns = pluginString(pc, "model_match_patterns", patterns)
			regex = pluginString(pc, "model_match_regex", regex)
		}
	}

	switch strings.ToLower(enabled) {
	case "on":
	case "off":
		return nil
	default: // inherit
		if !config.ClaudePromptCachingEnabled {
			return nil
		}
	}

	if !promptCachingModelMatched(mode, patterns, regex, modelName) {
		return nil
	}

	if ttl != "1h" {
		ttl = "5m"
	}
	if strategy != "system+last_user" {
		strategy = "system"
	}
	return &PromptCachingConfig{TTL: ttl, Strategy: strategy}
}

func pluginString(m map[string]interface{}, key, def string) string {
	if v, ok := m[key].(string); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

// promptCachingModelMatched 判定模型是否命中注入条件。
// 四种模式（默认 auto，完全向后兼容）：
//   - auto：内置 Anthropic 命名规则（model_utils.IsAnthropicModelName）。
//   - custom：用户通配符列表 model_match_patterns。
//   - regex：用户正则 model_match_regex（空/非法均视为不命中，永不 panic）。
//   - auto+custom：内置规则 OR 用户规则（通配符 OR 正则）的并集。
func promptCachingModelMatched(mode, patterns, regex, modelName string) bool {
	switch strings.ToLower(mode) {
	case "custom":
		return customPatternsMatch(patterns, modelName)
	case "regex":
		return regexMatch(regex, modelName)
	case "auto+custom":
		return model_utils.IsAnthropicModelName(modelName) ||
			customPatternsMatch(patterns, modelName) ||
			regexMatch(regex, modelName)
	default: // auto
		return model_utils.IsAnthropicModelName(modelName)
	}
}

// customPatternsMatch 按用户通配符列表（大小写不敏感）判定命中。
func customPatternsMatch(patterns, modelName string) bool {
	for _, pat := range splitPatterns(patterns) {
		if wildcardMatch(strings.ToLower(pat), strings.ToLower(modelName)) {
			return true
		}
	}
	return false
}

// regexMatch 按用户正则（大小写不敏感）判定命中。空或非法正则一律返回 false（失败安全，永不 panic）。
func regexMatch(pattern, modelName string) bool {
	if strings.TrimSpace(pattern) == "" {
		return false
	}
	re, err := regexp.Compile("(?i)" + pattern)
	if err != nil {
		return false
	}
	return re.MatchString(modelName)
}

func splitPatterns(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
}

// wildcardMatch 通配符匹配，'*' 匹配任意串（含空串），其余字符精确匹配。
// 调用方需自行统一大小写。
func wildcardMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s
	}
	if !strings.HasPrefix(s, parts[0]) {
		return false
	}
	s = s[len(parts[0]):]
	for i := 1; i < len(parts)-1; i++ {
		idx := strings.Index(s, parts[i])
		if idx < 0 {
			return false
		}
		s = s[idx+len(parts[i]):]
	}
	return strings.HasSuffix(s, parts[len(parts)-1])
}

func (c *PromptCachingConfig) cacheControl() map[string]any {
	cc := map[string]any{"type": "ephemeral"}
	if c.TTL == "1h" {
		cc["ttl"] = "1h"
	}
	return cc
}

// ApplyPromptCachingBeta 注入成功且 TTL 为 1h 时，按需追加 extended-cache-ttl beta 头。
// headers 中的 anthropic-beta 键可能为任意大小写（来自 ModelHeaders 自定义），按大小写不敏感处理。
func ApplyPromptCachingBeta(headers map[string]string, cfg *PromptCachingConfig) {
	if cfg == nil || cfg.TTL != "1h" {
		return
	}
	for k, v := range headers {
		if strings.EqualFold(k, "anthropic-beta") {
			if !strings.Contains(v, extendedCacheTTLBeta) {
				headers[k] = v + "," + extendedCacheTTLBeta
			}
			return
		}
	}
	headers["anthropic-beta"] = extendedCacheTTLBeta
}

// InjectPromptCachingStruct 在 OpenAI→Claude 转换出的结构体请求上注入 cache_control。
// 请求任意位置已带 cache_control 时整体跳过（用户显式优先）。返回是否实际注入。
func InjectPromptCachingStruct(req *ClaudeRequest, cfg *PromptCachingConfig) bool {
	if req == nil || cfg == nil || structHasCacheControl(req) {
		return false
	}
	cc := cfg.cacheControl()

	injected := injectSystemStruct(req, cc)
	if cfg.Strategy == "system+last_user" {
		if injectLastUserStruct(req, cc) {
			injected = true
		}
	}
	return injected
}

func structHasCacheControl(req *ClaudeRequest) bool {
	for _, t := range req.Tools {
		if t.CacheControl != nil {
			return true
		}
	}
	if valueHasCacheControl(req.System) {
		return true
	}
	for _, m := range req.Messages {
		if valueHasCacheControl(m.Content) {
			return true
		}
	}
	return false
}

// valueHasCacheControl 检查 any 形态的 system/content：
// []MessageContent（转换路径产物）或 []any（JSON 反序列化产物）。
func valueHasCacheControl(v any) bool {
	switch val := v.(type) {
	case []MessageContent:
		for _, p := range val {
			if p.CacheControl != nil {
				return true
			}
		}
	case []any:
		for _, item := range val {
			if m, ok := item.(map[string]any); ok {
				if _, exists := m["cache_control"]; exists {
					return true
				}
			}
		}
	}
	return false
}

func injectSystemStruct(req *ClaudeRequest, cc map[string]any) bool {
	switch sys := req.System.(type) {
	case string:
		if sys == "" {
			return false
		}
		req.System = []MessageContent{{Type: "text", Text: sys, CacheControl: cc}}
		return true
	case []MessageContent:
		if len(sys) == 0 {
			return false
		}
		sys[len(sys)-1].CacheControl = cc
		return true
	case []any:
		if len(sys) == 0 {
			return false
		}
		if m, ok := sys[len(sys)-1].(map[string]any); ok {
			m["cache_control"] = cc
			return true
		}
	}
	return false
}

func injectLastUserStruct(req *ClaudeRequest, cc map[string]any) bool {
	for i := len(req.Messages) - 1; i >= 0; i-- {
		if req.Messages[i].Role != "user" {
			continue
		}
		switch content := req.Messages[i].Content.(type) {
		case string:
			if content == "" {
				return false
			}
			req.Messages[i].Content = []MessageContent{{Type: "text", Text: content, CacheControl: cc}}
			return true
		case []MessageContent:
			if len(content) == 0 {
				return false
			}
			content[len(content)-1].CacheControl = cc
			return true
		case []any:
			if len(content) == 0 {
				return false
			}
			if m, ok := content[len(content)-1].(map[string]any); ok {
				m["cache_control"] = cc
				return true
			}
		}
		return false
	}
	return false
}

// InjectPromptCachingBytes 在原生字节透传路径上用 sjson 做最小注入，
// 与 patchClaudeRequestBody 同范式：只动需要的字段，其余字节原样保留。
// 已带 cache_control 或注入失败时返回原 body。返回是否实际注入。
func InjectPromptCachingBytes(body []byte, cfg *PromptCachingConfig) ([]byte, bool) {
	if cfg == nil || bytesHasCacheControl(body) {
		return body, false
	}
	cc := cfg.cacheControl()

	out, injected := injectSystemBytes(body, cc)
	if cfg.Strategy == "system+last_user" {
		var userInjected bool
		out, userInjected = injectLastUserBytes(out, cc)
		injected = injected || userInjected
	}
	return out, injected
}

func bytesHasCacheControl(body []byte) bool {
	found := false
	check := func(_, value gjson.Result) bool {
		if value.Get("cache_control").Exists() {
			found = true
			return false
		}
		return true
	}
	gjson.GetBytes(body, "system").ForEach(check)
	if found {
		return true
	}
	gjson.GetBytes(body, "tools").ForEach(check)
	if found {
		return true
	}
	gjson.GetBytes(body, "messages").ForEach(func(_, msg gjson.Result) bool {
		msg.Get("content").ForEach(check)
		return !found
	})
	return found
}

func injectSystemBytes(body []byte, cc map[string]any) ([]byte, bool) {
	sys := gjson.GetBytes(body, "system")
	switch {
	case sys.Type == gjson.String:
		if sys.String() == "" {
			return body, false
		}
		newSys := []map[string]any{{"type": "text", "text": sys.String(), "cache_control": cc}}
		if out, err := sjson.SetBytes(body, "system", newSys); err == nil {
			return out, true
		}
	case sys.IsArray():
		arr := sys.Array()
		if len(arr) == 0 {
			return body, false
		}
		path := fmt.Sprintf("system.%d.cache_control", len(arr)-1)
		if out, err := sjson.SetBytes(body, path, cc); err == nil {
			return out, true
		}
	}
	return body, false
}

func injectLastUserBytes(body []byte, cc map[string]any) ([]byte, bool) {
	msgs := gjson.GetBytes(body, "messages")
	if !msgs.IsArray() {
		return body, false
	}
	arr := msgs.Array()
	for i := len(arr) - 1; i >= 0; i-- {
		if arr[i].Get("role").String() != "user" {
			continue
		}
		content := arr[i].Get("content")
		switch {
		case content.Type == gjson.String:
			if content.String() == "" {
				return body, false
			}
			newContent := []map[string]any{{"type": "text", "text": content.String(), "cache_control": cc}}
			if out, err := sjson.SetBytes(body, fmt.Sprintf("messages.%d.content", i), newContent); err == nil {
				return out, true
			}
		case content.IsArray():
			n := len(content.Array())
			if n == 0 {
				return body, false
			}
			if out, err := sjson.SetBytes(body, fmt.Sprintf("messages.%d.content.%d.cache_control", i, n-1), cc); err == nil {
				return out, true
			}
		}
		return body, false
	}
	return body, false
}
