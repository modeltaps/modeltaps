package lang

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestNormalizeAndNegotiate(t *testing.T) {
	for tag, want := range map[string]string{
		"zh-Hant-TW": TraditionalChinese, "zh-HK": TraditionalChinese, "zh": SimplifiedChinese, "zh_CN": SimplifiedChinese,
		"ja": Japanese, "en-GB": English, "fr": "", "": "",
	} {
		if got := Normalize(tag); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", tag, got, want)
		}
	}
	if got := Negotiate("fr;q=1, ja;q=0.8, en;q=0.5"); got != Japanese {
		t.Errorf("Negotiate = %q", got)
	}
	if got := Negotiate("fr, de"); got != "" {
		t.Errorf("Negotiate unsupported = %q", got)
	}
}

func TestPickFallsBackToSiteDefault(t *testing.T) {
	old := config.DefaultLanguage
	t.Cleanup(func() { config.DefaultLanguage = old })

	config.DefaultLanguage = ""
	if got := OfRequest("", "fr"); got != English {
		t.Errorf("no default: %q", got)
	}
	config.DefaultLanguage = SimplifiedChinese
	if got := OfRequest("", "fr"); got != SimplifiedChinese {
		t.Errorf("site default: %q", got)
	}
	if got := OfRequest("ja_JP", "zh-CN"); got != Japanese {
		t.Errorf("cookie wins: %q", got)
	}
	if got := Pick("", "zh-TW"); got != TraditionalChinese {
		t.Errorf("pick: %q", got)
	}
}
