package requesty

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// routerModel is one entry of `GET /v1/models` or `GET /v1/models/managed`. Prices are USD per
// token; caching_price is the cache write price, cached_price the cache read price. pricing holds
// the tiers by prompt size, of which the top-level fields are the first.
type routerModel struct {
	Id                       string   `json:"id"`
	Api                      string   `json:"api"`
	Created                  int64    `json:"created"`
	InputPrice               float64  `json:"input_price"`
	OutputPrice              float64  `json:"output_price"`
	CachingPrice             *float64 `json:"caching_price"`
	CachedPrice              *float64 `json:"cached_price"`
	MaxOutputTokens          int      `json:"max_output_tokens"`
	ContextWindow            int      `json:"context_window"`
	SupportsVision           bool     `json:"supports_vision"`
	SupportsReasoning        bool     `json:"supports_reasoning"`
	SupportsToolCalling      bool     `json:"supports_tool_calling"`
	SupportsImageGeneration  bool     `json:"supports_image_generation"`
	SupportsOutputJSONSchema bool     `json:"supports_output_json_schema"`
	Description              string   `json:"description"`
	OpenWeights              *bool    `json:"open_weights"`
	ModelLab                 string   `json:"model_lab"`
	ModelCanonicalName       string   `json:"model_canonical_name"`
}

type modelListResponse struct {
	Data []routerModel `json:"data"`
}

// catalog is what Requesty lists: the managed names and the provider-pinned ids.
type catalog struct {
	managed []routerModel
	pinned  []routerModel
}

// getList reads a model list with the channel's key. The lists are public, but Requesty rejects
// a key it does not accept, so a refused key is retried without one: prices keep syncing while
// the channel's key is wrong or expired.
func (p *RequestyProvider) getList(path string) ([]routerModel, error) {
	data, status, err := p.requestList(path, p.GetRequestHeaders())
	if err != nil && (status == http.StatusUnauthorized || status == http.StatusForbidden) {
		headers := p.GetRequestHeaders()
		delete(headers, "Authorization")
		data, _, err = p.requestList(path, headers)
	}
	return data, err
}

func (p *RequestyProvider) requestList(path string, headers map[string]string) ([]routerModel, int, error) {
	req, err := p.Requester.NewRequest(http.MethodGet, p.GetFullRequestURL(path, ""), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, 0, errors.New("new_request_failed")
	}
	resp := &modelListResponse{}
	if _, errWithCode := p.Requester.SendRequest(req, resp, false); errWithCode != nil {
		return nil, errWithCode.StatusCode, errors.New(errWithCode.Message)
	}
	return resp.Data, http.StatusOK, nil
}

func (p *RequestyProvider) fetchCatalog() (*catalog, error) {
	pinned, err := p.getList(p.Config.ModelList)
	if err != nil {
		return nil, err
	}
	// The managed list is a convenience: without it the channel falls back to pinned ids.
	managed, _ := p.getList(p.Config.ModelList + "/managed")
	return &catalog{managed: managed, pinned: pinned}, nil
}

// canonicalKey identifies a model across its Requesty ids: the canonical name alone, since
// Requesty gives one model different labs on different hosts ("zai" and "deepinfra" for glm-5.2).
func canonicalKey(m routerModel) string {
	if m.ModelCanonicalName == "" {
		return strings.ToLower(bareID(m.Id))
	}
	return strings.ToLower(m.ModelCanonicalName)
}

// bareID is the id without its host prefix: "gemma-4-31b-it" for "sail/gemma-4-31b-it".
func bareID(id string) string {
	if i := strings.LastIndex(id, "/"); i >= 0 {
		return id[i+1:]
	}
	return id
}

// regional reports an id pinned to a region ("@eu", "@eu-central-1") or a service tier
// (":flex"): the same model at another price, left out of the default list.
func regional(id string) bool {
	return strings.ContainsAny(id, "@:")
}

