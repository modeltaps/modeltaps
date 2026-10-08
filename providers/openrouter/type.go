package openrouter

import "github.com/modeltaps/modeltaps/types"

type ChatCompletionRequest struct {
	types.ChatCompletionRequest
	Provider orProvider `json:"provider,omitempty"`
}

type orProvider struct {
	Order          []string `json:"order,omitempty"`
	Ignore         []string `json:"ignore,omitempty"`
	AllowFallbacks bool     `json:"allow_fallbacks,omitempty"`
}

// OpenRouter /v1/models 响应中每个模型自带的 pricing（值为字符串 USD/token 或 USD/次）。
type modelPricing struct {
	Prompt         string `json:"prompt"`
	Completion     string `json:"completion"`
	Request        string `json:"request"`
	Image          string `json:"image"`
	InputCacheRead string `json:"input_cache_read"`
}

type modelArchitecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

type modelTopProvider struct {
	MaxCompletionTokens int `json:"max_completion_tokens"`
}

type modelWithPricing struct {
	Id                  string            `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	ContextLen          int               `json:"context_length"`
	Architecture        modelArchitecture `json:"architecture"`
	TopProvider         modelTopProvider  `json:"top_provider"`
	Pricing             modelPricing      `json:"pricing"`
	SupportedParameters []string          `json:"supported_parameters"`
	SupportedVoices     []string          `json:"supported_voices"`
}

type modelPricingResponse struct {
	Data []modelWithPricing `json:"data"`
}

// GenerationResponse 是 GET /api/v1/generation?id=... 的响应包裹(W9-M6)。
type GenerationResponse struct {
	Data GenerationData `json:"data"`
}

// GenerationData 只取「供应商响应」图需要的 provider 侧真实字段:
// provider_name(实际路由到的供应商名)、latency(响应首字延迟 ms)、generation_time(生成耗时 ms)。
// 其余 OpenRouter 统计字段本任务不消费,故不建模。
type GenerationData struct {
	ID             string  `json:"id"`
	ProviderName   string  `json:"provider_name"`
	Latency        float64 `json:"latency"`
	GenerationTime float64 `json:"generation_time"`
}
