package codex

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
)

// codexDirectImageModels lists the models sent to the native images endpoints. The list is explicit
// instead of a prefix match: a future gpt-image-* model may not exist on the direct endpoint and
// would 404, while the orchestration path may still serve it; gpt-image-1 (retired upstream) stays
// on the orchestration side for the same reason. chatgpt-image-* is an alias entry whose behavior
// the upstream decides, so it is not listed either.
var codexDirectImageModels = map[string]struct{}{
	"gpt-image-1.5":                     {},
	"gpt-image-2":                       {},
	"gpt-image-2.5":                     {},
	"gpt-image-2.5-flare":               {},
	"gpt-image-2.5-sunburst":            {},
	"gpt-image-2.5-flare-2026-09-08":    {},
	"gpt-image-2.5-sunburst-2026-09-08": {},
	"gpt-image-2.6":                     {},
	"gpt-image-2.6-flare":               {},
	"gpt-image-2.6-sunburst":            {},
}

// isCodexDirectImageModel reports whether the image model is in the direct whitelist.
func isCodexDirectImageModel(model string) bool {
	_, ok := codexDirectImageModels[strings.ToLower(strings.TrimSpace(model))]
	return ok
}

// canUseDirectImages is false when the channel overrides the base URL: a relay in front of the
// Codex backend may only implement /backend-api/codex/responses, and sending it the native images
// path would turn requests that work today into 404s.
func (p *CodexProvider) canUseDirectImages() bool {
	custom := strings.TrimRight(strings.TrimSpace(p.Channel.GetBaseURL()), "/")
	return custom == "" || custom == strings.TrimRight(getConfig().BaseURL, "/")
}

// shouldUseDirectImages decides between the native images endpoint and the Responses orchestration.
func (p *CodexProvider) shouldUseDirectImages(model string) bool {
	return isCodexDirectImageModel(model) && p.canUseDirectImages()
}

// directImageOptions is the request body shared by the native generations and edits endpoints. It is
// an allow-list rather than a re-encoded ImageRequest: the endpoint is the undocumented ChatGPT
// backend, so client-controlled extras (extra_params, other providers' fields) and end-user
// identifiers (user, safety_identifier) are not forwarded, which also matches what the
// orchestration path sends. stream, partial_images and response_format are not sent either: the
// request is non-streaming (the relay degrades streams to a synthesized one), and the endpoint
// always returns b64_json, so response_format=url is applied locally.
type directImageOptions struct {
	Model             string          `json:"model"`
	Prompt            string          `json:"prompt"`
	N                 int             `json:"n,omitempty"`
	Size              string          `json:"size,omitempty"`
	Quality           string          `json:"quality,omitempty"`
	Background        *string         `json:"background,omitempty"`
	Moderation        *string         `json:"moderation,omitempty"`
	OutputFormat      *string         `json:"output_format,omitempty"`
	OutputCompression *int            `json:"output_compression,omitempty"`
	Complexity        *string         `json:"complexity,omitempty"`
	FrameCount        *int            `json:"frame_count,omitempty"`
	FrameDurationMs   *int            `json:"frame_duration_ms,omitempty"`
	SVG               json.RawMessage `json:"svg,omitempty"`
}

type directImageRef struct {
	ImageURL string `json:"image_url"`
}

type directImageEditsBody struct {
	directImageOptions
	Images []directImageRef `json:"images"`
	Mask   *directImageRef  `json:"mask,omitempty"`
}

