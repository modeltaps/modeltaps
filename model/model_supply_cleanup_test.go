package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// seedCleanupSupplies writes channels a, b (tag "grp", b disabled) and c, models m1 and m2, and a
// supply for every channel × model pair. ChannelModelAlias is migrated too because deleting a
// channel removes its aliases.
func seedCleanupSupplies(t *testing.T) (a, b, c *Channel) {
	t.Helper()
	useSupplyTestDB(t)
	if err := DB.AutoMigrate(&ChannelModelAlias{}); err != nil {
		t.Fatal(err)
	}
	weight := uint(1)
	a = &Channel{Name: "a", Status: config.ChannelStatusEnabled, Weight: &weight, Models: "m1,m2", Tag: "grp"}
	b = &Channel{Name: "b", Status: config.ChannelStatusManuallyDisabled, Weight: &weight, Models: "m1,m2", Tag: "grp"}
	c = &Channel{Name: "c", Status: config.ChannelStatusEnabled, Weight: &weight, Models: "m1,m2"}
	for _, ch := range []*Channel{a, b, c} {
		if err := DB.Create(ch).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"m1", "m2"} {
		if err := DB.Create(&ModelInfo{Model: name, Status: ModelStatusPublished}).Error; err != nil {
			t.Fatal(err)
		}
		for _, ch := range []*Channel{a, b, c} {
			if err := DB.Create(&ModelSupply{Model: name, ChannelId: ch.Id, UpstreamModel: name, Enabled: true}).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	return a, b, c
}

func supplyCount(t *testing.T, where string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := DB.Model(&ModelSupply{}).Where(where, args...).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// Every way of deleting channels takes their supplies along (MODEL-44).
func TestDeletingChannelsDeletesTheirSupplies(t *testing.T) {
	t.Run("single", func(t *testing.T) {
		a, _, _ := seedCleanupSupplies(t)
		if err := a.Delete(); err != nil {
			t.Fatal(err)
		}
		if n := supplyCount(t, "channel_id = ?", a.Id); n != 0 {
			t.Fatalf("supplies of deleted channel = %d, want 0", n)
		}
		if n := supplyCount(t, "1 = 1"); n != 4 {
			t.Fatalf("remaining supplies = %d, want 4", n)
		}
	})
	t.Run("batch", func(t *testing.T) {
		a, b, c := seedCleanupSupplies(t)
		if _, err := BatchDeleteChannel([]int{a.Id, b.Id}); err != nil {
			t.Fatal(err)
		}
		if n := supplyCount(t, "channel_id <> ?", c.Id); n != 0 {
			t.Fatalf("supplies of deleted channels = %d, want 0", n)
		}
	})
	t.Run("disabled", func(t *testing.T) {
		_, b, _ := seedCleanupSupplies(t)
		if _, err := DeleteDisabledChannel(); err != nil {
			t.Fatal(err)
		}
		if n := supplyCount(t, "channel_id = ?", b.Id); n != 0 {
			t.Fatalf("supplies of deleted disabled channel = %d, want 0", n)
		}
		if n := supplyCount(t, "1 = 1"); n != 4 {
			t.Fatalf("remaining supplies = %d, want 4", n)
		}
	})
	t.Run("tag", func(t *testing.T) {
		_, _, c := seedCleanupSupplies(t)
		if err := DeleteChannelsTag("grp", false); err != nil {
			t.Fatal(err)
		}
		if n := supplyCount(t, "channel_id <> ?", c.Id); n != 0 {
			t.Fatalf("supplies of the deleted tag group = %d, want 0", n)
		}
		if n := supplyCount(t, "channel_id = ?", c.Id); n != 2 {
			t.Fatalf("supplies of the untagged channel = %d, want 2", n)
		}
	})
	t.Run("tag disabled only", func(t *testing.T) {
		a, b, _ := seedCleanupSupplies(t)
		if err := DeleteChannelsTag("grp", true); err != nil {
			t.Fatal(err)
		}
		if n := supplyCount(t, "channel_id = ?", b.Id); n != 0 {
			t.Fatalf("supplies of the deleted disabled member = %d, want 0", n)
		}
		if n := supplyCount(t, "channel_id = ?", a.Id); n != 2 {
			t.Fatalf("supplies of the enabled member = %d, want 2", n)
		}
	})
}

func TestDeletingCatalogModelDeletesItsSupplies(t *testing.T) {
	seedCleanupSupplies(t)
	var m1 ModelInfo
	if err := DB.Where("model = ?", "m1").First(&m1).Error; err != nil {
		t.Fatal(err)
	}
	// A second row with the same name keeps the supplies.
	dup := &ModelInfo{Model: "m1", Status: ModelStatusPublished}
	if err := DB.Create(dup).Error; err != nil {
		t.Fatal(err)
	}
	if err := DeleteModelInfo(dup.Id); err != nil {
		t.Fatal(err)
	}
	if n := supplyCount(t, "model = ?", "m1"); n != 3 {
		t.Fatalf("supplies of m1 with its row left = %d, want 3", n)
	}
	if err := DeleteModelInfo(m1.Id); err != nil {
		t.Fatal(err)
	}
	if n := supplyCount(t, "model = ?", "m1"); n != 0 {
		t.Fatalf("supplies of deleted m1 = %d, want 0", n)
	}
	if n := supplyCount(t, "model = ?", "m2"); n != 3 {
		t.Fatalf("supplies of m2 = %d, want 3", n)
	}
	if err := DeleteModelInfo(999999); err != nil {
		t.Fatalf("deleting a missing model: %v", err)
	}
}

func TestCleanOrphanSupplies(t *testing.T) {
	a, _, _ := seedCleanupSupplies(t)
	// Leave orphans the old way: delete rows without the cleanup.
	if err := DB.Delete(&Channel{}, a.Id).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Where("model = ?", "m2").Delete(&ModelInfo{}).Error; err != nil {
		t.Fatal(err)
	}
	m := cleanOrphanSupplies()
	for i := 0; i < 2; i++ {
		if err := m.Migrate(DB); err != nil {
			t.Fatalf("run %d: %v", i+1, err)
		}
	}
	var left []ModelSupply
	DB.Order("id").Find(&left)
	if len(left) != 2 {
		t.Fatalf("supplies left = %+v, want the 2 m1 supplies of live channels", left)
	}
	for _, s := range left {
		if s.Model != "m1" || s.ChannelId == a.Id {
			t.Fatalf("unexpected supply left: %+v", s)
		}
	}
}
