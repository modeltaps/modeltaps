package vercel

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"slices"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/types"
)

// transcriptionPath is the AI Gateway REST endpoint for recorded audio. It takes the audio
// base64-encoded in a JSON body and returns the AI SDK transcription result.
// https://vercel.com/docs/ai-gateway/modalities/speech-to-text#transcribe-with-the-rest-api
const transcriptionPath = "/v4/ai/transcription-model"

type transcriptionRequest struct {
	Audio           string                    `json:"audio"`
	MediaType       string                    `json:"mediaType"`
	ProviderOptions map[string]map[string]any `json:"providerOptions,omitempty"`
}

type transcriptionResponse struct {
	Text     string `json:"text"`
	Segments []struct {
		Text        string  `json:"text"`
		StartSecond float64 `json:"startSecond"`
		EndSecond   float64 `json:"endSecond"`
	} `json:"segments"`
	Language          string                     `json:"language"`
	DurationInSeconds float64                    `json:"durationInSeconds"`
	ProviderMetadata  map[string]json.RawMessage `json:"providerMetadata"`
	types.OpenAIErrorResponse
}

// CreateTranscriptions sends /v1/audio/transcriptions to the gateway's transcription endpoint
// and turns the result back into the OpenAI response formats, diarized_json included.
func (p *VercelProvider) CreateTranscriptions(request *types.AudioRequest) (*types.AudioResponseWrapper, *types.OpenAIErrorWithStatusCode) {
	if request.File == nil {
		return nil, common.StringErrorWrapperLocal("audio file is required", "invalid_request", http.StatusBadRequest)
	}
	switch request.ResponseFormat {
	case "", "json", "text", "verbose_json", "srt", "vtt", types.AudioResponseFormatDiarizedJSON:
	default:
		return nil, common.StringErrorWrapperLocal("response_format "+request.ResponseFormat+" is not supported", "unsupported_response_format", http.StatusBadRequest)
	}

	options, errWithCode := transcriptionProviderOptions(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	if len(request.KnownSpeakerReferences) > 0 {
		logger.LogWarn(p.Context.Request.Context(), "known_speaker_references is not supported by the gateway and was dropped")
	}

	audio, mediaType, err := readAudio(request.File)
	if err != nil {
		return nil, common.ErrorWrapper(err, "read_audio_failed", http.StatusBadRequest)
	}

	headers := p.GetRequestHeaders()
	// The incoming request is multipart; the gateway takes JSON.
	headers["Content-Type"] = "application/json"
	headers["ai-gateway-protocol-version"] = "0.0.1"
	headers["ai-transcription-model-specification-version"] = "4"
	headers["ai-model-id"] = request.Model

	url := p.OpenAIProvider.BaseProvider.GetFullRequestURL(transcriptionPath, request.Model)
	body := &transcriptionRequest{Audio: audio, MediaType: mediaType, ProviderOptions: options}
	req, err := p.Requester.NewRequest(http.MethodPost, url, p.Requester.WithBody(body), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}
	defer req.Body.Close()

	result := &transcriptionResponse{}
	if _, errWithCode = p.Requester.SendRequest(req, result, false); errWithCode != nil {
		return nil, errWithCode
	}
	if result.Error.Message != "" {
		return nil, &types.OpenAIErrorWithStatusCode{OpenAIError: result.Error, StatusCode: http.StatusBadGateway}
	}

	wrapper, errWithCode := renderTranscription(request, result)
	if errWithCode != nil {
		return nil, errWithCode
	}

	if usage := p.GetUsage(); usage != nil {
		usage.AudioSeconds = result.DurationInSeconds
		usage.CompletionTokens = common.CountTokenText(result.Text, request.Model)
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return wrapper, nil
}

// transcriptionProviderOptions maps the OpenAI form fields onto the option names of the model's
// upstream SDK provider. A provider_options form field (a JSON object keyed by provider, as in
// the AI SDK) is merged on top for options the OpenAI form has no field for.
func transcriptionProviderOptions(request *types.AudioRequest) (map[string]map[string]any, *types.OpenAIErrorWithStatusCode) {
	vendor, _, _ := strings.Cut(request.Model, "/")
	diarize := request.WantsDiarization()
	timed := diarize || slices.Contains([]string{"verbose_json", "srt", "vtt"}, request.ResponseFormat)
	words := slices.Contains(request.TimestampGranularities, "word")

	var namespace string
	opts := map[string]any{}
	switch vendor {
	case "openai":
		namespace = "openai"
		setIf(opts, "language", request.Language)
		setIf(opts, "prompt", request.Prompt)
		if request.Temperature != 0 {
			opts["temperature"] = request.Temperature
		}
		if len(request.TimestampGranularities) > 0 {
			opts["timestampGranularities"] = request.TimestampGranularities
		}
	case "microsoft":
		namespace = "azure"
		if request.Language != "" {
			opts["locales"] = []string{request.Language}
		}
		if diarize {
			opts["diarization"] = map[string]any{"enabled": true}
		}
		// Without timestamps MAI returns one segment spanning the whole audio.
		if words {
			opts["timestamps"] = "word"
		} else if timed {
			opts["timestamps"] = "segment"
		}
	case "mistral":
		namespace = "mistral"
		setIf(opts, "language", request.Language)
		if diarize {
			opts["diarize"] = true
		}
		if timed {
			// Voxtral 3 needs segment timestamps for offline diarization.
			opts["timestampGranularities"] = []string{"segment"}
		}
	case "elevenlabs":
		namespace = "elevenlabs"
		setIf(opts, "languageCode", request.Language)
		if diarize {
			opts["diarize"] = true
			if n := len(request.KnownSpeakerNames); n > 0 {
				opts["numSpeakers"] = n
			}
		}
	}
	if diarize && namespace != "azure" && namespace != "mistral" && namespace != "elevenlabs" {
		return nil, common.StringErrorWrapperLocal("speaker diarization is not supported by "+request.Model, "unsupported_response_format", http.StatusBadRequest)
	}

	options := map[string]map[string]any{}
	if namespace != "" && len(opts) > 0 {
		options[namespace] = opts
	}
	if raw := formValue(request, "provider_options"); raw != "" {
		var extra map[string]map[string]any
		if err := json.Unmarshal([]byte(raw), &extra); err != nil {
			return nil, common.StringErrorWrapperLocal("provider_options must be a JSON object keyed by provider", "invalid_request", http.StatusBadRequest)
		}
		for ns, values := range extra {
			if options[ns] == nil {
				options[ns] = map[string]any{}
			}
			for k, v := range values {
				options[ns][k] = v
			}
		}
	}
	if len(options) == 0 {
		return nil, nil
	}
	return options, nil
}

func setIf(m map[string]any, key, value string) {
	if value != "" {
		m[key] = value
	}
}

func formValue(request *types.AudioRequest, key string) string {
	if v := request.FormValues[key]; len(v) > 0 {
		return v[0]
	}
	return ""
}

func readAudio(fh *multipart.FileHeader) (string, string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(data), audioMediaType(fh, data), nil
}

var audioExtTypes = map[string]string{
	".mp3": "audio/mpeg", ".mpga": "audio/mpeg", ".mpeg": "audio/mpeg",
	".wav": "audio/wav", ".flac": "audio/flac", ".ogg": "audio/ogg", ".oga": "audio/ogg", ".opus": "audio/ogg",
	".m4a": "audio/mp4", ".mp4": "audio/mp4", ".aac": "audio/aac", ".webm": "audio/webm",
}

// audioMediaType takes the upload's declared audio type, then its extension, then its magic bytes.
func audioMediaType(fh *multipart.FileHeader, data []byte) string {
	if ct := fh.Header.Get("Content-Type"); strings.HasPrefix(ct, "audio/") || strings.HasPrefix(ct, "video/") {
		return ct
	}
	if t, ok := audioExtTypes[strings.ToLower(filepath.Ext(fh.Filename))]; ok {
		return t
	}
	switch {
	case bytes.HasPrefix(data, []byte("RIFF")):
		return "audio/wav"
	case bytes.HasPrefix(data, []byte("fLaC")):
		return "audio/flac"
	case bytes.HasPrefix(data, []byte("OggS")):
		return "audio/ogg"
	case len(data) > 8 && bytes.Equal(data[4:8], []byte("ftyp")):
		return "audio/mp4"
	case bytes.HasPrefix(data, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		return "audio/webm"
	}
	return "audio/mpeg"
}

// renderTranscription writes the gateway result in the requested OpenAI response format.
func renderTranscription(request *types.AudioRequest, result *transcriptionResponse) (*types.AudioResponseWrapper, *types.OpenAIErrorWithStatusCode) {
	var usage *types.AudioUsage
	if result.DurationInSeconds > 0 {
		usage = &types.AudioUsage{Type: "duration", Seconds: result.DurationInSeconds}
	}

	segments := make([]types.AudioSegment, 0, len(result.Segments))
	for _, s := range result.Segments {
		segments = append(segments, types.AudioSegment{Start: s.StartSecond, End: s.EndSecond, Text: s.Text})
	}

	var out any
	switch request.ResponseFormat {
	case "text":
		return textWrapper("text/plain; charset=utf-8", result.Text), nil
	case "srt", "vtt":
		speakerSegments := speakerSegmentsFrom(result.ProviderMetadata)
		if len(speakerSegments) > 0 {
			segments = speakerSegments
		}
		if len(segments) == 0 {
			segments = []types.AudioSegment{{Start: 0, End: result.DurationInSeconds, Text: result.Text}}
		}
		if request.ResponseFormat == "srt" {
			return textWrapper("text/plain; charset=utf-8", renderSubtitles(segments, false)), nil
		}
		return textWrapper("text/vtt; charset=utf-8", renderSubtitles(segments, true)), nil
	case "verbose_json":
		for i := range segments {
			segments[i].ID = i
		}
		out = &types.AudioResponse{Task: "transcribe", Language: result.Language, Duration: result.DurationInSeconds,
			Text: result.Text, Segments: segments, Usage: usage}
	case types.AudioResponseFormatDiarizedJSON:
		diarized := speakerSegmentsFrom(result.ProviderMetadata)
		if len(diarized) == 0 {
			// The upstream returned no speaker data; keep the timing, without speakers.
			diarized = segments
		}
		for i := range diarized {
			diarized[i].ID = fmt.Sprintf("seg_%d", i)
			diarized[i].Type = "transcript.text.segment"
		}
		out = &types.AudioResponse{Task: "transcribe", Duration: result.DurationInSeconds, Text: result.Text,
			Segments: diarized, Usage: usage}
	default:
		out = &types.AudioResponse{Text: result.Text, Usage: usage}
	}

	body, err := json.Marshal(out)
	if err != nil {
		return nil, common.ErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
	}
	return &types.AudioResponseWrapper{Headers: map[string]string{"Content-Type": "application/json"}, Body: body}, nil
}

func textWrapper(contentType, text string) *types.AudioResponseWrapper {
	return &types.AudioResponseWrapper{Headers: map[string]string{"Content-Type": contentType}, Body: []byte(text)}
}

// speakerSegmentsFrom reads speaker-labelled segments from the provider metadata the gateway
// passes through: Azure phrases, Mistral segments, or ElevenLabs words grouped into speaker
// turns. Speaker IDs become "A", "B", … in order of first appearance, as in diarized_json.
func speakerSegmentsFrom(meta map[string]json.RawMessage) []types.AudioSegment {
	type rawSegment struct {
		speaker    string
		start, end float64
		text       string
	}
	var raw []rawSegment

	if data, ok := meta["azure"]; ok {
		var azure struct {
			Phrases []struct {
				Speaker              json.RawMessage `json:"speaker"`
				OffsetMilliseconds   float64         `json:"offsetMilliseconds"`
				DurationMilliseconds float64         `json:"durationMilliseconds"`
				Text                 string          `json:"text"`
			} `json:"phrases"`
		}
		if json.Unmarshal(data, &azure) == nil {
			for _, ph := range azure.Phrases {
				start := ph.OffsetMilliseconds / 1000
				raw = append(raw, rawSegment{rawID(ph.Speaker), start, start + ph.DurationMilliseconds/1000, ph.Text})
			}
		}
	}
	if data, ok := meta["mistral"]; ok && len(raw) == 0 {
		// The AI SDK normalizes Mistral segments to camelCase (startSecond, speakerId).
		var mistral struct {
			Segments []struct {
				SpeakerID   json.RawMessage `json:"speakerId"`
				StartSecond float64         `json:"startSecond"`
				EndSecond   float64         `json:"endSecond"`
				Text        string          `json:"text"`
			} `json:"segments"`
		}
		if json.Unmarshal(data, &mistral) == nil {
			for _, s := range mistral.Segments {
				raw = append(raw, rawSegment{rawID(s.SpeakerID), s.StartSecond, s.EndSecond, strings.TrimSpace(s.Text)})
			}
		}
	}
	if data, ok := meta["elevenlabs"]; ok && len(raw) == 0 {
		var eleven struct {
			Words []struct {
				Text      string          `json:"text"`
				Type      string          `json:"type"`
				Start     float64         `json:"start"`
				End       float64         `json:"end"`
				SpeakerID json.RawMessage `json:"speaker_id"`
			} `json:"words"`
		}
		if json.Unmarshal(data, &eleven) == nil {
			for _, w := range eleven.Words {
				id := rawID(w.SpeakerID)
				if n := len(raw); n > 0 && (raw[n-1].speaker == id || w.Type == "spacing") {
					raw[n-1].text += w.Text
					if w.End > raw[n-1].end {
						raw[n-1].end = w.End
					}
					continue
				}
				raw = append(raw, rawSegment{id, w.Start, w.End, w.Text})
			}
			for i := range raw {
				raw[i].text = strings.TrimSpace(raw[i].text)
			}
		}
	}

	labels := map[string]string{}
	var out []types.AudioSegment
	hasSpeaker := false
	for _, r := range raw {
		label := ""
		if r.speaker != "" {
			hasSpeaker = true
			if labels[r.speaker] == "" {
				labels[r.speaker] = speakerLabel(len(labels))
			}
			label = labels[r.speaker]
		}
		out = append(out, types.AudioSegment{Speaker: label, Start: r.start, End: r.end, Text: r.text})
	}
	if !hasSpeaker {
		return nil
	}
	return out
}

// rawID reads a speaker ID that may be a JSON number or string.
func rawID(v json.RawMessage) string {
	s := strings.Trim(strings.TrimSpace(string(v)), `"`)
	if s == "null" {
		return ""
	}
	return s
}

func speakerLabel(i int) string {
	if i < 26 {
		return string(rune('A' + i))
	}
	return fmt.Sprintf("S%d", i+1)
}

func renderSubtitles(segments []types.AudioSegment, vtt bool) string {
	var b strings.Builder
	if vtt {
		b.WriteString("WEBVTT\n\n")
	}
	for i, s := range segments {
		text := s.Text
		if s.Speaker != "" {
			text = s.Speaker + ": " + text
		}
		if vtt {
			fmt.Fprintf(&b, "%s --> %s\n%s\n\n", subtitleTime(s.Start, "."), subtitleTime(s.End, "."), text)
		} else {
			fmt.Fprintf(&b, "%d\n%s --> %s\n%s\n\n", i+1, subtitleTime(s.Start, ","), subtitleTime(s.End, ","), text)
		}
	}
	return b.String()
}

func subtitleTime(seconds float64, sep string) string {
	ms := int64(seconds*1000 + 0.5)
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", ms/3600000, ms/60000%60, ms/1000%60, sep, ms%1000)
}
