package model

import (
	"sort"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/settings"
)

// Every registered option is in the settings catalog (or listed as internal), and every site
// entry of the catalog is a registered option.
func TestSettingsCatalogCoversOptions(t *testing.T) {
	registerOptions()
	registered := config.GlobalOption.Keys()
	var missing []string
	for key := range registered {
		if settings.IsInternal(key) {
			continue
		}
		if s, ok := settings.Get(key); !ok || s.Scope != settings.ScopeSite {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Errorf("options without a site catalog entry (common/settings): %v", missing)
	}
	for _, s := range settings.All() {
		if s.Scope != settings.ScopeSite {
			continue
		}
		scope, ok := registered[s.Key]
		if !ok {
			t.Errorf("catalog entry %s is not a registered option", s.Key)
			continue
		}
		if (scope == config.ScopeSecret) != (s.Type == settings.TypeSecret) {
			t.Errorf("%s: a secret option must have type secret, and only a secret option", s.Key)
		}
	}
}
