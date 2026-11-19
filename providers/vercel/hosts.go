package vercel

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

// Hosting providers. Many models on the gateway are hosted by several providers at different
// prices and quality (quantization, context length, caching, uptime). Left to itself the gateway
// picks one per request by its own priority and recent health and bills that provider's price,
// so neither the cost nor what serves the request is known in advance. For such a model
// (varies_by_provider) the channel reads the hosts at price sync, picks an order (planHosts),
// keeps it with the model's channel price, and sends it with every request as
// providerOptions.gateway.order (hostOrderHook). The gateway tries the hosts in that order and
// still fails over to others, so a host being down never fails a request. The channel's price
// for the model is the first host's.

// endpointPricing is one hosting provider's price for a model, in USD per token.
type endpointPricing struct {
	Prompt          string `json:"prompt"`
	Completion      string `json:"completion"`
	InputCacheRead  string `json:"input_cache_read"`
	InputCacheWrite string `json:"input_cache_write"`
}

// modelEndpoint is one hosting provider of a model, from /v1/models/{id}/endpoints.
type modelEndpoint struct {
	ProviderName  string          `json:"provider_name"`
	Pricing       endpointPricing `json:"pricing"`
	ContextLength int             `json:"context_length"`
	// Quantization is the weights' precision when the host says ("fp8"); null for full precision.
	Quantization *string  `json:"quantization"`
	Status       int      `json:"status"` // 0 serving
	Uptime1d     *float64 `json:"uptime_last_1d"`
	Tags         []string `json:"tags"`
}

type modelEndpointsResponse struct {
	Data struct {
		Endpoints []modelEndpoint `json:"endpoints"`
	} `json:"data"`
}

// firstPartyAliases maps a model id's vendor prefix to the provider name the gateway gives the
// vendor's own hosting when the two differ.
var firstPartyAliases = map[string]string{"spacexai": "xai"}

// hostPriceFloor drops hosts with any price (input or output) below this share of the vendor's:
// so far below the vendor they are most likely a different deployment (a smaller context, a
// cut-down model, a promotion about to end, an input price that only pays with a long output)
// rather than the same model served cheaper.
const hostPriceFloor = 0.3

// maxHostOrder is how many hosts the order names. Hosts left out still serve when those fail, as
// the gateway falls back past the order; a short list keeps requests small and readable.
const maxHostOrder = 6

func (e modelEndpoint) priced() bool {
	return parseUSD(e.Pricing.Prompt) > 0 || parseUSD(e.Pricing.Completion) > 0
}

// caches reports whether the host discounts repeated prompts: a cache read cheaper than input, or
// a caching tag.
func (e modelEndpoint) caches() bool {
	if read := parseUSD(e.Pricing.InputCacheRead); read > 0 && read < parseUSD(e.Pricing.Prompt) {
		return true
	}
	for _, t := range e.Tags {
		if t == "implicit-caching" || t == "explicit-caching" {
			return true
		}
	}
	return false
}

// quantized reports whether the host serves reduced-precision weights.
func (e modelEndpoint) quantized() bool {
	if e.Quantization == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(*e.Quantization)) {
	case "", "bf16", "fp16", "fp32", "unknown":
		return false
	}
	return true
}

// healthy reports whether the host is serving and, when the success rate check is on, was up at
// least SupplyCostMinSuccessPercent of the last day.
func (e modelEndpoint) healthy() bool {
	if e.Status != 0 {
		return false
	}
	if min := config.SupplyCostMinSuccessPercent; min > 0 && e.Uptime1d != nil && *e.Uptime1d < min {
		return false
	}
	return true
}

// costTo is the host's price over the vendor's: the dearer of input and output (the cost it is
// ordered by) and the cheaper one (checked against hostPriceFloor).
func (e modelEndpoint) costTo(vendor modelEndpoint) (dearer, cheaper float64) {
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
	over(parseUSD(e.Pricing.Prompt), parseUSD(vendor.Pricing.Prompt))
	over(parseUSD(e.Pricing.Completion), parseUSD(vendor.Pricing.Completion))
	return dearer, cheaper
}

