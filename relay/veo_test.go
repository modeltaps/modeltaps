package relay

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/providers/gemini"
)

// The gateway bills a Veo request one fixed price and returns only the first sample, so the
// request sent upstream must ask for one sample of at most the covered duration (SEC-78).
func TestClampVeoParameters(t *testing.T) {
	cases := []struct {
		name         string
		body         string
		wantDuration int
	}{
		{"four long samples", `{"instances":[{"prompt":"p"}],"parameters":{"sampleCount":4,"durationSeconds":60}}`, veoMaxDurationSeconds},
		{"no parameters", `{"instances":[{"prompt":"p"}]}`, 0},
		{"shorter duration kept", `{"instances":[{"prompt":"p"}],"parameters":{"durationSeconds":4,"aspectRatio":"9:16"}}`, 4},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := &gemini.VeoVideoRequest{}
			if err := json.Unmarshal([]byte(tc.body), request); err != nil {
				t.Fatal(err)
			}
			clampVeoParameters(request)

			sent, err := json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			var upstream struct {
				Parameters struct {
					SampleCount     int    `json:"sampleCount"`
					DurationSeconds int    `json:"durationSeconds"`
					AspectRatio     string `json:"aspectRatio"`
				} `json:"parameters"`
			}
			if err := json.Unmarshal(sent, &upstream); err != nil {
				t.Fatal(err)
			}
			if upstream.Parameters.SampleCount != 1 {
				t.Fatalf("sampleCount sent upstream = %d, want 1 (%s)", upstream.Parameters.SampleCount, sent)
			}
			if upstream.Parameters.DurationSeconds != tc.wantDuration {
				t.Fatalf("durationSeconds sent upstream = %d, want %d (%s)", upstream.Parameters.DurationSeconds, tc.wantDuration, sent)
			}
			if tc.name == "shorter duration kept" && upstream.Parameters.AspectRatio != "9:16" {
				t.Fatalf("other parameters dropped: %s", sent)
			}
		})
	}
}
