package codex

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func strPtr(s string) *string { return &s }
func intPtr(n int) *int       { return &n }

// decodeBody decodes a request body into a generic map so tests can assert on the JSON keys.
func decodeBody(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode body: %v (%s)", err, body)
	}
	return decoded
}

func TestIsCodexDirectImageModel(t *testing.T) {
	for _, model := range []string{"gpt-image-2", " GPT-Image-2.6-Flare ", "gpt-image-1.5", "gpt-image-2.5-sunburst-2026-09-08"} {
		if !isCodexDirectImageModel(model) {
			t.Errorf("%q must use the direct endpoint", model)
		}
	}
	// gpt-image-1 is retired upstream, chatgpt-image-* is an alias entry, and unknown future
	// models may not exist on the direct endpoint: all stay on the orchestration path.
	for _, model := range []string{"gpt-image-1", "gpt-image-1-mini", "chatgpt-image-latest", "gpt-image-3", "dall-e-3", ""} {
		if isCodexDirectImageModel(model) {
			t.Errorf("%q must stay on the Responses orchestration path", model)
		}
	}
}

func TestCanUseDirectImagesOnlyWithoutBaseURLOverride(t *testing.T) {
	relay, official, trailing := "https://relay.example.com", "https://chatgpt.com", "https://chatgpt.com/"
	empty := " "
	cases := []struct {
		name    string
		baseURL *string
		want    bool
	}{
		{"unset", nil, true},
		{"blank", &empty, true},
		{"explicit official host", &official, true},
		{"official host with trailing slash", &trailing, true},
		{"relay", &relay, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			provider := &CodexProvider{}
			provider.Channel = &model.Channel{BaseURL: c.baseURL}
			if got := provider.canUseDirectImages(); got != c.want {
				t.Fatalf("canUseDirectImages = %v, want %v", got, c.want)
			}
			if got := provider.shouldUseDirectImages("gpt-image-2"); got != c.want {
				t.Fatalf("shouldUseDirectImages(gpt-image-2) = %v, want %v", got, c.want)
			}
			if provider.shouldUseDirectImages("gpt-image-1") {
				t.Fatal("a model outside the whitelist must never use the direct endpoint")
			}
		})
	}
}

