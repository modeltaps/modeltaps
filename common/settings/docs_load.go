package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
)

// docsLanguageDirs maps the console languages to their directory under docs/ops.
var docsLanguageDirs = []struct{ code, dir string }{
	{"en_US", "en"}, {"zh_CN", ""}, {"zh_HK", "hk"}, {"ja_JP", "ja"},
}

// LoadDocsLanguages reads the translations the reference needs from the console's locale files
// under root (the repository root): the catalog translations and the names of the settings page
// groups, cards and object kinds.
func LoadDocsLanguages(root string) ([]DocsLanguage, error) {
	locales := filepath.Join(root, "web", "src", "i18n", "locales")
	var out []DocsLanguage
	for _, ld := range docsLanguageDirs {
		l := DocsLanguage{Code: ld.code, Dir: ld.dir}
		var own map[string]any
		if err := readJSON(filepath.Join(locales, "modeltaps", ld.code+".json"), &own); err != nil {
			return nil, err
		}
		l.GroupTitles = titles(own, "settingsUi", "platform", "groups", "label")
		l.CardTitles = titles(own, "settingsUi", "platform", "cards", "title")
		l.ObjectTitles = stringValues(dig(own, "settingsUi", "catalog", "objects"))
		if ld.code != "en_US" {
			files, _ := filepath.Glob(filepath.Join(locales, "catalog", ld.code, "*.json"))
			sort.Strings(files)
			merged := map[string]any{}
			for _, f := range files {
				var doc map[string]any
				if err := readJSON(f, &doc); err != nil {
					return nil, err
				}
				if sc, ok := doc["settingsCatalog"].(map[string]any); ok {
					deepMerge(merged, sc)
				}
			}
			l.Catalog = merged
		}
		out = append(out, l)
	}
	return out, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func dig(m map[string]any, path ...string) map[string]any {
	for _, p := range path {
		next, ok := m[p].(map[string]any)
		if !ok {
			return nil
		}
		m = next
	}
	return m
}

// titles reads { id: { field: "..." } } under path into { id: "..." }.
func titles(m map[string]any, pathAndField ...string) map[string]string {
	field := pathAndField[len(pathAndField)-1]
	out := map[string]string{}
	for id, v := range dig(m, pathAndField[:len(pathAndField)-1]...) {
		if node, ok := v.(map[string]any); ok {
			if s, ok := node[field].(string); ok {
				out[id] = s
			}
		}
	}
	return out
}

func stringValues(m map[string]any) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if s, ok := v.(string); ok {
			out[k] = s
		}
	}
	return out
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if sm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				deepMerge(dm, sm)
				continue
			}
		}
		dst[k] = v
	}
}
