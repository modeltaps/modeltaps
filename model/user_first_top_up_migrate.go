package model

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// backfillUserFirstTopUp sets users.first_top_up_time for accounts that already have a credited
// order, so their next order is not taken for the first one and does not pay the inviter a
// second rebate. The time is the earliest credited order's last update (when it was credited),
// or 1 when that is unknown; deleted orders count, since they were paid too.
func backfillUserFirstTopUp() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611170102",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("users") || !tx.Migrator().HasTable("orders") ||
				!tx.Migrator().HasColumn(&User{}, "first_top_up_time") {
				return nil
			}
			err := tx.Exec(`UPDATE users SET first_top_up_time = (
					SELECT CASE WHEN MIN(o.updated_at) > 0 THEN MIN(o.updated_at) ELSE 1 END
					FROM orders o WHERE o.user_id = users.id AND o.status = ?)
				WHERE first_top_up_time = 0
				AND EXISTS (SELECT 1 FROM orders o WHERE o.user_id = users.id AND o.status = ?)`,
				OrderStatusSuccess, OrderStatusSuccess).Error
			if err != nil {
				logger.SysLog("failed to backfill users.first_top_up_time: " + err.Error())
				return err
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
