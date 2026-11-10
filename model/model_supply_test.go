package model

import (
	"errors"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func useSupplyTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&Channel{}, &ModelInfo{}, &Price{}, &ModelSupply{}, &RoutingPolicy{}); err != nil {
		t.Fatal(err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

func TestModelSupplyCRUD(t *testing.T) {
	useSupplyTestDB(t)
	weight := uint(1)
	channel := &Channel{Name: "c", Status: config.ChannelStatusEnabled, Weight: &weight, Models: "m"}
	if err := DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	for _, info := range []*ModelInfo{
		{Model: "m", Status: ModelStatusPublished},
		{Model: "m-alias", AliasOf: "m"},
		{Model: "auto/x", Kind: ModelInfoKindRoutingPolicy},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		name   string
		supply ModelSupply
		want   error
	}{
		{"unknown model", ModelSupply{Model: "nope", ChannelId: channel.Id, UpstreamModel: "x"}, ErrSupplyModelNotFound},
		{"alias", ModelSupply{Model: "m-alias", ChannelId: channel.Id, UpstreamModel: "x"}, ErrSupplyModelNotPublic},
		{"routing policy", ModelSupply{Model: "auto/x", ChannelId: channel.Id, UpstreamModel: "x"}, ErrSupplyModelNotPublic},
		{"unknown channel", ModelSupply{Model: "m", ChannelId: 999, UpstreamModel: "x"}, ErrSupplyChannelNotFound},
		{"no upstream", ModelSupply{Model: "m", ChannelId: channel.Id, UpstreamModel: " "}, ErrSupplyUpstreamMissing},
	} {
		supply := tc.supply
		if err := CreateModelSupply(&supply); !errors.Is(err, tc.want) {
			t.Errorf("%s: err = %v, want %v", tc.name, err, tc.want)
		}
	}

	// A supply created disabled stays disabled (no column default turns it back on).
	supply := &ModelSupply{Model: "m", ChannelId: channel.Id, UpstreamModel: "vendor/m", Priority: 5, Weight: 2, Enabled: false}
	if err := CreateModelSupply(supply); err != nil {
		t.Fatal(err)
	}
	stored, err := GetModelSupply(supply.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Enabled || stored.Source != ModelSupplySourceManual || stored.UpstreamModel != "vendor/m" {
		t.Fatalf("stored = %+v", stored)
	}
	if err := CreateModelSupply(&ModelSupply{Model: "m", ChannelId: channel.Id, UpstreamModel: "other"}); !errors.Is(err, ErrSupplyExists) {
		t.Fatalf("second supply on the same channel: err = %v, want ErrSupplyExists", err)
	}

	stored.Enabled = true
	stored.Priority = 9
	stored.Model = "ignored"
	if err := UpdateModelSupply(stored); err != nil {
		t.Fatal(err)
	}
	updated, _ := GetModelSupply(supply.Id)
	if !updated.Enabled || updated.Priority != 9 || updated.Model != "m" {
		t.Fatalf("updated = %+v, want enabled, priority 9, model unchanged", updated)
	}

	if err := DeleteModelSupply(supply.Id); err != nil {
		t.Fatal(err)
	}
	if list, _ := GetModelSupplies("m", 0); len(list) != 0 {
		t.Fatalf("supplies left after delete: %+v", list)
	}
}

// Applying the migration writes the plan and a second run adds nothing.
func TestMigrateChannelsToSuppliesApply(t *testing.T) {
	useSupplyTestDB(t)
	in := migrationFixture()
	for _, ch := range in.channels {
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, info := range in.infos {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range in.prices {
		if err := DB.Create(p).Error; err != nil {
			t.Fatal(err)
		}
	}

	dry, err := MigrateChannelsToSupplies(true)
	if err != nil {
		t.Fatal(err)
	}
	if !dry.DryRun || dry.SuppliesPlanned != 7 {
		t.Fatalf("dry run planned %d supplies", dry.SuppliesPlanned)
	}
	if list, _ := GetModelSupplies("", 0); len(list) != 0 {
		t.Fatalf("dry run wrote %d supplies", len(list))
	}

	report, err := MigrateChannelsToSupplies(false)
	if err != nil {
		t.Fatal(err)
	}
	list, _ := GetModelSupplies("", 0)
	if report.DryRun || len(list) != 7 {
		t.Fatalf("applied %d supplies, want 7", len(list))
	}
	if info, _ := GetModelInfoByModel("claude-opus-5"); info.Status != ModelStatusPublished {
		t.Fatalf("claude-opus-5 status = %q, want published", info.Status)
	}
	if info, _ := GetModelInfoByModel("deepseek-chat"); info == nil || info.Status != ModelStatusPublished || info.Source != ModelInfoSourceMigrated {
		t.Fatalf("deepseek-chat row = %+v", info)
	}
	if info, _ := GetModelInfoByModel("gpt-4o-mini"); info.Status != ModelStatusDraft {
		t.Fatalf("hidden gpt-4o-mini status = %q, want draft", info.Status)
	}
	var copied Price
	if err := DB.Where("model = ?", "claude-opus-5").First(&copied).Error; err != nil || copied.Input != 5 {
		t.Fatalf("copied price = %+v, err %v", copied, err)
	}

	again, err := MigrateChannelsToSupplies(false)
	if err != nil {
		t.Fatal(err)
	}
	if again.SuppliesPlanned != 0 || len(again.NewModels) != 0 || len(again.PriceCopies) != 0 || len(again.StatusChanges) != 0 {
		t.Fatalf("second run changed things: %+v", again)
	}
}
