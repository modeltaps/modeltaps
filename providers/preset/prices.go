package preset

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// The platforms below publish their prices in their own public model list, which is fresher and
// more complete than models.dev's copy (checked on 2026-11-18: DeepInfra lists 177 priced models
// against models.dev's 80, Hugging Face 164 against 86, Novita 108 against 94). The lists are read
// at the platform's own address without the channel's key: they are public, the prices are the
// platform's whatever relay a channel is pointed at, and a wrong key does not stop the sync.
//
// Together, Nebius and Baseten also price their lists but only for a key, and their fields could
// not be checked, so they stay on models.dev. Fireworks' list carries no price.
// Filled at package initialisation, before price_source.go's init reads it.
var ownPriceLists = map[int]func(p *Provider) ([]*model.ChannelPrice, error){
	config.ChannelTypeDeepInfra:   priceList("https://api.deepinfra.com/v1/openai/models", parseDeepInfraPrices),
	config.ChannelTypeNovita:      priceList("https://api.novita.ai/openai/v1/models", parseNovitaPrices),
	config.ChannelTypeHuggingFace: priceList("https://router.huggingface.co/v1/models", parseHuggingFacePrices),
	config.ChannelTypeChutes:      priceList("https://llm.chutes.ai/v1/models", parseChutesPrices),
	// Cerebras serves its priced list under /public; /v1/models needs a key and has no prices.
	config.ChannelTypeCerebras:  priceList("https://api.cerebras.ai/public/v1/models", parsePerTokenPrices),
	config.ChannelTypeSambaNova: priceList("https://api.sambanova.ai/v1/models", parsePerTokenPrices),
}

// priceList reads listURL through the channel's proxy and parses it.
func priceList(listURL string, parse func([]byte) ([]*model.ChannelPrice, error)) func(p *Provider) ([]*model.ChannelPrice, error) {
	return func(p *Provider) ([]*model.ChannelPrice, error) {
		req, err := p.Requester.NewRequest(http.MethodGet, listURL, p.Requester.WithHeader(map[string]string{"Accept": "application/json"}))
		if err != nil {
			return nil, errors.New("new_request_failed")
		}
		var raw json.RawMessage
		if _, errWithCode := p.Requester.SendRequest(req, &raw, false); errWithCode != nil {
			return nil, errors.New(errWithCode.Message)
		}
		prices, err := parse(raw)
		if err != nil {
			return nil, err
		}
		if len(prices) == 0 {
			return nil, errors.New(p.preset.Name + " listed no prices")
		}
		for _, pr := range prices {
			for _, f := range []*float64{&pr.Input, &pr.Output, &pr.CacheRead, &pr.CacheWrite, &pr.Image, &pr.VideoSecond, &pr.AudioSecond} {
				*f = tidy(*f)
			}
		}
		sort.Slice(prices, func(a, b int) bool { return prices[a].Model < prices[b].Model })
		return prices, nil
	}
}

// tidy drops the float noise of unit conversions (0.35000000000000003 → 0.35).
func tidy(v float64) float64 {
	t, _ := strconv.ParseFloat(strconv.FormatFloat(v, 'g', 12, 64), 64)
	return t
}

// flexNumber reads a price given as a JSON number or a numeric string; a negative or unreadable
// value counts as missing.
type flexNumber struct {
	value float64
	set   bool
}

func (n *flexNumber) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || v < 0 {
		return nil
	}
	n.value, n.set = v, true
	return nil
}

