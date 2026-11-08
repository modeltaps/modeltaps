package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
)

// The scheduled drift check builds providers without a client request (nil gin context).
// An Anthropic API-key channel used to dereference that nil context while building its
// headers and panic, which took down the whole process. It must list models instead.
func TestFetchUpstreamModelsAnthropicWithoutRequestContext(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var gotVersion, gotKey string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotVersion = r.Header.Get("anthropic-version")
		gotKey = r.Header.Get("x-api-key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-opus-5-5"},{"id":"claude-haiku-5-5"}]}`))
	}))
	defer upstream.Close()

	baseURL := upstream.URL
	channel := &model.Channel{Type: 14, Key: "sk-ant-test\nsk-ant-second", BaseURL: &baseURL}

	models, err := fetchUpstreamModels(channel)
	if err != nil {
		t.Fatalf("fetchUpstreamModels: %v", err)
	}
	if len(models) != 2 || models[0] != "claude-opus-5-5" {
		t.Fatalf("unexpected models %v", models)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want the default", gotVersion)
	}
	if gotKey != "sk-ant-test" {
		t.Fatalf("x-api-key = %q, want the first key line", gotKey)
	}
}
