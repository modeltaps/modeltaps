package model

import "testing"

// gormigrate stops the server at startup when two migrations share an ID, and no other test
// runs the whole list.
func TestPostMigrationIDsAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, m := range postMigrations() {
		if m.ID == "" {
			t.Fatal("a migration has no ID")
		}
		if seen[m.ID] {
			t.Fatalf("migration ID %s is used twice", m.ID)
		}
		seen[m.ID] = true
	}
	if len(seen) == 0 {
		t.Fatal("no migrations listed")
	}
}
