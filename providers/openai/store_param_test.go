package openai

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestFilterCustomParamsForRelayMode(t *testing.T) {
	t.Run("nil passthrough", func(t *testing.T) {
		if got := FilterCustomParamsForRelayMode(nil, config.RelayModeImagesGenerations); got != nil {
			t.Fatalf("expected nil, got %v", got)
		}
	})

	t.Run("chat keeps store", func(t *testing.T) {
		in := map[string]interface{}{"overwrite": true, "store": false}
		got := FilterCustomParamsForRelayMode(in, config.RelayModeChatCompletions)
		if _, ok := got["store"]; !ok {
			t.Fatalf("chat completions should keep store, got %v", got)
		}
	})

	t.Run("responses keeps store", func(t *testing.T) {
		// The chat-to-Responses path reaches GetRequestTextBody with RelayModeResponses and must keep store
		in := map[string]interface{}{"store": false}
		got := FilterCustomParamsForRelayMode(in, config.RelayModeResponses)
		if _, ok := got["store"]; !ok {
			t.Fatalf("responses should keep store, got %v", got)
		}
	})

	t.Run("legacy completions strips store", func(t *testing.T) {
		// /v1/completions has no store parameter
		in := map[string]interface{}{"store": false}
		got := FilterCustomParamsForRelayMode(in, config.RelayModeCompletions)
		if _, ok := got["store"]; ok {
			t.Fatalf("legacy completions should strip store, got %v", got)
		}
	})

	t.Run("no store anywhere returns same map", func(t *testing.T) {
		in := map[string]interface{}{"overwrite": true, "quality": "hd"}
		got := FilterCustomParamsForRelayMode(in, config.RelayModeImagesGenerations)
		if reflect.ValueOf(got).Pointer() != reflect.ValueOf(in).Pointer() {
			t.Fatalf("map without store should be returned as-is, not copied")
		}
	})

	t.Run("non per_model nested store key is preserved", func(t *testing.T) {
		in := map[string]interface{}{
			"metadata": map[string]interface{}{"store": "x", "k": "v"},
		}
		got := FilterCustomParamsForRelayMode(in, config.RelayModeImagesGenerations)
		sub, ok := got["metadata"].(map[string]interface{})
		if !ok || sub["store"] != "x" {
			t.Fatalf("nested key under plain object must not be touched outside per_model, got %v", got)
		}
	})

	t.Run("image strips top-level store but keeps others", func(t *testing.T) {
		in := map[string]interface{}{"overwrite": true, "store": false, "quality": "hd"}
		got := FilterCustomParamsForRelayMode(in, config.RelayModeImagesGenerations)
		if _, ok := got["store"]; ok {
			t.Fatalf("image generation should strip store, got %v", got)
		}
		if got["overwrite"] != true || got["quality"] != "hd" {
			t.Fatalf("other params must survive, got %v", got)
		}
		// the input map must not be mutated
		if _, ok := in["store"]; !ok {
			t.Fatalf("input map must not be mutated")
		}
	})

	t.Run("image strips per_model nested store", func(t *testing.T) {
		in := map[string]interface{}{
			"per_model": true,
			"gpt-image-1": map[string]interface{}{
				"store":   false,
				"quality": "hd",
			},
		}
		got := FilterCustomParamsForRelayMode(in, config.RelayModeImagesGenerations)
		sub, ok := got["gpt-image-1"].(map[string]interface{})
		if !ok {
			t.Fatalf("per_model sub config missing: %v", got)
		}
		if _, has := sub["store"]; has {
			t.Fatalf("nested store should be stripped, got %v", sub)
		}
		if sub["quality"] != "hd" {
			t.Fatalf("nested non-store params must survive, got %v", sub)
		}
		// the nested input map must not be mutated
		if _, has := in["gpt-image-1"].(map[string]interface{})["store"]; !has {
			t.Fatalf("input nested map must not be mutated")
		}
	})

	t.Run("embeddings and speech strip store", func(t *testing.T) {
		for _, mode := range []int{
			config.RelayModeCompletions,
			config.RelayModeEmbeddings,
			config.RelayModeAudioSpeech,
			config.RelayModeModerations,
			config.RelayModeImagesEdits,
			config.RelayModeImagesVariations,
		} {
			got := FilterCustomParamsForRelayMode(map[string]interface{}{"store": false}, mode)
			if _, ok := got["store"]; ok {
				t.Fatalf("relayMode %d should strip store", mode)
			}
		}
	})

	t.Run("empty after filter returns nil", func(t *testing.T) {
		// A channel with only {"store": false} leaves nothing to merge for non-chat endpoints: return nil
		// so callers skip merging, like CustomParameterHandler treating "{}" as nil.
		in := map[string]interface{}{"store": false}
		if got := FilterCustomParamsForRelayMode(in, config.RelayModeImagesGenerations); got != nil {
			t.Fatalf("expected nil after filtering out the only key, got %v", got)
		}
		if _, ok := in["store"]; !ok {
			t.Fatalf("input map must not be mutated")
		}
	})
}

