package openai

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func TestOfficialOpenAIDoesNotQueryBalance(t *testing.T) {
	ptr := func(s string) *string { return &s }
	cases := []struct {
		name    string
		channel model.Channel
		want    bool
	}{
		{"official, default URL", model.Channel{Type: config.ChannelTypeOpenAI}, false},
		{"official, explicit URL", model.Channel{Type: config.ChannelTypeOpenAI, BaseURL: ptr("https://api.openai.com/")}, false},
		{"relay", model.Channel{Type: config.ChannelTypeOpenAI, BaseURL: ptr("https://relay.example.com")}, true},
	}
	for _, tc := range cases {
		p := OpenAIProviderFactory{}.Create(&tc.channel).(*OpenAIProvider)
		if got := p.CanQueryBalance(); got != tc.want {
			t.Errorf("%s: CanQueryBalance = %v, want %v", tc.name, got, tc.want)
		}
	}
}
