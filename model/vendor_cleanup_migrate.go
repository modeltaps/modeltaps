package model

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// vendorMerges are default vendors that duplicate another one: Google PaLM and Google Gemini are
// both Google, Tencent and Hunyuan both Tencent. The models of the first move to the second.
var vendorMerges = []struct {
	from, to int
	fromName string
}{
	{config.ChannelTypePaLM, config.ChannelTypeGemini, "Google PaLM"},
	{config.ChannelTypeTencent, config.ChannelTypeHunyuan, "Tencent"},
}

// platformVendors are default rows that name a platform serving other labs' models, or a model
// kind, not a model maker. They came from the vendor table starting as a copy of the channel types.
var platformVendors = map[int]string{
	config.ChannelTypeOpenRouter:   "OpenRouter",
	config.ChannelTypeGroq:         "Groq",
	config.ChannelTypeCloudflareAI: "Cloudflare AI",
	config.ChannelTypeSiliconflow:  "Siliconflow",
	config.ChannelTypeOllama:       "Ollama",
	config.ChannelTypeCoze:         "Coze",
	config.ChannelTypeRerank:       "Rerank",
}

// migrateVendorCleanup merges the duplicate default vendors and removes the platform rows. A row
// is only touched while it still has its default name: one an admin renamed is theirs. A platform
// row that models still point at is kept, so no admin choice is lost; the log names it.
func migrateVendorCleanup() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611140003",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable(&ModelOwnedBy{}) || !tx.Migrator().HasTable(&ModelInfo{}) {
				return nil
			}
			return cleanupVendors(tx)
		},
	}
}

func cleanupVendors(tx *gorm.DB) error {
	hasLibrary := tx.Migrator().HasTable(&ModelLibraryEntry{})
	isDefault := func(id int, name string) (bool, error) {
		var row ModelOwnedBy
		err := tx.Where("id = ?", id).Limit(1).Find(&row).Error
		return err == nil && row.Id == id && row.Name == name, err
	}
	for _, m := range vendorMerges {
		ok, err := isDefault(m.from, m.fromName)
		if err != nil {
			return err
		}
		var target int64
		if err := tx.Model(&ModelOwnedBy{}).Where("id = ?", m.to).Count(&target).Error; err != nil {
			return err
		}
		if !ok || target == 0 {
			continue
		}
		if err := tx.Model(&ModelInfo{}).Where("vendor_id = ?", m.from).Update("vendor_id", m.to).Error; err != nil {
			return err
		}
		if hasLibrary {
			if err := tx.Model(&ModelLibraryEntry{}).Where("vendor_id = ?", m.from).Update("vendor_id", m.to).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&ModelOwnedBy{}, m.from).Error; err != nil {
			return err
		}
		logger.SysLog(fmt.Sprintf("vendor %q merged into vendor %d", m.fromName, m.to))
	}
	var kept []string
	for id, name := range platformVendors {
		ok, err := isDefault(id, name)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		var used int64
		if err := tx.Model(&ModelInfo{}).Where("vendor_id = ?", id).Count(&used).Error; err != nil {
			return err
		}
		if used > 0 {
			kept = append(kept, name+" ("+strconv.FormatInt(used, 10)+" models)")
			continue
		}
		if hasLibrary {
			if err := tx.Model(&ModelLibraryEntry{}).Where("vendor_id = ?", id).Update("vendor_id", 0).Error; err != nil {
				return err
			}
		}
		if err := tx.Delete(&ModelOwnedBy{}, id).Error; err != nil {
			return err
		}
	}
	if len(kept) > 0 {
		logger.SysLog("platform vendors kept because catalog models use them; point the models at their " +
			"makers and delete these vendors in the admin panel: " + strings.Join(kept, ", "))
	}
	return nil
}
