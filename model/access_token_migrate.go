package model

import (
	"fmt"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// legacyAccessTokenRow is a row of the plaintext users.access_token column, which the User model
// no longer maps.
type legacyAccessTokenRow struct {
	Id          int
	AccessToken string
	Type        int
	Deleted     bool
}

// migrateAccessTokensToManagementKeys stops storing the per-user system access token in plaintext
// (AGENT-4). Each token of a live, non-shadow user becomes a legacy-scope management key (hash and
// hint only) that keeps working, with a deprecation warning, for one release; the column is then
// cleared to NULL (not an empty string: it keeps its unique index). Tokens of deleted users and organization
// shadow accounts, which could not sign in, are just cleared. Running it again finds nothing left
// to move. It cannot be rolled back: the plaintext is gone.
func migrateAccessTokensToManagementKeys() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611170201",
		Migrate: func(tx *gorm.DB) error {
			return moveAccessTokensToManagementKeys(tx)
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

// usersHaveAccessTokenColumn reports whether users still has the plaintext access_token column
// (databases created after AGENT-4 never had it). It reads the column list for the same reason
// as tokensHaveKeyColumn.
func usersHaveAccessTokenColumn(tx *gorm.DB) (bool, error) {
	if !tx.Migrator().HasTable("users") {
		return false, nil
	}
	columns, err := tx.Migrator().ColumnTypes("users")
	if err != nil {
		return false, err
	}
	for _, column := range columns {
		if strings.EqualFold(column.Name(), "access_token") {
			return true, nil
		}
	}
	return false, nil
}

func moveAccessTokensToManagementKeys(tx *gorm.DB) error {
	hasColumn, err := usersHaveAccessTokenColumn(tx)
	if err != nil || !hasColumn {
		return err
	}
	if !tx.Migrator().HasTable(&ManagementKey{}) {
		if err := tx.AutoMigrate(&ManagementKey{}); err != nil {
			return err
		}
	}
	// The column is no longer written: new users leave it NULL.
	switch tx.Dialector.Name() {
	case "mysql":
		if err := tx.Exec("ALTER TABLE users MODIFY COLUMN `access_token` char(32) NULL").Error; err != nil {
			return err
		}
	case "postgres":
		if err := tx.Exec(`ALTER TABLE users ALTER COLUMN access_token DROP NOT NULL`).Error; err != nil {
			return err
		}
	}
	if err := tx.Table("users").Where("access_token = ?", "").UpdateColumn("access_token", nil).Error; err != nil {
		return err
	}
	now := utils.GetTimestamp()
	moved := 0
	for {
		var rows []legacyAccessTokenRow
		err := tx.Table("users").
			Select("id", "access_token", "COALESCE(type, 0) AS type", "deleted_at IS NOT NULL AS deleted").
			Where("access_token IS NOT NULL").Order("id").Limit(500).Find(&rows).Error
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			break
		}
		for _, row := range rows {
			token := common.NormalizeManagementKey(row.AccessToken)
			if token != "" && !row.Deleted && row.Type != config.UserTypeOrgShadow {
				if err := insertLegacyAccessToken(tx, row.Id, token, now); err != nil {
					return err
				}
				moved++
			}
			if err := tx.Table("users").Where("id = ?", row.Id).UpdateColumn("access_token", nil).Error; err != nil {
				return err
			}
		}
	}
	if moved > 0 {
		logger.SysLog(fmt.Sprintf("moved %d access tokens to deprecated management keys and cleared their plaintext", moved))
	}
	return nil
}

// insertLegacyAccessToken stores one migrated access token, unless an interrupted earlier run
// already did.
func insertLegacyAccessToken(tx *gorm.DB, userId int, token string, now int64) error {
	// Count rather than First: a miss is the normal case here and First logs it as an error.
	var existing int64
	if err := tx.Model(&ManagementKey{}).Where("key_hash = ?", common.HashManagementKey(token)).Count(&existing).Error; err != nil {
		return err
	}
	if existing > 0 {
		return nil
	}
	return tx.Create(&ManagementKey{
		UserId:      userId,
		Name:        LegacyAccessTokenName,
		Scope:       ManagementKeyScopeLegacy,
		Key:         token,
		CreatedTime: now,
		ExpiredTime: -1,
	}).Error
}
