package claude

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

func channelWithPromptCaching(params map[string]interface{}) *model.Channel {
	ch := &model.Channel{}
	if params != nil {
		data := datatypes.NewJSONType(model.PluginType{"prompt_caching": params})
		ch.Plugin = &data
	}
	return ch
}

func withGlobalCaching(t *testing.T, enabled bool) {
	t.Helper()
	old := config.ClaudePromptCachingEnabled
	config.ClaudePromptCachingEnabled = enabled
	t.Cleanup(func() { config.ClaudePromptCachingEnabled = old })
}

// 矩阵：渠道 inherit/on/off × 全局开/关
func TestGetPromptCachingConfigMatrix(t *testing.T) {
	cases := []struct {
		name    string
		global  bool
		enabled string // "" 表示渠道未配置 plugin
		want    bool
	}{
		{"全局关+未配置", false, "", false},
		{"全局关+inherit", false, "inherit", false},
		{"全局关+on", false, "on", true},
		{"全局关+off", false, "off", false},
		{"全局开+未配置", true, "", true},
		{"全局开+inherit", true, "inherit", true},
		{"全局开+on", true, "on", true},
		{"全局开+off", true, "off", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			withGlobalCaching(t, c.global)
			var params map[string]interface{}
			if c.enabled != "" {
				params = map[string]interface{}{"enabled": c.enabled}
			}
			got := GetPromptCachingConfig(channelWithPromptCaching(params), "claude-sonnet-4-5")
			if (got != nil) != c.want {
				t.Fatalf("want inject=%v, got cfg=%v", c.want, got)
			}
		})
	}
}

// auto/custom/regex/auto+custom 命中判定（在 model_mapping 之后的上游模型名上判定）
func TestPromptCachingModelMatched(t *testing.T) {
	cases := []struct {
		name, mode, patterns, regex, model string
		want                               bool
	}{
		{"auto命中claude前缀", "auto", "", "", "claude-sonnet-4-5", true},
		{"auto命中厂商前缀", "auto", "", "", "anthropic/claude-3-5-sonnet", true},
		{"auto命中bedrock形式", "auto", "", "", "us.anthropic.claude-3-7-sonnet-20250219-v1:0", true},
		{"auto命中映射还原名", "auto", "", "", "claude-3-7-sonnet-20250219", true}, // 用户别名 my-alias 经映射后的上游名
		{"auto命中独立opus分段", "auto", "", "", "house-opus", true},
		{"auto不命中gpt", "auto", "", "", "gpt-4o", false},
		{"auto不命中corpus子串陷阱", "auto", "", "", "corpus-embed-v2", false},
		{"custom命中通配符", "custom", "claude-*,my-model", "", "claude-3-haiku", true},
		{"custom命中精确名", "custom", "claude-*,my-model", "", "my-model", true},
		{"custom命中非claude名", "custom", "my-renamed-*", "", "my-renamed-sonnet", true},
		{"custom不命中", "custom", "claude-*", "", "gpt-4o", false},
		{"custom空列表不命中claude", "custom", "", "", "claude-sonnet-4-5", false},
		{"regex命中", "regex", "", "my-claude-.*", "my-claude-prod", true},
		{"regex大小写不敏感", "regex", "", "MY-CLAUDE-.*", "my-claude-prod", true},
		{"regex不命中", "regex", "", "my-claude-.*", "gpt-4o", false},
		{"regex非法正则不命中", "regex", "", "(", "my-claude-prod", false},
		{"regex空正则不命中", "regex", "", "", "claude-sonnet-4-5", false},
		{"auto+custom内置命中空规则", "auto+custom", "", "", "claude-3-5-sonnet", true},
		{"auto+custom自定义通配符回填", "auto+custom", "house-llm-*", "", "house-llm-v2", true},
		{"auto+custom自定义正则回填", "auto+custom", "", "^acme-.*", "acme-bot", true},
		{"auto+custom均不命中", "auto+custom", "house-*", "^acme-.*", "gpt-4o", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := promptCachingModelMatched(c.mode, c.patterns, c.regex, c.model); got != c.want {
				t.Fatalf("mode=%s patterns=%q regex=%q model=%s: want %v got %v", c.mode, c.patterns, c.regex, c.model, c.want, got)
			}
		})
	}
}

