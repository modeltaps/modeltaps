package cheaperinference

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

func TestRequestPromptCacheModeWinsOverChannel(t *testing.T) {
	channel := channelWith(model.PluginType{"cheaperinference": {"prompt_cache": "passthrough"}})
	for mode, want := range map[string]string{"on": "on", "off": "off", "auto": "passthrough"} {
		p := CheaperInferenceProviderFactory{}.Create(channel).(*CheaperInferenceProvider)
		c := testContext(`{}`)
		c.Set(config.GinPromptCacheModeKey, mode)
		p.SetContext(c)
		if got := p.GetRequestHeaders()[headerPromptCache]; got != want {
			t.Errorf("mode %s: x-ci-prompt-cache = %q, want %q", mode, got, want)
		}
		got, sent := p.claudeProvider().GetRequestHeaders()[headerPromptCache]
		if mode == "off" && got != "off" {
			t.Errorf("messages with mode off: x-ci-prompt-cache = %q, want off", got)
		}
		if mode != "off" && sent {
			t.Errorf("messages with mode %s sent x-ci-prompt-cache = %q", mode, got)
		}
	}
}
