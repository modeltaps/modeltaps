package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/types"
)

// SVG and GIF output parameters, and the SVG echo fields, have to be declared on the structs or
// the struct-encoding path (PassThroughBody off) drops them when it re-encodes the request or
// the response.
func TestImageSVGGIFOptions_RequestBodyCarriesFields(t *testing.T) {
	p := newStoreTestProvider("{}", false)
	complexity := "high"
	format := "svg"
	frames, duration := 8, 120
	req, errWithCode := p.GetRequestTextBody(config.RelayModeImagesGenerations, "gpt-image-2.6-sunburst", &types.ImageRequest{
		Model:           "gpt-image-2.6-sunburst",
		Prompt:          "a logo",
		OutputFormat:    &format,
		Complexity:      &complexity,
		FrameCount:      &frames,
		FrameDurationMs: &duration,
		SVG:             json.RawMessage(`{"model":"gpt-5.6","reasoning":{"effort":"low"},"future_field":true}`),
	})
	if errWithCode != nil {
		t.Fatalf("unexpected error: %v", errWithCode)
	}
	got := decodeBody(t, req)
	if got["complexity"] != "high" || got["output_format"] != "svg" {
		t.Fatalf("complexity / output_format must be forwarded, got %v", got)
	}
	if got["frame_count"] != float64(8) || got["frame_duration_ms"] != float64(120) {
		t.Fatalf("frame options must be forwarded, got %v", got)
	}
	svg, ok := got["svg"].(map[string]any)
	if !ok || svg["model"] != "gpt-5.6" || svg["future_field"] != true {
		t.Fatalf("svg must be forwarded verbatim, including sub-fields not known here, got %v", got["svg"])
	}
}

func TestImageSVGGIFOptions_OmittedWhenUnset(t *testing.T) {
	p := newStoreTestProvider("{}", false)
	req, errWithCode := p.GetRequestTextBody(config.RelayModeImagesGenerations, "gpt-image-1", &types.ImageRequest{
		Model: "gpt-image-1", Prompt: "a cat",
	})
	if errWithCode != nil {
		t.Fatalf("unexpected error: %v", errWithCode)
	}
	got := decodeBody(t, req)
	for _, key := range []string{"complexity", "frame_count", "frame_duration_ms", "svg"} {
		if _, ok := got[key]; ok {
			t.Fatalf("%s must be omitted when unset, got %v", key, got)
		}
	}
}

func TestImageSVGGIFOptions_ResponseRoundTrip(t *testing.T) {
	upstream := `{"created":1700000000,"data":[{"b64_json":"QUJD"}],"output_format":"svg","svg":{"model":"gpt-5.6"},"svg_usage":{"model":"gpt-5.6","input_tokens":10,"output_tokens":20,"reasoning_tokens":5,"total_tokens":35},"usage":{"total_tokens":10,"input_tokens":2,"output_tokens":8}}`

	var resp OpenAIProviderImageResponse
	if err := json.Unmarshal([]byte(upstream), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := json.Marshal(&resp.ImageResponse)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, want := range []string{`"svg":{"model":"gpt-5.6"}`, `"svg_usage":{"model":"gpt-5.6","input_tokens":10,"output_tokens":20,"reasoning_tokens":5,"total_tokens":35}`} {
		if !strings.Contains(string(out), want) {
			t.Fatalf("response lost %s after round trip: %s", want, out)
		}
	}
	// svg_usage is echoed only: the billing usage stays the top-level one.
	if resp.Usage == nil || resp.Usage.TotalTokens != 10 {
		t.Fatalf("billing usage must stay the top-level usage, got %+v", resp.Usage)
	}

	t.Run("omitted when upstream has none", func(t *testing.T) {
		var resp OpenAIProviderImageResponse
		if err := json.Unmarshal([]byte(`{"created":1,"data":[{"url":"https://x/y.png"}]}`), &resp); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		out, _ := json.Marshal(&resp.ImageResponse)
		if strings.Contains(string(out), "svg") {
			t.Fatalf("missing svg fields must not be emitted: %s", out)
		}
	})
}

func TestImageSVGGIFOptions_SyntheticSSECarriesFields(t *testing.T) {
	resp := &types.ImageResponse{
		Created:  1700000000,
		Data:     []types.ImageResponseDataInner{{B64JSON: "QUJD"}},
		SVG:      json.RawMessage(`{"model":"gpt-5.6"}`),
		SVGUsage: json.RawMessage(`{"total_tokens":35}`),
	}
	sse := ImageResponseToSSE(ImageGenerationStreamPrefix, resp)
	for _, want := range []string{`"svg":{"model":"gpt-5.6"}`, `"svg_usage":{"total_tokens":35}`} {
		if !strings.Contains(sse, want) {
			t.Fatalf("synthetic completed event must echo %s: %s", want, sse)
		}
	}
}
