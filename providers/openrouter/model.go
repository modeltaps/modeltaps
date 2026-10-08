package openrouter

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/model"
)

// audioOutputModalities 是 OpenRouter /v1/models 默认列表不返回、需按 output_modalities 单独筛选的音频模型类别。
var audioOutputModalities = []string{"speech", "transcription"}

// fetchOpenRouterModels 拉取并解析 OpenRouter /v1/models（价格 + 元数据一次性取回），
// 并合并 TTS / STT 模型列表；音频列表拉取失败不影响主列表。
func (p *OpenRouterProvider) fetchOpenRouterModels() (*modelPricingResponse, error) {
	resp, err := p.fetchOpenRouterModelList(p.Config.ModelList)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(resp.Data))
	for _, m := range resp.Data {
		seen[m.Id] = true
	}
	for _, modality := range audioOutputModalities {
		extra, err := p.fetchOpenRouterModelList(p.Config.ModelList + "?output_modalities=" + modality)
		if err != nil {
			continue
		}
		for _, m := range extra.Data {
			if seen[m.Id] {
				continue
			}
			seen[m.Id] = true
			resp.Data = append(resp.Data, m)
		}
	}
	return resp, nil
}

func (p *OpenRouterProvider) fetchOpenRouterModelList(path string) (*modelPricingResponse, error) {
	fullRequestURL := p.GetFullRequestURL(path, "")
	headers := p.GetRequestHeaders()

	req, err := p.Requester.NewRequest(http.MethodGet, fullRequestURL, p.Requester.WithHeader(headers))
	if err != nil {
		return nil, errors.New("new_request_failed")
	}

	resp := &modelPricingResponse{}
	_, errWithCode := p.Requester.SendRequest(req, resp, false)
	if errWithCode != nil {
		return nil, errors.New(errWithCode.Message)
	}
	return resp, nil
}

// GetModelList 覆盖 OpenAI 的默认实现：默认 /v1/models 不含 TTS / STT，
// 这里复用合并了音频模型的列表，使渠道「拉取模型」能拿到音频模型。
func (p *OpenRouterProvider) GetModelList() ([]string, error) {
	resp, err := p.fetchOpenRouterModels()
	if err != nil {
		return nil, err
	}
	list := make([]string, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.Id != "" {
			list = append(list, m.Id)
		}
	}
	return list, nil
}

// GetModelPriceList 拉取 OpenRouter 模型真实价格并转为内部 Price。实现 base.ModelPriceListInterface。
func (p *OpenRouterProvider) GetModelPriceList() ([]*model.Price, error) {
	prices, _, err := p.GetModelPriceSync()
	return prices, err
}

// GetModelPriceSync 同 GetModelPriceList，并返回价格换算不了的模型（供同步结果列出）。
func (p *OpenRouterProvider) GetModelPriceSync() ([]*model.Price, []model.UnconvertiblePrice, error) {
	resp, err := p.fetchOpenRouterModels()
	if err != nil {
		return nil, nil, err
	}
	prices, unconvertible := buildOpenRouterPriceSync(resp)
	return prices, unconvertible, nil
}

// GetModelInfoList 拉取 OpenRouter 模型元信息（名称/描述/上下文/模态）。实现 base.ModelInfoListInterface。
func (p *OpenRouterProvider) GetModelInfoList() ([]*model.ModelInfo, error) {
	resp, err := p.fetchOpenRouterModels()
	if err != nil {
		return nil, err
	}
	return buildOpenRouterModelInfos(resp), nil
}

