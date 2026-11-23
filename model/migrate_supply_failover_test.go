package model

import (
	"testing"
)

// Failover models that keep their own numbers get one tier per weight, in the order failover
// tried them; models on the allocation policy, balance models and tiers of one weight stay.
func TestMigrateFailoverToTiers(t *testing.T) {
	usePreviewTestDB(t)
	if err := DB.AutoMigrate(&Option{}, &AllocationPolicy{}); err != nil {
		t.Fatal(err)
	}
	if err := DB.Exec("ALTER TABLE model_info ADD COLUMN supply_mode varchar(16) DEFAULT 'default'").Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Option{Key: "SupplyModeDefault", Value: "failover"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&AllocationPolicy{Scope: AllocationScopeDefault, Tiers: "[]"}).Error; err != nil {
		t.Fatal(err)
	}
	models := []struct{ name, mode, allocation string }{
		{"follows", "default", AllocationManual},
		{"own-balance", "balance", AllocationManual},
		{"on-policy", "failover", ""},
		{"one-weight", "failover", AllocationManual},
	}
	for _, m := range models {
		if err := DB.Create(&ModelInfo{Model: m.name, Status: ModelStatusPublished, Allocation: m.allocation}).Error; err != nil {
			t.Fatal(err)
		}
		if err := DB.Exec("UPDATE model_info SET supply_mode = ? WHERE model = ?", m.mode, m.name).Error; err != nil {
			t.Fatal(err)
		}
	}
	type key struct {
		model   string
		channel int
	}
	ids := map[key]int{}
	add := func(model string, channel int, priority int64, weight uint) {
		s := &ModelSupply{Model: model, ChannelId: channel, UpstreamModel: model, Priority: priority, Weight: weight, Enabled: true}
		if err := DB.Create(s).Error; err != nil {
			t.Fatal(err)
		}
		ids[key{model, channel}] = s.Id
	}
	for _, name := range []string{"follows", "own-balance", "on-policy"} {
		add(name, 1, 10, 3)
		add(name, 2, 10, 1)
		add(name, 3, 5, 1)
	}
	add("one-weight", 1, 10, 2)
	add("one-weight", 2, 10, 2)
	add("one-weight", 3, 5, 1)

	if err := migrateFailoverToTiers().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	priority := func(model string, channel int) int64 {
		var s ModelSupply
		if err := DB.First(&s, ids[key{model, channel}]).Error; err != nil {
			t.Fatal(err)
		}
		return s.Priority
	}
	if got := [3]int64{priority("follows", 1), priority("follows", 2), priority("follows", 3)}; got != [3]int64{3, 2, 1} {
		t.Errorf("follows priorities = %v, want the heavier supply above the lighter one", got)
	}
	for _, name := range []string{"own-balance", "on-policy"} {
		if got := [3]int64{priority(name, 1), priority(name, 2), priority(name, 3)}; got != [3]int64{10, 10, 5} {
			t.Errorf("%s priorities = %v, want unchanged", name, got)
		}
	}
	if got := [3]int64{priority("one-weight", 1), priority("one-weight", 2), priority("one-weight", 3)}; got != [3]int64{10, 10, 5} {
		t.Errorf("one-weight priorities = %v, want unchanged", got)
	}
	var allocation string
	DB.Table("model_info").Where("model = ?", "on-policy").Pluck("allocation", &allocation)
	if allocation != "" {
		t.Errorf("on-policy allocation = %q, want it to keep following the policy", allocation)
	}
	var left int64
	DB.Model(&Option{}).Where("key = ?", "SupplyModeDefault").Count(&left)
	if left != 0 {
		t.Error("the SupplyModeDefault option is still stored")
	}
}
