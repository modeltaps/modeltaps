package model

import (
	"net/url"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// ChannelPriceSource says where a channel type's upstream prices come from and where a person
// can check one of them: the platform's name, the price list the prices were read from when it
// is not the platform's own, and the web page that shows a model's price.
type ChannelPriceSource struct {
	// Name is the platform, as the console names it ("OpenRouter").
	Name string
	// Via is the list the prices were read from when it is not the platform's own API
	// ("models.dev"); empty when the platform publishes them itself.
	Via string
	// Page builds the page that shows the price of the model the channel sends upstream under
	// `model`. exact is false when the page is the platform's general pricing page rather than
	// one for that model. nil when there is no public page.
	Page func(model string) (pageURL string, exact bool)
}

var channelPriceSources = map[int]ChannelPriceSource{}

// SetChannelPriceSource records where channels of a type get their upstream prices.
func SetChannelPriceSource(channelType int, source ChannelPriceSource) {
	channelPriceSources[channelType] = source
}

// DescribeChannelPrices fills in, on each price, the platform it comes from, the list it was
// read from and the page to check it on, for channels of channelType.
func DescribeChannelPrices(channelType int, prices ...*ChannelPrice) {
	source, ok := channelPriceSources[channelType]
	if !ok {
		return
	}
	for _, p := range prices {
		if p == nil {
			continue
		}
		p.Source, p.Via = source.Name, source.Via
		if source.Page != nil {
			p.PageURL, p.PageExact = source.Page(p.Model)
		}
	}
}

// ExactPage builds a per-model page from a template: "{id}" is the model name as sent upstream,
// "{name}" the part after its last "/". Both are path-escaped segment by segment.
func ExactPage(template string) func(string) (string, bool) {
	return func(model string) (string, bool) {
		return fillPageTemplate(template, model), true
	}
}

// PricingPage is a platform's general pricing page, for platforms without a page per model.
func PricingPage(pageURL string) func(string) (string, bool) {
	return func(string) (string, bool) { return pageURL, false }
}

func fillPageTemplate(template, model string) string {
	name := model
	if i := strings.LastIndex(model, "/"); i >= 0 {
		name = model[i+1:]
	}
	return strings.NewReplacer("{id}", escapePath(model), "{name}", url.PathEscape(name)).Replace(template)
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

// The platforms that publish their own prices through their API. Their pages were checked on
// 2026-11-15; Vercel's pages drop the provider prefix ("anthropic/"). Requesty's pages need it,
// so a name without one ("claude-haiku-4-5", Requesty's own alias) gets its model list.
func init() {
	SetChannelPriceSource(config.ChannelTypeOpenRouter, ChannelPriceSource{Name: "OpenRouter", Page: ExactPage("https://openrouter.ai/{id}")})
	SetChannelPriceSource(config.ChannelTypeVercelAIGateway, ChannelPriceSource{Name: "Vercel AI Gateway", Page: ExactPage("https://vercel.com/ai-gateway/models/{name}")})
	SetChannelPriceSource(config.ChannelTypeRequesty, ChannelPriceSource{Name: "Requesty", Page: requestyPage})
}

func requestyPage(model string) (string, bool) {
	if !strings.Contains(model, "/") {
		return "https://www.requesty.ai/models", false
	}
	return fillPageTemplate("https://www.requesty.ai/models/{id}", model), true
}
