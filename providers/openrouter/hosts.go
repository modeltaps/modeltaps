package openrouter

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Hosting endpoints. Most open-weight models on OpenRouter are served by many hosts at different
// prices and quality (quantization, context length, caching, uptime), and the price /v1/models
// lists for a model is its cheapest endpoint's: often a heavily quantized host or a promotion,
// far below what most hosts charge. Left to itself OpenRouter spreads requests over the hosts,
// weighted by price and health, so neither the cost nor what serves the request is known in
// advance. For every model the channel reads the endpoints at price sync, picks an order
// (planHosts), keeps it with the model's channel price, and sends it with every request as
// provider.order (hostOrderHook). OpenRouter tries the endpoints in that order and still falls
// back to others, so an endpoint being down never fails a request. The channel's price for the
// model is the first endpoint's.

// endpointPricing is one endpoint's price for a model, in USD per token, discounts applied.
type endpointPricing struct {
	Prompt          string `json:"prompt"`
	Completion      string `json:"completion"`
	InputCacheRead  string `json:"input_cache_read"`
	InputCacheWrite string `json:"input_cache_write"`
}

// modelEndpoint is one hosting endpoint of a model, from /v1/models/{id}/endpoints.
type modelEndpoint struct {
	ProviderName string `json:"provider_name"`
	// Tag is the endpoint's slug ("z-ai/fp8", "deepinfra/fp4/flex"): provider[/variant][/region]
	// [/tier]. provider.order takes it to pin that endpoint; its first segment is the provider.
	Tag           string          `json:"tag"`
	Pricing       endpointPricing `json:"pricing"`
	ContextLength int             `json:"context_length"`
	// Quantization is the weights' precision when the host says ("fp8"); null or "unknown" when
	// it does not.
	Quantization *string `json:"quantization"`
	// ServiceTier is set on endpoints sold as a slower or priority tier ("flex"); null for the
	// standard one.
	ServiceTier             *string  `json:"service_tier"`
	Status                  int      `json:"status"` // 0 serving
	Uptime1d                *float64 `json:"uptime_last_1d"`
	SupportsImplicitCaching bool     `json:"supports_implicit_caching"`
}

type modelEndpointsResponse struct {
	Data struct {
		Endpoints []modelEndpoint `json:"endpoints"`
	} `json:"data"`
}

// vendorProviders maps a model id's vendor prefix to the provider slugs of the vendor's own
// hosting when they differ from the prefix.
var vendorProviders = map[string][]string{
	"qwen":           {"alibaba"},
	"x-ai":           {"xai"},
	"google":         {"google-vertex", "google-ai-studio"},
	"amazon":         {"amazon-bedrock"},
	"bytedance-seed": {"seed"},
	"mistralai":      {"mistral"},
	"meta-llama":     {"meta"},
	"rekaai":         {"reka"},
}

// hostPriceFloor drops endpoints with any price (input or output) below this share of the
// reference's: so far below it they are most likely a different deployment (a smaller context, a
// cut-down model, a promotion about to end, an input price that only pays with a long output)
// rather than the same model served cheaper.
const hostPriceFloor = 0.3

// maxHostOrder is how many endpoints the order names. Endpoints left out still serve when those
// fail, as OpenRouter falls back past the order; a short list keeps requests small and readable.
const maxHostOrder = 6

func (e modelEndpoint) priced() bool {
	return parseUSD(e.Pricing.Prompt) > 0 || parseUSD(e.Pricing.Completion) > 0
}

func (e modelEndpoint) provider() string {
	slug, _, _ := strings.Cut(e.Tag, "/")
	return strings.ToLower(slug)
}

// standard reports whether the endpoint is sold at the standard service tier.
func (e modelEndpoint) standard() bool {
	if e.ServiceTier == nil {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(*e.ServiceTier)) {
	case "", "standard", "default":
		return true
	}
	return false
}

// caches reports whether the endpoint discounts repeated prompts.
func (e modelEndpoint) caches() bool {
	if e.SupportsImplicitCaching {
		return true
	}
	read := parseUSD(e.Pricing.InputCacheRead)
	return read > 0 && read < parseUSD(e.Pricing.Prompt)
}

// fullPrecision is the bits per weight of unquantized weights.
const fullPrecision = 16

