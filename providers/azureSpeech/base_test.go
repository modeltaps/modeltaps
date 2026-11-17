package azureSpeech

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

func speechProvider(other string, baseURL *string) *AzureSpeechProvider {
	return AzureSpeechProviderFactory{}.Create(&model.Channel{Id: 1, Key: "k", Other: other, BaseURL: baseURL}).(*AzureSpeechProvider)
}

// SEC-54: the region in Other is part of the host, so only a plain label may build a URL; the
// subscription key goes with every request.
func TestRegionMustBeHostLabel(t *testing.T) {
	if got := speechProvider("eastus", nil).GetFullRequestURL("/cognitiveservices/v1"); got != "https://eastus.tts.speech.microsoft.com/cognitiveservices/v1" {
		t.Fatalf("region url = %q", got)
	}
	base := "https://speech.example.com/"
	if got := speechProvider("", &base).GetFullRequestURL("/cognitiveservices/v1"); got != "https://speech.example.com/cognitiveservices/v1" {
		t.Fatalf("base url = %q", got)
	}
	for _, other := range []string{"attacker.tld/x?", "evil.com#", "a@b", "EastUS"} {
		if got := speechProvider(other, nil).GetFullRequestURL("/cognitiveservices/v1"); got != "" {
			t.Errorf("other %q built %q, want rejection", other, got)
		}
	}
}
