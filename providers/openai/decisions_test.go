package openai

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// Only OpenAI's own channel serves the Decisions API; providers built on CreateOpenAIProvider
// (DeepSeek, custom OpenAI-compatible endpoints, ...) get no path, so a call answers 501.
func TestDecisionsPathOnlyForOpenAI(t *testing.T) {
	openai := OpenAIProviderFactory{}.Create(&model.Channel{Type: config.ChannelTypeOpenAI}).(*OpenAIProvider)
	if openai.Config.Decisions != "/v1/decisions" {
		t.Errorf("OpenAI decisions path = %q", openai.Config.Decisions)
	}
	other := CreateOpenAIProvider(&model.Channel{Type: config.ChannelTypeCustom}, "https://example.com")
	if other.Config.Decisions != "" {
		t.Errorf("compatible provider decisions path = %q, want none", other.Config.Decisions)
	}
	if _, err := other.CreateDecision([]byte(`{"model":"x"}`), "x"); err == nil || err.StatusCode != 501 {
		t.Errorf("compatible provider CreateDecision err = %v", err)
	}
}