// precision is the bits per weight the endpoint serves: fullPrecision for unquantized weights,
// fewer for quantized ones, and 0 when the host does not say.
func (e modelEndpoint) precision() int {
	if e.Quantization == nil {
		return 0
	}
	q := strings.ToLower(strings.TrimSpace(*e.Quantization))
	switch q {
	case "", "unknown":
		return 0
	case "bf16", "fp16", "fp32":
		return fullPrecision
	}
	for _, bits := range []int{8, 6, 4} {
		if strings.Contains(q, strconv.Itoa(bits)) {
			return bits
		}
	}
	return 1
}

// preciseEnough reports whether the endpoint serves at least floor bits per weight; an endpoint
// that does not say is given the benefit of the doubt.
func (e modelEndpoint) preciseEnough(floor int) bool {
	p := e.precision()
	return p == 0 || p >= floor
}

// healthy reports whether the endpoint is serving and, when the success rate check is on, was up
// at least SupplyCostMinSuccessPercent of the last day.
func (e modelEndpoint) healthy() bool {
	if e.Status != 0 {
		return false
	}
	if min := config.SupplyCostMinSuccessPercent; min > 0 && e.Uptime1d != nil && *e.Uptime1d < min {
		return false
	}
	return true
}

// costTo is the endpoint's price over the reference's: the dearer of input and output (the cost
// it is ordered by) and the cheaper one (checked against hostPriceFloor).
func (e modelEndpoint) costTo(ref modelEndpoint) (dearer, cheaper float64) {
	first := true
	over := func(c, v float64) {
		if v <= 0 {
			return
		}
		r := c / v
		if first || r > dearer {
			dearer = r
		}
		if first || r < cheaper {
			cheaper = r
		}
		first = false
	}
	over(parseUSD(e.Pricing.Prompt), parseUSD(ref.Pricing.Prompt))
	over(parseUSD(e.Pricing.Completion), parseUSD(ref.Pricing.Completion))
	return dearer, cheaper
}

// cheaperThan orders endpoints by output price, then input price.
func cheaperThan(a, b modelEndpoint) bool {
	ao, bo := parseUSD(a.Pricing.Completion), parseUSD(b.Pricing.Completion)
	if ao != bo {
		return ao < bo
	}
	return parseUSD(a.Pricing.Prompt) < parseUSD(b.Pricing.Prompt)
}

// isVendor reports whether the endpoint is the model vendor's own hosting.
func isVendor(modelID string, e modelEndpoint) bool {
	prefix, _, _ := strings.Cut(strings.ToLower(modelID), "/")
	slugs, ok := vendorProviders[prefix]
	if !ok {
		slugs = []string{prefix}
	}
	for _, s := range slugs {
		if e.provider() == s {
			return true
		}
	}
	return false
}

