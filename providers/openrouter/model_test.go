package openrouter

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// 模拟 OpenRouter /v1/models 的 pricing 结构（值为字符串 USD/token）。
// buildOpenRouterPrices 应：① token 模型按 USD×500000 转倍率；② input_cache_read 映射为
// ExtraRatios（相对 input 倍数）；③ 免费(全 0) 模型保留为 0；④ 按次(request>0) 模型 Phase 1 跳过。
const fixture = `{
  "data": [
    {"id": "anthropic/claude-3.5-sonnet", "pricing": {"prompt": "0.000003", "completion": "0.000015", "request": "0", "input_cache_read": "0.0000003"}},
    {"id": "vendor/free-model", "pricing": {"prompt": "0", "completion": "0", "request": "0"}},
    {"id": "vendor/per-call", "pricing": {"prompt": "0", "completion": "0", "request": "0.01"}}
  ]
}`

func TestBuildOpenRouterPrices(t *testing.T) {
	resp := &modelPricingResponse{}
	if err := json.Unmarshal([]byte(fixture), resp); err != nil {
		t.Fatalf("unmarshal fixture: %v", err)
	}

	prices := buildOpenRouterPrices(resp)

	byModel := map[string]bool{}
	for _, p := range prices {
		byModel[p.Model] = true
	}

	// 按次模型应被跳过（避免被误标为免费），并列入换算不了的结果
	if byModel["vendor/per-call"] {
		t.Error("按次计费模型 vendor/per-call 不应出现在 Phase 1 结果中")
	}
	if _, unconvertible := buildOpenRouterPriceSync(resp); len(unconvertible) != 1 ||
		unconvertible[0].Model != "vendor/per-call" || unconvertible[0].Reason != model.UnconvertiblePerRequest {
		t.Errorf("unconvertible = %+v, want [vendor/per-call per_request]", unconvertible)
	}
	// 免费模型应保留
	if !byModel["vendor/free-model"] {
		t.Error("免费模型 vendor/free-model 应保留")
	}

	var sonnet *struct{ in, out, cache float64 }
	for _, p := range prices {
		if p.Model == "anthropic/claude-3.5-sonnet" {
			c := 0.0
			if p.ExtraRatios != nil {
				c = p.ExtraRatios.Data()[config.UsageExtraCachedRead]
			}
			sonnet = &struct{ in, out, cache float64 }{p.Input, p.Output, c}
			if p.ChannelType != config.ChannelTypeOpenRouter {
				t.Errorf("ChannelType = %d, want %d", p.ChannelType, config.ChannelTypeOpenRouter)
			}
		}
	}
	if sonnet == nil {
		t.Fatal("缺少 anthropic/claude-3.5-sonnet")
	}
	if !floatEq(sonnet.in, 1.5) || !floatEq(sonnet.out, 7.5) {
		t.Errorf("sonnet in/out = %g/%g, want 1.5/7.5", sonnet.in, sonnet.out)
	}
	if !floatEq(sonnet.cache, 0.1) {
		t.Errorf("sonnet cached_read = %g, want 0.1", sonnet.cache)
	}
}

const infoFixture = `{
  "data": [
    {"id": "openai/gpt-4o", "name": "OpenAI: GPT-4o", "description": "omni model", "context_length": 128000,
     "architecture": {"input_modalities": ["text","image"], "output_modalities": ["text"]},
     "top_provider": {"max_completion_tokens": 16384},
     "pricing": {"prompt": "0.0000025", "completion": "0.00001"}},
    {"id": "", "name": "skip me", "pricing": {"prompt": "0"}}
  ]
}`

