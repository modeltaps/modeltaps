package vercel

import (
	"encoding/json"
	"testing"
)

func TestBuildChannelPrices(t *testing.T) {
	var resp modelListResponse
	if err := json.Unmarshal([]byte(sampleModels), &resp); err != nil {
		t.Fatal(err)
	}
	var video gatewayModel
	if err := json.Unmarshal([]byte(`{"id":"alibaba/wan-video","type":"video","pricing":{"video_duration_pricing":[
	  {"resolution":"720p","cost_per_second":"0.1"},{"resolution":"480p","cost_per_second":"0.05"}]}}`), &video); err != nil {
		t.Fatal(err)
	}
	var speech gatewayModel
	if err := json.Unmarshal([]byte(`{"id":"vendor/tts","pricing":{"speech_output_cost_per_second":"0.001"}}`), &speech); err != nil {
		t.Fatal(err)
	}
	resp.Data = append(resp.Data, gatewayModel{Id: "vendor/unpriced"}, video, speech)
	prices := buildChannelPrices(&resp)
	byModel := map[string]int{}
	for i, p := range prices {
		byModel[p.Model] = i
	}
	if len(prices) != 5 {
		t.Fatalf("prices = %d, want 5 (sonnet, embedding, image, video, speech): %+v", len(prices), prices)
	}
	if s := prices[byModel["vendor/tts"]]; !s.Other {
		t.Errorf("speech = %+v, want marked as priced another way", s)
	}
	if prices[byModel["anthropic/claude-sonnet-4"]].Other {
		t.Error("a token-priced model must not be marked as priced another way")
	}
	if v := prices[byModel["alibaba/wan-video"]]; v.VideoSecond != 0.05 {
		t.Errorf("video = %+v, want the lowest per-second price 0.05", v)
	}
	if _, ok := byModel["vendor/unpriced"]; ok {
		t.Error("a model without price fields must be skipped")
	}
	sonnet := prices[byModel["anthropic/claude-sonnet-4"]]
	if sonnet.Input != 3 || sonnet.Output != 15 || sonnet.CacheRead != 0.3 || sonnet.CacheWrite != 3.75 {
		t.Errorf("sonnet = %+v, want 3 / 15 / 0.3 / 3.75 per million", sonnet)
	}
	if image := prices[byModel["openai/gpt-image-1"]]; image.Image != 0.04 || image.Input != 0 {
		t.Errorf("image = %+v, want 0.04 per image", image)
	}
}
