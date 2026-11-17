package vertexai_express

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// SEC-54: the region in Other is part of the host, so only a plain label may build a URL; the key
// travels in its query string.
func TestRegionMustBeHostLabel(t *testing.T) {
	ok := VertexAIExpressProviderFactory{}.Create(&model.Channel{Id: 1, Key: "k", Other: "europe-west4|proj"}).(*VertexAIExpressProvider)
	want := "https://europe-west4-aiplatform.googleapis.com/v1beta1/projects/proj/locations/europe-west4/publishers/google/models/m:generateContent?key=k"
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