// defaultModels lists each model once: its managed name, which Requesty routes across providers
// with failover, else one provider-pinned id, preferring the lab's own provider. A managed name
// that costs more than a pinned id of the same model gives way to the cheapest such id: managed
// names are often priced at a regional endpoint (about 10% above the global one), and the
// gateway fails over between its own channels anyway. Regional and tier variants are left out;
// they can still be added to a channel by hand and are priced.
func defaultModels(c *catalog) []routerModel {
	if c == nil {
		return nil
	}
	out := make([]routerModel, 0, len(c.managed)+len(c.pinned))
	seen := map[string]bool{}
	for _, m := range c.managed {
		if m.Id == "" || regional(m.Id) || seen[canonicalKey(m)] {
			continue
		}
		seen[canonicalKey(m)] = true
		seen[strings.ToLower(m.Id)] = true
		if cheaper, ok := cheapestPinned(c.pinned, m); ok {
			m = cheaper
		}
		out = append(out, m)
	}
	best := map[string]routerModel{}
	var order []string
	for _, m := range c.pinned {
		// A pinned id whose name is a managed name is that model on one host.
		if m.Id == "" || regional(m.Id) || seen[canonicalKey(m)] || seen[strings.ToLower(bareID(m.Id))] {
			continue
		}
		key := canonicalKey(m)
		cur, ok := best[key]
		if !ok {
			order = append(order, key)
		}
		if !ok || pinnedRank(m) > pinnedRank(cur) || (pinnedRank(m) == pinnedRank(cur) && m.Id < cur.Id) {
			best[key] = m
		}
	}
	sort.Strings(order)
	for _, key := range order {
		out = append(out, best[key])
	}
	return out
}

// cheaper reports whether a costs less than b: no more for input and output, less for one of them.
func cheaper(a, b routerModel) bool {
	return a.InputPrice <= b.InputPrice && a.OutputPrice <= b.OutputPrice &&
		(a.InputPrice < b.InputPrice || a.OutputPrice < b.OutputPrice)
}

// cheapestPinned returns the cheapest pinned id of the managed model's canonical model that costs
// less than the managed name, preferring the lab's own provider on equal prices. Regional and tier
// variants and unpriced ids are not considered.
func cheapestPinned(pinned []routerModel, managed routerModel) (routerModel, bool) {
	var best routerModel
	found := false
	key := canonicalKey(managed)
	for _, m := range pinned {
		if m.Id == "" || regional(m.Id) || canonicalKey(m) != key || (m.InputPrice <= 0 && m.OutputPrice <= 0) {
			continue
		}
		if !cheaper(m, managed) {
			continue
		}
		if !found || cheaper(m, best) || (!cheaper(best, m) && (pinnedRank(m) > pinnedRank(best) ||
			(pinnedRank(m) == pinnedRank(best) && m.Id < best.Id))) {
			best, found = m, true
		}
	}
	return best, found
}

// pinnedRank prefers the lab's own provider ("anthropic/claude-x" for an Anthropic model).
func pinnedRank(m routerModel) int {
	prefix, _, _ := strings.Cut(m.Id, "/")
	if m.ModelLab != "" && strings.EqualFold(prefix, m.ModelLab) {
		return 1
	}
	return 0
}

// allModels is every id Requesty lists, managed names first, each once.
func allModels(c *catalog) []routerModel {
	if c == nil {
		return nil
	}
	out := make([]routerModel, 0, len(c.managed)+len(c.pinned))
	seen := map[string]bool{}
	for _, list := range [][]routerModel{c.managed, c.pinned} {
		for _, m := range list {
			if m.Id != "" && !seen[m.Id] {
				seen[m.Id] = true
				out = append(out, m)
			}
		}
	}
	return out
}

// GetModelList returns the default model list for the channel's "fetch models".
func (p *RequestyProvider) GetModelList() ([]string, error) {
	c, err := p.fetchCatalog()
	if err != nil {
		return nil, err
	}
	models := defaultModels(c)
	list := make([]string, 0, len(models))
	for _, m := range models {
		list = append(list, m.Id)
	}
	return list, nil
}

// GetModelPriceList implements base.ModelPriceListInterface.
func (p *RequestyProvider) GetModelPriceList() ([]*model.Price, error) {
	prices, _, err := p.GetModelPriceSync()
	return prices, err
}

// GetModelPriceSync returns the catalog prices of the default list; a model with no token price
// is reported as unconvertible.
func (p *RequestyProvider) GetModelPriceSync() ([]*model.Price, []model.UnconvertiblePrice, error) {
	c, err := p.fetchCatalog()
	if err != nil {
		return nil, nil, err
	}
	prices, unconvertible := buildPriceSync(defaultModels(c))
	return prices, unconvertible, nil
}

