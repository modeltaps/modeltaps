package relay_util

import (
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// responseModelChecked reports whether the channel type sends a real model id upstream.
// Azure channels send deployment names, which the response never repeats.
func responseModelChecked(channelType int) bool {
	switch channelType {
	case config.ChannelTypeAzure, config.ChannelTypeAzureV1, config.ChannelTypeAzureDatabricks:
		return false
	}
	return true
}

// normalizeModelID lowercases a model id, drops a provider path ("vendor/", "accounts/x/models/")
// and keeps only letters and digits, so "anthropic/claude-opus-4.5" and "claude-opus-4-5" compare
// equal.
func normalizeModelID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if i := strings.LastIndex(id, "/"); i >= 0 {
		id = id[i+1:]
	}
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// responseModelMismatch reports whether the model the upstream declares in its response is
// neither the requested model nor the one sent upstream. Dated or regional variants
// ("gpt-4o-2024-08-06", "us.anthropic.claude-…-v1:0") contain the expected id and match.
func responseModelMismatch(returned string, expected ...string) bool {
	got := normalizeModelID(returned)
	if got == "" {
		return false
	}
	for _, e := range expected {
		want := normalizeModelID(e)
		if want != "" && (strings.Contains(got, want) || strings.Contains(want, got)) {
			return false
		}
	}
	return true
}
