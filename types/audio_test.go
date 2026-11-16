package types

import "testing"

func TestAudioRequestWantsDiarization(t *testing.T) {
	cases := []struct {
		name string
		req  AudioRequest
		want bool
	}{
		{"plain json", AudioRequest{ResponseFormat: "json"}, false},
		{"verbose_json", AudioRequest{ResponseFormat: "verbose_json"}, false},
		{"diarized_json", AudioRequest{ResponseFormat: AudioResponseFormatDiarizedJSON}, true},
		{"known speaker names", AudioRequest{KnownSpeakerNames: []string{"alice"}}, true},
		{"known speaker references", AudioRequest{KnownSpeakerReferences: []string{"data:audio/wav;base64,AAA="}}, true},
	}
	for _, tc := range cases {
		if got := tc.req.WantsDiarization(); got != tc.want {
			t.Errorf("%s: WantsDiarization() = %v, want %v", tc.name, got, tc.want)
		}
	}
}
