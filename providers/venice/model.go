package venice

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/datatypes"
)

// usdPrice is one price in USD and in DIEM (the same number: a DIEM buys $1 of inference).
type usdPrice struct {
	USD float64 `json:"usd"`
}

// pricing is a model's price. Token prices are USD per million tokens (TTS input per million
// characters); extended is the price once the prompt passes context_token_threshold.
type pricing struct {
	Input        *usdPrice `json:"input"`
	Output       *usdPrice `json:"output"`
	CacheInput   *usdPrice `json:"cache_input"`
	CacheWrite   *usdPrice `json:"cache_write"`
	Generation   *usdPrice `json:"generation"`
	AudioSeconds *usdPrice `json:"per_audio_second"`
	Extended     *struct {
		Threshold  int       `json:"context_token_threshold"`
		Input      *usdPrice `json:"input"`
		Output     *usdPrice `json:"output"`
		CacheInput *usdPrice `json:"cache_input"`
	} `json:"extended"`
}

type capabilities struct {
	SupportsFunctionCalling bool `json:"supportsFunctionCalling"`
	SupportsReasoning       bool `json:"supportsReasoning"`
	SupportsResponseSchema  bool `json:"supportsResponseSchema"`
	SupportsVision          bool `json:"supportsVision"`
	SupportsAudioInput      bool `json:"supportsAudioInput"`
	SupportsVideoInput      bool `json:"supportsVideoInput"`
	SupportsE2EE            bool `json:"supportsE2EE"`
}

// veniceModel is one entry of `GET /models?type=all`.
type veniceModel struct {
	Id      string `json:"id"`
	Type    string `json:"type"`
	Created int64  `json:"created"`
	Spec    struct {
		Pricing                pricing       `json:"pricing"`
		AvailableContextTokens int           `json:"availableContextTokens"`
		MaxCompletionTokens    int           `json:"maxCompletionTokens"`
		Capabilities           *capabilities `json:"capabilities"`
		Description            string        `json:"description"`
		Offline                bool          `json:"offline"`
		ModelSource            string        `json:"modelSource"`
	} `json:"model_spec"`
}

type modelListResponse struct {
	Data []veniceModel `json:"data"`
}

func price(p *usdPrice) float64 {
	if p == nil || p.USD < 0 {
		return 0
	}
	return p.USD
}

// fetchModels reads every model Venice lists. The list is public and ignores the key, so it is
// read without one: prices keep syncing while the channel's key is wrong or expired.
func (p *VeniceProvider) fetchModels() ([]veniceModel, error) {
	headers := p.GetRequestHeaders()
	delete(headers, "Authorization")
	req, err := p.Requester.NewRequest(http.MethodGet, p.GetFullRequestURL(p.Config.ModelList+"?type=all", ""), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, errors.New("new_request_failed")
	}
	resp := &modelListResponse{}
	if _, errWithCode := p.Requester.SendRequest(req, resp, false); errWithCode != nil {
		return nil, errors.New(errWithCode.Message)
	}
	return resp.Data, nil
}

// e2ee reports a model that only takes end-to-end encrypted requests: the client encrypts the
// prompt for the model's enclave, which a gateway in the middle cannot do for it.
func e2ee(m veniceModel) bool {
	return strings.HasPrefix(m.Id, "e2ee-") || (m.Spec.Capabilities != nil && m.Spec.Capabilities.SupportsE2EE)
}

// defaultModels is what "fetch models" adds: the text and embedding models that are online,
// without the end-to-end encrypted ones. Image, speech and transcription models keep a channel
// price, so one added by hand is priced.
func defaultModels(models []veniceModel) []veniceModel {
	out := make([]veniceModel, 0, len(models))
	for _, m := range models {
		if m.Id == "" || m.Spec.Offline || e2ee(m) {
			continue
		}
		if m.Type == "text" || m.Type == "embedding" {
			out = append(out, m)
		}
	}
	return out
}

// GetModelList returns the default model list for the channel's "fetch models".
func (p *VeniceProvider) GetModelList() ([]string, error) {
	models, err := p.fetchModels()
	if err != nil {
		return nil, err
	}
	list := []string{}
	for _, m := range defaultModels(models) {
		list = append(list, m.Id)
	}
	return list, nil
}

// GetModelPriceList implements base.ModelPriceListInterface.
func (p *VeniceProvider) GetModelPriceList() ([]*model.Price, error) {
	prices, _, err := p.GetModelPriceSync()
	return prices, err
}

// GetModelPriceSync returns the prices of the default list and of the transcription models; a
// model priced per image or not at all is reported as unconvertible.
func (p *VeniceProvider) GetModelPriceSync() ([]*model.Price, []model.UnconvertiblePrice, error) {
	models, err := p.fetchModels()
	if err != nil {
		return nil, nil, err
	}
	prices, unconvertible := buildPriceSync(models)
	return prices, unconvertible, nil
}

