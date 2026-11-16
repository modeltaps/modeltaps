package vercel

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

const audioBytes = "RIFF\x00\x00\x00\x00WAVEfake"

// newSTTProvider builds a provider against upstream and an AudioRequest parsed from a form
// the way the relay parses it.
func newSTTProvider(t *testing.T, upstream *httptest.Server, fields [][2]string) (*VercelProvider, *types.AudioRequest) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, _ := w.CreateFormFile("file", "meeting.bin")
	_, _ = fw.Write([]byte(audioBytes))
	for _, f := range fields {
		_ = w.WriteField(f[0], f[1])
	}
	_ = w.Close()

	baseURL := upstream.URL
	p := VercelProviderFactory{}.Create(&model.Channel{Type: config.ChannelTypeVercelAIGateway, Key: "vk-test", BaseURL: &baseURL}).(*VercelProvider)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", bytes.NewReader(buf.Bytes()))
	c.Request.Header.Set("Content-Type", w.FormDataContentType())
	c.Set(config.GinRequestBodyKey, buf.Bytes())
	request := &types.AudioRequest{}
	if err := common.UnmarshalBodyReusable(c, request); err != nil {
		t.Fatal(err)
	}
	request.FormValues = c.Request.MultipartForm.Value
	p.SetContext(c)
	p.SetUsage(&types.Usage{})
	return p, request
}

