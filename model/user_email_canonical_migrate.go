package model

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// backfillUserEmailCanonical fills users.email_canonical for accounts created before the column
// existed. AutoMigrate adds the column with an empty default; the canonical form cannot be
// computed in portable SQL (Gmail dot removal), so the rows are read and written in batches.
// Accounts that already share a mailbox keep their emails: the column has no unique constraint,
// and only new registrations and email changes are checked against it.
func backfillUserEmailCanonical() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611170101",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("users") || !tx.Migrator().HasColumn(&User{}, "email_canonical") {
				return nil
			}
			var users []User
			result := tx.Model(&User{}).Unscoped().
				Select("id", "email", "email_canonical").
				Where("email IS NOT NULL AND email <> ''").
				FindInBatches(&users, 500, func(batch *gorm.DB, _ int) error {
					for _, user := range users {
						canonical := common.CanonicalEmail(string(user.Email))
						if canonical == user.EmailCanonical {
							continue
						}
						if err := tx.Model(&User{}).Unscoped().Where("id = ?", user.Id).
							Update("email_canonical", canonical).Error; err != nil {
							return err
						}
					}
					return nil
				})
			if result.Error != nil {
				logger.SysLog("failed to backfill users.email_canonical: " + result.Error.Error())
				return result.Error
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