func buildPriceSync(models []veniceModel) ([]*model.Price, []model.UnconvertiblePrice) {
	var prices []*model.Price
	var unconvertible []model.UnconvertiblePrice
	for _, m := range models {
		if m.Id == "" || m.Spec.Offline || e2ee(m) {
			continue
		}
		pr := m.Spec.Pricing
		switch m.Type {
		case "text", "embedding":
		case "asr":
			if perSecond := price(pr.AudioSeconds); perSecond > 0 {
				prices = append(prices, model.PriceFromUSDPerAudioSecond(m.Id, config.ChannelTypeVenice, perSecond))
			} else {
				unconvertible = append(unconvertible, model.UnconvertiblePrice{Model: m.Id, Reason: model.UnconvertibleNoPrice})
			}
			continue
		default:
			continue
		}
		input, output := price(pr.Input), price(pr.Output)
		if input == 0 && output == 0 {
			unconvertible = append(unconvertible, model.UnconvertiblePrice{Model: m.Id, Reason: model.UnconvertibleNoPrice})
			continue
		}
		pc := model.PriceFromUSDPerToken(m.Id, config.ChannelTypeVenice, input/1e6, output/1e6, price(pr.CacheInput)/1e6)
		pc.LongContext = longContext(pr, input, output)
		prices = append(prices, pc)
	}
	return prices, unconvertible
}

// longContext turns the extended price into a long context tier: the ratios to the base input and
// output prices once the prompt passes the threshold. The extended cache read price is not kept;
// the tier applies the input ratio to it, as it does for every catalog price.
func longContext(pr pricing, input, output float64) *datatypes.JSONType[model.LongContextTier] {
	ext := pr.Extended
	if ext == nil || ext.Threshold <= 0 {
		return nil
	}
	tier := model.LongContextTier{Threshold: ext.Threshold}
	if v := price(ext.Input); v > 0 && input > 0 {
		tier.InputRatio = roundRatio(v / input)
	}
	if v := price(ext.Output); v > 0 && output > 0 {
		tier.OutputRatio = roundRatio(v / output)
	}
	if tier.InputRatio == 0 && tier.OutputRatio == 0 {
		return nil
	}
	lc := datatypes.NewJSONType(tier)
	return &lc
}

func roundRatio(v float64) float64 {
	return float64(int64(v*1e6+0.5)) / 1e6
}

// GetModelInfoList implements base.ModelInfoListInterface.
func (p *VeniceProvider) GetModelInfoList() ([]*model.ModelInfo, error) {
	models, err := p.fetchModels()
	if err != nil {
		return nil, err
	}
	return buildModelInfos(defaultModels(models)), nil
}

func buildModelInfos(models []veniceModel) []*model.ModelInfo {
	infos := make([]*model.ModelInfo, 0, len(models))
	for _, m := range models {
		input := []string{"text"}
		output := []string{"text"}
		var caps []string
		if c := m.Spec.Capabilities; c != nil {
			if c.SupportsVision {
				input = append(input, "image")
			}
			if c.SupportsAudioInput {
				input = append(input, "audio")
			}
			if c.SupportsVideoInput {
				input = append(input, "video")
			}
			if c.SupportsFunctionCalling {
				caps = append(caps, model.ModelCapabilityToolCall)
			}
			if c.SupportsReasoning {
				caps = append(caps, model.ModelCapabilityReasoning)
			}
			if c.SupportsResponseSchema {
				caps = append(caps, model.ModelCapabilityStructuredOutput)
			}
		}
		info := &model.ModelInfo{
			Model:            m.Id,
			Description:      m.Spec.Description,
			ContextLength:    m.Spec.AvailableContextTokens,
			MaxTokens:        m.Spec.MaxCompletionTokens,
			InputModalities:  jsonList(input),
			OutputModalities: jsonList(output),
			Capabilities:     model.ModelCapabilitiesJSON(caps),
			VendorID:         model.VendorIDFromNameRules(m.Id),
			Endpoints:        model.ModelEndpointsFromModalities(m.Id, input, output),
			ReleasedAt:       m.Created,
		}
		if m.Type == "embedding" {
			info.Endpoints = model.ModelEndpointsJSON([]string{model.ModelEndpointEmbeddings})
		}
		infos = append(infos, info)
	}
	return infos
}

func jsonList(v []string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// GetChannelPrices implements base.ChannelPriceListInterface: every model Venice prices in a way
// a channel price keeps (tokens, per image, per audio second), so a model added by hand is priced
// too. Speech is priced per million characters, which has no field here: it is marked Other.
func (p *VeniceProvider) GetChannelPrices() ([]*model.ChannelPrice, error) {
	models, err := p.fetchModels()
	if err != nil {
		return nil, err
	}
	return buildChannelPrices(models), nil
}

func buildChannelPrices(models []veniceModel) []*model.ChannelPrice {
	out := make([]*model.ChannelPrice, 0, len(models))
	for _, m := range models {
		if m.Id == "" {
			continue
		}
		pr := m.Spec.Pricing
		cp := &model.ChannelPrice{Model: m.Id, Kind: model.ModelKindFromUpstreamType(m.Type)}
		switch m.Type {
		case "text", "embedding":
			cp.Input, cp.Output = price(pr.Input), price(pr.Output)
			cp.CacheRead, cp.CacheWrite = price(pr.CacheInput), price(pr.CacheWrite)
		case "image":
			cp.Image = price(pr.Generation)
		case "asr":
			cp.AudioSecond = price(pr.AudioSeconds)
		case "tts":
			cp.Other = price(pr.Input) > 0
		default:
			// Video, music, inpainting and upscaling are priced per job through quote endpoints
			// the gateway does not relay.
			continue
		}
		if cp.Input == 0 && cp.Output == 0 && cp.Image == 0 && cp.AudioSecond == 0 && !cp.Other {
			continue
		}
		out = append(out, cp)
	}
	return out
}