// trimmedStringPtr trims a *string option; blank becomes nil so omitempty drops it instead of
// sending an empty enum value the upstream would reject.
func trimmedStringPtr(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func newDirectImageOptions(model, prompt string, n int, size, quality string, fields imageOptionFields, complexity *string) directImageOptions {
	options := directImageOptions{
		Model:             strings.TrimSpace(model),
		Prompt:            strings.TrimSpace(prompt),
		Size:              strings.TrimSpace(size),
		Quality:           strings.TrimSpace(quality),
		Background:        trimmedStringPtr(fields.Background),
		Moderation:        trimmedStringPtr(fields.Moderation),
		OutputFormat:      trimmedStringPtr(fields.OutputFormat),
		OutputCompression: fields.OutputCompression,
		Complexity:        trimmedStringPtr(complexity),
		FrameCount:        fields.FrameCount,
		FrameDurationMs:   fields.FrameDurationMs,
		SVG:               normalizeSVGRawMessage(fields.SVG),
	}
	if shouldPassImagesN(model, n) {
		options.N = n
	}
	return options
}

// buildDirectImageGenerationsBody serializes the native /images/generations request body.
func buildDirectImageGenerationsBody(request *types.ImageRequest) ([]byte, error) {
	return json.Marshal(newDirectImageOptions(request.Model, request.Prompt, request.N, request.Size, request.Quality, imageOptionFields{
		Background:        request.Background,
		OutputFormat:      request.OutputFormat,
		Moderation:        request.Moderation,
		OutputCompression: request.OutputCompression,
		FrameCount:        request.FrameCount,
		FrameDurationMs:   request.FrameDurationMs,
		SVG:               request.SVG,
	}, request.Complexity))
}

// buildDirectImageEditsBody serializes the native /images/edits request body. The uploaded files are
// sent as data URLs in the official JSON shape (images[].image_url), not forwarded as multipart:
// the model name may have been rewritten and the file handles have been read already. Only modeled
// fields are sent; unknown multipart keys are not forwarded.
func buildDirectImageEditsBody(request *types.ImageEditRequest, imageURLs []string, maskURL string) ([]byte, error) {
	body := directImageEditsBody{
		directImageOptions: newDirectImageOptions(request.Model, request.Prompt, request.N, request.Size, request.Quality, imageOptionFields{
			Background:        request.Background,
			OutputFormat:      request.OutputFormat,
			Moderation:        request.Moderation,
			OutputCompression: request.OutputCompression,
			FrameCount:        request.FrameCount,
			FrameDurationMs:   request.FrameDurationMs,
			SVG:               rawMessageFromFormValue(request.SVG),
		}, request.Complexity),
	}
	for _, imageURL := range imageURLs {
		body.Images = append(body.Images, directImageRef{ImageURL: imageURL})
	}
	if maskURL != "" {
		body.Mask = &directImageRef{ImageURL: maskURL}
	}
	return json.Marshal(&body)
}

// createImageGenerationsDirect posts to /backend-api/codex/images/generations (non-streaming).
func (p *CodexProvider) createImageGenerationsDirect(request *types.ImageRequest) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	body, err := buildDirectImageGenerationsBody(request)
	if err != nil {
		return nil, common.ErrorWrapperLocal(err, "build_request_failed", http.StatusBadRequest)
	}
	return p.postDirectImages(codexDirectImagesGenerations, body, directImageResultOptions{
		Model:          request.Model,
		Quality:        request.Quality,
		Size:           request.Size,
		OutputFormat:   request.OutputFormat,
		ResponseFormat: request.ResponseFormat,
	})
}

// createImageEditsDirect posts to /backend-api/codex/images/edits (non-streaming).
func (p *CodexProvider) createImageEditsDirect(request *types.ImageEditRequest) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	imageURLs, maskURL, errCode := collectEditImages(request)
	if errCode != nil {
		return nil, errCode
	}

	body, err := buildDirectImageEditsBody(request, imageURLs, maskURL)
	if err != nil {
		return nil, common.ErrorWrapperLocal(err, "build_request_failed", http.StatusBadRequest)
	}
	return p.postDirectImages(codexDirectImagesEdits, body, directImageResultOptions{
		Model:          request.Model,
		Quality:        request.Quality,
		Size:           request.Size,
		OutputFormat:   request.OutputFormat,
		ResponseFormat: request.ResponseFormat,
	})
}