// planHosts orders the endpoints of a model, returning their slugs and the first one's price; ok
// is false when no standard-tier endpoint states a price, and the model then keeps OpenRouter's
// routing and listed price.
//
// The other endpoints are judged against a reference: the vendor's own hosting (its cheapest
// standard-tier endpoint, a serving one when there is one), or, for a model the vendor does not
// host there, the median-priced serving endpoint among those with the most precise weights any of
// them states. An endpoint qualifies if it is serving, up at least SupplyCostMinSuccessPercent of
// the last day, at the standard tier, not stating less precise weights than the reference's (full
// weights for a vendor that does not say, the most precise stated without the vendor), with a context no shorter than the model's, prompt caching when the
// reference caches, and input and output prices no lower than hostPriceFloor of the reference's.
//
// With the vendor's hosting as the reference it comes first and qualifying endpoints follow,
// cheapest first; with SupplyCostOrder "cost", those cheaper than the vendor by at least
// SupplyCostMinSavingPercent go ahead of it, and a vendor that is not serving or up enough goes
// after them. Without the vendor the qualifying endpoints, the reference among them, are ordered
// cheapest first. The order names at most maxHostOrder endpoints.
func planHosts(m modelWithPricing, endpoints []modelEndpoint) (order []string, price endpointPricing, ok bool) {
	var candidates, vendors, serving []modelEndpoint
	for _, e := range endpoints {
		if e.Tag == "" || !e.priced() || !e.standard() {
			continue
		}
		candidates = append(candidates, e)
		if isVendor(m.Id, e) {
			vendors = append(vendors, e)
		}
		if e.healthy() {
			serving = append(serving, e)
		}
	}
	var ref *modelEndpoint
	floor := 0
	vendor := len(vendors) > 0
	if vendor {
		sort.SliceStable(vendors, func(i, j int) bool {
			if vendors[i].healthy() != vendors[j].healthy() {
				return vendors[i].healthy()
			}
			return cheaperThan(vendors[i], vendors[j])
		})
		ref = &vendors[0]
		// A vendor that does not state its precision is taken to serve full weights.
		if floor = ref.precision(); floor == 0 {
			floor = fullPrecision
		}
	} else {
		// The most precise weights any serving endpoint states set the floor, and the reference
		// is the median-priced endpoint at it (or not saying).
		for _, e := range serving {
			floor = max(floor, e.precision())
		}
		var pool []modelEndpoint
		for _, e := range serving {
			if e.preciseEnough(floor) {
				pool = append(pool, e)
			}
		}
		if len(pool) == 0 {
			return nil, endpointPricing{}, false
		}
		sort.SliceStable(pool, func(i, j int) bool { return cheaperThan(pool[i], pool[j]) })
		ref = &pool[len(pool)/2]
	}

	type host struct {
		e    modelEndpoint
		cost float64
	}
	var hosts []host
	for _, e := range candidates {
		if vendor && e.Tag == ref.Tag {
			continue
		}
		if !e.healthy() || !e.preciseEnough(floor) || (m.ContextLen > 0 && e.ContextLength > 0 && e.ContextLength < m.ContextLen) {
			continue
		}
		if ref.caches() && !e.caches() {
			continue
		}
		cost, lowest := e.costTo(*ref)
		if lowest < hostPriceFloor {
			continue
		}
		hosts = append(hosts, host{e, cost})
	}
	sort.SliceStable(hosts, func(i, j int) bool {
		if hosts[i].cost != hosts[j].cost {
			return hosts[i].cost < hosts[j].cost
		}
		return cheaperThan(hosts[i].e, hosts[j].e)
	})

	if !vendor {
		// When every endpoint falls short of the model's context, the reference alone is pinned.
		if len(hosts) == 0 {
			return []string{ref.Tag}, ref.Pricing, true
		}
		names := make([]string, 0, len(hosts))
		for _, h := range hosts {
			names = append(names, h.e.Tag)
		}
		if len(names) > maxHostOrder {
			names = names[:maxHostOrder]
		}
		return names, hosts[0].e.Pricing, true
	}

	ahead := 0
	if config.SupplyCostOrder == model.SupplyCostOrderCost {
		saving := config.SupplyCostMinSavingPercent / 100
		for ahead < len(hosts) && hosts[ahead].cost <= 1-saving {
			ahead++
		}
	}
	names := make([]string, 0, len(hosts)+1)
	first := ref.Pricing
	for i := 0; i < ahead; i++ {
		names = append(names, hosts[i].e.Tag)
	}
	if ahead > 0 {
		first = hosts[0].e.Pricing
	}
	if ref.healthy() {
		names = append(names, ref.Tag)
	}
	for _, h := range hosts[ahead:] {
		names = append(names, h.e.Tag)
	}
	if !ref.healthy() {
		if ahead == 0 && len(hosts) > 0 {
			first = hosts[0].e.Pricing
		}
		names = append(names, ref.Tag)
	}
	if len(names) > maxHostOrder {
		names = names[:maxHostOrder]
	}
	return names, first, true
}

// applyEndpointPricing replaces a model's token prices with the endpoint's.
func applyEndpointPricing(m *modelWithPricing, e endpointPricing) {
	m.Pricing.Prompt, m.Pricing.Completion = e.Prompt, e.Completion
	m.Pricing.InputCacheRead, m.Pricing.InputCacheWrite = e.InputCacheRead, e.InputCacheWrite
}

// hostPlanned reports whether a listed model gets an order of endpoints: a token-priced model
// that is not free, not audio and not one of OpenRouter's own routers or aliases.
func hostPlanned(m modelWithPricing) bool {
	if m.Id == "" || strings.HasPrefix(m.Id, "~") || strings.HasPrefix(m.Id, "openrouter/") || strings.HasSuffix(m.Id, ":free") {
		return false
	}
	out := m.Architecture.OutputModalities
	if hasModality(out, "speech") || hasModality(out, "transcription") {
		return false
	}
	return parseUSD(m.Pricing.Prompt) > 0 || parseUSD(m.Pricing.Completion) > 0
}