func TestBuildOpenRouterModelInfos(t *testing.T) {
	resp := &modelPricingResponse{}
	if err := json.Unmarshal([]byte(infoFixture), resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	infos := buildOpenRouterModelInfos(resp)
	if len(infos) != 1 {
		t.Fatalf("want 1 info (empty id skipped), got %d", len(infos))
	}
	m := infos[0]
	if m.Model != "openai/gpt-4o" {
		t.Errorf("Model=%q", m.Model)
	}
	if m.Name != "OpenAI: GPT-4o" {
		t.Errorf("Name=%q", m.Name)
	}
	if m.ContextLength != 128000 {
		t.Errorf("ContextLength=%d", m.ContextLength)
	}
	if m.MaxTokens != 16384 {
		t.Errorf("MaxTokens=%d", m.MaxTokens)
	}
	if m.InputModalities != `["text","image"]` {
		t.Errorf("InputModalities=%q", m.InputModalities)
	}
	if m.OutputModalities != `["text"]` {
		t.Errorf("OutputModalities=%q", m.OutputModalities)
	}
}

// supported_parameters → capabilities：仅 response_format 不算结构化输出，
// include_reasoning 算推理，一项都不命中写 "[]"。
func TestBuildOpenRouterModelInfosCapabilities(t *testing.T) {
	const fx = `{
  "data": [
    {"id": "all", "supported_parameters": ["tools","reasoning","structured_outputs","temperature"]},
    {"id": "none", "supported_parameters": ["temperature","top_p"]},
    {"id": "missing"},
    {"id": "partial", "supported_parameters": ["tools","response_format"]},
    {"id": "include-reasoning", "supported_parameters": ["include_reasoning"]},
    {"id": "both-reasoning", "supported_parameters": ["reasoning","include_reasoning"]}
  ]
}`
	resp := &modelPricingResponse{}
	if err := json.Unmarshal([]byte(fx), resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	byModel := map[string]string{}
	for _, m := range buildOpenRouterModelInfos(resp) {
		byModel[m.Model] = m.Capabilities
	}

	cases := []struct{ model, want string }{
		{"all", `["tool_call","reasoning","structured_output"]`},
		{"none", `[]`},
		{"missing", `[]`},
		{"partial", `["tool_call"]`},
		{"include-reasoning", `["reasoning"]`},
		{"both-reasoning", `["reasoning"]`},
	}
	for _, c := range cases {
		if got := byModel[c.model]; got != c.want {
			t.Errorf("%s capabilities = %q, want %q", c.model, got, c.want)
		}
	}
}

// supported_voices 原样存为 JSON 数组；没有音色的模型留空串。
func TestBuildOpenRouterModelInfosSupportedVoices(t *testing.T) {
	const fx = `{
  "data": [
    {"id": "hexgrad/kokoro-82m", "architecture": {"output_modalities": ["speech"]}, "supported_voices": ["af_alloy","zf_xiaoxiao"]},
    {"id": "fish-audio/s1", "architecture": {"output_modalities": ["speech"]}, "supported_voices": []},
    {"id": "openai/gpt-4o"}
  ]
}`
	resp := &modelPricingResponse{}
	if err := json.Unmarshal([]byte(fx), resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	byModel := map[string]string{}
	for _, m := range buildOpenRouterModelInfos(resp) {
		byModel[m.Model] = m.SupportedVoices
	}
	if got := byModel["hexgrad/kokoro-82m"]; got != `["af_alloy","zf_xiaoxiao"]` {
		t.Errorf("kokoro supported_voices = %q", got)
	}
	if byModel["fish-audio/s1"] != "" || byModel["openai/gpt-4o"] != "" {
		t.Errorf("models without voices should stay empty: %v", byModel)
	}
}

// 音频模型：STT 按秒计价一律不导入价格；TTS 仅纯按字符计价(completion=0)的导入；
// endpoints 分别推出 audio.transcription / audio.speech。
func TestOpenRouterAudioModels(t *testing.T) {
	const fx = `{
  "data": [
    {"id": "openai/whisper-1", "architecture": {"input_modalities": ["audio"], "output_modalities": ["transcription"]},
     "pricing": {"prompt": "0.0001", "completion": "0"}},
    {"id": "hexgrad/kokoro-82m", "architecture": {"input_modalities": ["text"], "output_modalities": ["speech"]},
     "pricing": {"prompt": "0.000004", "completion": "0"}},
    {"id": "google/gemini-tts", "architecture": {"input_modalities": ["text"], "output_modalities": ["speech"]},
     "pricing": {"prompt": "0.0000005", "completion": "0.000006"}}
  ]
}`
	resp := &modelPricingResponse{}
	if err := json.Unmarshal([]byte(fx), resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	prices := map[string]float64{}
	for _, p := range buildOpenRouterPrices(resp) {
		prices[p.Model] = p.Input
	}
	if _, ok := prices["openai/whisper-1"]; ok {
		t.Error("STT 模型不应导入价格")
	}
	if _, ok := prices["google/gemini-tts"]; ok {
		t.Error("按音频 token 计价的 TTS 模型不应导入价格")
	}
	if !floatEq(prices["hexgrad/kokoro-82m"], 2) {
		t.Errorf("kokoro input ratio = %g, want 2", prices["hexgrad/kokoro-82m"])
	}

	_, unconvertible := buildOpenRouterPriceSync(resp)
	reasons := map[string]string{}
	for _, u := range unconvertible {
		reasons[u.Model] = u.Reason
	}
	if reasons["openai/whisper-1"] != model.UnconvertibleAudioDuration {
		t.Errorf("whisper reason = %q, want %q", reasons["openai/whisper-1"], model.UnconvertibleAudioDuration)
	}
	if reasons["google/gemini-tts"] != model.UnconvertibleAudioOutputTokens {
		t.Errorf("gemini-tts reason = %q, want %q", reasons["google/gemini-tts"], model.UnconvertibleAudioOutputTokens)
	}
	if _, ok := reasons["hexgrad/kokoro-82m"]; ok {
		t.Error("可换算的 TTS 不应列入 unconvertible")
	}

	endpoints := map[string]string{}
	for _, m := range buildOpenRouterModelInfos(resp) {
		endpoints[m.Model] = m.Endpoints
	}
	if endpoints["openai/whisper-1"] != `["audio.transcription"]` {
		t.Errorf("whisper endpoints = %q", endpoints["openai/whisper-1"])
	}
	if endpoints["hexgrad/kokoro-82m"] != `["audio.speech"]` {
		t.Errorf("kokoro endpoints = %q", endpoints["hexgrad/kokoro-82m"])
	}
}

func floatEq(a, b float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < 1e-9
}
