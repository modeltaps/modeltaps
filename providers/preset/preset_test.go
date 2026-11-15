package preset

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
)

func TestPresetsAreComplete(t *testing.T) {
	if len(Presets) != 16 {
		t.Errorf("presets = %d", len(Presets))
	}
	for channelType, p := range Presets {
		if p.Type != channelType || p.Name == "" || p.ModelsDev == "" {
			t.Errorf("preset %d = %+v", channelType, p)
		}
		if !strings.HasPrefix(p.Root, "https://") || strings.HasSuffix(p.Root, "/") {
			t.Errorf("%s root %q must be an https URL without a trailing slash", p.Name, p.Root)
		}
	}
}

func TestURLs(t *testing.T) {
	p := ProviderFactory{Preset: Presets[config.ChannelTypeFireworks]}.Create(&model.Channel{Key: "fw-key"}).(*Provider)
	if got := p.GetFullRequestURL(p.Config.ChatCompletions, ""); got != "https://api.fireworks.ai/inference/v1/chat/completions" {
		t.Errorf("chat URL = %q", got)
	}
	if got := p.GetRequestHeaders()["Authorization"]; got != "Bearer fw-key" {
		t.Errorf("auth = %q", got)
	}
	base := "https://my-relay.example.com/v1/"
	relay := ProviderFactory{Preset: Presets[config.ChannelTypeFireworks]}.Create(&model.Channel{BaseURL: &base}).(*Provider)
	if got := relay.GetFullRequestURL(relay.Config.ModelList, ""); got != "https://my-relay.example.com/v1/models" {
		t.Errorf("a channel's own address should replace the root, got %q", got)
	}
	cerebras := ProviderFactory{Preset: Presets[config.ChannelTypeCerebras]}.Create(&model.Channel{}).(*Provider)
	if cerebras.Config.Embeddings != "" {
		t.Error("Cerebras serves no embeddings")
	}
}

func TestModelListFromThePlatform(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"zai-org/GLM-5.2"},{"id":"moonshotai/Kimi-K3"}]}`))
	}))
	defer srv.Close()
	base := srv.URL + "/v1"
	p := ProviderFactory{Preset: Presets[config.ChannelTypeTogether]}.Create(&model.Channel{BaseURL: &base}).(*Provider)
	list, err := p.GetModelList()
	if err != nil || len(list) != 2 || list[0] != "zai-org/GLM-5.2" {
		t.Fatalf("list = %v, %v", list, err)
	}
	if gotPath != "/v1/models" {
		t.Errorf("path = %q", gotPath)
	}
}
