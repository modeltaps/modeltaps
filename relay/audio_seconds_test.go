package relay

import (
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
	cases := []struct {
		name     string
		preset   float64
		estimate float64
		response *types.AudioResponseWrapper
		want     float64
	}{
		{"provider value wins", 12, 30, jsonResp(`{"duration":40}`), 12},
		{"upstream usage seconds", 0, 30, jsonResp(`{"text":"x","usage":{"type":"duration","seconds":41}}`), 41},
		{"subtitle cues", 0, 30, &types.AudioResponseWrapper{Headers: map[string]string{"Content-Type": "text/plain"}, Body: []byte("1\n00:00:00,000 --> 00:00:42,000\nhi\n")}, 42},
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
