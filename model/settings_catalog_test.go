package model

import (
	"os/exec"
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

// The settings reference of the operator docs is up to date with the catalog and its translations.
// It runs the generator in a process of its own: the defaults it prints are the values right
// after registration, which other tests of this package change.
func TestSettingsReferenceDocsUpToDate(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the generator")
	}
	cmd := exec.Command("go", "run", "./common/settings/gen", "-check")
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("%v: %s", err, out)
	}
}

// Every object field the effective-value resolver answers for is in the catalog.
func TestEffectiveResolversAreCatalogued(t *testing.T) {
	for _, key := range EffectiveResolvableKeys() {
		if _, ok := settings.Get(key); !ok {
			t.Errorf("resolver for %s, which is not in the catalog", key)
		}
	}
}
