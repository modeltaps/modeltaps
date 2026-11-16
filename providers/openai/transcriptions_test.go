package openai

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

const diarizedBody = `{"task":"transcribe","duration":4.2,"text":"hello there. hi.",` +
	`"segments":[{"id":"seg_0","type":"transcript.text.segment","speaker":"A","start":0,"end":2.1,"text":"hello there."},` +
	`{"id":"seg_1","type":"transcript.text.segment","speaker":"B","start":2.1,"end":4.2,"text":"hi."}],` +
	`"usage":{"type":"duration","seconds":5}}`

// buildAudioForm writes a multipart body the way the OpenAI SDKs do, array fields included.
func buildAudioForm(t *testing.T, fields [][2]string) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("file", "meeting.wav")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("RIFFfake-audio"))
	for _, f := range fields {
		if err := w.WriteField(f[0], f[1]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

// newTranscriptionProvider parses body into an AudioRequest the way the relay does.
func newTranscriptionProvider(t *testing.T, upstream *httptest.Server, body []byte, contentType string) (*OpenAIProvider, *types.AudioRequest) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	baseURL := upstream.URL
	channel := &model.Channel{Type: config.ChannelTypeOpenAI, Key: "sk-test", BaseURL: &baseURL}
	p := OpenAIProviderFactory{}.Create(channel).(*OpenAIProvider)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", contentType)
	c.Set(config.GinRequestBodyKey, body)

	request := &types.AudioRequest{}
	if err := common.UnmarshalBodyReusable(c, request); err != nil {
		t.Fatalf("bind: %v", err)
	}
	request.FormValues = c.Request.MultipartForm.Value

	p.SetContext(c)
	p.SetUsage(&types.Usage{})
	return p, request
}

// A remapped model rebuilds the form; every caller field must still reach upstream.
func TestTranscriptionsRemapKeepsAllFields(t *testing.T) {
	var got *multipart.Form
	var gotFile []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("upstream parse: %v", err)
		}
		got = r.MultipartForm
		if f, _, err := r.FormFile("file"); err == nil {
			var b bytes.Buffer
			_, _ = b.ReadFrom(f)
			gotFile = b.Bytes()
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(diarizedBody))
	}))
	defer upstream.Close()

	body, contentType := buildAudioForm(t, [][2]string{
		{"model", "meeting-stt"},
		{"response_format", "diarized_json"},
		{"chunking_strategy", "auto"},
		{"known_speaker_names[]", "alice"},
		{"known_speaker_names[]", "bob"},
		{"known_speaker_references[]", "data:audio/wav;base64,AAA="},
		{"timestamp_granularities[]", "segment"},
		{"x_future_option", "kept"},
	})
	p, request := newTranscriptionProvider(t, upstream, body, contentType)

	if !request.WantsDiarization() {
		t.Fatal("diarized_json request should want diarization")
	}
	if !reflect.DeepEqual(request.KnownSpeakerNames, []string{"alice", "bob"}) {
		t.Fatalf("known_speaker_names[] bound as %v", request.KnownSpeakerNames)
	}

	prevApprox := config.ApproximateTokenEnabled
	config.ApproximateTokenEnabled = true
	defer func() { config.ApproximateTokenEnabled = prevApprox }()

	p.SetOriginalModel("meeting-stt")
	request.Model = "gpt-4o-transcribe-diarize"

	resp, errWithCode := p.CreateTranscriptions(request)
	if errWithCode != nil {
		t.Fatalf("CreateTranscriptions: %+v", errWithCode)
	}

	want := map[string][]string{
		"model":                      {"gpt-4o-transcribe-diarize"},
		"response_format":            {"diarized_json"},
		"chunking_strategy":          {"auto"},
		"known_speaker_names[]":      {"alice", "bob"},
		"known_speaker_references[]": {"data:audio/wav;base64,AAA="},
		"timestamp_granularities[]":  {"segment"},
		"x_future_option":            {"kept"},
	}
	if !reflect.DeepEqual(got.Value, want) {
		t.Errorf("upstream fields = %v\nwant %v", got.Value, want)
	}
	if string(gotFile) != "RIFFfake-audio" {
		t.Errorf("upstream file = %q", gotFile)
	}

	// Body passes through untouched and decodes into the standard shape.
	var parsed types.AudioResponse
	if err := json.Unmarshal(resp.Body, &parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(parsed.Segments) != 2 || parsed.Segments[1].Speaker != "B" || parsed.Usage.Seconds != 5 {
		t.Errorf("parsed response = %+v", parsed)
	}

	// Tokens are counted on the transcript text, not on the raw JSON.
	if wantTokens := common.CountTokenText("hello there. hi.", request.Model); p.Usage.CompletionTokens != wantTokens {
		t.Errorf("CompletionTokens = %d, want %d", p.Usage.CompletionTokens, wantTokens)
	}
}

// Requests built in code (no FormValues) fall back to the typed fields.
func TestAudioFormFieldsTypedFallback(t *testing.T) {
	request := &types.AudioRequest{
		Model:             "m",
		ResponseFormat:    "diarized_json",
		Language:          "en",
		ChunkingStrategy:  "auto",
		KnownSpeakerNames: []string{"alice"},
	}
	got := audioFormFields(request)
	want := [][2]string{
		{"response_format", "diarized_json"},
		{"language", "en"},
		{"chunking_strategy", "auto"},
		{"known_speaker_names[]", "alice"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fields = %v, want %v", got, want)
	}
}

func TestHasJSONResponseIncludesDiarized(t *testing.T) {
	for format, want := range map[string]bool{"": true, "json": true, "verbose_json": true, "diarized_json": true, "text": false, "srt": false} {
		if got := hasJSONResponse(&types.AudioRequest{ResponseFormat: format}); got != want {
			t.Errorf("hasJSONResponse(%q) = %v, want %v", format, got, want)
		}
	}
}
