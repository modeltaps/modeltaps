package preset

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
)

// The fixtures are entries of the platforms' lists as served on 2026-11-18, trimmed.

func TestDeepInfraPrices(t *testing.T) {
	prices, err := parseDeepInfraPrices([]byte(`{"data":[
	  {"id":"anthropic/claude-opus-5-5","metadata":{"pricing":{"input_tokens":4.0,"output_tokens":20.0,"cache_read_tokens":0.2},"tags":["chat","prompt_cache"]}},
	  {"id":"google/embeddinggemma-300m","metadata":{"pricing":{"input_tokens":0.002},"tags":["embed"]}},
	  {"id":"ByteDance/Seedream-4","metadata":{"pricing":{"per_image_unit":0.04},"tags":["image-gen"]}},
	  {"id":"Qwen/Qwen3-ASR-1.7B","metadata":{"pricing":{"input_seconds":0.0000075},"tags":["stt"]}},
	  {"id":"PrunaAI/p-video","metadata":{"pricing":{"output_seconds":0.02},"tags":["video-gen"]}},
	  {"id":"sesame/csm-1b","metadata":{"pricing":{"input_characters":7.0},"tags":["tts"]}},
	  {"id":"vendor/unpriced","metadata":{"pricing":{},"tags":["chat"]}},
	  {"id":"vendor/no-metadata","metadata":null}
	]}`))
	if err != nil || len(prices) != 6 {
		t.Fatalf("prices = %d, %v", len(prices), err)
	}
	if p := prices[0]; p.Input != 4 || p.Output != 20 || p.CacheRead != 0.2 || p.Kind != "" {
		t.Errorf("opus = %+v", p)
	}
	if p := prices[1]; p.Input != 0.002 || p.Kind != model.ModelKindEmbedding {
		t.Errorf("embedding = %+v", p)
	}
	if p := prices[2]; p.Image != 0.04 || p.Kind != model.ModelKindImage {
		t.Errorf("image = %+v", p)
	}
	if p := prices[3]; p.AudioSecond != 0.0000075 || p.Kind != model.ModelKindTranscription {
		t.Errorf("transcription = %+v", p)
	}
	if p := prices[4]; p.VideoSecond != 0.02 || p.Kind != model.ModelKindVideo {
		t.Errorf("video = %+v", p)
	}
	if p := prices[5]; !p.Other || p.Kind != model.ModelKindSpeech {
		t.Errorf("speech priced per character should be marked other: %+v", p)
	}
}

func TestNovitaPrices(t *testing.T) {
	prices, err := parseNovitaPrices([]byte(`{"data":[
	  {"id":"tencent/hy4-flash","input_token_price_per_m":700,"pricing":{
	    "prompt":{"origin_price_per_m":800,"price_per_m":700,"origin_price_per_m_decimal":"0.08","price_per_m_decimal":"0.07"},
	    "completion":{"price_per_m":2810,"price_per_m_decimal":"0.281"},
	    "input_cache_read":{"price_per_m":30,"price_per_m_decimal":"0.003"}},"output_modalities":["text"]},
	  {"id":"vendor/unpriced","pricing":null}
	]}`))
	if err != nil || len(prices) != 1 {
		t.Fatalf("prices = %d, %v", len(prices), err)
	}
	if p := prices[0]; p.Input != 0.07 || p.Output != 0.281 || p.CacheRead != 0.003 {
		t.Errorf("hy4-flash = %+v, want the discounted decimal prices", p)
	}
}

func TestHuggingFacePricesKeepTheDearestProvider(t *testing.T) {
	prices, err := parseHuggingFacePrices([]byte(`{"data":[
	  {"id":"Qwen/Qwen3.8-27B","architecture":{"output_modalities":["text"]},"providers":[
	    {"provider":"akashml","status":"live","pricing":{"input":0.225,"output":1.98}},
	    {"provider":"cerebras","status":"live","pricing":{"input":0.99,"output":1.49}},
	    {"provider":"gone","status":"staging","pricing":{"input":9,"output":9}},
	    {"provider":"unpriced","status":"live"}]},
	  {"id":"vendor/unpriced","providers":[{"provider":"x","status":"live"}]}
	]}`))
	if err != nil || len(prices) != 1 {
		t.Fatalf("prices = %d, %v", len(prices), err)
	}
	if p := prices[0]; p.Input != 0.99 || p.Output != 1.98 {
		t.Errorf("qwen = %+v, want the highest live input and output", p)
	}
}

func TestTokenPriceLists(t *testing.T) {
	chutes, err := parseChutesPrices([]byte(`{"data":[{"id":"Qwen/Qwen3.8-27B-TEE","pricing":{"prompt":0.039,"completion":1.25,"input_cache_read":0.0039}}]}`))
	if err != nil || len(chutes) != 1 || chutes[0].Input != 0.039 || chutes[0].Output != 1.25 || chutes[0].CacheRead != 0.0039 {
		t.Errorf("chutes = %+v, %v", chutes, err)
	}
	samba, err := parsePerTokenPrices([]byte(`{"data":[
	  {"id":"GLM-5.3","pricing":{"completion":"0.00000880","input_cache_read":"0.00000014","input_cache_write":"0.00000000","prompt":"0.00000280"}},
	  {"id":"no-price"}]}`))
	if err != nil || len(samba) != 1 {
		t.Fatalf("sambanova = %+v, %v", samba, err)
	}
	if p := samba[0]; p.Input != 2.8 || p.Output != 8.8 || p.CacheRead != 0.14 || p.CacheWrite != 0 {
		t.Errorf("GLM-5.3 = %+v, want USD per million", p)
	}
}

func TestOwnPriceListIsRead(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var auth, path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, path = r.Header.Get("Authorization"), r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"b","pricing":{"prompt":"0.00000035","completion":"0.00000075"}},{"id":"a","pricing":{"prompt":"0","completion":"0"}}]}`))
	}))
	defer srv.Close()
	read := priceList(srv.URL+"/public/v1/models", parsePerTokenPrices)
	p := ProviderFactory{Preset: Presets[config.ChannelTypeCerebras]}.Create(&model.Channel{Key: "csk-wrong"}).(*Provider)
	prices, err := read(p)
	if err != nil || len(prices) != 2 {
		t.Fatalf("prices = %+v, %v", prices, err)
	}
	if auth != "" || path != "/public/v1/models" {
		t.Errorf("request sent auth %q to %q; the public list is read without the key", auth, path)
	}
	if prices[0].Model != "a" || prices[1].Input != 0.35 || prices[1].Output != 0.75 {
		t.Errorf("prices = %+v %+v, want sorted, free model kept, conversion noise dropped", *prices[0], *prices[1])
	}
	if ownPriceLists[config.ChannelTypeTogether] != nil {
		t.Error("Together's priced list needs a key and stays on models.dev")
	}
}
