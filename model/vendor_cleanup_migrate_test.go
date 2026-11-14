package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func TestCleanupVendors(t *testing.T) {
	setupModelOwnedByTestDB(t)
	if err := DB.AutoMigrate(&ModelInfo{}, &ModelLibraryEntry{}); err != nil {
		t.Fatal(err)
	}
	for _, v := range []*ModelOwnedBy{
		{Id: config.ChannelTypePaLM, Name: "Google PaLM"},
		{Id: config.ChannelTypeGemini, Name: "Google Gemini", Slug: "google"},
		{Id: config.ChannelTypeTencent, Name: "Tencent"},
		{Id: config.ChannelTypeHunyuan, Name: "Hunyuan", Slug: "tencent"},
		{Id: config.ChannelTypeOpenRouter, Name: "OpenRouter"},
		{Id: config.ChannelTypeGroq, Name: "Groq", Slug: "groq"}, // a model still points at it
		{Id: config.ChannelTypeOllama, Name: "My local lab"},     // renamed by an admin
		{Id: config.ChannelTypeSiliconflow, Name: "Siliconflow"},
	} {
		if err := DB.Create(v).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range []*ModelInfo{
		{Model: "palm-2", VendorID: config.ChannelTypePaLM},
		{Model: "hunyuan-pro", VendorID: config.ChannelTypeTencent},
		{Model: "groq-thing", VendorID: config.ChannelTypeGroq},
	} {
		if err := DB.Create(m).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := DB.Create(&ModelLibraryEntry{Canonical: "x/y", Model: "y", VendorID: config.ChannelTypeOpenRouter}).Error; err != nil {
		t.Fatal(err)
	}
	if err := cleanupVendors(DB); err != nil {
		t.Fatal(err)
	}

	var ids []int
	DB.Model(&ModelOwnedBy{}).Order("id").Pluck("id", &ids)
	want := []int{config.ChannelTypeGroq, config.ChannelTypeGemini, config.ChannelTypeOllama, config.ChannelTypeHunyuan}
	got := map[int]bool{}
	for _, id := range ids {
		got[id] = true
	}
	if len(ids) != len(want) {
		t.Errorf("vendors left = %v, want %v", ids, want)
	}
	for _, id := range want {
		if !got[id] {
			t.Errorf("vendor %d should be kept, left %v", id, ids)
		}
	}
	vendorOf := func(name string) int {
		var m ModelInfo
		DB.Where("model = ?", name).First(&m)
		return m.VendorID
	}
	if v := vendorOf("palm-2"); v != config.ChannelTypeGemini {
		t.Errorf("palm-2 vendor = %d, want Google", v)
	}
	if v := vendorOf("hunyuan-pro"); v != config.ChannelTypeHunyuan {
		t.Errorf("hunyuan-pro vendor = %d, want the Tencent row with the slug", v)
	}
	var e ModelLibraryEntry
	DB.Where("canonical = ?", "x/y").First(&e)
	if e.VendorID != 0 {
		t.Errorf("library entry still points at the removed platform: %d", e.VendorID)
	}
	if err := cleanupVendors(DB); err != nil { // a second run changes nothing
		t.Fatal(err)
	}
}
