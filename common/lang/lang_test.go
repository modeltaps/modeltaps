package lang

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestNormalizeAndNegotiate(t *testing.T) {
	for tag, want := range map[string]string{
		"zh-Hant-TW": TraditionalChinese, "zh-HK": TraditionalChinese, "zh": SimplifiedChinese, "zh_CN": SimplifiedChinese,
		"ja": Japanese, "en-GB": English, "xx": "", "": "",
	} {
		if got := Normalize(tag); got != want {
			t.Errorf("Normalize(%q) = %q, want %q", tag, got, want)
		}
	}
	if got := Negotiate("xx;q=1, ja;q=0.8, en;q=0.5"); got != Japanese {
		t.Errorf("Negotiate = %q", got)
	}
	if got := Negotiate("xx, yy"); got != "" {
		t.Errorf("Negotiate unsupported = %q", got)
	}
}

func TestPickFallsBackToSiteDefault(t *testing.T) {
	old := config.DefaultLanguage
	t.Cleanup(func() { config.DefaultLanguage = old })

	config.DefaultLanguage = ""
	if got := OfRequest("", "xx"); got != English {
		t.Errorf("no default: %q", got)
	}
	config.DefaultLanguage = SimplifiedChinese
	if got := OfRequest("", "xx"); got != SimplifiedChinese {
		t.Errorf("site default: %q", got)
	}
	if got := OfRequest("ja_JP", "zh-CN"); got != Japanese {
		t.Errorf("cookie wins: %q", got)
	}
	if got := Pick("", "zh-TW"); got != TraditionalChinese {
		t.Errorf("pick: %q", got)
	}
}

func TestChainAndTags(t *testing.T) {
	if got := Chain(TraditionalChinese); len(got) != 3 || got[0] != TraditionalChinese || got[1] != SimplifiedChinese || got[2] != English {
		t.Errorf("Chain(zh_HK) = %v", got)
	}
	if got := Chain(English); len(got) != 1 {
		t.Errorf("Chain(en_US) = %v", got)
	}
	if got := Chain("xx_XX"); len(got) != 1 || got[0] != English {
		t.Errorf("Chain(unknown) = %v", got)
	}
	if BCP47(TraditionalChinese) != "zh-HK" || FromBCP47("ja") != Japanese || FromBCP47("xx") != "" {
		t.Error("BCP47 mapping")
	}
	if Normalize("zh-Hans-SG") != SimplifiedChinese || Normalize("zh-Hant-MO") != TraditionalChinese {
		t.Error("script subtags")
	}
}
