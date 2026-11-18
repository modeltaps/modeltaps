package settings

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The console's copy of the catalog is up to date.
func TestGeneratedCatalogUpToDate(t *testing.T) {
	want, err := GeneratedJSON()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("..", "..", GeneratedPath()))
	if err != nil {
		t.Fatalf("read the generated catalog: %v (run go generate ./common/settings)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("web/src/generated/settings-catalog.json is out of date: run go generate ./common/settings")
	}
}

// Every entry is complete enough to be found and understood.
func TestEntriesComplete(t *testing.T) {
	for _, s := range All() {
		if s.Scope == "" || s.Group == "" || s.Card == "" || s.Type == "" || s.Label == "" || len(s.Description) < 20 {
			t.Errorf("%s: scope, group, card, type, label and a description of a sentence or more are required", s.Key)
		}
		if s.Scope != ScopeSite && s.Field == "" {
			t.Errorf("%s: an object field needs its field name", s.Key)
		}
		if s.Type == TypeEnum && len(s.Enum) == 0 {
			t.Errorf("%s: an enum needs its values", s.Key)
		}
		for _, r := range s.Related {
			if _, ok := Get(r); !ok {
				t.Errorf("%s: related key %q is not in the catalog", s.Key, r)
			}
		}
	}
}
