package cheaperinference

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tidwall/gjson"

	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
)

func TestRoutingOf(t *testing.T) {
	cases := []struct {
		opts map[string]interface{}
		want routing
	}{
		{nil, routing{}},
		{map[string]interface{}{"zdr": "on"}, routing{zdr: true}},
		{map[string]interface{}{"zdr": "off", "min_discount_percent": "20"}, routing{minDiscount: 20}},
		{map[string]interface{}{"zdr": true, "min_discount_percent": 12.345}, routing{zdr: true, minDiscount: 12.35}},
		{map[string]interface{}{"min_discount_percent": "25%"}, routing{minDiscount: 25}},
		{map[string]interface{}{"min_discount_percent": "0"}, routing{}},
		{map[string]interface{}{"min_discount_percent": "100"}, routing{}},
		{map[string]interface{}{"min_discount_percent": "abc"}, routing{}},
	}
	for _, c := range cases {
		var plugin model.PluginType
		if c.opts != nil {
			plugin = model.PluginType{"cheaperinference_routing": c.opts}
		}
		if got := routingOf(channelWith(plugin)); got != c.want {
			t.Errorf("%v: routing = %+v, want %+v", c.opts, got, c.want)
		}
	}
}

// The channel's fields reach the upstream on every inference endpoint, over a client's own
// values, and nowhere else.
func TestRoutingHookWritesTheChannelsFields(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	bodies := map[string]string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if int64(len(b)) != r.ContentLength {
			t.Errorf("%s: content length %d for %d bytes", r.URL.Path, r.ContentLength, len(b))
		}
		bodies[r.URL.Path] = string(b)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer upstream.Close()

	hook := routingHook(channelWith(model.PluginType{"cheaperinference_routing": {"zdr": "on", "min_discount_percent": "20"}}))
	if hook == nil {
		t.Fatal("no hook for a channel with routing settings")
	}
	r := requester.NewHTTPRequester("", nil)
	r.RequestHook = hook
	send := func(path, body string) {
		req, err := r.NewRequest(http.MethodPost, upstream.URL+path, r.WithBody(strings.NewReader(body)), r.WithHeader(map[string]string{"Content-Type": "application/json"}))
		if err != nil {
			t.Fatal(err)
		}
		if _, errWithCode := r.SendRequest(req, &map[string]interface{}{}, false); errWithCode != nil {
			t.Fatalf("%s: %v", path, errWithCode.Message)
		}
	}
	client := `{"model":"m","zdr":false,"min_discount_percent":1,"messages":[]}`
	for _, path := range routedPaths {
		send(path, client)
		b := bodies[path]
		if !gjson.Get(b, "zdr").Bool() || gjson.Get(b, "min_discount_percent").Float() != 20 || gjson.Get(b, "model").String() != "m" {
			t.Errorf("%s body = %s", path, b)
		}
	}
	send("/v1/embeddings", `{"model":"e","input":"x"}`)
	if b := bodies["/v1/embeddings"]; gjson.Get(b, "zdr").Exists() || gjson.Get(b, "min_discount_percent").Exists() {
		t.Errorf("embeddings got routing fields: %s", b)
	}
}

func TestRoutingRefusesANonObjectBody(t *testing.T) {
	if _, err := (routing{zdr: true}).apply([]byte(`[1,2]`)); err == nil {
		t.Error("a ZDR request must not go out without the field")
	}
	if routingHook(channelWith(nil)) != nil {
		t.Error("a channel without routing settings needs no hook")
	}
}

func TestMinDiscountNarrowsTheDefaultModels(t *testing.T) {
	models := []publicModel{
		{Id: "a", Type: "text", DiscountPercent: "35.00", ReferenceInput: strp("5")},
		{Id: "b", Type: "text", DiscountPercent: "15.00", ReferenceInput: strp("5")},
		{Id: "c", Type: "text", DiscountPercent: "45.00"}, // no list price
		{Id: "e", Type: "embedding", DiscountPercent: "0.00"},
	}
	var got []string
	for _, m := range defaultModels(models, 30) {
		got = append(got, m.Id)
	}
	if strings.Join(got, ",") != "a,e" {
		t.Errorf("models with a 30%% minimum = %v", got)
	}
	if n := len(defaultModels(models, 0)); n != 4 {
		t.Errorf("without a minimum = %d models", n)
	}
}

func TestZDRChannelReadsTheZDRCatalog(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var gotQuery string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"models":[]}`))
	}))
	defer upstream.Close()
	channel := channelWith(model.PluginType{"cheaperinference_routing": {"zdr": "on"}})
	channel.BaseURL = &upstream.URL
	p := CheaperInferenceProviderFactory{}.Create(channel).(*CheaperInferenceProvider)
	if _, err := p.fetchModels(); err != nil {
		t.Fatal(err)
	}
	if gotQuery != "zdr=true" {
		t.Errorf("query = %q", gotQuery)
	}
}

func TestMessagesRequestsKeepTheRoutingHook(t *testing.T) {
	p := CheaperInferenceProviderFactory{}.Create(channelWith(model.PluginType{"cheaperinference_routing": {"zdr": "on"}})).(*CheaperInferenceProvider)
	p.SetContext(testContext(`{}`))
	if p.claudeProvider().Requester.RequestHook == nil {
		t.Fatal("Claude format requests would go out without zdr")
	}
}
