package model

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/lang"
	"github.com/modeltaps/modeltaps/common/stmp"
	"gorm.io/gorm"
)

// migrateMailTemplateLanguages moves the admin's email templates into the per-language shape:
// texts written before the languages count as the site default language, and the old quota
// warning becomes both quota emails (stmp.UpgradeTemplates).
func migrateMailTemplateLanguages() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611150201",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("options") {
				return nil
			}
			var options []Option
			if err := tx.Find(&options).Error; err != nil {
				return err
			}
			values := map[string]string{}
			for _, option := range options {
				values[option.Key] = option.Value
			}
			raw, exists := values["MailTemplates"]
			if !exists {
				return nil
			}
			legacy := lang.Normalize(values["DefaultLanguage"])
			if legacy == "" {
				legacy = lang.English
			}
			upgraded, changed := stmp.UpgradeTemplates(raw, legacy)
			if !changed {
				return nil
			}
			return tx.Save(&Option{Key: "MailTemplates", Value: upgraded}).Error
		},
	}
}