func TestInjectPromptCachingStruct(t *testing.T) {
	cfg := &PromptCachingConfig{TTL: "5m", Strategy: "system"}

	t.Run("system字符串转数组并注入", func(t *testing.T) {
		req := &ClaudeRequest{System: "you are helpful"}
		if !InjectPromptCachingStruct(req, cfg) {
			t.Fatal("expected injection")
		}
		sys, ok := req.System.([]MessageContent)
		if !ok || len(sys) != 1 || sys[0].CacheControl == nil {
			t.Fatalf("unexpected system: %#v", req.System)
		}
	})

	t.Run("已有cache_control时跳过", func(t *testing.T) {
		req := &ClaudeRequest{
			System: "s",
			Messages: []Message{{Role: "user", Content: []MessageContent{
				{Type: "text", Text: "hi", CacheControl: map[string]any{"type": "ephemeral"}},
			}}},
		}
		if InjectPromptCachingStruct(req, cfg) {
			t.Fatal("expected skip when cache_control already present")
		}
		if _, ok := req.System.(string); !ok {
			t.Fatalf("system should be untouched: %#v", req.System)
		}
	})

	t.Run("system+last_user策略注入末位user", func(t *testing.T) {
		cfg2 := &PromptCachingConfig{TTL: "1h", Strategy: "system+last_user"}
		req := &ClaudeRequest{
			System: "s",
			Messages: []Message{
				{Role: "user", Content: []MessageContent{{Type: "text", Text: "q1"}}},
				{Role: "assistant", Content: []MessageContent{{Type: "text", Text: "a1"}}},
				{Role: "user", Content: []MessageContent{{Type: "text", Text: "q2"}}},
			},
		}
		if !InjectPromptCachingStruct(req, cfg2) {
			t.Fatal("expected injection")
		}
		last := req.Messages[2].Content.([]MessageContent)
		cc, ok := last[0].CacheControl.(map[string]any)
		if !ok || cc["ttl"] != "1h" {
			t.Fatalf("unexpected cache_control on last user: %#v", last[0].CacheControl)
		}
		if first := req.Messages[0].Content.([]MessageContent); first[0].CacheControl != nil {
			t.Fatal("earlier user message should be untouched")
		}
	})
}

// TestCachedWriteBucketSplit 验证 1h 缓存写入与 5m 独立成桶，并按官方倍率计费
// （5m=1.25x、1h=2.0x）。扁平 cache_creation_input_tokens 为权威总数，嵌套
// ephemeral_1h_input_tokens 决定 1h 占比，其余归入 5m 桶。
func TestCachedWriteBucketSplit(t *testing.T) {
	cUsage := &Usage{
		InputTokens:              10,
		OutputTokens:             5,
		CacheCreationInputTokens: 100,
		CacheCreation:            &CacheCreationUsage{Ephemeral1hInputTokens: 30},
	}
	usage := &types.Usage{}
	if !ClaudeUsageToOpenaiUsage(cUsage, usage) {
		t.Fatal("expected conversion to succeed")
	}

	if got := usage.PromptTokensDetails.CachedWriteTokens; got != 70 {
		t.Fatalf("5m bucket: want 70 got %d", got)
	}
	if got := usage.PromptTokensDetails.CachedWrite1hTokens; got != 30 {
		t.Fatalf("1h bucket: want 30 got %d", got)
	}

	extra := usage.GetExtraTokens()
	if got := extra[config.UsageExtraCachedWrite]; got != 70 {
		t.Fatalf("extra cached_write_tokens: want 70 got %d", got)
	}
	if got := extra[config.UsageExtraCachedWrite1h]; got != 30 {
		t.Fatalf("extra cached_write_1h_tokens: want 30 got %d", got)
	}

	p := &model.Price{}
	if got := p.GetExtraRatio(config.UsageExtraCachedWrite); got != 1.25 {
		t.Fatalf("5m ratio: want 1.25 got %v", got)
	}
	if got := p.GetExtraRatio(config.UsageExtraCachedWrite1h); got != 2 {
		t.Fatalf("1h ratio: want 2.0 got %v", got)
	}
}

// TestCachedWriteBucketDefault5m 验证缺少嵌套 1h 字段时全部归入 5m 桶，默认行为不回归。
func TestCachedWriteBucketDefault5m(t *testing.T) {
	cUsage := &Usage{
		InputTokens:              10,
		OutputTokens:             5,
		CacheCreationInputTokens: 100,
	}
	usage := &types.Usage{}
	if !ClaudeUsageToOpenaiUsage(cUsage, usage) {
		t.Fatal("expected conversion to succeed")
	}
	if got := usage.PromptTokensDetails.CachedWriteTokens; got != 100 {
		t.Fatalf("5m bucket: want 100 got %d", got)
	}
	if got := usage.PromptTokensDetails.CachedWrite1hTokens; got != 0 {
		t.Fatalf("1h bucket: want 0 got %d", got)
	}
}

func mustValidJSON(t *testing.T, b []byte) {
	t.Helper()
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("invalid json after injection: %v\n%s", err, b)
	}
}

