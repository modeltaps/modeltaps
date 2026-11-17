package model

import (
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// cleanOrphanSupplies removes supplies left behind before deleting a channel or a catalog model
// took its supplies along (MODEL-44): supplies of channels that are deleted (soft-deleted ones
// included) and supplies of models that no longer have a catalog row.
func cleanOrphanSupplies() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611170301",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable(&ModelSupply{}) || !tx.Migrator().HasTable(&Channel{}) || !tx.Migrator().HasTable(&ModelInfo{}) {
				return nil
			}
			live := tx.Model(&Channel{}).Select("id")
			byChannel := tx.Where("channel_id NOT IN (?)", live).Delete(&ModelSupply{})
			if byChannel.Error != nil {
				return byChannel.Error
			}
			catalog := tx.Model(&ModelInfo{}).Select("model")
			byModel := tx.Where("model NOT IN (?)", catalog).Delete(&ModelSupply{})
			if byModel.Error != nil {
				return byModel.Error
			}
			if n := byChannel.RowsAffected + byModel.RowsAffected; n > 0 {
				logger.SysLog(fmt.Sprintf("removed %d orphan supplies (%d of deleted channels, %d of deleted models)",
					n, byChannel.RowsAffected, byModel.RowsAffected))
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
