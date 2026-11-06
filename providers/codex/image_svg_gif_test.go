package codex

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/types"
)

func TestBuildToolFromImageRequestMapsSVGAndGIFOptions(t *testing.T) {
	complexity := "  medium "
	format := "svg"
	frames, duration := 6, 200
	tool := buildToolFromImageRequest(&types.ImageRequest{
		Model:           "gpt-image-2.6-flare",
		Prompt:          "a logo",
		OutputFormat:    &format,
		Complexity:      &complexity,
		FrameCount:      &frames,
		FrameDurationMs: &duration,
		SVG:             json.RawMessage(`{"model":"gpt-5.6"}`),
	})

	body, err := json.Marshal(buildImagesRequestBody(imageToolActionGenerate, "a logo", tool, nil, "", "gpt-5.6"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode request body: %v (%s)", err, body)
	}
	if len(decoded.Tools) != 1 {
		t.Fatalf("expected one tool, got %s", body)
	}
	got := decoded.Tools[0]
	if got["complexity"] != "medium" {
		t.Fatalf("complexity must be trimmed and forwarded, got %v", got["complexity"])
	}
	if got["frame_count"] != float64(6) || got["frame_duration_ms"] != float64(200) {
		t.Fatalf("frame options must be forwarded, got %v", got)
	}
	if got["output_format"] != "svg" {
		t.Fatalf("output_format must be forwarded, got %v", got["output_format"])
	}
	svg, ok := got["svg"].(map[string]any)
	if !ok || svg["model"] != "gpt-5.6" {
		t.Fatalf("svg must be forwarded, got %v", got["svg"])
	}
}

func TestBuildToolFromImageRequestOmitsUnsetSVGAndGIFOptions(t *testing.T) {
	tool := buildToolFromImageRequest(&types.ImageRequest{Model: "gpt-image-1", Prompt: "a cat"})
	body, err := json.Marshal(buildImagesRequestBody(imageToolActionGenerate, "a cat", tool, nil, "", "gpt-5.6"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"complexity", "frame_count", "frame_duration_ms", `"svg"`} {
		if strings.Contains(string(body), key) {
			t.Fatalf("%s must be omitted when unset: %s", key, body)
		}
	}
}

func TestImageMIMEType(t *testing.T) {
	cases := map[string]string{
		"":      "image/png",
		"png":   "image/png",
		"jpg":   "image/jpeg",
		"JPEG":  "image/jpeg",
		"webp":  "image/webp",
		"gif":   "image/gif",
		" SVG ": "image/svg+xml",
		"bmp":   "image/png",
	}
	for format, want := range cases {
		if got := imageMIMEType(format); got != want {
			t.Errorf("imageMIMEType(%q) = %q, want %q", format, got, want)
		}
	}
}