func TestInjectPromptCachingBytes(t *testing.T) {
	t.Run("system字符串转数组并注入", func(t *testing.T) {
		body := []byte(`{"model":"claude-sonnet-4-5","system":"you are helpful","messages":[{"role":"user","content":"hi"}],"max_tokens":100}`)
		out, injected := InjectPromptCachingBytes(body, &PromptCachingConfig{TTL: "5m", Strategy: "system"})
		if !injected {
			t.Fatal("expected injection")
		}
		mustValidJSON(t, out)
		var req struct {
			System []map[string]any `json:"system"`
		}
		if err := json.Unmarshal(out, &req); err != nil || len(req.System) != 1 || req.System[0]["cache_control"] == nil {
			t.Fatalf("unexpected system after injection: %s", out)
		}
	})

	t.Run("system数组注入末位且1h带ttl", func(t *testing.T) {
		body := []byte(`{"system":[{"type":"text","text":"a"},{"type":"text","text":"b"}],"messages":[{"role":"user","content":[{"type":"text","text":"q"}]}]}`)
		out, injected := InjectPromptCachingBytes(body, &PromptCachingConfig{TTL: "1h", Strategy: "system+last_user"})
		if !injected {
			t.Fatal("expected injection")
		}
		mustValidJSON(t, out)
		var req struct {
			System   []map[string]any `json:"system"`
			Messages []struct {
				Content []map[string]any `json:"content"`
			} `json:"messages"`
		}
		if err := json.Unmarshal(out, &req); err != nil {
			t.Fatal(err)
		}
		if req.System[0]["cache_control"] != nil {
			t.Fatal("first system block should be untouched")
		}
		cc, _ := req.System[1]["cache_control"].(map[string]any)
		if cc == nil || cc["ttl"] != "1h" {
			t.Fatalf("unexpected system cache_control: %s", out)
		}
		ucc, _ := req.Messages[0].Content[0]["cache_control"].(map[string]any)
		if ucc == nil || ucc["ttl"] != "1h" {
			t.Fatalf("unexpected user cache_control: %s", out)
		}
	})

	t.Run("已有cache_control时跳过", func(t *testing.T) {
		body := []byte(`{"system":"s","messages":[{"role":"user","content":[{"type":"text","text":"q","cache_control":{"type":"ephemeral"}}]}]}`)
		out, injected := InjectPromptCachingBytes(body, &PromptCachingConfig{TTL: "5m", Strategy: "system"})
		if injected {
			t.Fatal("expected skip when cache_control already present")
		}
		if string(out) != string(body) {
			t.Fatal("body should be untouched")
		}
	})

	t.Run("未知字段与顺序保持", func(t *testing.T) {
		body := []byte(`{"model":"claude-sonnet-4-5","service_tier":"standard_only","messages":[{"role":"user","content":"hi"}]}`)
		out, injected := InjectPromptCachingBytes(body, &PromptCachingConfig{TTL: "5m", Strategy: "system"})
		if injected {
			t.Fatal("no system and string-content untouched under system strategy")
		}
		if string(out) != string(body) {
			t.Fatal("body should be untouched")
		}
	})
}

func TestApplyPromptCachingBeta(t *testing.T) {
	t.Run("仅1h追加beta", func(t *testing.T) {
		h := map[string]string{}
		ApplyPromptCachingBeta(h, &PromptCachingConfig{TTL: "5m"})
		if len(h) != 0 {
			t.Fatal("5m should not add beta header")
		}
		ApplyPromptCachingBeta(h, &PromptCachingConfig{TTL: "1h"})
		if h["anthropic-beta"] != "extended-cache-ttl-2025-04-11" {
			t.Fatalf("unexpected headers: %v", h)
		}
	})

	t.Run("已有beta时逗号追加且不重复", func(t *testing.T) {
		h := map[string]string{"anthropic-beta": "output-128k-2025-02-19"}
		ApplyPromptCachingBeta(h, &PromptCachingConfig{TTL: "1h"})
		if h["anthropic-beta"] != "output-128k-2025-02-19,extended-cache-ttl-2025-04-11" {
			t.Fatalf("unexpected: %v", h)
		}
		ApplyPromptCachingBeta(h, &PromptCachingConfig{TTL: "1h"})
		if h["anthropic-beta"] != "output-128k-2025-02-19,extended-cache-ttl-2025-04-11" {
			t.Fatalf("should not duplicate: %v", h)
		}
	})
}

func TestRecordPromptCachingInjection(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		return c
	}

	t.Run("注入成功写入三键", func(t *testing.T) {
		c := newCtx()
		RecordPromptCachingInjection(c, &PromptCachingConfig{TTL: "1h", Strategy: "system+last_user"}, true)
		if v := c.GetBool(GinKeyPromptCachingInjected); !v {
			t.Fatal("expected injected=true")
		}
		if v := c.GetString(GinKeyPromptCachingTTL); v != "1h" {
			t.Fatalf("unexpected ttl: %q", v)
		}
		if v := c.GetString(GinKeyPromptCachingStrategy); v != "system+last_user" {
			t.Fatalf("unexpected strategy: %q", v)
		}
	})

	t.Run("未注入不留痕", func(t *testing.T) {
		c := newCtx()
		RecordPromptCachingInjection(c, &PromptCachingConfig{TTL: "5m", Strategy: "system"}, false)
		if _, ok := c.Get(GinKeyPromptCachingInjected); ok {
			t.Fatal("expected no key when not injected")
		}
	})

	t.Run("nil配置安全", func(t *testing.T) {
		c := newCtx()
		RecordPromptCachingInjection(c, nil, true)
		if _, ok := c.Get(GinKeyPromptCachingInjected); ok {
			t.Fatal("expected no key for nil cfg")
		}
	})
}
