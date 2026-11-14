package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// A model a new channel lists only gets its catalog row from the sync that runs after the save,
// so the save could not attach it; the step after the sync does.
func TestSupplyAfterCatalogSync(t *testing.T) {
	setupBalanceTestDB(t)
	if err := model.DB.AutoMigrate(&model.ModelSupply{}, &model.ModelInfo{}); err != nil {
		t.Fatal(err)
	}
	oldEnabled := config.ModelSupplyEnabled
	t.Cleanup(func() { config.ModelSupplyEnabled = oldEnabled })

	ch := model.Channel{Name: "c", Type: config.ChannelTypeOpenAI, Key: "sk", Models: "m-late,m-unknown", Group: "default", Status: config.ChannelStatusEnabled}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}
	count := func() int64 {
		var n int64
		model.DB.Model(&model.ModelSupply{}).Where("channel_id = ?", ch.Id).Count(&n)
		return n
	}
	targets := []ChannelSupplyNames{{ChannelId: ch.Id, Names: model.SplitChannelModels(ch.Models)}}

	// The catalog row arrives after the save, as the sync creates it.
	if err := model.DB.Create(&model.ModelInfo{Model: "m-late", Status: model.ModelStatusDraft}).Error; err != nil {
		t.Fatal(err)
	}

	config.ModelSupplyEnabled = false
	supplyAfterCatalogSync(targets, "test")
	if n := count(); n != 0 {
		t.Fatalf("supplies with supply routing off = %d, want 0", n)
	}

	config.ModelSupplyEnabled = true
	supplyAfterCatalogSync(targets, "test")
	var supplies []model.ModelSupply
	model.DB.Where("channel_id = ?", ch.Id).Find(&supplies)
	if len(supplies) != 1 || supplies[0].Model != "m-late" || supplies[0].Source != model.ModelSupplySourceChannel {
		t.Fatalf("supplies = %+v, want one channel supply for m-late", supplies)
	}

	// Running it again adds nothing; a name with no catalog row stays unattached.
	supplyAfterCatalogSync(targets, "test")
	if n := count(); n != 1 {
		t.Fatalf("supplies after second run = %d, want 1", n)
	}
}