func buildPriceSync(models []routerModel) ([]*model.Price, []model.UnconvertiblePrice) {
	prices := make([]*model.Price, 0, len(models))
	var unconvertible []model.UnconvertiblePrice
	for _, m := range models {
		if m.InputPrice <= 0 && m.OutputPrice <= 0 {
			unconvertible = append(unconvertible, model.UnconvertiblePrice{Model: m.Id, Reason: model.UnconvertibleNoPrice})
			continue
		}
		prices = append(prices, model.PriceFromUSDPerToken(m.Id, config.ChannelTypeRequesty,
			m.InputPrice, m.OutputPrice, deref(m.CachedPrice)))
	}
	return prices, unconvertible
}

func deref(v *float64) float64 {
	if v == nil || *v < 0 {
		return 0
	}
	return *v
}

// GetModelInfoList implements base.ModelInfoListInterface.
func (p *RequestyProvider) GetModelInfoList() ([]*model.ModelInfo, error) {
	c, err := p.fetchCatalog()
	if err != nil {
		return nil, err
	}
	return buildModelInfos(defaultModels(c)), nil
}

func buildModelInfos(models []routerModel) []*model.ModelInfo {
	infos := make([]*model.ModelInfo, 0, len(models))
	for _, m := range models {
		input := []string{"text"}
		if m.SupportsVision {
			input = append(input, "image")
		}
		output := []string{"text"}
		if m.SupportsImageGeneration {
			output = append(output, "image")
		}
		var caps []string
		if m.SupportsToolCalling {
			caps = append(caps, model.ModelCapabilityToolCall)
		}
		if m.SupportsReasoning {
			caps = append(caps, model.ModelCapabilityReasoning)
		}
		if m.SupportsOutputJSONSchema {
			caps = append(caps, model.ModelCapabilityStructuredOutput)
		}
		infos = append(infos, &model.ModelInfo{
			Model:            m.Id,
			Description:      m.Description,
			ContextLength:    m.ContextWindow,
			MaxTokens:        m.MaxOutputTokens,
			InputModalities:  jsonList(input),
			OutputModalities: jsonList(output),
			Capabilities:     model.ModelCapabilitiesJSON(caps),
			VendorID:         vendorOf(m),
			Endpoints:        model.ModelEndpointsFromModalities(m.Id, input, output),
			OpenWeights:      m.OpenWeights,
			ReleasedAt:       m.Created,
		})
	}
	return infos
}

// vendorOf is who made the model. The id prefix names the hosting provider ("vertex/",
// "bedrock/", "deepinfra/"), not the maker, so Requesty's model_lab comes first.
func vendorOf(m routerModel) int {
	if m.ModelLab != "" {
		if id := model.GuessVendorID(m.ModelLab + "/" + m.ModelCanonicalName); id != 0 {
			return id
		}
	}
	if m.ModelCanonicalName != "" {
		if id := model.GuessVendorID(m.ModelCanonicalName); id != 0 {
			return id
		}
	}
	return model.VendorIDFromNameRules(bareID(m.Id))
}

func jsonList(v []string) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

// GetChannelPrices implements base.ChannelPriceListInterface: every id Requesty lists, regional
// and tier variants included, so a variant added by hand is priced too.
func (p *RequestyProvider) GetChannelPrices() ([]*model.ChannelPrice, error) {
	c, err := p.fetchCatalog()
	if err != nil {
		return nil, err
	}
	return buildChannelPrices(allModels(c)), nil
}

func buildChannelPrices(models []routerModel) []*model.ChannelPrice {
	out := make([]*model.ChannelPrice, 0, len(models))
	for _, m := range models {
		price := &model.ChannelPrice{
			Model:      m.Id,
			Input:      model.USDPerMillionTokens(m.InputPrice),
			Output:     model.USDPerMillionTokens(m.OutputPrice),
			CacheRead:  model.USDPerMillionTokens(deref(m.CachedPrice)),
			CacheWrite: model.USDPerMillionTokens(deref(m.CachingPrice)),
		}
		if m.SupportsImageGeneration && m.InputPrice <= 0 && m.OutputPrice <= 0 {
			price.Other = true
		}
		out = append(out, price)
	}
	return out
}