// DeepInfra's /v1/openai/models: metadata.pricing in USD per million tokens (cache_read_tokens
// included), per image (per_image_unit), per second of audio in (input_seconds) or video out
// (output_seconds), and per million characters of speech input (input_characters, not kept).
func parseDeepInfraPrices(body []byte) ([]*model.ChannelPrice, error) {
	var resp struct {
		Data []struct {
			Id       string `json:"id"`
			Metadata *struct {
				Pricing map[string]flexNumber `json:"pricing"`
				Tags    []string              `json:"tags"`
			} `json:"metadata"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	out := make([]*model.ChannelPrice, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.Id == "" || m.Metadata == nil {
			continue
		}
		pr := m.Metadata.Pricing
		priced := false
		for _, v := range pr {
			priced = priced || v.set
		}
		if !priced {
			continue
		}
		price := &model.ChannelPrice{
			Model:       m.Id,
			Input:       pr["input_tokens"].value,
			Output:      pr["output_tokens"].value,
			CacheRead:   pr["cache_read_tokens"].value,
			Image:       pr["per_image_unit"].value,
			VideoSecond: pr["output_seconds"].value,
			AudioSecond: pr["input_seconds"].value,
			Other:       pr["input_characters"].set,
			Kind:        deepInfraKind(m.Metadata.Tags),
		}
		out = append(out, price)
	}
	return out, nil
}

func deepInfraKind(tags []string) string {
	for _, t := range tags {
		switch t {
		case "embed":
			return model.ModelKindEmbedding
		case "image-gen":
			return model.ModelKindImage
		case "video-gen":
			return model.ModelKindVideo
		case "tts":
			return model.ModelKindSpeech
		case "stt":
			return model.ModelKindTranscription
		}
	}
	return ""
}

// Novita's /openai/v1/models: pricing.{prompt,completion,input_cache_read}.price_per_m_decimal,
// the price charged (after any discount) in USD per million tokens. A tiered model lists its first
// tier.
func parseNovitaPrices(body []byte) ([]*model.ChannelPrice, error) {
	type tier struct {
		PerMillion flexNumber `json:"price_per_m_decimal"`
	}
	var resp struct {
		Data []struct {
			Id      string `json:"id"`
			Pricing *struct {
				Prompt         *tier `json:"prompt"`
				Completion     *tier `json:"completion"`
				InputCacheRead *tier `json:"input_cache_read"`
			} `json:"pricing"`
			OutputModalities []string `json:"output_modalities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	get := func(t *tier) flexNumber {
		if t == nil {
			return flexNumber{}
		}
		return t.PerMillion
	}
	out := make([]*model.ChannelPrice, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.Id == "" || m.Pricing == nil {
			continue
		}
		in, outp := get(m.Pricing.Prompt), get(m.Pricing.Completion)
		if !in.set && !outp.set {
			continue
		}
		out = append(out, &model.ChannelPrice{
			Model:     m.Id,
			Input:     in.value,
			Output:    outp.value,
			CacheRead: get(m.Pricing.InputCacheRead).value,
			Kind:      model.ModelKindFromOutputModalities(m.OutputModalities),
		})
	}
	return out, nil
}

// Hugging Face's router lists each model once with the price of every provider serving it, in USD
// per million tokens. A request without a ":provider" suffix may go to any of them, so the model
// keeps the highest input and output price among the live ones: the cost is never understated.
func parseHuggingFacePrices(body []byte) ([]*model.ChannelPrice, error) {
	var resp struct {
		Data []struct {
			Id           string `json:"id"`
			Architecture struct {
				OutputModalities []string `json:"output_modalities"`
			} `json:"architecture"`
			Providers []struct {
				Status  string `json:"status"`
				Pricing *struct {
					Input  flexNumber `json:"input"`
					Output flexNumber `json:"output"`
				} `json:"pricing"`
			} `json:"providers"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	out := make([]*model.ChannelPrice, 0, len(resp.Data))
	for _, m := range resp.Data {
		if m.Id == "" {
			continue
		}
		var price *model.ChannelPrice
		for _, pv := range m.Providers {
			if pv.Pricing == nil || (pv.Status != "" && pv.Status != "live") || (!pv.Pricing.Input.set && !pv.Pricing.Output.set) {
				continue
			}
			if price == nil {
				price = &model.ChannelPrice{Model: m.Id, Kind: model.ModelKindFromOutputModalities(m.Architecture.OutputModalities)}
			}
			price.Input = max(price.Input, pv.Pricing.Input.value)
			price.Output = max(price.Output, pv.Pricing.Output.value)
		}
		if price != nil {
			out = append(out, price)
		}
	}
	return out, nil
}

// Chutes' /v1/models: pricing.{prompt,completion,input_cache_read} in USD per million tokens.
func parseChutesPrices(body []byte) ([]*model.ChannelPrice, error) {
	return parseTokenPrices(body, 1)
}

// Cerebras' and SambaNova's lists: pricing.{prompt,completion,input_cache_read,input_cache_write}
// as USD per token strings, the OpenRouter convention.
func parsePerTokenPrices(body []byte) ([]*model.ChannelPrice, error) {
	return parseTokenPrices(body, 1e6)
}

func parseTokenPrices(body []byte, toPerMillion float64) ([]*model.ChannelPrice, error) {
	var resp struct {
		Data []struct {
			Id      string `json:"id"`
			Pricing *struct {
				Prompt          flexNumber `json:"prompt"`
				Completion      flexNumber `json:"completion"`
				InputCacheRead  flexNumber `json:"input_cache_read"`
				InputCacheWrite flexNumber `json:"input_cache_write"`
			} `json:"pricing"`
			OutputModalities []string `json:"output_modalities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	out := make([]*model.ChannelPrice, 0, len(resp.Data))
	for _, m := range resp.Data {
		pr := m.Pricing
		if m.Id == "" || pr == nil || (!pr.Prompt.set && !pr.Completion.set) {
			continue
		}
		out = append(out, &model.ChannelPrice{
			Model:      m.Id,
			Input:      pr.Prompt.value * toPerMillion,
			Output:     pr.Completion.value * toPerMillion,
			CacheRead:  pr.InputCacheRead.value * toPerMillion,
			CacheWrite: pr.InputCacheWrite.value * toPerMillion,
			Kind:       model.ModelKindFromOutputModalities(m.OutputModalities),
		})
	}
	return out, nil
}
