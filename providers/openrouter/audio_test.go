package openrouter

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// newAudioTestProvider 构造指向假上游的 OpenRouter provider，BaseURL 与官方一样带 /api 前缀。
func newAudioTestProvider(t *testing.T, upstream *httptest.Server, contentType string, body []byte) *OpenRouterProvider {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	baseURL := upstream.URL + "/api"
	channel := &model.Channel{Type: config.ChannelTypeOpenRouter, Key: "sk-test", BaseURL: &baseURL}
	p := OpenRouterProviderFactory{}.Create(channel).(*OpenRouterProvider)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	c.Set(config.GinRequestBodyKey, body)
	p.SetContext(c)
	p.SetUsage(&types.Usage{})
	return p
}

func TestOpenRouterDefaultAudioURLs(t *testing.T) {
	cfg := getConfig()
	if got := cfg.BaseURL + cfg.AudioSpeech; got != "https://openrouter.ai/api/v1/audio/speech" {
		t.Errorf("speech URL = %q", got)
	}
	if got := cfg.BaseURL + cfg.AudioTranscriptions; got != "https://openrouter.ai/api/v1/audio/transcriptions" {
		t.Errorf("transcriptions URL = %q", got)
	}
	if cfg.AudioTranslations != "" {
		t.Errorf("OpenRouter has no translations endpoint, got %q", cfg.AudioTranslations)
	}
}

func TestOpenRouterCreateSpeech(t *testing.T) {
	var gotPath, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "audio/mpeg")
		_, _ = w.Write([]byte("ID3fake"))
	}))
	defer upstream.Close()

	body := []byte(`{"model":"openai/gpt-4o-mini-tts","input":"hello","voice":"alloy"}`)
	p := newAudioTestProvider(t, upstream, "application/json", body)
	p.Usage.PromptTokens = 5

	resp, errWithCode := p.CreateSpeech(&types.SpeechAudioRequest{Model: "openai/gpt-4o-mini-tts", Input: "hello", Voice: "alloy"})
	if errWithCode != nil {
		t.Fatalf("CreateSpeech error: %+v", errWithCode)
	}
	defer resp.Body.Close()
	audio, _ := io.ReadAll(resp.Body)

	if gotPath != "/api/v1/audio/speech" {
		t.Errorf("upstream path = %q, want /api/v1/audio/speech", gotPath)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("Authorization = %q", gotAuth)
	}
	if string(audio) != "ID3fake" {
		t.Errorf("audio body = %q", audio)
	}
	if p.Usage.TotalTokens != 5 {
		t.Errorf("TotalTokens = %d, want 5 (input chars)", p.Usage.TotalTokens)
	}
}