func TestBuildDirectImageGenerationsBody(t *testing.T) {
	stream := true
	partial := 2
	body, err := buildDirectImageGenerationsBody(&types.ImageRequest{
		Model:             " gpt-image-2.6 ",
		Prompt:            " a logo ",
		N:                 3,
		Size:              "1024x1024",
		Quality:           "high",
		Style:             "vivid",
		ResponseFormat:    "url",
		User:              "end-user-1",
		SafetyIdentifier:  "hash-1",
		AspectRatio:       strPtr("16:9"),
		Background:        strPtr(" transparent "),
		Moderation:        strPtr("low"),
		OutputFormat:      strPtr("svg"),
		OutputCompression: intPtr(80),
		Complexity:        strPtr(" medium "),
		FrameCount:        intPtr(6),
		FrameDurationMs:   intPtr(200),
		SVG:               json.RawMessage(`{"model":"gpt-5.6"}`),
		Stream:            &stream,
		PartialImages:     &partial,
		ExtraParams:       map[string]interface{}{"x": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeBody(t, body)

	want := map[string]any{
		"model":              "gpt-image-2.6",
		"prompt":             "a logo",
		"n":                  float64(3),
		"size":               "1024x1024",
		"quality":            "high",
		"background":         "transparent",
		"moderation":         "low",
		"output_format":      "svg",
		"output_compression": float64(80),
		"complexity":         "medium",
		"frame_count":        float64(6),
		"frame_duration_ms":  float64(200),
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %v, want %v", key, got[key], value)
		}
	}
	if svg, ok := got["svg"].(map[string]any); !ok || svg["model"] != "gpt-5.6" {
		t.Errorf("svg must be forwarded as an object, got %v", got["svg"])
	}
	// Streaming is handled by the relay degradation, response_format is applied locally, and
	// client-controlled extras and end-user identifiers never reach the ChatGPT backend.
	for _, key := range []string{"stream", "partial_images", "response_format", "style", "user", "safety_identifier", "aspect_ratio", "extra_params"} {
		if _, ok := got[key]; ok {
			t.Errorf("%s must not be forwarded, body: %s", key, body)
		}
	}
}

func TestBuildDirectImageGenerationsBodyOmitsUnsetOptions(t *testing.T) {
	body, err := buildDirectImageGenerationsBody(&types.ImageRequest{
		Model:      "gpt-image-2",
		Prompt:     "a cat",
		N:          1,
		Background: strPtr("  "),
	})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeBody(t, body)
	if len(got) != 2 || got["model"] != "gpt-image-2" || got["prompt"] != "a cat" {
		t.Fatalf("only model and prompt expected, got %s", body)
	}
}

func newEditRequest(t *testing.T, fields map[string]string, files map[string][]byte) *types.ImageEditRequest {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for name, data := range files {
		header := make(map[string][]string)
		header["Content-Disposition"] = []string{`form-data; name="` + name + `"; filename="` + name + `.png"`}
		header["Content-Type"] = []string{"image/png"}
		part, err := writer.CreatePart(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/edits", &buf)
	ctx.Request.Header.Set("Content-Type", writer.FormDataContentType())
	request := &types.ImageEditRequest{}
	if err := ctx.ShouldBind(request); err != nil {
		t.Fatalf("bind edits form: %v", err)
	}
	return request
}

func TestImageEditRequestBindsOptionFieldsFromForm(t *testing.T) {
	request := newEditRequest(t, map[string]string{
		"model":              "gpt-image-2.6",
		"prompt":             "recolor",
		"background":         "transparent",
		"complexity":         "high",
		"moderation":         "low",
		"output_compression": "70",
		"output_format":      "gif",
		"frame_count":        "4",
		"frame_duration_ms":  "120",
		"svg":                `{"model":"gpt-5.6"}`,
	}, map[string][]byte{"image": []byte("png-bytes")})

	if request.Background == nil || *request.Background != "transparent" ||
		request.Complexity == nil || *request.Complexity != "high" ||
		request.Moderation == nil || *request.Moderation != "low" ||
		request.OutputCompression == nil || *request.OutputCompression != 70 ||
		request.OutputFormat == nil || *request.OutputFormat != "gif" ||
		request.FrameCount == nil || *request.FrameCount != 4 ||
		request.FrameDurationMs == nil || *request.FrameDurationMs != 120 ||
		request.SVG == nil || *request.SVG != `{"model":"gpt-5.6"}` {
		t.Fatalf("form fields not bound: %+v", request)
	}
}

func TestImageEditRequestJSONIgnoresSVGKey(t *testing.T) {
	// "svg" in a JSON-shaped edits body must stay an ignored key. Without json:"-" it would match
	// the *string field case-insensitively and fail to decode as an object (a new 400).
	var request types.ImageEditRequest
	if err := json.Unmarshal([]byte(`{"model":"gpt-image-2","prompt":"p","svg":{"model":"x"}}`), &request); err != nil {
		t.Fatalf("JSON edits body with an svg object must decode: %v", err)
	}
	if request.SVG != nil {
		t.Fatalf("svg must not bind from JSON, got %q", *request.SVG)
	}
}

func TestBuildDirectImageEditsBody(t *testing.T) {
	request := newEditRequest(t, map[string]string{
		"model":              " gpt-image-2.6-flare ",
		"prompt":             " fix the sky ",
		"n":                  "2",
		"size":               "1024x1024",
		"quality":            "medium",
		"background":         "transparent",
		"complexity":         " low ",
		"moderation":         "auto",
		"output_compression": "60",
		"output_format":      "svg",
		"frame_count":        "5",
		"frame_duration_ms":  "100",
		"svg":                `"{\"model\":\"gpt-5.6\"}"`,
		"user":               "end-user-1",
		"response_format":    "url",
		"surprise":           "unknown multipart key",
	}, map[string][]byte{"image": []byte("image-bytes"), "mask": []byte("mask-bytes")})

	imageURLs, maskURL, errCode := collectEditImages(request)
	if errCode != nil {
		t.Fatal(errCode.Message)
	}
	body, err := buildDirectImageEditsBody(request, imageURLs, maskURL)
	if err != nil {
		t.Fatal(err)
	}
	got := decodeBody(t, body)

	want := map[string]any{
		"model":              "gpt-image-2.6-flare",
		"prompt":             "fix the sky",
		"n":                  float64(2),
		"size":               "1024x1024",
		"quality":            "medium",
		"background":         "transparent",
		"complexity":         "low",
		"moderation":         "auto",
		"output_compression": float64(60),
		"output_format":      "svg",
		"frame_count":        float64(5),
		"frame_duration_ms":  float64(100),
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("%s = %v, want %v", key, got[key], value)
		}
	}
	// The form carries svg as quoted JSON text; the upstream needs an object.
	if svg, ok := got["svg"].(map[string]any); !ok || svg["model"] != "gpt-5.6" {
		t.Errorf("svg must be unwrapped into an object, got %v", got["svg"])
	}
	images, _ := got["images"].([]any)
	if len(images) != 1 || !strings.HasPrefix(images[0].(map[string]any)["image_url"].(string), "data:image/png;base64,") {
		t.Errorf("images must be data URLs, got %v", got["images"])
	}
	mask, _ := got["mask"].(map[string]any)
	if mask == nil || !strings.HasPrefix(mask["image_url"].(string), "data:image/png;base64,") {
		t.Errorf("mask must be a data URL, got %v", got["mask"])
	}
	for _, key := range []string{"user", "response_format", "surprise", "stream"} {
		if _, ok := got[key]; ok {
			t.Errorf("%s must not be forwarded, body: %s", key, body)
		}
	}
}

func TestBuildToolFromImageEditRequestForwardsOptionsButNotComplexity(t *testing.T) {
	request := newEditRequest(t, map[string]string{
		"model":              "gpt-image-1",
		"prompt":             "recolor",
		"background":         " transparent ",
		"complexity":         "high",
		"moderation":         "low",
		"output_compression": "70",
		"output_format":      "gif",
		"frame_count":        "4",
		"frame_duration_ms":  "120",
		"svg":                `{"model":"gpt-5.6"}`,
	}, map[string][]byte{"image": []byte("png-bytes")})

	body, err := json.Marshal(buildImagesRequestBody(imageToolActionEdit, "recolor", buildToolFromImageEditRequest(request), nil, "", "gpt-5.6"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Tools []map[string]any `json:"tools"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil || len(decoded.Tools) != 1 {
		t.Fatalf("decode tools: %v (%s)", err, body)
	}
	tool := decoded.Tools[0]
	want := map[string]any{
		"background":         "transparent",
		"moderation":         "low",
		"output_compression": float64(70),
		"output_format":      "gif",
		"frame_count":        float64(4),
		"frame_duration_ms":  float64(120),
	}
	for key, value := range want {
		if tool[key] != value {
			t.Errorf("tool %s = %v, want %v", key, tool[key], value)
		}
	}
	if svg, ok := tool["svg"].(map[string]any); !ok || svg["model"] != "gpt-5.6" {
		t.Errorf("svg must be forwarded as an object, got %v", tool["svg"])
	}
	if _, ok := tool["complexity"]; ok {
		t.Errorf("complexity must not reach the Responses tool: %s", body)
	}
}

func TestNormalizeSVGRawMessage(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"empty", ``, ``},
		{"blank", `  `, ``},
		{"object", ` {"model":"m"} `, `{"model":"m"}`},
		{"empty object is kept", `{}`, `{}`},
		{"quoted object", `"{\"model\":\"m\"}"`, `{"model":"m"}`},
		{"quoted blank", `"  "`, ``},
		{"broken quote", `"abc`, ``},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := string(normalizeSVGRawMessage(json.RawMessage(c.in))); got != c.want {
				t.Fatalf("normalizeSVGRawMessage(%s) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

// directTestUpstream serves the native images endpoint and records what it was sent.
type directTestUpstream struct {
	path    string
	headers http.Header
	body    map[string]any
}

// newDirectProvider points a provider that does not override the base URL at a fake upstream, so
// the direct path is taken for gpt-image-* models.
func newDirectProvider(t *testing.T, status int, response string) (*CodexProvider, *directTestUpstream) {
	t.Helper()
	seen := &directTestUpstream{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen.path = r.URL.Path
		seen.headers = r.Header.Clone()
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &seen.body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(upstream.Close)

	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()

	channel := &model.Channel{Type: 59, Key: "token"}
	provider := CodexProviderFactory{}.Create(channel).(*CodexProvider)
	provider.Config.BaseURL = upstream.URL
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
	provider.SetContext(ctx)
	provider.SetUsage(&types.Usage{PromptTokens: 7})
	return provider, seen
}

func TestCreateImageGenerationsUsesDirectEndpoint(t *testing.T) {
	provider, seen := newDirectProvider(t, http.StatusOK,
		`{"created":1,"data":[{"b64_json":"QUJD"}],"usage":{"input_tokens":10,"output_tokens":90,"input_tokens_details":{"text_tokens":10}}}`)

	response, errWithCode := provider.CreateImageGenerations(&types.ImageRequest{
		Model:      "gpt-image-2.6",
		Prompt:     "a cat",
		N:          1,
		Complexity: strPtr("medium"),
	})
	if errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}

	if seen.path != codexDirectImagesGenerations {
		t.Fatalf("path = %q, want %q", seen.path, codexDirectImagesGenerations)
	}
	if seen.body["complexity"] != "medium" || seen.body["model"] != "gpt-image-2.6" {
		t.Fatalf("unexpected body %v", seen.body)
	}
	if got := seen.headers.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q, want application/json", got)
	}
	if got := seen.headers.Get("Authorization"); got != "Bearer token" {
		t.Fatalf("Authorization = %q", got)
	}
	if len(response.Data) != 1 || response.Data[0].B64JSON != "QUJD" {
		t.Fatalf("unexpected response %+v", response)
	}
	// Usage without total_tokens must be completed, not replaced by the estimate.
	if provider.Usage.CompletionTokens != 90 || provider.Usage.TotalTokens != 100 {
		t.Fatalf("usage = %+v, want completion 90 total 100", provider.Usage)
	}
}

func TestDirectImagesResponseFormatURLBecomesDataURL(t *testing.T) {
	provider, _ := newDirectProvider(t, http.StatusOK,
		`{"data":[{"b64_json":"QUJD"}],"output_format":"webp","usage":{"total_tokens":50,"input_tokens":5,"output_tokens":45}}`)

	response, errWithCode := provider.CreateImageGenerations(&types.ImageRequest{
		Model:          "gpt-image-2",
		Prompt:         "a cat",
		ResponseFormat: "url",
	})
	if errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	item := response.Data[0]
	if item.URL != "data:image/webp;base64,QUJD" || item.B64JSON != "" {
		t.Fatalf("response_format=url must give a data URL with the echoed MIME type, got %+v", item)
	}
}

func TestDirectImagesFallbackUsageWhenUpstreamOmitsIt(t *testing.T) {
	provider, _ := newDirectProvider(t, http.StatusOK, `{"data":[{"b64_json":"QUJD"},{"b64_json":"REVG"}]}`)

	if _, errWithCode := provider.CreateImageGenerations(&types.ImageRequest{
		Model: "gpt-image-2", Prompt: "two cats", Quality: "medium", Size: "1024x1024",
	}); errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	if provider.Usage.CompletionTokens <= 0 || provider.Usage.TotalTokens != provider.Usage.PromptTokens+provider.Usage.CompletionTokens {
		t.Fatalf("an estimate must be billed when usage is missing, got %+v", provider.Usage)
	}
}

func TestDirectImagesEmptyDataIsUpstreamError(t *testing.T) {
	provider, _ := newDirectProvider(t, http.StatusOK, `{"data":[]}`)

	_, errWithCode := provider.CreateImageGenerations(&types.ImageRequest{Model: "gpt-image-2", Prompt: "a cat"})
	if errWithCode == nil || errWithCode.StatusCode != http.StatusBadGateway || errWithCode.Code != "no_image_output" {
		t.Fatalf("an empty answer must be a retryable upstream error, got %+v", errWithCode)
	}
}

func TestDirectImagesUpstreamErrorKeepsUsageAndStatus(t *testing.T) {
	provider, _ := newDirectProvider(t, http.StatusBadRequest,
		`{"error":{"message":"Complexity 'medium' is not supported for this image model","type":"image_generation_user_error","code":"bad"}}`)

	_, errWithCode := provider.CreateImageGenerations(&types.ImageRequest{Model: "gpt-image-2", Prompt: "a cat"})
	if errWithCode == nil || errWithCode.StatusCode != http.StatusBadRequest ||
		!strings.Contains(errWithCode.Message, "not supported for this image model") {
		t.Fatalf("upstream 400 must pass through, got %+v", errWithCode)
	}
}

func TestCreateImageEditsUsesDirectEndpoint(t *testing.T) {
	provider, seen := newDirectProvider(t, http.StatusOK,
		`{"data":[{"b64_json":"QUJD"}],"usage":{"total_tokens":30,"input_tokens":10,"output_tokens":20}}`)
	request := newEditRequest(t, map[string]string{
		"model": "gpt-image-2.5", "prompt": "fix", "complexity": "low", "output_format": "png",
	}, map[string][]byte{"image": []byte("png-bytes")})

	response, errWithCode := provider.CreateImageEdits(request)
	if errWithCode != nil {
		t.Fatal(errWithCode.Message)
	}
	if seen.path != codexDirectImagesEdits {
		t.Fatalf("path = %q, want %q", seen.path, codexDirectImagesEdits)
	}
	if seen.body["complexity"] != "low" || seen.body["output_format"] != "png" {
		t.Fatalf("unexpected body %v", seen.body)
	}
	if images, _ := seen.body["images"].([]any); len(images) != 1 {
		t.Fatalf("images = %v", seen.body["images"])
	}
	if len(response.Data) != 1 || provider.Usage.TotalTokens != 30 {
		t.Fatalf("unexpected result %+v usage %+v", response, provider.Usage)
	}
}

func TestImagesUseResponsesPathForOtherModelsAndRelayBaseURLs(t *testing.T) {
	events := []string{
		`{"type":"response.completed","response":{"output":[{"type":"image_generation_call","result":"QUJD","output_format":"png"}],"usage":{"total_tokens":12,"input_tokens":2,"output_tokens":10}}}`,
	}
	cases := []struct {
		name  string
		model string
		relay bool
	}{
		{"model outside the whitelist", "gpt-image-1", false},
		{"whitelisted model through a relay base URL", "gpt-image-2", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var paths []string
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				w.Header().Set("Content-Type", "text/event-stream")
				for _, event := range events {
					_, _ = io.WriteString(w, "data: "+event+"\n\n")
				}
			}))
			t.Cleanup(upstream.Close)

			oldLogger := logger.Logger
			logger.Logger = zap.NewNop()
			t.Cleanup(func() { logger.Logger = oldLogger })
			requester.InitHttpClient()

			channel := &model.Channel{Type: 59, Key: "token"}
			if c.relay {
				baseURL := upstream.URL
				channel.BaseURL = &baseURL
			}
			provider := CodexProviderFactory{}.Create(channel).(*CodexProvider)
			if !c.relay {
				provider.Config.BaseURL = upstream.URL
			}
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil)
			provider.SetContext(ctx)
			provider.SetUsage(&types.Usage{})

			if _, errWithCode := provider.CreateImageGenerations(&types.ImageRequest{Model: c.model, Prompt: "a cat"}); errWithCode != nil {
				t.Fatal(errWithCode.Message)
			}
			if len(paths) != 1 || paths[0] != "/backend-api/codex/responses" {
				t.Fatalf("paths = %v, want the Responses endpoint", paths)
			}
		})
	}
}