// planHosts orders the hosts of a model hosted by several providers, returning the providers'
// names and the first one's price; ok is false when the vendor does not host the model on the
// gateway, which then keeps its own routing and listed price: without the vendor's price there is
// nothing to judge the others' prices by.
//
// The vendor's own hosting comes first. Other hosts follow, cheapest first, if they qualify:
// serving, up at least SupplyCostMinSuccessPercent of the last day, full precision, a context no
// shorter than the model's, prompt caching when the vendor caches, and input and output prices no
// lower than hostPriceFloor of the vendor's. The order names at most maxHostOrder hosts. With SupplyCostOrder "cost", qualifying hosts cheaper than the
// vendor by at least SupplyCostMinSavingPercent go ahead of it, the way cheaper supplies do. A
// vendor that is not serving or up enough goes after the qualifying hosts.
func planHosts(m gatewayModel, endpoints []modelEndpoint) (order []string, price endpointPricing, ok bool) {
	vendorName, _, _ := strings.Cut(m.Id, "/")
	if alias, found := firstPartyAliases[vendorName]; found {
		vendorName = alias
	}
	var vendor *modelEndpoint
	for i := range endpoints {
		if strings.EqualFold(endpoints[i].ProviderName, vendorName) && endpoints[i].priced() {
			vendor = &endpoints[i]
			break
		}
	}
	if vendor == nil {
		return nil, endpointPricing{}, false
	}
	type host struct {
		e    modelEndpoint
		cost float64
	}
	var hosts []host
	for _, e := range endpoints {
		if strings.EqualFold(e.ProviderName, vendor.ProviderName) || e.ProviderName == "" || !e.priced() {
			continue
		}
		if !e.healthy() || e.quantized() || (m.ContextWindow > 0 && e.ContextLength > 0 && e.ContextLength < m.ContextWindow) {
			continue
		}
		if vendor.caches() && !e.caches() {
			continue
		}
		cost, lowest := e.costTo(*vendor)
		if lowest < hostPriceFloor {
			continue
		}
		hosts = append(hosts, host{e, cost})
	}
	sort.SliceStable(hosts, func(i, j int) bool { return hosts[i].cost < hosts[j].cost })

	ahead := 0
	if config.SupplyCostOrder == model.SupplyCostOrderCost {
		saving := config.SupplyCostMinSavingPercent / 100
		for ahead < len(hosts) && hosts[ahead].cost <= 1-saving {
			ahead++
		}
	}
	names := make([]string, 0, len(hosts)+1)
	first := vendor.Pricing
	for i := 0; i < ahead; i++ {
		names = append(names, hosts[i].e.ProviderName)
	}
	if ahead > 0 {
		first = hosts[0].e.Pricing
	}
	if vendor.healthy() {
		names = append(names, vendor.ProviderName)
	}
	for _, h := range hosts[ahead:] {
		names = append(names, h.e.ProviderName)
	}
	if !vendor.healthy() {
		if ahead == 0 && len(hosts) > 0 {
			first = hosts[0].e.Pricing
		}
		names = append(names, vendor.ProviderName)
	}
	if len(names) > maxHostOrder {
		names = names[:maxHostOrder]
	}
	return names, first, true
}

// applyEndpointPricing replaces a model's token prices with the provider's.
func applyEndpointPricing(m *gatewayModel, e endpointPricing) {
	m.Pricing.Input, m.Pricing.Output = e.Prompt, e.Completion
	m.Pricing.InputCacheRead, m.Pricing.InputCacheWrite = e.InputCacheRead, e.InputCacheWrite
}

// endpointFetchWorkers bounds the concurrent /endpoints requests of one price sync.
const endpointFetchWorkers = 8

func (p *VercelProvider) fetchEndpoints(id string) ([]modelEndpoint, error) {
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

// fetchPricedModels is fetchModels with, for each model hosted by several providers, the order
// of hosts the channel asks for and the first host's token prices (planHosts). A model whose
// hosts cannot be read, or whose vendor does not host it there, keeps the listed price and the
// gateway's own routing.
func (p *VercelProvider) fetchPricedModels() (*modelListResponse, error) {
	resp, err := p.fetchModels()
	if err != nil {
		return nil, err
	}
	var varied []int
	for i, m := range resp.Data {
		if m.Id != "" && m.Pricing.VariesByProvider && m.Type != "transcription" {
			varied = append(varied, i)
		}
	}
	jobs := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < endpointFetchWorkers && w < len(varied); w++ {
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
	for _, i := range varied {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return resp, nil
}

// hostOrderedPaths are the request paths that take providerOptions.
var hostOrderedPaths = []string{"/v1/chat/completions", "/v1/responses", "/v1/messages"}

// maxHostOrderedBody bounds the body the hook reads to add the host order.
const maxHostOrderedBody = 64 << 20

// hostOrderHook adds the host order kept with the model's channel price to every chat, responses
// and messages request, as providerOptions.gateway.order. It replaces an order the client sent,
// as the channel's cost rests on it, and keeps the client's other gateway options. A request for
// a model with no order is sent as it is.
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

// withHostOrder sets providerOptions.gateway.order on a JSON object body from the channel's
// price for the body's model; the body unchanged when there is no order.
func withHostOrder(body []byte, channelID int) ([]byte, error) {
	if !gjson.ValidBytes(body) {
		return body, nil
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
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
	order := strings.Split(cp.ProviderOrder, ",")
	out, err := sjson.SetBytes(body, "providerOptions.gateway.order", order)
	if err != nil {
		return body, fmt.Errorf("set providerOptions.gateway.order: %w", err)
	}
	return out, nil
}