// Gemini TTS 请求 wav：向上游要 pcm，补 44 字节 WAV 头后以 audio/wav 返回；pcm 原样透传。
func TestOpenRouterCreateSpeechGeminiWAV(t *testing.T) {
	pcm := []byte{1, 2, 3, 4, 5, 6}
	var gotFormat string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var req types.SpeechAudioRequest
		_ = json.Unmarshal(raw, &req)
		gotFormat = req.ResponseFormat
		w.Header().Set("Content-Type", "audio/pcm")
		_, _ = w.Write(pcm)
	}))
	defer upstream.Close()

	const modelName = "google/gemini-3.1-flash-tts-preview"
	body := []byte(`{"model":"` + modelName + `","input":"hi","voice":"Kore","response_format":"wav"}`)
	p := newAudioTestProvider(t, upstream, "application/json", body)
	p.Usage.PromptTokens = 2

	resp, errWithCode := p.CreateSpeech(&types.SpeechAudioRequest{Model: modelName, Input: "hi", Voice: "Kore", ResponseFormat: "wav"})
	if errWithCode != nil {
		t.Fatalf("CreateSpeech error: %+v", errWithCode)
	}
	defer resp.Body.Close()
	audio, _ := io.ReadAll(resp.Body)

	if gotFormat != "pcm" {
		t.Errorf("upstream response_format = %q, want pcm", gotFormat)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "audio/wav" {
		t.Errorf("Content-Type = %q, want audio/wav", ct)
	}
	if len(audio) != 44+len(pcm) || resp.Header.Get("Content-Length") != strconv.Itoa(44+len(pcm)) {
		t.Fatalf("wav length = %d (Content-Length %q), want %d", len(audio), resp.Header.Get("Content-Length"), 44+len(pcm))
	}
	if string(audio[0:4]) != "RIFF" || string(audio[8:12]) != "WAVE" || string(audio[36:40]) != "data" {
		t.Errorf("bad wav header: %q", audio[:44])
	}
	le := binary.LittleEndian
	if got := le.Uint32(audio[4:8]); got != uint32(36+len(pcm)) {
		t.Errorf("RIFF size = %d", got)
	}
	if ch, rate, bits := le.Uint16(audio[22:24]), le.Uint32(audio[24:28]), le.Uint16(audio[34:36]); ch != 1 || rate != 24000 || bits != 16 {
		t.Errorf("fmt = %d ch / %d Hz / %d bit, want 1 / 24000 / 16", ch, rate, bits)
	}
	if got := le.Uint32(audio[40:44]); got != uint32(len(pcm)) || !bytes.Equal(audio[44:], pcm) {
		t.Errorf("data chunk size = %d, payload = %v", got, audio[44:])
	}
	if p.Usage.TotalTokens != 2 {
		t.Errorf("TotalTokens = %d, want 2", p.Usage.TotalTokens)
	}

	pcmBody := []byte(`{"model":"` + modelName + `","input":"hi","voice":"Kore","response_format":"pcm"}`)
	p = newAudioTestProvider(t, upstream, "application/json", pcmBody)
	resp, errWithCode = p.CreateSpeech(&types.SpeechAudioRequest{Model: modelName, Input: "hi", Voice: "Kore", ResponseFormat: "pcm"})
	if errWithCode != nil {
		t.Fatalf("CreateSpeech pcm error: %+v", errWithCode)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if gotFormat != "pcm" || !bytes.Equal(raw, pcm) || resp.Header.Get("Content-Type") != "audio/pcm" {
		t.Errorf("pcm passthrough: format %q, body %v, Content-Type %q", gotFormat, raw, resp.Header.Get("Content-Type"))
	}
}

// 透传渠道：Gemini wav / 未指定格式的请求在原始请求体里改为 pcm 并补 WAV 头；非 Gemini 模型 body 按字节原样发出。
func TestOpenRouterCreateSpeechPassThrough(t *testing.T) {
	pcm := []byte{1, 2, 3, 4}
	var gotBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "audio/pcm")
		_, _ = w.Write(pcm)
	}))
	defer upstream.Close()

	const modelName = "google/gemini-3.1-flash-tts-preview"
	for _, format := range []string{"wav", ""} {
		body := []byte(`{"model":"` + modelName + `","input":"hi","voice":"Kore","extra_field":1}`)
		if format != "" {
			body = []byte(`{"model":"` + modelName + `","input":"hi","voice":"Kore","response_format":"` + format + `","extra_field":1}`)
		}
		p := newAudioTestProvider(t, upstream, "application/json", body)
		p.Channel.PassThroughBody = true

		resp, errWithCode := p.CreateSpeech(&types.SpeechAudioRequest{Model: modelName, Input: "hi", Voice: "Kore", ResponseFormat: format})
		if errWithCode != nil {
			t.Fatalf("format %q: CreateSpeech error: %+v", format, errWithCode)
		}
		audio, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var sent map[string]any
		if err := json.Unmarshal(gotBody, &sent); err != nil {
			t.Fatalf("format %q: upstream body %q: %v", format, gotBody, err)
		}
		if sent["response_format"] != "pcm" || sent["extra_field"] != float64(1) {
			t.Errorf("format %q: upstream body = %s, want response_format pcm with extra_field kept", format, gotBody)
		}
		if resp.Header.Get("Content-Type") != "audio/wav" || len(audio) != 44+len(pcm) ||
			string(audio[0:4]) != "RIFF" || string(audio[8:12]) != "WAVE" || !bytes.Equal(audio[44:], pcm) {
			t.Errorf("format %q: want wav, got Content-Type %q, %d bytes", format, resp.Header.Get("Content-Type"), len(audio))
		}
	}

	otherBody := []byte(`{"model":"openai/gpt-4o-mini-tts","input":"hi","voice":"alloy","response_format":"wav","extra_field":1}`)
	p := newAudioTestProvider(t, upstream, "application/json", otherBody)
	p.Channel.PassThroughBody = true
	resp, errWithCode := p.CreateSpeech(&types.SpeechAudioRequest{Model: "openai/gpt-4o-mini-tts", Input: "hi", Voice: "alloy", ResponseFormat: "wav"})
	if errWithCode != nil {
		t.Fatalf("non-Gemini CreateSpeech error: %+v", errWithCode)
	}
	resp.Body.Close()
	if !bytes.Equal(gotBody, otherBody) {
		t.Errorf("non-Gemini upstream body = %s, want unchanged %s", gotBody, otherBody)
	}
	if resp.Header.Get("Content-Type") != "audio/pcm" {
		t.Errorf("non-Gemini Content-Type = %q, want upstream audio/pcm", resp.Header.Get("Content-Type"))
	}
}

