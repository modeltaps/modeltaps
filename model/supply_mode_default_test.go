package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// A model that follows the default routes with the site default; a model with its own mode
// keeps it whatever the default is.
func TestSupplyModeFollowsSiteDefault(t *testing.T) {
	usePreviewTestDB(t)
	old := config.SupplyModeDefault
	t.Cleanup(func() { config.SupplyModeDefault = old })

	for _, info := range []*ModelInfo{
		{Model: "follows", Status: ModelStatusPublished, SupplyMode: SupplyModeDefault},
		{Model: "unset", Status: ModelStatusPublished},
		{Model: "own-balance", Status: ModelStatusPublished, SupplyMode: SupplyModeBalance},
		{Model: "own-failover", Status: ModelStatusPublished, SupplyMode: SupplyModeFailover},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	// A row created without a mode follows the default.
	var unset ModelInfo
	if err := DB.Where("model = ?", "unset").First(&unset).Error; err != nil || unset.SupplyMode != SupplyModeDefault {
		t.Fatalf("unset row stored %q, err %v", unset.SupplyMode, err)
	}

	cases := []struct{ site, model, mode, own string }{
		{SupplyModeBalance, "follows", SupplyModeBalance, SupplyModeDefault},
		{SupplyModeBalance, "own-failover", SupplyModeFailover, SupplyModeFailover},
		{SupplyModeFailover, "follows", SupplyModeFailover, SupplyModeDefault},
		{SupplyModeFailover, "unset", SupplyModeFailover, SupplyModeDefault},
		{SupplyModeFailover, "own-balance", SupplyModeBalance, SupplyModeBalance},
		{"nonsense", "follows", SupplyModeBalance, SupplyModeDefault},
	}
	for _, c := range cases {
		config.SupplyModeDefault = c.site
		if got := supplyModeOf(c.model); got != c.mode {
			t.Errorf("site %s, %s: routes with %s, want %s", c.site, c.model, got, c.mode)
		}
		p, err := PreviewModelSupply(c.model, "")
		if err != nil {
			t.Fatal(err)
		}
		if p.Mode != c.mode || p.OwnMode != c.own || p.DefaultMode != EffectiveSupplyMode("") {
			t.Errorf("site %s, %s: preview mode %s own %s default %s", c.site, c.model, p.Mode, p.OwnMode, p.DefaultMode)
		}
	}
	if !IsValidSupplyMode(SupplyModeDefault) || IsValidSupplyMode("random") {
		t.Fatal("IsValidSupplyMode accepts the wrong values")
	}
}

// The one-time migration moves models off the old column default and keeps failover.
func TestMigrateSupplyModeFollowDefault(t *testing.T) {
	usePreviewTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "a", SupplyMode: SupplyModeBalance},
		{Model: "b", SupplyMode: SupplyModeFailover},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := migrateSupplyModeFollowDefault().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	modes := map[string]string{}
	var infos []*ModelInfo
	DB.Find(&infos)
	for _, info := range infos {
		modes[info.Model] = info.SupplyMode
	}
	if modes["a"] != SupplyModeDefault || modes["b"] != SupplyModeFailover {
		t.Fatalf("modes after migration = %v", modes)
	}
}