func newStoreTestProvider(customParameter string, passThrough bool) *OpenAIProvider {
	return CreateOpenAIProvider(&model.Channel{
		Type:            config.ChannelTypeOpenAI,
		Key:             "sk-test",
		CustomParameter: &customParameter,
		PassThroughBody: passThrough,
	}, "https://api.openai.com")
}

func decodeBody(t *testing.T, req *http.Request) map[string]interface{} {
	t.Helper()
	raw, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode body %q: %v", raw, err)
	}
	return got
}

func TestGetRequestTextBody_StoreOnlyOnChat(t *testing.T) {
	custom := `{"overwrite": true, "store": false}`

	t.Run("image generation does not carry store", func(t *testing.T) {
		p := newStoreTestProvider(custom, false)
		body := map[string]interface{}{"model": "gpt-image-1", "prompt": "a cat"}
		req, errWithCode := p.GetRequestTextBody(config.RelayModeImagesGenerations, "gpt-image-1", body)
		if errWithCode != nil {
			t.Fatalf("unexpected error: %v", errWithCode)
		}
		got := decodeBody(t, req)
		if _, ok := got["store"]; ok {
			t.Fatalf("image body must not contain store: %v", got)
		}
	})

	t.Run("responses carries store=false", func(t *testing.T) {
		p := newStoreTestProvider(custom, false)
		body := map[string]interface{}{"model": "gpt-5.3-codex", "input": "hi", "store": true}
		req, errWithCode := p.GetRequestTextBody(config.RelayModeResponses, "gpt-5.3-codex", body)
		if errWithCode != nil {
			t.Fatalf("unexpected error: %v", errWithCode)
		}
		got := decodeBody(t, req)
		if v, ok := got["store"]; !ok || v != false {
			t.Fatalf("responses body must contain store=false: %v", got)
		}
	})

	t.Run("chat completions carries store=false", func(t *testing.T) {
		p := newStoreTestProvider(custom, false)
		body := map[string]interface{}{"model": "gpt-4o", "messages": []interface{}{}}
		req, errWithCode := p.GetRequestTextBody(config.RelayModeChatCompletions, "gpt-4o", body)
		if errWithCode != nil {
			t.Fatalf("unexpected error: %v", errWithCode)
		}
		got := decodeBody(t, req)
		if v, ok := got["store"]; !ok || v != false {
			t.Fatalf("chat body must contain store=false: %v", got)
		}
	})
}

func newStoreTestContext(path, rawBody string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(rawBody))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(config.GinRequestBodyKey, []byte(rawBody))
	return c
}

func TestGetRequestTextBody_PassThroughStoreOnlyOnChat(t *testing.T) {
	custom := `{"overwrite": true, "store": false}`

	t.Run("pass-through image generation does not carry store", func(t *testing.T) {
		p := newStoreTestProvider(custom, true)
		p.Context = newStoreTestContext("/v1/images/generations", `{"model":"gpt-image-1","prompt":"a cat"}`)
		req, errWithCode := p.GetRequestTextBody(config.RelayModeImagesGenerations, "gpt-image-1", nil)
		if errWithCode != nil {
			t.Fatalf("unexpected error: %v", errWithCode)
		}
		got := decodeBody(t, req)
		if _, ok := got["store"]; ok {
			t.Fatalf("pass-through image body must not contain store: %v", got)
		}
		if got["prompt"] != "a cat" {
			t.Fatalf("pass-through body must be preserved: %v", got)
		}
	})

	t.Run("pass-through chat completions overrides store", func(t *testing.T) {
		p := newStoreTestProvider(custom, true)
		p.Context = newStoreTestContext("/v1/chat/completions", `{"model":"gpt-4o","messages":[],"store":true}`)
		req, errWithCode := p.GetRequestTextBody(config.RelayModeChatCompletions, "gpt-4o", nil)
		if errWithCode != nil {
			t.Fatalf("unexpected error: %v", errWithCode)
		}
		got := decodeBody(t, req)
		if v, ok := got["store"]; !ok || v != false {
			t.Fatalf("pass-through chat body must have store overridden to false: %v", got)
		}
	})
}
