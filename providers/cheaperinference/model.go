package cheaperinference

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/datatypes"
)

// publicModel is one entry of `GET /public/models`. Prices are USD per million tokens, as decimal
// strings, and are what a request pays (the route-dependent price, capped at the lab's list
// price); reference_* are the lab's list prices. aliases are vendor-qualified ids that resolve to
// the same model ("anthropic/claude-opus-5-5" for "claude-opus-5.5").
type publicModel struct {
	Id                 string   `json:"id"`
	Aliases            []string `json:"aliases"`
	Type               string   `json:"model_type"`
	ProviderName       string   `json:"provider_name"`
	ContextLength      int      `json:"context_length"`
	MaxOutputTokens    int      `json:"max_output_tokens"`
	Input              string   `json:"input_per_million"`
	Output             string   `json:"output_per_million"`
	CacheRead          string   `json:"cache_read_per_million"`
	CacheWrite         string   `json:"cache_write_per_million"`
	MediaUnitPrice     *string  `json:"media_unit_price"`
	MediaUnit          *string  `json:"media_unit"`
	Threshold          *int     `json:"input_token_price_threshold"`
	InputAbove         *string  `json:"input_per_million_above_threshold"`
	OutputAbove        *string  `json:"output_per_million_above_threshold"`
	SupportsVision     bool     `json:"supports_vision"`
	SupportsVideo      bool     `json:"supports_video"`
	SupportsReasoning  bool     `json:"supports_reasoning"`
	SupportsImageEdit  bool     `json:"supports_image_edit"`
	Features           []string `json:"features"`
	IsVisible          *bool    `json:"is_visible"`
	SupportedEndpoints []string `json:"supported_endpoints"`
}

type publicModelList struct {
	Models []publicModel `json:"models"`
}

// usd parses a price string; "" and unparsable or negative values are 0.
func usd(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || v < 0 {
		return 0
	}
	return v
}

func usdPtr(s *string) float64 {
	if s == nil {
		return 0
	}
	return usd(*s)
}

// fetchModels reads the public catalog. It takes no key, so prices keep syncing while the
// channel's key is wrong or expired. Marketplace-only models are left out of it; they can still
// be added to a channel by hand.
func (p *CheaperInferenceProvider) fetchModels() ([]publicModel, error) {
	headers := p.GetRequestHeaders()
	delete(headers, "Authorization")
	req, err := p.Requester.NewRequest(http.MethodGet, p.GetFullRequestURL("/public/models", ""), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, errors.New("new_request_failed")
	}
	resp := &publicModelList{}
	if _, errWithCode := p.Requester.SendRequest(req, resp, false); errWithCode != nil {
		return nil, errors.New(errWithCode.Message)
	}
	return resp.Models, nil
}

// listedName is the id a channel lists the model under: its first vendor-qualified alias, whose
// bare name is the lab's own id ("anthropic/claude-opus-5-5" for "claude-opus-5.5") and so
// resolves to the catalog name, else the id itself. Both reach the same model upstream.
func listedName(m publicModel) string {
	for _, a := range m.Aliases {
		if a = strings.TrimSpace(a); strings.Contains(a, "/") {
			return a
		}
	}
	return m.Id
}

// batch reports a discounted batch variant ("claude-opus-5.5-batch"): the same model served
// later, not a model a gateway should send live traffic to by default.
func batch(m publicModel) bool {
	return strings.HasSuffix(m.Id, "-batch")
}

func hidden(m publicModel) bool {
	return m.IsVisible != nil && !*m.IsVisible
}

// defaultModels is what "fetch models" adds: the visible text and embedding models, without the
// batch variants. Other kinds keep a channel price, so one added by hand is priced.
func defaultModels(models []publicModel) []publicModel {
	out := make([]publicModel, 0, len(models))
	seen := map[string]bool{}
	for _, m := range models {
		if m.Id == "" || hidden(m) || batch(m) || (m.Type != "text" && m.Type != "embedding") {
			continue
		}
		if name := listedName(m); !seen[name] {
			seen[name] = true
			out = append(out, m)
		}
	}
	return out
}

// GetModelList returns the default model list for the channel's "fetch models".
func (p *CheaperInferenceProvider) GetModelList() ([]string, error) {
	models, err := p.fetchModels()
	if err != nil {
		return nil, err
	}
	list := []string{}
	for _, m := range defaultModels(models) {
		list = append(list, listedName(m))
	}
	return list, nil
}

// GetModelPriceList implements base.ModelPriceListInterface.
func (p *CheaperInferenceProvider) GetModelPriceList() ([]*model.Price, error) {
	prices, _, err := p.GetModelPriceSync()
	return prices, err
}

// GetModelPriceSync returns the prices of the default list; a model with no token price is
// reported as unconvertible.
func (p *CheaperInferenceProvider) GetModelPriceSync() ([]*model.Price, []model.UnconvertiblePrice, error) {
	models, err := p.fetchModels()
	if err != nil {
		return nil, nil, err
	}
	prices, unconvertible := buildPriceSync(defaultModels(models))
	return prices, unconvertible, nil
}

func buildPriceSync(models []publicModel) ([]*model.Price, []model.UnconvertiblePrice) {
	prices := make([]*model.Price, 0, len(models))
	var unconvertible []model.UnconvertiblePrice
	for _, m := range models {
		name := listedName(m)
		input, output := usd(m.Input), usd(m.Output)
		if input == 0 && output == 0 {
			unconvertible = append(unconvertible, model.UnconvertiblePrice{Model: name, Reason: model.UnconvertibleNoPrice})
			continue
		}
		pc := model.PriceFromUSDPerToken(name, config.ChannelTypeCheaperInference, input/1e6, output/1e6, usd(m.CacheRead)/1e6)
		pc.LongContext = longContext(m, input, output)
		prices = append(prices, pc)
	}
	return prices, unconvertible
}

