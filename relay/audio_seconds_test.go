package relay

import (
	"bytes"
	"encoding/binary"
	"mime/multipart"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/types"
)

func TestSetAudioSecondsOrder(t *testing.T) {
	gin.SetMode(gin.TestMode)
	jsonResp := func(body string) *types.AudioResponseWrapper {
		return &types.AudioResponseWrapper{Headers: map[string]string{"Content-Type": "application/json"}, Body: []byte(body)}
	}
	textResp := func(body string) *types.AudioResponseWrapper {
		return &types.AudioResponseWrapper{Headers: map[string]string{"Content-Type": "text/plain"}, Body: []byte(body)}
	}
	cases := []struct {
		name     string
		preset   float64
		estimate float64
		response *types.AudioResponseWrapper
		want     float64
	}{
		{"provider value wins", 12, 30, jsonResp(`{"duration":40}`), 12},
		{"upstream usage seconds", 0, 30, jsonResp(`{"text":"x","usage":{"type":"duration","seconds":41}}`), 41},
		{"upstream duration below probe", 0, 30, jsonResp(`{"text":"x","duration":10}`), 10},
		{"subtitle cues", 0, 30, textResp("1\n00:00:00,000 --> 00:00:42,000\nhi\n"), 42},
		{"subtitle cues end before probe", 0, 300, textResp("1\n00:00:00,000 --> 00:00:05,000\nhi\n"), 300},
		{"segments end before probe", 0, 300, jsonResp(`{"text":"x","segments":[{"end":3}]}`), 300},
		{"text shaped like a cue", 0, 600, textResp("hello --> 00:00:01,000"), 600},
		{"probed file", 0, 30, jsonResp(`{"text":"hello"}`), 30},
		{"default length", 0, 0, jsonResp(`{"text":"hello"}`), config.DefaultAudioSeconds},
	}
	for _, tc := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest("POST", "/v1/audio/transcriptions", nil)
		if tc.estimate > 0 {
			c.Set(config.GinAudioSecondsEstimateKey, tc.estimate)
		}
		usage := &types.Usage{AudioSeconds: tc.preset}
		setAudioSeconds(c, usage, tc.response)
		if usage.AudioSeconds != tc.want {
			t.Errorf("%s: AudioSeconds = %v, want %v", tc.name, usage.AudioSeconds, tc.want)
		}
	}
}

// TestProbeAudioSecondsForgedWAV uploads 90 s of 16 kHz mono PCM whose byte_rate claims
// 0xFFFFFFFF (a 0.003 s clip by that field) and a plain-text answer: the bill must be the
// real length.
func TestProbeAudioSecondsForgedWAV(t *testing.T) {
	gin.SetMode(gin.TestMode)
	data := make([]byte, 16000*2*90)
	var wav bytes.Buffer
	wav.WriteString("RIFF")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(36+len(data)))
	wav.WriteString("WAVEfmt ")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(16))
	_ = binary.Write(&wav, binary.LittleEndian, []uint16{1, 1})
	_ = binary.Write(&wav, binary.LittleEndian, []uint32{16000, 0xFFFFFFFF})
	_ = binary.Write(&wav, binary.LittleEndian, []uint16{2, 16})
	wav.WriteString("data")
	_ = binary.Write(&wav, binary.LittleEndian, uint32(len(data)))
	wav.Write(data)

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, _ := w.CreateFormFile("file", "a.wav")
	_, _ = part.Write(wav.Bytes())
	_ = w.Close()
	req := httptest.NewRequest("POST", "/v1/audio/transcriptions", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if err := req.ParseMultipartForm(1 << 20); err != nil {
		t.Fatal(err)
	}
	defer req.MultipartForm.RemoveAll()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	request := &types.AudioRequest{File: req.MultipartForm.File["file"][0]}
	if got := probeAudioSeconds(c, request); got < 89.9 || got > 90.1 {
		t.Fatalf("probeAudioSeconds = %v, want 90", got)
	}
	usage := &types.Usage{}
	setAudioSeconds(c, usage, &types.AudioResponseWrapper{Headers: map[string]string{"Content-Type": "text/plain"}, Body: []byte("hello")})
	if usage.AudioSeconds < 89.9 || usage.AudioSeconds > 90.1 {
		t.Errorf("AudioSeconds = %v, want 90", usage.AudioSeconds)
	}
}
