package vercel

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// gatewayModel 是 Vercel AI Gateway `GET /v1/models` 的一条模型（OpenAI 列表格式加网关扩展字段）。
// pricing 里的金额是「USD/token」字符串；image / video 模型按张计价（pricing.image）。
type gatewayModel struct {
	Id            string       `json:"id"`
	OwnedBy       string       `json:"owned_by"`
	Name          string       `json:"name"`
	Description   string       `json:"description"`
	ContextWindow int          `json:"context_window"`
	MaxTokens     int          `json:"max_tokens"`
	Type          string       `json:"type"` // language / embedding / reranking / image / video
	Tags          []string     `json:"tags"` // reasoning / tool-use / vision / file-input …
	Pricing       modelPricing `json:"pricing"`
}

type modelPricing struct {
	Input           string `json:"input"`
	Output          string `json:"output"`
	InputCacheRead  string `json:"input_cache_read"`
	InputCacheWrite string `json:"input_cache_write"`
	Image           string `json:"image"`
}

type modelListResponse struct {
	Data []gatewayModel `json:"data"`
}

func (p *VercelProvider) fetchModels() (*modelListResponse, error) {
	fullRequestURL := p.GetFullRequestURL(p.Config.ModelList, "")
	headers := p.GetRequestHeaders()

	req, err := p.Requester.NewRequest(http.MethodGet, fullRequestURL, p.Requester.WithHeader(headers))
	if err != nil {
		return nil, errors.New("new_request_failed")
	}
	resp := &modelListResponse{}
	_, errWithCode := p.Requester.SendRequest(req, resp, false)
	if errWithCode != nil {
		return nil, errors.New(errWithCode.Message)
	}
	return resp, nil
}

// GetModelList 返回网关全部模型 id，供渠道「拉取模型」使用。
func (p *VercelProvider) GetModelList() ([]string, error) {
	resp, err := p.fetchModels()
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

// GetModelPriceList 实现 base.ModelPriceListInterface。
func (p *VercelProvider) GetModelPriceList() ([]*model.Price, error) {
	prices, _, err := p.GetModelPriceSync()
	return prices, err
}

// GetModelPriceSync 同 GetModelPriceList，并返回价格换算不了的模型（按张计价的生图 / 视频模型）。
func (p *VercelProvider) GetModelPriceSync() ([]*model.Price, []model.UnconvertiblePrice, error) {
	resp, err := p.fetchModels()
	if err != nil {
		return nil, nil, err
	}
	prices, unconvertible := buildPriceSync(resp)
	return prices, unconvertible, nil
}

// GetModelInfoList 实现 base.ModelInfoListInterface。
func (p *VercelProvider) GetModelInfoList() ([]*model.ModelInfo, error) {
	resp, err := p.fetchModels()
	if err != nil {
		return nil, err
	}
	return buildModelInfos(resp), nil
}

// parseUSD 把字符串价格解析为 float（USD）。空 / 非法 / 负值一律按 0。
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

// buildPriceSync 纯函数：把网关模型列表转为 []*model.Price。
// 只导入 token 计费的模型；按张计价（pricing.image）且无 token 价的模型列为不可换算，交管理员手工定价。
func buildPriceSync(resp *modelListResponse) ([]*model.Price, []model.UnconvertiblePrice) {
	if resp == nil {
		return nil, nil
	}
	prices := make([]*model.Price, 0, len(resp.Data))
	var unconvertible []model.UnconvertiblePrice
	for _, m := range resp.Data {
		if m.Id == "" {
			continue
		}
		input := parseUSD(m.Pricing.Input)
		output := parseUSD(m.Pricing.Output)
		if input == 0 && output == 0 {
			reason := model.UnconvertibleNoPrice
			if parseUSD(m.Pricing.Image) > 0 {
				reason = model.UnconvertiblePerRequest
			}
			unconvertible = append(unconvertible, model.UnconvertiblePrice{Model: m.Id, Reason: reason})
			continue
		}
		prices = append(prices, model.PriceFromUSDPerToken(m.Id, config.ChannelTypeVercelAIGateway,
			input, output, parseUSD(m.Pricing.InputCacheRead)))
	}
	return prices, unconvertible
}

// modalitiesFor 由模型类型与能力标签推导输入 / 输出模态（网关列表不直接给模态数组）。
func modalitiesFor(m gatewayModel) (input, output []string) {
	switch m.Type {
	case "image":
		return []string{"text"}, []string{"image"}
	case "video":
		return []string{"text"}, []string{"video"}
	case "embedding", "reranking":
		return []string{"text"}, []string{"text"}
	}
	input = []string{"text"}
	for _, tag := range m.Tags {
		switch tag {
		case "vision":
			input = append(input, "image")
		case "file-input":
			input = append(input, "file")
		}
	}
	return input, []string{"text"}
}

// capabilitiesJSON 把网关的能力标签映射为目录能力数组。网关没有结构化输出标签，所以这里只能给出两项；
// 空标签视为「未知」而不是「都不支持」。
func capabilitiesJSON(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	caps := make([]string, 0, 2)
	for _, tag := range tags {
		switch tag {
		case "tool-use":
			caps = append(caps, model.ModelCapabilityToolCall)
		case "reasoning":
			caps = append(caps, model.ModelCapabilityReasoning)
		}
	}
	return model.ModelCapabilitiesJSON(caps)
}

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

// buildModelInfos 纯函数：把网关模型列表转为 []*model.ModelInfo。
func buildModelInfos(resp *modelListResponse) []*model.ModelInfo {
	if resp == nil {
		return nil
	}
	infos := make([]*model.ModelInfo, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.Id == "" {
			continue
		}
		input, output := modalitiesFor(m)
		infos = append(infos, &model.ModelInfo{
			Model:            m.Id,
			Name:             m.Name,
			Description:      m.Description,
			ContextLength:    m.ContextWindow,
			MaxTokens:        m.MaxTokens,
			InputModalities:  modalitiesJSON(input),
			OutputModalities: modalitiesJSON(output),
			Capabilities:     capabilitiesJSON(m.Tags),
			// 模型名形如 "anthropic/claude-sonnet-4"，前缀即厂商 slug。
			VendorID:  model.VendorIDFromModelName(m.Id),
			Endpoints: model.ModelEndpointsFromModalities(m.Id, input, output),
		})
	}
	return infos
}
