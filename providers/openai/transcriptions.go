package openai

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/types"
	"io"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func (p *OpenAIProvider) CreateTranscriptions(request *types.AudioRequest) (*types.AudioResponseWrapper, *types.OpenAIErrorWithStatusCode) {
	req, errWithCode := p.getRequestAudioBody(config.RelayModeAudioTranscription, request.Model, request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	var textResponse string
	var resp *http.Response
	var err error
	audioResponseWrapper := &types.AudioResponseWrapper{}
	if hasJSONResponse(request) {
		openAIProviderTranscriptionsResponse := &OpenAIProviderTranscriptionsResponse{}
		resp, errWithCode = p.Requester.SendRequest(req, openAIProviderTranscriptionsResponse, true)
		if errWithCode != nil {
			return nil, errWithCode
		}
		textResponse = openAIProviderTranscriptionsResponse.Text
	} else {
		openAIProviderTranscriptionsTextResponse := new(OpenAIProviderTranscriptionsTextResponse)
		resp, errWithCode = p.Requester.SendRequest(req, openAIProviderTranscriptionsTextResponse, true)
		if errWithCode != nil {
			return nil, errWithCode
		}
		textResponse = getTextContent(*openAIProviderTranscriptionsTextResponse.GetString(), request.ResponseFormat)
	}

	defer resp.Body.Close()

	audioResponseWrapper.Headers = map[string]string{
		"Content-Type": resp.Header.Get("Content-Type"),
	}

	audioResponseWrapper.Body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}

	completionTokens := common.CountTokenText(textResponse, request.Model)

	p.Usage.CompletionTokens = completionTokens
	p.Usage.TotalTokens = p.Usage.PromptTokens + p.Usage.CompletionTokens

	return audioResponseWrapper, nil
}

func hasJSONResponse(request *types.AudioRequest) bool {
	switch request.ResponseFormat {
	case "", "json", "verbose_json", types.AudioResponseFormatDiarizedJSON:
		return true
	}
	return false
}

func (p *OpenAIProvider) getRequestAudioBody(relayMode int, ModelName string, request *types.AudioRequest) (*http.Request, *types.OpenAIErrorWithStatusCode) {
	url, errWithCode := p.GetSupportedAPIUri(relayMode)
	if errWithCode != nil {
		return nil, errWithCode
	}
	// 获取请求地址
	fullRequestURL := p.GetFullRequestURL(url, ModelName)

	// 获取请求头
	headers := p.GetRequestHeaders()
	// 创建请求
	var req *http.Request
	var err error
	if p.OriginalModel != request.Model {
		var formBody bytes.Buffer
		builder := p.Requester.CreateFormBuilder(&formBody)
		if err := audioMultipartForm(request, builder); err != nil {
			return nil, common.ErrorWrapper(err, "create_form_builder_failed", http.StatusInternalServerError)
		}
		req, err = p.Requester.NewRequest(
			http.MethodPost,
			fullRequestURL,
			p.Requester.WithBody(&formBody),
			p.Requester.WithHeader(headers),
			p.Requester.WithContentType(builder.FormDataContentType()))
		req.ContentLength = int64(formBody.Len())
	} else {
		body, exists := p.GetRawBody()
		if !exists {
			return nil, common.StringErrorWrapperLocal("request body not found", "request_body_not_found", http.StatusInternalServerError)
		}

		req, err = p.Requester.NewRequest(
			http.MethodPost,
			fullRequestURL,
			p.Requester.WithBody(body),
			p.Requester.WithHeader(headers),
			p.Requester.WithContentType(p.Context.Request.Header.Get("Content-Type")))
		req.ContentLength = p.Context.Request.ContentLength
	}

	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}

	return req, nil
}

// audioMultipartForm rebuilds the upstream form when the model name was remapped. Every
// field the caller sent is forwarded as-is (only model is replaced), so options such as
// diarization, chunking_strategy or timestamp_granularities[] survive the rebuild.
func audioMultipartForm(request *types.AudioRequest, b requester.FormBuilder) error {
	if err := b.CreateFormFile("file", request.File); err != nil {
		return fmt.Errorf("creating form file: %w", err)
	}

	if err := b.WriteField("model", request.Model); err != nil {
		return fmt.Errorf("writing model name: %w", err)
	}

	for _, field := range audioFormFields(request) {
		if err := b.WriteField(field[0], field[1]); err != nil {
			return fmt.Errorf("writing %s: %w", field[0], err)
		}
	}

	return b.Close()
}

// audioFormFields lists the non-file, non-model fields to send upstream in a stable order.
// It prefers the raw incoming form; requests built in code fall back to the typed fields.
func audioFormFields(request *types.AudioRequest) [][2]string {
	var fields [][2]string
	if request.FormValues != nil {
		keys := make([]string, 0, len(request.FormValues))
		for key := range request.FormValues {
			if key != "model" && key != "file" {
				keys = append(keys, key)
			}
		}
		sort.Strings(keys)
		for _, key := range keys {
			for _, value := range request.FormValues[key] {
				fields = append(fields, [2]string{key, value})
			}
		}
		return fields
	}

	add := func(key, value string) {
		if value != "" {
			fields = append(fields, [2]string{key, value})
		}
	}
	addAll := func(key string, values []string) {
		for _, value := range values {
			fields = append(fields, [2]string{key, value})
		}
	}
	add("prompt", request.Prompt)
	add("response_format", request.ResponseFormat)
	if request.Temperature != 0 {
		add("temperature", fmt.Sprintf("%.2f", request.Temperature))
	}
	add("language", request.Language)
	add("chunking_strategy", request.ChunkingStrategy)
	addAll("timestamp_granularities[]", request.TimestampGranularities)
	addAll("include[]", request.Include)
	addAll("known_speaker_names[]", request.KnownSpeakerNames)
	addAll("known_speaker_references[]", request.KnownSpeakerReferences)
	return fields
}

func getTextContent(text, format string) string {
	switch format {
	case "srt":
		return extractTextFromSRT(text)
	case "vtt":
		return extractTextFromVTT(text)
	default:
		return text
	}
}

func extractTextFromVTT(vttContent string) string {
	scanner := bufio.NewScanner(strings.NewReader(vttContent))
	re := regexp.MustCompile(`\d{2}:\d{2}:\d{2}\.\d{3} --> \d{2}:\d{2}:\d{2}\.\d{3}`)
	var text []string
	isStart := true

	for scanner.Scan() {
		line := scanner.Text()
		if isStart && strings.HasPrefix(line, "WEBVTT") {
			isStart = false
			continue
		}
		if !re.MatchString(line) && !isNumber(line) && line != "" {
			text = append(text, line)
		}
	}

	return strings.Join(text, " ")
}

func extractTextFromSRT(srtContent string) string {
	scanner := bufio.NewScanner(strings.NewReader(srtContent))
	re := regexp.MustCompile(`\d{2}:\d{2}:\d{2},\d{3} --> \d{2}:\d{2}:\d{2},\d{3}`)
	var text []string
	isContent := false

	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			isContent = true
		} else if line == "" {
			isContent = false
		} else if isContent {
			text = append(text, line)
		}
	}

	return strings.Join(text, " ")
}

func isNumber(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