func TestTranscriptionRequestToGateway(t *testing.T) {
	var gotPath string
	var gotHeaders http.Header
	var gotBody transcriptionRequest
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotHeaders = r.URL.Path, r.Header
		data, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(data, &gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"hello","segments":[],"language":"en","durationInSeconds":75.5,"warnings":[]}`))
	}))
	defer upstream.Close()

	prev := config.ApproximateTokenEnabled
	config.ApproximateTokenEnabled = true
	defer func() { config.ApproximateTokenEnabled = prev }()

	p, request := newSTTProvider(t, upstream, [][2]string{
		{"model", "microsoft/mai-transcribe-2"},
		{"response_format", "diarized_json"},
		{"language", "en"},
		{"provider_options", `{"azure":{"phraseList":{"phrases":["Modeltaps"]}}}`},
	})
	resp, errWithCode := p.CreateTranscriptions(request)
	if errWithCode != nil {
		t.Fatalf("CreateTranscriptions: %+v", errWithCode)
	}

	if gotPath != transcriptionPath {
		t.Errorf("path = %q", gotPath)
	}
	for k, want := range map[string]string{
		"Authorization": "Bearer vk-test", "Content-Type": "application/json", "Ai-Model-Id": "microsoft/mai-transcribe-2",
		"Ai-Gateway-Protocol-Version": "0.0.1", "Ai-Transcription-Model-Specification-Version": "4",
	} {
		if got := gotHeaders.Get(k); got != want {
			t.Errorf("header %s = %q, want %q", k, got, want)
		}
	}
	if raw, _ := base64.StdEncoding.DecodeString(gotBody.Audio); string(raw) != audioBytes || gotBody.MediaType != "audio/wav" {
		t.Errorf("audio = %q (%s)", raw, gotBody.MediaType)
	}
	azure, _ := json.Marshal(gotBody.ProviderOptions["azure"])
	if want := `{"diarization":{"enabled":true},"locales":["en"],"phraseList":{"phrases":["Modeltaps"]},"timestamps":"segment"}`; string(azure) != want {
		t.Errorf("azure options = %s\nwant %s", azure, want)
	}
	if p.Usage.AudioSeconds != 75.5 {
		t.Errorf("AudioSeconds = %v, want 75.5", p.Usage.AudioSeconds)
	}
	if !strings.Contains(string(resp.Body), `"usage":{"type":"duration","seconds":75.5}`) {
		t.Errorf("body = %s", resp.Body)
	}
}

func TestTranscriptionProviderOptions(t *testing.T) {
	cases := []struct {
		model, format string
		names         []string
		want          string
	}{
		{"mistral/voxtral-mini-transcribe-3", "diarized_json", nil, `{"mistral":{"diarize":true,"timestampGranularities":["segment"]}}`},
		{"mistral/voxtral-mini-transcribe-3", "json", nil, `null`},
		{"elevenlabs/scribe-v2", "diarized_json", []string{"a", "b"}, `{"elevenlabs":{"diarize":true,"numSpeakers":2}}`},
		{"openai/whisper-1", "verbose_json", nil, `null`},
		{"microsoft/mai-transcribe-2", "srt", nil, `{"azure":{"timestamps":"segment"}}`},
	}
	for _, tc := range cases {
		got, errWithCode := transcriptionProviderOptions(&types.AudioRequest{Model: tc.model, ResponseFormat: tc.format, KnownSpeakerNames: tc.names})
		if errWithCode != nil {
			t.Errorf("%s: %+v", tc.model, errWithCode)
			continue
		}
		if b, _ := json.Marshal(got); string(b) != tc.want {
			t.Errorf("%s %s: options = %s, want %s", tc.model, tc.format, b, tc.want)
		}
	}
	for _, m := range []string{"openai/whisper-1", "google/gemini-3.5-transcribe"} {
		if _, errWithCode := transcriptionProviderOptions(&types.AudioRequest{Model: m, ResponseFormat: "diarized_json"}); errWithCode == nil || errWithCode.StatusCode != 400 {
			t.Errorf("%s: diarization should be rejected with 400, got %+v", m, errWithCode)
		}
	}
}

func decode(t *testing.T, body []byte) types.AudioResponse {
	t.Helper()
	var r types.AudioResponse
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return r
}

func TestRenderDiarizedFromMetadata(t *testing.T) {
	cases := map[string]string{
		// Shapes as the gateway returned them for mai-transcribe-2, voxtral-mini-transcribe-3 and scribe-v2.
		"azure": `{"azure":{"phrases":[{"speaker":1,"offsetMilliseconds":0,"durationMilliseconds":2500,"text":"Hi."},
			{"speaker":2,"offsetMilliseconds":2500,"durationMilliseconds":1500,"text":"Hello."},
			{"speaker":1,"offsetMilliseconds":4000,"durationMilliseconds":1000,"text":"Bye."}]}}`,
		"mistral": `{"mistral":{"segments":[{"text":" Hi.","startSecond":0,"endSecond":2.5,"type":"transcription_segment","speakerId":"speaker_1"},
			{"text":" Hello.","startSecond":2.5,"endSecond":4,"type":"transcription_segment","speakerId":"speaker_2"},
			{"text":" Bye.","startSecond":4,"endSecond":5,"type":"transcription_segment","speakerId":"speaker_1"}]}}`,
		"elevenlabs": `{"elevenlabs":{"words":[{"text":"Hi.","type":"word","start":0,"end":2.5,"speaker_id":"speaker_0"},
			{"text":" ","type":"spacing","start":2.5,"end":2.5,"speaker_id":"speaker_0"},
			{"text":"Hello.","type":"word","start":2.5,"end":4,"speaker_id":"speaker_1"},
			{"text":" ","type":"spacing","start":4,"end":4,"speaker_id":"speaker_1"},
			{"text":"Bye.","type":"word","start":4,"end":5,"speaker_id":"speaker_0"}]}}`,
	}
	for name, meta := range cases {
		result := &transcriptionResponse{Text: "Hi. Hello. Bye.", DurationInSeconds: 5}
		_ = json.Unmarshal([]byte(meta), &result.ProviderMetadata)
		wrapper, errWithCode := renderTranscription(&types.AudioRequest{ResponseFormat: "diarized_json"}, result)
		if errWithCode != nil {
			t.Fatal(errWithCode)
		}
		r := decode(t, wrapper.Body)
		var got []string
		for _, s := range r.Segments {
			got = append(got, s.ID.(string)+"|"+s.Speaker+"|"+s.Text)
		}
		want := "seg_0|A|Hi.,seg_1|B|Hello.,seg_2|A|Bye."
		if strings.Join(got, ",") != want {
			t.Errorf("%s: segments = %v, want %s", name, got, want)
		}
		if r.Segments[1].Start != 2.5 || r.Segments[1].End != 4 {
			t.Errorf("%s: segment 1 timing = %v-%v", name, r.Segments[1].Start, r.Segments[1].End)
		}
	}
}

func TestRenderFormats(t *testing.T) {
	result := &transcriptionResponse{Text: "one two", Language: "en", DurationInSeconds: 4}
	result.Segments = append(result.Segments,
		struct {
			Text        string  `json:"text"`
			StartSecond float64 `json:"startSecond"`
			EndSecond   float64 `json:"endSecond"`
		}{"one", 0, 1.5},
		struct {
			Text        string  `json:"text"`
			StartSecond float64 `json:"startSecond"`
			EndSecond   float64 `json:"endSecond"`
		}{"two", 1.5, 3725.25})

	render := func(format string) string {
		w, errWithCode := renderTranscription(&types.AudioRequest{ResponseFormat: format}, result)
		if errWithCode != nil {
			t.Fatal(errWithCode)
		}
		return string(w.Body)
	}
	if got := render("text"); got != "one two" {
		t.Errorf("text = %q", got)
	}
	if got := render(""); got != `{"text":"one two","usage":{"type":"duration","seconds":4}}` {
		t.Errorf("json = %s", got)
	}
	if r := decode(t, []byte(render("verbose_json"))); r.Language != "en" || len(r.Segments) != 2 || r.Segments[1].ID != float64(1) {
		t.Errorf("verbose_json = %+v", r)
	}
	if got := render("srt"); got != "1\n00:00:00,000 --> 00:00:01,500\none\n\n2\n00:00:01,500 --> 01:02:05,250\ntwo\n\n" {
		t.Errorf("srt = %q", got)
	}
	if got := render("vtt"); !strings.HasPrefix(got, "WEBVTT\n\n00:00:00.000 --> 00:00:01.500\none\n") {
		t.Errorf("vtt = %q", got)
	}
	// Diarization requested but the upstream sent no speaker data: timing kept, no speakers.
	if r := decode(t, []byte(render("diarized_json"))); len(r.Segments) != 2 || r.Segments[0].Speaker != "" || r.Segments[0].ID != "seg_0" {
		t.Errorf("diarized without metadata = %+v", r)
	}
}

func TestAudioMediaType(t *testing.T) {
	mk := func(name, ct string) *multipart.FileHeader {
		fh := &multipart.FileHeader{Filename: name, Header: map[string][]string{}}
		if ct != "" {
			fh.Header.Set("Content-Type", ct)
		}
		return fh
	}
	cases := []struct {
		fh   *multipart.FileHeader
		data string
		want string
	}{
		{mk("a.bin", "audio/flac"), "", "audio/flac"},
		{mk("talk.M4A", "application/octet-stream"), "", "audio/mp4"},
		{mk("blob", ""), "OggS....", "audio/ogg"},
		{mk("blob", ""), "\xff\xfb\x90\x00", "audio/mpeg"},
	}
	for _, tc := range cases {
		if got := audioMediaType(tc.fh, []byte(tc.data)); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.fh.Filename, got, tc.want)
		}
	}
}

func TestBuildPriceSyncTranscription(t *testing.T) {
	const fx = `{"data":[
 {"id":"openai/whisper-1","type":"transcription","pricing":{"input":"0.0000000001","transcription_duration_cost_per_second":"0.0001"}},
 {"id":"openai/gpt-4o-transcribe","type":"transcription","pricing":{"input":"0.0000025","output":"0.00001"}},
 {"id":"openai/gpt-live-transcribe-realtime","type":"transcription","pricing":{}}
]}`
	var resp modelListResponse
	if err := json.Unmarshal([]byte(fx), &resp); err != nil {
		t.Fatal(err)
	}
	prices, unconvertible := buildPriceSync(&resp)
	// $0.0001 / s = $0.006 / min = 3 rate per minute, not the nominal token price.
	if len(prices) != 1 || prices[0].Model != "openai/whisper-1" || prices[0].Type != model.MinutesPriceType || prices[0].Input != 3 {
		t.Fatalf("prices = %+v", prices)
	}
	reasons := map[string]string{}
	for _, u := range unconvertible {
		reasons[u.Model] = u.Reason
	}
	if reasons["openai/gpt-4o-transcribe"] != model.UnconvertibleAudioTokenPricing || reasons["openai/gpt-live-transcribe-realtime"] != model.UnconvertibleNoPrice {
		t.Errorf("unconvertible = %v", reasons)
	}

	infos := buildModelInfos(&resp)
	if infos[0].Endpoints != `["audio.transcription"]` {
		t.Errorf("endpoints = %s", infos[0].Endpoints)
	}
}