// longContext turns the price above the prompt threshold into a long context tier: the ratios to
// the base input and output prices. The tier applies the input ratio to cache reads too.
func longContext(m publicModel, input, output float64) *datatypes.JSONType[model.LongContextTier] {
	if m.Threshold == nil || *m.Threshold <= 0 {
		return nil
	}
	tier := model.LongContextTier{Threshold: *m.Threshold}
	if v := usdPtr(m.InputAbove); v > 0 && input > 0 && v != input {
		tier.InputRatio = roundRatio(v / input)
	}
	if v := usdPtr(m.OutputAbove); v > 0 && output > 0 && v != output {
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
func (p *CheaperInferenceProvider) GetModelInfoList() ([]*model.ModelInfo, error) {
	models, err := p.fetchModels()
	if err != nil {
		return nil, err
	}
	return buildModelInfos(defaultModels(models)), nil
}

func hasFeature(m publicModel, f string) bool {
	for _, v := range m.Features {
		if v == f {
			return true
		}
	}
	return false
}

func buildModelInfos(models []publicModel) []*model.ModelInfo {
	infos := make([]*model.ModelInfo, 0, len(models))
	for _, m := range models {
		name := listedName(m)
		input := []string{"text"}
		if m.SupportsVision {
			input = append(input, "image")
		}
		if hasFeature(m, "audio-input") {
			input = append(input, "audio")
		}
		if m.SupportsVideo {
			input = append(input, "video")
		}
		output := []string{"text"}
		var caps []string
		if hasFeature(m, "tool-use") {
			caps = append(caps, model.ModelCapabilityToolCall)
		}
		if m.SupportsReasoning {
			caps = append(caps, model.ModelCapabilityReasoning)
		}
		if hasFeature(m, "structured-output") {
			caps = append(caps, model.ModelCapabilityStructuredOutput)
		}
		info := &model.ModelInfo{
			Model:            name,
			ContextLength:    m.ContextLength,
			MaxTokens:        m.MaxOutputTokens,
			InputModalities:  jsonList(input),
			OutputModalities: jsonList(output),
			Capabilities:     model.ModelCapabilitiesJSON(caps),
			VendorID:         vendorOf(m, name),
			Endpoints:        model.ModelEndpointsFromModalities(name, input, output),
		}
		if m.Type == "embedding" {
			info.Endpoints = model.ModelEndpointsJSON([]string{model.ModelEndpointEmbeddings})
		}
		infos = append(infos, info)
	}
	return infos
}

// vendorOf is who made the model: the provider name the catalog gives ("Anthropic", "DeepSeek"),
// else the name rules on the id.
func vendorOf(m publicModel, name string) int {
	if m.ProviderName != "" {
		if id := model.GuessVendorID(m.ProviderName + "/" + m.Id); id != 0 {
			return id
		}
	}
	if id := model.GuessVendorID(name); id != 0 {
		return id
	}
	return model.VendorIDFromNameRules(m.Id)
}

func jsonList(v []string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// GetChannelPrices implements base.ChannelPriceListInterface: every model of the catalog, under
// its id and each alias, so any spelling added by hand is priced. Media models are priced per
// image, second or request in ways a channel price only keeps in part: per-second audio is kept,
// the rest is marked Other.
func (p *CheaperInferenceProvider) GetChannelPrices() ([]*model.ChannelPrice, error) {
	models, err := p.fetchModels()
	if err != nil {
		return nil, err
	}
	return buildChannelPrices(models), nil
}

func buildChannelPrices(models []publicModel) []*model.ChannelPrice {
	out := make([]*model.ChannelPrice, 0, len(models))
	seen := map[string]bool{}
	for _, m := range models {
		if m.Id == "" {
			continue
		}
		cp := model.ChannelPrice{Kind: model.ModelKindFromUpstreamType(m.Type)}
		switch m.Type {
		case "text", "embedding", "rerank":
			cp.Input, cp.Output = usd(m.Input), usd(m.Output)
			cp.CacheRead, cp.CacheWrite = usd(m.CacheRead), usd(m.CacheWrite)
		default:
			unit := ""
			if m.MediaUnit != nil {
				unit = *m.MediaUnit
			}
			switch media := usdPtr(m.MediaUnitPrice); {
			case unit == "image" && media > 0:
				cp.Image = media
			case unit == "second" && media > 0 && (m.Type == "transcription" || m.Type == "speech"):
				cp.AudioSecond = media
			case unit == "second" && media > 0 && m.Type == "video":
				cp.VideoSecond = media
			default:
				cp.Other = media > 0 || usd(m.Input) > 0 || usd(m.Output) > 0
			}
		}
		if cp.Input == 0 && cp.Output == 0 && cp.Image == 0 && cp.AudioSecond == 0 && cp.VideoSecond == 0 && !cp.Other {
			continue
		}
		for _, name := range append([]string{m.Id}, m.Aliases...) {
			if name = strings.TrimSpace(name); name == "" || seen[name] {
				continue
			}
			seen[name] = true
			price := cp
			price.Model = name
			out = append(out, &price)
		}
	}
	return out
}