// parseUSD 把 OpenRouter 的字符串价格解析为 float（USD）。空/非法/负值一律按 0。
func parseUSD(s string) float64 {
	if s == "" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

// audioPriceSkipReason 判断音频模型的价格是否无法映射到网关计费口径，返回原因（空串表示可导入）：
// STT 按音频秒数计价而网关只计转写文本 token，一律跳过；
// TTS 网关按输入字符计费，仅纯按字符计价（completion 为 0）的模型可导入。
func audioPriceSkipReason(m modelWithPricing, completion float64) string {
	output := m.Architecture.OutputModalities
	if hasModality(output, "transcription") {
		return model.UnconvertibleAudioDuration
	}
	if hasModality(output, "speech") && completion > 0 {
		return model.UnconvertibleAudioOutputTokens
	}
	return ""
}

func hasModality(modalities []string, want string) bool {
	for _, m := range modalities {
		if m == want {
			return true
		}
	}
	return false
}

// buildOpenRouterPrices 纯函数：把 OpenRouter 模型价格响应转为 []*model.Price。
// Phase 1 仅处理 token 计费（含缓存读取）；按次计费(request>0 且 token 价为 0)的模型跳过；
// 无法映射计费口径的 TTS / STT 模型跳过（落到未配置价格策略，由管理员手工定价）。
func buildOpenRouterPrices(resp *modelPricingResponse) []*model.Price {
	prices, _ := buildOpenRouterPriceSync(resp)
	return prices
}

// buildOpenRouterPriceSync 同 buildOpenRouterPrices，并把跳过的模型连同原因列出（不静默写 0）。
func buildOpenRouterPriceSync(resp *modelPricingResponse) ([]*model.Price, []model.UnconvertiblePrice) {
	if resp == nil {
		return nil, nil
	}
	prices := make([]*model.Price, 0, len(resp.Data))
	var unconvertible []model.UnconvertiblePrice
	for _, m := range resp.Data {
		if m.Id == "" {
			continue
		}
		request := parseUSD(m.Pricing.Request)
		prompt := parseUSD(m.Pricing.Prompt)
		completion := parseUSD(m.Pricing.Completion)
		if request > 0 && prompt == 0 && completion == 0 {
			unconvertible = append(unconvertible, model.UnconvertiblePrice{Model: m.Id, Reason: model.UnconvertiblePerRequest})
			continue
		}
		if reason := audioPriceSkipReason(m, completion); reason != "" {
			unconvertible = append(unconvertible, model.UnconvertiblePrice{Model: m.Id, Reason: reason})
			continue
		}
		cacheRead := parseUSD(m.Pricing.InputCacheRead)
		prices = append(prices, model.PriceFromOpenRouter(m.Id, prompt, completion, cacheRead))
	}
	return prices, unconvertible
}

// modalitiesJSON 把模态数组序列化为 JSON 字符串（与 ModelInfo 存储格式一致）；空则空串。
func modalitiesJSON(m []string) string {
	if len(m) == 0 {
		return ""
	}
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// capabilitiesJSON 把 OpenRouter 的 supported_parameters 映射为能力数组。
// 仅 response_format 不算结构化输出（它可能只是 json_object）。
func capabilitiesJSON(params []string) string {
	caps := make([]string, 0, 3)
	for _, p := range params {
		switch p {
		case "tools":
			caps = append(caps, model.ModelCapabilityToolCall)
		case "reasoning", "include_reasoning":
			caps = append(caps, model.ModelCapabilityReasoning)
		case "structured_outputs":
			caps = append(caps, model.ModelCapabilityStructuredOutput)
		}
	}
	return model.ModelCapabilitiesJSON(caps)
}

// buildOpenRouterModelInfos 纯函数：把 OpenRouter 响应转为 []*model.ModelInfo（元数据）。
func buildOpenRouterModelInfos(resp *modelPricingResponse) []*model.ModelInfo {
	if resp == nil {
		return nil
	}
	infos := make([]*model.ModelInfo, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.Id == "" {
			continue
		}
		infos = append(infos, &model.ModelInfo{
			Model:            m.Id,
			Name:             m.Name,
			Description:      m.Description,
			ContextLength:    m.ContextLen,
			MaxTokens:        m.TopProvider.MaxCompletionTokens,
			InputModalities:  modalitiesJSON(m.Architecture.InputModalities),
			OutputModalities: modalitiesJSON(m.Architecture.OutputModalities),
			Capabilities:     capabilitiesJSON(m.SupportedParameters),
			SupportedVoices:  modalitiesJSON(m.SupportedVoices),
			// OpenRouter 模型名形如 "google/gemini-2.5-pro"，前缀即厂商 slug。
			VendorID:  model.VendorIDFromModelName(m.Id),
			Endpoints: model.ModelEndpointsFromModalities(m.Id, m.Architecture.InputModalities, m.Architecture.OutputModalities),
		})
	}
	return infos
}