// directImageResultOptions carries what the response handling needs from the request: the fallback
// usage estimate (model, quality, size) and how to present the images (response_format, and the
// requested output_format for the data URL MIME type).
type directImageResultOptions struct {
	Model          string
	Quality        string
	Size           string
	OutputFormat   *string
	ResponseFormat string
}

// postDirectImages is the transport shared by both native endpoints: build the URL and headers,
// send the JSON body, and turn the answer into a billed ImageResponse.
func (p *CodexProvider) postDirectImages(path string, body []byte, result directImageResultOptions) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	fullRequestURL := p.GetFullRequestURL(path, "")

	// Reuses the Codex credentials and client headers; only Accept changes to JSON.
	headers, err := p.getRequestHeadersInternal()
	if err != nil {
		return nil, p.handleTokenError(err)
	}
	p.applyDefaultHeaders(headers)
	headers["Accept"] = "application/json"

	req, reqErr := p.Requester.NewRequest(http.MethodPost, fullRequestURL,
		p.Requester.WithBody(body),
		p.Requester.WithHeader(headers))
	if reqErr != nil {
		return nil, common.ErrorWrapper(reqErr, "new_request_failed", http.StatusInternalServerError)
	}
	defer req.Body.Close()

	response := &openai.OpenAIProviderImageResponse{}
	_, errWithCode := p.Requester.SendRequest(req, response, false)

	// The endpoint may report only input_tokens and output_tokens, and ToOpenAIUsage copies
	// total_tokens without deriving it. Without this the fallback below would take the usage for
	// missing and overwrite the real numbers with an estimate from the image.
	if response.Usage != nil && response.Usage.TotalTokens == 0 {
		response.Usage.TotalTokens = response.Usage.InputTokens + response.Usage.OutputTokens
	}
	// Same condition as the openai channel: record usage even when a 200 body carries an error
	// next to it, so such an answer is not free.
	if response.Usage != nil && response.Usage.TotalTokens > 0 {
		*p.Usage = *response.Usage.ToOpenAIUsage()
	}
	if errWithCode != nil {
		return nil, errWithCode
	}
	if openaiErr := openai.ErrorHandle(&response.OpenAIErrorResponse); openaiErr != nil {
		return nil, &types.OpenAIErrorWithStatusCode{
			OpenAIError: *openaiErr,
			StatusCode:  http.StatusBadRequest,
		}
	}
	if len(response.Data) == 0 {
		// A 200 without images is an upstream fault: not a local error, so the relay retry and
		// circuit-breaker chain sees it (same as the orchestration path).
		return nil, common.StringErrorWrapper("upstream did not return any image output", "no_image_output", http.StatusBadGateway)
	}

	if p.Usage.TotalTokens == 0 {
		quality, size := result.Quality, result.Size
		if v := openai.RawMessageToString(response.Quality); v != "" {
			quality = v
		}
		if v := openai.RawMessageToString(response.Size); v != "" {
			size = v
		}
		perImage := openai.ImageFallbackOutputTokens(result.Model, quality, size)
		p.Usage.CompletionTokens = len(response.Data) * perImage
		p.Usage.TotalTokens = p.Usage.PromptTokens + p.Usage.CompletionTokens
	}

	if strings.EqualFold(strings.TrimSpace(result.ResponseFormat), "url") {
		outputFormat := openai.RawMessageToString(response.OutputFormat)
		if outputFormat == "" && result.OutputFormat != nil {
			outputFormat = *result.OutputFormat
		}
		toDataURLs(response.Data, outputFormat)
	}

	return &response.ImageResponse, nil
}

// toDataURLs rewrites b64_json results into data URLs, the url form the orchestration path returns
// for response_format=url (the native endpoint always answers b64_json).
func toDataURLs(items []types.ImageResponseDataInner, outputFormat string) {
	for i := range items {
		if items[i].B64JSON == "" || items[i].URL != "" {
			continue
		}
		items[i].URL = "data:" + imageMIMEType(outputFormat) + ";base64," + items[i].B64JSON
		items[i].B64JSON = ""
	}
}
