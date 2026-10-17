package common

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"
)

// Every code declared in api_error.go needs a translation in each console locale; a code without
// one would silently fall back to the English message.
func TestAPICodesHaveConsoleTranslations(t *testing.T) {
	source, err := os.ReadFile("api_error.go")
	if err != nil {
		t.Fatalf("read api_error.go: %v", err)
	}
	codes := regexp.MustCompile(`(?m)^\s+APICode\w+\s*=\s*"([a-z0-9_]+)"`).FindAllSubmatch(source, -1)
	if len(codes) == 0 {
		t.Fatal("no codes found in api_error.go")
	}
	for _, locale := range []string{"en_US", "zh_CN", "zh_HK", "ja_JP"} {
		raw, err := os.ReadFile("../web/src/i18n/locales/modeltaps/" + locale + ".json")
		if err != nil {
			t.Fatalf("read %s: %v", locale, err)
		}
		var file struct {
			APIError map[string]string `json:"apiError"`
		}
		if err := json.Unmarshal(raw, &file); err != nil {
			t.Fatalf("parse %s: %v", locale, err)
		}
		for _, match := range codes {
			if code := string(match[1]); file.APIError[code] == "" {
				t.Errorf("%s has no apiError.%s", locale, code)
			}
		}
	}
}
