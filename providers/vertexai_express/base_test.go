package vertexai_express

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// SEC-54: the region in Other is part of the host, so only a plain label may build a URL.
func TestRegionMustBeHostLabel(t *testing.T) {
	ok := VertexAIExpressProviderFactory{}.Create(&model.Channel{Id: 1, Key: "k", Other: "europe-west4|proj"}).(*VertexAIExpressProvider)
	want := "https://europe-west4-aiplatform.googleapis.com/v1beta1/projects/proj/locations/europe-west4/publishers/google/models/m:generateContent"
	if got := ok.GetFullRequestURL("generateContent", "m"); got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	def := VertexAIExpressProviderFactory{}.Create(&model.Channel{Id: 2, Key: "k", Other: "proj"}).(*VertexAIExpressProvider)
	if def.Region != "us-central1" || def.GetFullRequestURL("generateContent", "m") == "" {
		t.Fatalf("project only: region %q, url %q", def.Region, def.GetFullRequestURL("generateContent", "m"))
	}
	for _, other := range []string{"attacker.tld/x?|p", "evil.com#|p", "a@b|p", "host:9977|p"} {
		p := VertexAIExpressProviderFactory{}.Create(&model.Channel{Id: 3, Key: "k", Other: other}).(*VertexAIExpressProvider)
		if p.Config.BaseURL != "" {
			t.Errorf("other %q set base URL %q", other, p.Config.BaseURL)
		}
		if got := p.GetFullRequestURL("generateContent", "m"); got != "" {
			t.Errorf("other %q built %q, want rejection", other, got)
		}
	}
}

// SEC-83: the key goes in the x-goog-api-key header, never in the URL, which transport errors
// quote and the system log records.
func TestKeyTravelsInHeaderNotURL(t *testing.T) {
	p := VertexAIExpressProviderFactory{}.Create(&model.Channel{Id: 1, Key: "AIzaSecret", Other: "us-central1|proj"}).(*VertexAIExpressProvider)
	for _, action := range []string{"generateContent", "streamGenerateContent?alt=sse", "countTokens"} {
		if got := p.GetFullRequestURL(action, "m"); strings.Contains(got, "AIzaSecret") || strings.Contains(got, "key=") {
			t.Errorf("action %q: URL %q carries the key", action, got)
		}
	}
	if got := p.GetFullRequestURL("streamGenerateContent?alt=sse", "m"); !strings.HasSuffix(got, "/m:streamGenerateContent?alt=sse") {
		t.Errorf("stream URL %q", got)
	}
	if got := p.GetRequestHeaders()["x-goog-api-key"]; got != "AIzaSecret" {
		t.Errorf("x-goog-api-key header %q, want the channel key", got)
	}
}