// endpointFetchWorkers bounds the concurrent /endpoints requests of one price sync.
const endpointFetchWorkers = 8

func (p *OpenRouterProvider) fetchEndpoints(id string) ([]modelEndpoint, error) {
	path := p.Config.ModelList + "/" + (&url.URL{Path: id}).EscapedPath() + "/endpoints"
	req, err := p.Requester.NewRequest(http.MethodGet, p.GetFullRequestURL(path, ""), p.Requester.WithHeader(p.GetRequestHeaders()))
	if err != nil {
		return nil, errors.New("new_request_failed")
	}
	resp := &modelEndpointsResponse{}
	if _, errWithCode := p.Requester.SendRequest(req, resp, false); errWithCode != nil {
		return nil, errors.New(errWithCode.Message)
	}
	return resp.Data.Endpoints, nil
}

// fetchPricedModels is fetchOpenRouterModels with, for each planned model, the order of endpoints
// the channel asks for and the first endpoint's token prices (planHosts). A model whose endpoints
// cannot be read keeps the listed price and OpenRouter's own routing.
func (p *OpenRouterProvider) fetchPricedModels() (*modelPricingResponse, error) {
	resp, err := p.fetchOpenRouterModels()
	if err != nil {
		return nil, err
	}
	var planned []int
	for i, m := range resp.Data {
		if hostPlanned(m) {
			planned = append(planned, i)
		}
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < endpointFetchWorkers && w < len(planned); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				m := &resp.Data[i]
				endpoints, err := p.fetchEndpoints(m.Id)
				if err != nil {
					continue
				}
				if order, price, ok := planHosts(*m, endpoints); ok {
					applyEndpointPricing(m, price)
					m.hostOrder = order
				}
			}
		}()
	}
	for _, i := range planned {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return resp, nil
}

// hostOrderedPaths are the request paths that take provider preferences.
var hostOrderedPaths = []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"}

// maxHostOrderedBody bounds the body the hook reads to add the endpoint order.
const maxHostOrderedBody = 64 << 20

// hostOrderHook adds the endpoint order kept with the model's channel price to every chat,
// responses and messages request, as provider.order, keeping the other provider preferences. An
// order already in the body (the channel's own provider settings for the model) is kept. A
// request for a model with no order is sent as it is.
func hostOrderHook(channel *model.Channel) func(*http.Request) error {
	if channel == nil || channel.Id == 0 {
		return nil
	}
	channelID := channel.Id
	return func(req *http.Request) error {
		if req.Method != http.MethodPost || req.Body == nil || !hostOrdered(req.URL.Path) {
			return nil
		}
		if ct := req.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "json") {
			return nil
		}
		body, err := io.ReadAll(io.LimitReader(req.Body, maxHostOrderedBody+1))
		_ = req.Body.Close()
		if err != nil {
			return err
		}
		if len(body) <= maxHostOrderedBody {
			if ordered, err := withHostOrder(body, channelID); err == nil {
				body = ordered
			}
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Length", strconv.Itoa(len(body)))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
		return nil
	}
}

func hostOrdered(path string) bool {
	for _, p := range hostOrderedPaths {
		if strings.HasSuffix(path, p) {
			return true
		}
	}
	return false
}

// withHostOrder sets provider.order on a JSON object body from the channel's price for the body's
// model; the body unchanged when there is no order or the body already names one.
func withHostOrder(body []byte, channelID int) ([]byte, error) {
	if !gjson.ValidBytes(body) {
		return body, nil
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() || root.Get("provider.order").Exists() {
		return body, nil
	}
	name := root.Get("model").String()
	if name == "" {
		return body, nil
	}
	cp := model.CachedChannelPrice(channelID, name)
	if cp == nil || cp.ProviderOrder == "" {
		return body, nil
	}
	if p := root.Get("provider"); p.Exists() && !p.IsObject() {
		return body, nil
	}
	out, err := sjson.SetBytes(body, "provider.order", strings.Split(cp.ProviderOrder, ","))
	if err != nil {
		return body, fmt.Errorf("set provider.order: %w", err)
	}
	return out, nil
}
