package utils

import (
	"strings"
	"testing"
)

func TestMaskSensitiveInfoKeepsPublicDocLinks(t *testing.T) {
	cases := []struct {
		in     string
		keep   []string
		hidden []string
	}{
		{
			in:   "You exceeded your current quota, see https://platform.openai.com/docs/guides/error-codes for details.",
			keep: []string{"https://platform.openai.com/docs/guides/error-codes"},
		},
		{
			in:   "Read docs.anthropic.com/en/api/errors before retrying",
			keep: []string{"docs.anthropic.com/en/api/errors"},
		},
		{
			in:     "upstream https://relay.example.com/v1/chat failed via proxy 10.0.0.5",
			hidden: []string{"relay.example.com", "10.0.0.5", "/v1/chat"},
		},
		{
			// a query string may carry a key even on a doc host
			in:     "see https://platform.openai.com/x?api_key=sk-secret",
			hidden: []string{"sk-secret"},
		},
		{
			in:     `{"api_key": "sk-live-123"} rejected by api.openai.com`,
			hidden: []string{"sk-live-123", "api.openai.com"},
		},
	}
	for _, tc := range cases {
		out := MaskSensitiveInfo(tc.in)
		for _, k := range tc.keep {
			if !strings.Contains(out, k) {
				t.Errorf("%q: lost %q -> %q", tc.in, k, out)
			}
		}
		for _, h := range tc.hidden {
			if strings.Contains(out, h) {
				t.Errorf("%q: kept %q -> %q", tc.in, h, out)
			}
		}
	}
}