func TestOpenRouterCreateTranscriptions(t *testing.T) {
	var gotPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello world","usage":{"seconds":1.2}}`))
	}))
	defer upstream.Close()

	body := []byte("--x\r\nContent-Disposition: form-data; name=\"model\"\r\n\r\nopenai/whisper-1\r\n--x--\r\n")
	p := newAudioTestProvider(t, upstream, "multipart/form-data; boundary=x", body)
	p.SetOriginalModel("openai/whisper-1")
	prevApprox := config.ApproximateTokenEnabled
	config.ApproximateTokenEnabled = true
	defer func() { config.ApproximateTokenEnabled = prevApprox }()

	resp, errWithCode := p.CreateTranscriptions(&types.AudioRequest{Model: "openai/whisper-1"})
	if errWithCode != nil {
		t.Fatalf("CreateTranscriptions error: %+v", errWithCode)
	}
	if gotPath != "/api/v1/audio/transcriptions" {
		t.Errorf("upstream path = %q, want /api/v1/audio/transcriptions", gotPath)
	}
	if !bytes.Contains(resp.Body, []byte("hello world")) {
		t.Errorf("response body = %q", resp.Body)
	}
	if p.Usage.CompletionTokens == 0 {
		t.Error("CompletionTokens should count transcribed text")
	}
}

// 模型同步：默认列表之外按 output_modalities=speech / transcription 追加音频模型（去重）。
func TestFetchOpenRouterModelsMergesAudio(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("output_modalities") {
		case "speech":
			_, _ = w.Write([]byte(`{"data":[{"id":"openai/gpt-4o"},{"id":"vendor/tts","architecture":{"input_modalities":["text"],"output_modalities":["speech"]}}]}`))
		case "transcription":
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom"}}`))
		default:
			_, _ = w.Write([]byte(`{"data":[{"id":"openai/gpt-4o"}]}`))
		}
	}))
	defer upstream.Close()

	p := newAudioTestProvider(t, upstream, "application/json", nil)
	resp, err := p.fetchOpenRouterModels()
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(resp.Data) != 2 || resp.Data[1].Id != "vendor/tts" {
		t.Fatalf("merged models = %+v, want [openai/gpt-4o vendor/tts]", resp.Data)
	}

	list, err := p.GetModelList()
	if err != nil {
		t.Fatalf("GetModelList: %v", err)
	}
	if len(list) != 2 || list[0] != "openai/gpt-4o" || list[1] != "vendor/tts" {
		t.Fatalf("GetModelList = %v, want [openai/gpt-4o vendor/tts]", list)
	}
}
