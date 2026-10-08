package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

func removeKeyIndexMigration() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202405152141",
		Migrate: func(tx *gorm.DB) error {
			dialect := tx.Dialector.Name()
			if dialect == "sqlite" {
				return nil
			}

			if !tx.Migrator().HasIndex(&Channel{}, "idx_channels_key") {
				return nil
			}

			err := tx.Migrator().DropIndex(&Channel{}, "idx_channels_key")
			if err != nil {
				logger.SysLog("remove idx_channels_key  Failure: " + err.Error())
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

func changeTokenKeyColumnType() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202411300001",
		Migrate: func(tx *gorm.DB) error {
			// 如果表不存在，说明是新数据库，直接跳过
			if !tx.Migrator().HasTable("tokens") {
				return nil
			}

			dialect := tx.Dialector.Name()
			var err error

			switch dialect {
			case "mysql":
				err = tx.Exec("ALTER TABLE tokens MODIFY COLUMN `key` varchar(59)").Error
			case "postgres":
				err = tx.Exec("ALTER TABLE tokens ALTER COLUMN key TYPE varchar(59)").Error
			case "sqlite":
				return nil
			}

			if err != nil {
				logger.SysLog("failed to change column type of tokens.key: " + err.Error())
				return err
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("tokens") {
				return nil
			}

			dialect := tx.Dialector.Name()
			var err error

			switch dialect {
			case "mysql":
				err = tx.Exec("ALTER TABLE tokens MODIFY COLUMN `key` char(48)").Error
			case "postgres":
				err = tx.Exec("ALTER TABLE tokens ALTER COLUMN key TYPE char(48)").Error
			}
			return err
		},
	}
}

// changeQuotaColumnsToBigint 将历史上为 int32 的额度列加宽为 bigint,
// 防止大额计费溢出回绕成负数。仅 MySQL/PostgreSQL 需要;SQLite 的 INTEGER
// 本就是动态最长 8 字节,无需处理。
func changeQuotaColumnsToBigint() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202607070001",
		Migrate: func(tx *gorm.DB) error {
			dialect := tx.Dialector.Name()
			if dialect == "sqlite" {
				return nil
			}

			// 表名 -> 需要加宽的列
			targets := map[string][]string{
				"users":  {"quota", "used_quota", "aff_quota", "aff_history"},
				"orders": {"quota"},
			}

			for table, cols := range targets {
				if !tx.Migrator().HasTable(table) {
					continue
				}
				for _, col := range cols {
					if !tx.Migrator().HasColumn(table, col) {
						continue
					}
					var stmt string
					switch dialect {
					case "mysql":
						stmt = "ALTER TABLE `" + table + "` MODIFY COLUMN `" + col + "` bigint NOT NULL DEFAULT 0"
					case "postgres":
						stmt = "ALTER TABLE " + table + " ALTER COLUMN " + col + " TYPE bigint"
					default:
						continue
					}
					if err := tx.Exec(stmt).Error; err != nil {
						logger.SysLog("failed to widen " + table + "." + col + " to bigint: " + err.Error())
						return err
					}
				}
			}
			return nil
		},
		// 不做缩窄回滚:bigint -> int 可能丢数据,故留空。
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

// changeOrgMemberBudgetToBigint 将组织成员预算计数列 budget_used 从 int32 加宽为 bigint,
// 与 users/orders 额度列加宽(changeQuotaColumnsToBigint)同口径,防止长期大额组织溢出。
// organization_members 是 modeltaps 自研表,不在上游迁移 targets 内,故独立迁移;
// budget_start 建表即 bigint,无需处理。SQLite 的 INTEGER 本就是动态最长 8 字节,跳过。
func changeOrgMemberBudgetToBigint() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202608100001",
		Migrate: func(tx *gorm.DB) error {
			dialect := tx.Dialector.Name()
			if dialect == "sqlite" {
				return nil
			}
			if !tx.Migrator().HasTable("organization_members") ||
				!tx.Migrator().HasColumn("organization_members", "budget_used") {
				return nil
			}
			var stmt string
			switch dialect {
			case "mysql":
				stmt = "ALTER TABLE `organization_members` MODIFY COLUMN `budget_used` bigint NOT NULL DEFAULT 0"
			case "postgres":
				stmt = "ALTER TABLE organization_members ALTER COLUMN budget_used TYPE bigint"
			default:
				return nil
			}
			if err := tx.Exec(stmt).Error; err != nil {
				logger.SysLog("failed to widen organization_members.budget_used to bigint: " + err.Error())
				return err
			}
			return nil
		},
		// 不做缩窄回滚:bigint -> int 可能丢数据,故留空。
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

func migrationBefore(db *gorm.DB) error {
	// 从库不执行
	if !config.IsMasterNode {
		logger.SysLog("replica database, skipping pre-migration steps")
		return nil
	}

	// 如果是第一次运行 直接跳过
	if !db.Migrator().HasTable("channels") {
		return nil
	}

	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		removeKeyIndexMigration(),
		changeTokenKeyColumnType(),
		changeQuotaColumnsToBigint(),
		changeOrgMemberBudgetToBigint(),
	})
	return m.Migrate()
}

func addStatistics() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202408100001",
		Migrate: func(tx *gorm.DB) error {
			go UpdateStatistics(StatisticsUpdateTypeALL)
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

func changeChannelApiVersion() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202408190001",
		Migrate: func(tx *gorm.DB) error {
			plugin := `{"customize": {"1": "{version}/chat/completions", "2": "{version}/completions", "3": "{version}/embeddings", "4": "{version}/moderations", "5": "{version}/images/generations", "6": "{version}/images/edits", "7": "{version}/images/variations", "9": "{version}/audio/speech", "10": "{version}/audio/transcriptions", "11": "{version}/audio/translations"}}`

			// 查询 channel 表中的type 为 8，且 other = disable 的数据,直接更新
			var jsonMap map[string]map[string]interface{}
			err := json.Unmarshal([]byte(strings.Replace(plugin, "{version}", "", -1)), &jsonMap)
			if err != nil {
				logger.SysLog("changeChannelApiVersion Failure: " + err.Error())
				return err
			}
			disableApi := map[string]interface{}{
				"other":  "",
				"plugin": datatypes.NewJSONType(jsonMap),
			}

			err = tx.Model(&Channel{}).Where("type = ? AND other = ?", 8, "disable").Updates(disableApi).Error
			if err != nil {
				logger.SysLog("changeChannelApiVersion Failure: " + err.Error())
				return err
			}

			// 查询 channel 表中的type 为 8，且 other != disable 并且不为空 的数据,直接更新
			var channels []*Channel
			err = tx.Model(&Channel{}).Where("type = ? AND other != ? AND other != ?", 8, "disable", "").Find(&channels).Error
			if err != nil {
				logger.SysLog("changeChannelApiVersion Failure: " + err.Error())
				return err
			}

			for _, channel := range channels {
				var jsonMap map[string]map[string]interface{}
				err := json.Unmarshal([]byte(strings.Replace(plugin, "{version}", "/"+channel.Other, -1)), &jsonMap)
				if err != nil {
					logger.SysLog("changeChannelApiVersion Failure: " + err.Error())
					return err
				}
				changeApi := map[string]interface{}{
					"other":  "",
					"plugin": datatypes.NewJSONType(jsonMap),
				}
				err = tx.Model(&Channel{}).Where("id = ?", channel.Id).Updates(changeApi).Error
				if err != nil {
					logger.SysLog("changeChannelApiVersion Failure: " + err.Error())
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Rollback().Error
		},
	}
}

func initUserGroup() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202410300001",
		Migrate: func(tx *gorm.DB) error {
			userGroups := map[string]*UserGroup{
				"default": {
					Symbol: "default",
					Name:   "Default group",
					Ratio:  1,
					Public: true,
				},
				"vip": {
					Symbol: "vip",
					Name:   "VIP group",
					Ratio:  1,
					Public: false,
				},
				"svip": {
					Symbol: "svip",
					Name:   "SVIP group",
					Ratio:  1,
					Public: false,
				},
			}
			option, err := GetOption("GroupRatio")
			if err == nil && option.Value != "" {
				oldGroup := make(map[string]float64)
				err = json.Unmarshal([]byte(option.Value), &oldGroup)
				if err != nil {
					return err
				}

				for k, v := range oldGroup {
					isPublic := false
					if k == "default" {
						isPublic = true
					}
					userGroups[k] = &UserGroup{
						Symbol: k,
						Name:   k,
						Ratio:  v,
						Public: isPublic,
					}
				}
			}

			for k, v := range userGroups {
				err := tx.Where("symbol = ?", k).FirstOrCreate(v).Error
				if err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Rollback().Error
		},
	}
}

func addOldTokenMaxId() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202411300002",
		Migrate: func(tx *gorm.DB) error {
			var token Token
			tx.Last(&token)
			tokenMaxId := token.Id
			option := Option{
				Key: "OldTokenMaxId",
			}

			DB.FirstOrCreate(&option, Option{Key: "OldTokenMaxId"})
			option.Value = strconv.Itoa(tokenMaxId)
			return DB.Save(&option).Error
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Rollback().Error
		},
	}
}

func addExtraRatios() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202504300001",
		Migrate: func(tx *gorm.DB) error {
			extraTokenPriceJson := ""
			extraRatios := make(map[string]map[string]float64)
			// 先查询数据库中是否存在extra_ratios
			option, err := GetOption("ExtraTokenPriceJson")
			if err == nil {
				extraTokenPriceJson = option.Value

			} else {
				extraTokenPriceJson = GetDefaultExtraRatio()
			}

			err = json.Unmarshal([]byte(extraTokenPriceJson), &extraRatios)
			if err != nil {
				return err
			}

			if len(extraRatios) == 0 {
				return nil
			}

			models := make([]string, 0)
			for model := range extraRatios {
				models = append(models, model)
			}

			// 查询数据库中是否存在
			var prices []*Price
			err = tx.Where("model IN (?)", models).Find(&prices).Error
			if err != nil {
				return err
			}

			for _, price := range prices {
				extraRatios := extraRatios[price.Model]
				jsonData := datatypes.NewJSONType(extraRatios)
				price.ExtraRatios = &jsonData
				err = tx.Model(&Price{}).Where("model = ?", price.Model).Updates(map[string]interface{}{
					"extra_ratios": jsonData,
				}).Error
				if err != nil {
					return err
				}
			}

			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Rollback().Error
		},
	}
}

func migrateTokenLimitsStructure() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202510160002",
		Migrate: func(tx *gorm.DB) error {
			// 直接查询原始JSON字符串，避免GORM自动转换
			type TokenRaw struct {
				Id      int    `gorm:"column:id"`
				Name    string `gorm:"column:name"`
				Setting string `gorm:"column:setting;type:json"`
			}

			var tokens []TokenRaw
			err := tx.Table("tokens").Select("id, name, setting").Find(&tokens).Error
			if err != nil {
				logger.SysLog("failed to query API key list: " + err.Error())
				return err
			}

			// 遍历每个 token，转换 limits 结构
			for _, token := range tokens {
				// 解析为 map 以便灵活处理
				var settingMap map[string]interface{}
				err = json.Unmarshal([]byte(token.Setting), &settingMap)
				if err != nil || settingMap == nil {
					// 如果解析失败或为空，跳过
					continue
				}

				// 检查是否有 limits 字段
				limitsRaw, exists := settingMap["limits"]
				if !exists || limitsRaw == nil {
					continue
				}

				// 将 limits 转换为 map
				limitsMap, ok := limitsRaw.(map[string]interface{})
				if !ok {
					continue
				}

				// 检查是否已经是新结构（包含 limit_model_setting）
				if _, hasNew := limitsMap["limit_model_setting"]; hasNew {
					// 已经是新结构，跳过
					continue
				}

				// 检查是否是旧结构（包含 enabled 或 models 字段，说明是直接在 limits 下的旧结构）
				_, hasEnabled := limitsMap["enabled"]
				_, hasModels := limitsMap["models"]
				if !hasEnabled && !hasModels {
					// 既没有 enabled 也没有 models，说明不是旧结构，跳过
					continue
				}

				// 转换为新结构：将旧的 limits 内容移到 limit_model_setting 下
				newLimits := map[string]interface{}{
					"limit_model_setting": limitsMap,
					"limits_ip_setting":   LimitsIPSetting{},
				}

				// 更新 settingMap
				settingMap["limits"] = newLimits

				// 序列化回 JSON
				newSettingBytes, err := json.Marshal(settingMap)
				if err != nil {
					logger.SysLog("failed to serialize API key setting: " + err.Error())
					continue
				}

				// 更新数据库
				err = tx.Model(&Token{}).Where("id = ?", token.Id).Update("setting", datatypes.JSON(newSettingBytes)).Error
				if err != nil {
					logger.SysLog("failed to update API key setting: " + err.Error())
					continue
				}
			}

			logger.SysLog("upgraded limits structure of the tokens.setting column")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			// 回滚：将新结构转回旧结构
			var tokens []Token
			err := tx.Find(&tokens).Error
			if err != nil {
				return err
			}

			for _, token := range tokens {
				settingBytes, err := token.Setting.MarshalJSON()
				if err != nil {
					continue
				}

				var settingMap map[string]interface{}
				err = json.Unmarshal(settingBytes, &settingMap)
				if err != nil || settingMap == nil {
					continue
				}

				limitsRaw, exists := settingMap["limits"]
				if !exists || limitsRaw == nil {
					continue
				}

				limitsMap, ok := limitsRaw.(map[string]interface{})
				if !ok {
					continue
				}

				// 检查是否有 limit_model_setting
				modelSettingRaw, hasModelSetting := limitsMap["limit_model_setting"]
				if !hasModelSetting {
					continue
				}

				// 将 limit_model_setting 的内容提升到 limits 层级
				settingMap["limits"] = modelSettingRaw

				newSettingBytes, err := json.Marshal(settingMap)
				if err != nil {
					continue
				}

				tx.Model(&Token{}).Where("id = ?", token.Id).Update("setting", datatypes.JSON(newSettingBytes))
			}

			return nil
		},
	}
}

// migrateTokenLogIONullable 将 tokens.log_io 由「布尔 + 默认 false」迁移为「可空三态(NULL=继承)」。
// 不改既有数据值:现存 false=显式强制关、true=显式强制开,迁移后行为保持完全一致(决策 6);
// 仅放开列约束/默认,使后续新建「继承」令牌可写入 NULL。
func migrateTokenLogIONullable() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202606160001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("tokens") {
				return nil
			}
			switch tx.Dialector.Name() {
			case "mysql":
				return tx.Exec("ALTER TABLE tokens MODIFY COLUMN `log_io` tinyint(1) NULL DEFAULT NULL").Error
			case "postgres":
				if err := tx.Exec(`ALTER TABLE tokens ALTER COLUMN log_io DROP DEFAULT`).Error; err != nil {
					return err
				}
				return tx.Exec(`ALTER TABLE tokens ALTER COLUMN log_io DROP NOT NULL`).Error
			default:
				// sqlite: 原 bool 列仅带 DEFAULT、无 NOT NULL 约束,既有值原样保留;
				// 新建「继承」令牌由 GORM 显式写入 NULL,无需重建表。
				return nil
			}
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

func migrationAfter(db *gorm.DB) error {
	// 从库不执行
	if !config.IsMasterNode {
		logger.SysLog("replica database, skipping post-migration steps")
		return nil
	}
	m := gormigrate.New(db, gormigrate.DefaultOptions, []*gormigrate.Migration{
		addStatistics(),
		changeChannelApiVersion(),
		initUserGroup(),
		addOldTokenMaxId(),
		addExtraRatios(),
		migrateTokenLimitsStructure(),
		migrateTokenLogIONullable(),
		migrateOrgMemberBudgetColumns(),
		migrateTokenPeriodColumns(),
		migrateUserEmailUnique(),
		migrateUserPhoneUnique(),
		migrateUserNameWithoutAt(),
		migrateLegacyOidcProvider(),
		migrateAccountSystem(),
		dropOidcLogoutSessions(),
		backfillUserEmailVerified(),
		backfillModelInfoPublished(),
		migrateModelOwnedBySlugUnique(),
		migrateModelInfoHidden(),
		migrateModelOwnedByIcon(),
		migrateModelInfoAudioSpeechEndpoints(),
	})
	return m.Migrate()
}

// migrateModelInfoAudioSpeechEndpoints 修正旧版同步推导误给对话出音频模型（及纯 TTS 模型）追加的
// audio.speech：同步只在 endpoints 为空时填入，已入库的误标不会被新推导覆盖。
// 只处理未锁定、来源不是 manual、且 endpoints 仍与旧版推导逐字相同的行（视为同步原值、未被管理员改过），
// 改写为当前推导结果；重复执行无副作用。
func migrateModelInfoAudioSpeechEndpoints() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609250002",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("model_info") || !tx.Migrator().HasColumn(&ModelInfo{}, "endpoints") {
				return nil
			}
			var rows []*ModelInfo
			if err := tx.Where("locked = ? AND (source IS NULL OR source <> ?) AND endpoints LIKE ?",
				false, ModelInfoSourceManual, "%"+ModelEndpointAudioSpeech+"%").Find(&rows).Error; err != nil {
				return err
			}
			fixed := 0
			for _, row := range rows {
				var input, output []string
				_ = json.Unmarshal([]byte(row.InputModalities), &input)
				_ = json.Unmarshal([]byte(row.OutputModalities), &output)
				if row.Endpoints != legacyModelEndpointsFromModalities(row.Model, input, output) {
					continue
				}
				derived := ModelEndpointsFromModalities(row.Model, input, output)
				if derived == row.Endpoints {
					continue
				}
				if err := tx.Model(&ModelInfo{}).Where("id = ?", row.Id).Update("endpoints", derived).Error; err != nil {
					return err
				}
				fixed++
			}
			logger.SysLog("model_info audio.speech endpoints migration: fixed rows = " + strconv.Itoa(fixed))
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

// migrateModelOwnedByIcon 把厂商表 icon 中的外链转成本地取值（见 convertLegacyVendorIcon），
// 原值备份到 icon_legacy 供回滚。只处理非空且不是 brand: / upload: 的行，重复执行无副作用；
// 单行转换失败时置空（交给自动解析）并记日志，不中断启动。
func migrateModelOwnedByIcon() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609250001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable(&ModelOwnedBy{}) || !tx.Migrator().HasColumn(&ModelOwnedBy{}, "icon_legacy") {
				return nil
			}
			var rows []*ModelOwnedBy
			if err := tx.Where("icon <> ? AND icon NOT LIKE ? AND icon NOT LIKE ?",
				"", VendorIconBrandPrefix+"%", VendorIconUploadPrefix+"%").Find(&rows).Error; err != nil {
				return err
			}
			for _, row := range rows {
				icon, convErr := convertLegacyVendorIcon(context.Background(), tx, row.Icon)
				if convErr != nil {
					logger.SysError(fmt.Sprintf("model_owned_by icon migration: vendor %d icon cleared: %v", row.Id, convErr))
				}
				updates := map[string]any{"icon": icon}
				if row.IconLegacy == "" {
					updates["icon_legacy"] = row.Icon
				}
				if err := tx.Model(&ModelOwnedBy{}).Where("id = ?", row.Id).Updates(updates).Error; err != nil {
					return err
				}
			}
			logger.SysLog("model_owned_by icon migration: converted rows = " + strconv.Itoa(len(rows)))
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn(&ModelOwnedBy{}, "icon_legacy") {
				return nil
			}
			return tx.Model(&ModelOwnedBy{}).Where("icon_legacy <> ?", "").
				Updates(map[string]any{"icon": gorm.Expr("icon_legacy"), "icon_legacy": ""}).Error
		},
	}
}

// backfillModelInfoPublished 曾为存量 model_info 行回填 published。可见性已改为
// 「有启用渠道可路由且未隐藏」（见 migrateModelInfoHidden），published 不再被读取，
// 本迁移保留 ID 以维持迁移历史，内容为空操作。
func backfillModelInfoPublished() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609210001",
		Migrate: func(tx *gorm.DB) error {
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

// legacySeedAuditSnapshot 是旧版 apply_seed 审计快照里与发布状态相关的字段。
type legacySeedAuditSnapshot struct {
	DryRun          bool     `json:"dry_run"`
	CreatedModels   []string `json:"created_models"`
	PublishedModels []string `json:"published_models"`
	UnpublishedRows []string `json:"unpublished_rows"`
}

// migrateModelInfoHidden 把「管理员发布才可见」迁移为「未隐藏即可见」：
// 之前被管理员明确下架过的行置 hidden=true，其余行一律 hidden=false。
// 「明确下架」按 model_catalog_audit_log 回放判定（见 explicitlyUnpublishedModels）；
// 仅因新行默认未发布的行不算下架。迁移前后各打印一次隐藏行数量。
func migrateModelInfoHidden() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609240001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("model_info") || !tx.Migrator().HasColumn(&ModelInfo{}, "hidden") {
				return nil
			}
			var before int64
			if err := tx.Model(&ModelInfo{}).Where("hidden = ?", true).Count(&before).Error; err != nil {
				return err
			}
			logger.SysLog("model_info hidden migration: hidden rows before = " + strconv.FormatInt(before, 10))

			var unpublished []string
			if tx.Migrator().HasTable(&ModelCatalogAuditLog{}) {
				names, err := explicitlyUnpublishedModels(tx)
				if err != nil {
					return err
				}
				unpublished = names
			}

			if err := tx.Model(&ModelInfo{}).Where("1 = 1").Update("hidden", false).Error; err != nil {
				return err
			}
			// 分批 IN 更新，避免下架记录很多时把 SQL 参数撑爆。
			const chunkSize = 200
			for start := 0; start < len(unpublished); start += chunkSize {
				end := start + chunkSize
				if end > len(unpublished) {
					end = len(unpublished)
				}
				if err := tx.Model(&ModelInfo{}).Where("model IN ?", unpublished[start:end]).
					Update("hidden", true).Error; err != nil {
					return err
				}
			}

			var after int64
			if err := tx.Model(&ModelInfo{}).Where("hidden = ?", true).Count(&after).Error; err != nil {
				return err
			}
			logger.SysLog("model_info hidden migration: hidden rows after = " + strconv.FormatInt(after, 10))
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

// explicitlyUnpublishedModels 按时间顺序回放目录审计，返回最终处于「被明确下架」状态的模型（已排序）：
// 单条 unpublish 与实跑 apply_seed 的 unpublished_rows 记为下架；
// 单条 publish 与 apply_seed 的 published_models / created_models 记为发布；以最后一次为准。
func explicitlyUnpublishedModels(tx *gorm.DB) ([]string, error) {
	var logs []*ModelCatalogAuditLog
	if err := tx.Where("action IN ?", []string{
		ModelCatalogActionLegacyPublish, ModelCatalogActionLegacyUnpublish, ModelCatalogActionApplySeed,
	}).Order("created_time asc, id asc").Find(&logs).Error; err != nil {
		return nil, err
	}
	unpublished := make(map[string]bool)
	for _, log := range logs {
		switch log.Action {
		case ModelCatalogActionLegacyPublish:
			if log.Model != "" {
				unpublished[log.Model] = false
			}
		case ModelCatalogActionLegacyUnpublish:
			if log.Model != "" {
				unpublished[log.Model] = true
			}
		case ModelCatalogActionApplySeed:
			snapshot := legacySeedAuditSnapshot{}
			if err := json.Unmarshal([]byte(log.After), &snapshot); err != nil || snapshot.DryRun {
				continue
			}
			for _, name := range snapshot.UnpublishedRows {
				unpublished[name] = true
			}
			for _, name := range snapshot.PublishedModels {
				unpublished[name] = false
			}
			for _, name := range snapshot.CreatedModels {
				unpublished[name] = false
			}
		}
	}
	names := make([]string, 0, len(unpublished))
	for name, isUnpublished := range unpublished {
		if isUnpublished {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// backfillUserEmailVerified 为存量非空邮箱回填 users.email_verified = true（AUTH-4）。
// 列本身由 AutoMigrate 依 User 结构体先建（默认 false），这里只做一次性数据回填。
//
// 存量邮箱的来源历史上没有记录，无法逐行区分是验证码绑定、IdP 下发还是管理员代填；
// 绝大多数属于前两者，故一律按已验证处理：宁可放过少量管理员代填的存量邮箱，
// 也不能让全站用户在升级后突然看到「未验证」并被迫重新验证。升级之后写入的邮箱
// 才按真实来源分流（见 controller 各写入路径）。
func backfillUserEmailVerified() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609170001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("users") || !tx.Migrator().HasColumn(&User{}, "email_verified") {
				return nil
			}
			if err := tx.Model(&User{}).Where("email IS NOT NULL AND email <> ''").
				Update("email_verified", true).Error; err != nil {
				logger.SysLog("failed to backfill users.email_verified: " + err.Error())
				return err
			}
			logger.SysLog("existing non-empty emails backfilled as verified")
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("users") || !tx.Migrator().HasColumn(&User{}, "email_verified") {
				return nil
			}
			return tx.Migrator().DropColumn(&User{}, "email_verified")
		},
	}
}

// dropOidcLogoutSessions 退出用的 id_token 已并入 user_sessions 行，旧表不再读写。
// 升级后所有会话都要重新登录（cookie 改名 + 会话行），旧表里的行没有存在意义，直接删表。
func dropOidcLogoutSessions() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609140002",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("oidc_logout_sessions") {
				return nil
			}
			return tx.Migrator().DropTable("oidc_logout_sessions")
		},
	}
}

// migrateAccountSystem 为存量站点推导账号体系：升级前「密码登录已关闭 + 有启用的 OIDC 提供方」
// 的站点实际就是外部身份提供方模式，写入 AccountSystem=external；其余站点写 builtin。
// options 里已有 AccountSystem 行时不动它。与 readLegacyOidcOptions 一样全量读回 options 在 Go 侧筛选，
// 避开 MySQL 保留字 key 的引号差异。
func migrateAccountSystem() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609140001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("options") {
				return nil
			}
			var options []Option
			if err := tx.Find(&options).Error; err != nil {
				return err
			}
			values := make(map[string]string, len(options))
			for _, option := range options {
				values[option.Key] = option.Value
			}
			if _, exists := values["AccountSystem"]; exists {
				return nil
			}
			system := config.AccountSystemBuiltin
			if values["PasswordLoginEnabled"] == "false" && tx.Migrator().HasTable("oidc_providers") {
				var enabled int64
				if err := tx.Model(&OidcProvider{}).Where("enabled = ?", true).Count(&enabled).Error; err != nil {
					return err
				}
				if enabled > 0 {
					system = config.AccountSystemExternal
				}
			}
			if err := tx.Create(&Option{Key: "AccountSystem", Value: system}).Error; err != nil {
				return err
			}
			if err := config.GlobalOption.Set("AccountSystem", system); err != nil {
				return err
			}
			logger.SysLog("account system derived from existing settings: " + system)
			return nil
		},
	}
}

// legacyOidcOptionKeys 存量单提供方的七个 OIDC 选项键。
var legacyOidcOptionKeys = map[string]bool{
	"OIDCAuthEnabled":    true,
	"OIDCClientId":       true,
	"OIDCClientSecret":   true,
	"OIDCIssuer":         true,
	"OIDCScopes":         true,
	"OIDCUsernameClaims": true,
	"OIDCDisplayName":    true,
}

// readLegacyOidcOptions 读出 options 表里的旧 OIDC 选项。
// 不在 SQL 里按 key 过滤：列名 key 在 MySQL 是保留字，各方言引号写法不一致，
// options 表本就只有几十行，全量取回在 Go 侧筛选更稳。
func readLegacyOidcOptions(tx *gorm.DB) (map[string]string, error) {
	var options []Option
	if err := tx.Model(&Option{}).Find(&options).Error; err != nil {
		return nil, err
	}
	values := make(map[string]string, len(legacyOidcOptionKeys))
	for _, option := range options {
		if legacyOidcOptionKeys[option.Key] {
			values[option.Key] = option.Value
		}
	}
	return values, nil
}

// migrateLegacyOidcProvider 把存量单提供方配置迁进 oidc_providers / user_oidc_identities：
// ① options 里的旧 OIDC* 选项 → 一行 slug=oidc 的提供方（display_name_claim/avatar_claim
// 沿用旧硬编码 claim 名、link_by_verified_email=true，保持升级前后行为一致）；
// ② users.oidc_id 非空的行 → 该提供方下的身份行。
//
// 两张表由 AutoMigrate 先建，这里只做数据搬运。除 gormigrate 的 ID 记录外，插入前
// 都按唯一键查一次，重复执行不会新增行。旧选项与 users.oidc_id 都不清理，回滚可用。
func migrateLegacyOidcProvider() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609030001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("options") || !tx.Migrator().HasTable("users") ||
				!tx.Migrator().HasTable(&OidcProvider{}) || !tx.Migrator().HasTable(&UserOidcIdentity{}) {
				return nil
			}

			values, err := readLegacyOidcOptions(tx)
			if err != nil {
				logger.SysLog("failed to read legacy OIDC options: " + err.Error())
				return err
			}
			// 没配过 OIDC 的部署：不建提供方，也就没有身份可迁。
			if values["OIDCIssuer"] == "" || values["OIDCClientId"] == "" {
				return nil
			}

			provider, err := ensureLegacyOidcProvider(tx, values)
			if err != nil {
				return err
			}
			return migrateLegacyOidcIdentities(tx, provider.Id)
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

// ensureLegacyOidcProvider 取出（必要时创建）slug=oidc 的提供方行。已存在则原样返回，
// 不覆盖管理员可能已经改过的配置。
func ensureLegacyOidcProvider(tx *gorm.DB, values map[string]string) (*OidcProvider, error) {
	var provider OidcProvider
	err := tx.Where("slug = ?", LegacyOidcProviderSlug).First(&provider).Error
	if err == nil {
		return &provider, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		logger.SysLog("failed to query legacy OIDC identity provider: " + err.Error())
		return nil, err
	}

	displayName := values["OIDCDisplayName"]
	if displayName == "" {
		displayName = "OIDC"
	}
	now := time.Now().Unix()
	provider = OidcProvider{
		Slug:         LegacyOidcProviderSlug,
		DisplayName:  displayName,
		Issuer:       values["OIDCIssuer"],
		ClientId:     values["OIDCClientId"],
		ClientSecret: values["OIDCClientSecret"],
		Scopes:       values["OIDCScopes"],
		// 旧实现里这三个 claim 名分别来自 OIDCUsernameClaims 与两处硬编码，原样搬过来。
		UsernameClaim:    values["OIDCUsernameClaims"],
		DisplayNameClaim: "displayName",
		AvatarClaim:      "avatar",
		// 旧实现按已验证邮箱自动关联，迁移出来的这一行必须保持开启，否则老用户会被重复注册。
		LinkByVerifiedEmail: true,
		Enabled:             values["OIDCAuthEnabled"] == "true",
		CreatedTime:         now,
		UpdatedTime:         now,
	}
	if err := tx.Create(&provider).Error; err != nil {
		logger.SysLog("failed to create legacy OIDC identity provider: " + err.Error())
		return nil, err
	}
	logger.SysLog("legacy OIDC config migrated to identity provider slug=" + LegacyOidcProviderSlug)
	return &provider, nil
}

// migrateLegacyOidcIdentities 把 users.oidc_id 非空的行写入 user_oidc_identities。
// 只取未软删除的用户：软删除账号本来就登录不上（旧的 FillUserByOidcId 也查不到），
// 迁进来只会白占 (provider_id, subject) 唯一键、挡住同一 subject 重新注册。
// (provider_id, subject) 已存在时记日志跳过，不中止整批迁移。
func migrateLegacyOidcIdentities(tx *gorm.DB, providerId int) error {
	type legacyIdentity struct {
		Id     int    `gorm:"column:id"`
		OidcId string `gorm:"column:oidc_id"`
	}
	var rows []legacyIdentity
	if err := tx.Table("users").
		Select("id, oidc_id").
		Where("oidc_id IS NOT NULL AND oidc_id <> ''").
		Where("deleted_at IS NULL").
		Order("id").
		Scan(&rows).Error; err != nil {
		logger.SysLog("failed to read legacy OIDC user identities: " + err.Error())
		return err
	}

	migrated := 0
	for _, row := range rows {
		var existing UserOidcIdentity
		err := tx.Where("provider_id = ? AND subject = ?", providerId, row.OidcId).First(&existing).Error
		if err == nil {
			if existing.UserId != row.Id {
				logger.SysLog("OIDC identity subject " + row.OidcId + " already belongs to user id " +
					strconv.Itoa(existing.UserId) + ", skipping user id " + strconv.Itoa(row.Id))
			}
			continue
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			logger.SysLog("failed to query OIDC identity: " + err.Error())
			return err
		}
		identity := UserOidcIdentity{
			UserId:      row.Id,
			ProviderId:  providerId,
			Subject:     row.OidcId,
			CreatedTime: time.Now().Unix(),
		}
		if err := tx.Create(&identity).Error; err != nil {
			logger.SysLog("failed to migrate OIDC identity of user id " + strconv.Itoa(row.Id) + ": " + err.Error())
			continue
		}
		migrated++
	}
	if migrated > 0 {
		logger.SysLog("migrated " + strconv.Itoa(migrated) + " legacy OIDC user identities")
	}
	return nil
}

// migrateUserNameWithoutAt 拦下历史遗留的含 @ 用户名：登录解析按「标识符含 @ 即查 email 列」
// 显式分列（见 ResolveLoginUser），这类账号的用户名会被路由到邮箱列而永远登录不上。
// 与重复邮箱同策略 fail-closed：列出 id + username，由管理员改名后重启。
// 迁移只读不写，且由 gormigrate 以 ID 记录，天然幂等。
//
// 不纳入软删除用户：它们本就无法登录，且 Delete() 会给用户名追加 _del_ 后缀、
// 管理端也不再能改名，纳入只会造成无法解除的启动失败。
func migrateUserNameWithoutAt() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609020002",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("users") {
				return nil
			}

			type usernameConflict struct {
				Id       int    `gorm:"column:id"`
				Username string `gorm:"column:username"`
			}
			var conflicts []usernameConflict
			if err := tx.Table("users").
				Select("id, username").
				Where("username LIKE ?", "%@%").
				Where("deleted_at IS NULL").
				Order("id").
				Scan(&conflicts).Error; err != nil {
				logger.SysLog("failed to check for usernames containing @: " + err.Error())
				return err
			}
			if len(conflicts) > 0 {
				details := make([]string, 0, len(conflicts))
				for _, c := range conflicts {
					details = append(details, "user id ["+strconv.Itoa(c.Id)+"] -> username "+c.Username)
				}
				msg := "users.username contains usernames with @; these accounts cannot sign in by username. An admin must rename them and restart:\n" +
					strings.Join(details, "\n")
				logger.SysLog(msg)
				return errors.New(msg)
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}

// normalizeUserEmails 归一化 users.email：trim + 小写；空串（含仅空白）统一写为 NULL。
// 不加 email <> LOWER(TRIM(email)) 过滤：MySQL 默认 ci 排序规则下该条件恒不成立，
// 历史大小写邮箱不会被改写；对非 NULL 行全量重写是方言无关的写法。
func normalizeUserEmails(tx *gorm.DB) error {
	// 先清空已软删账号的邮箱，与 User.Delete() 的语义对齐：唯一索引不区分软删，
	// 历史软删行残留的邮箱会白占地址，还会与活跃账号构成"归一化后重复"而挡住升级。
	if err := tx.Exec("UPDATE users SET email = NULL WHERE deleted_at IS NOT NULL AND email IS NOT NULL").Error; err != nil {
		logger.SysLog("failed to clear emails of soft-deleted users: " + err.Error())
		return err
	}
	if err := tx.Exec("UPDATE users SET email = LOWER(TRIM(email)) WHERE email IS NOT NULL").Error; err != nil {
		logger.SysLog("failed to normalize users.email: " + err.Error())
		return err
	}
	if err := tx.Exec("UPDATE users SET email = NULL WHERE email = ''").Error; err != nil {
		logger.SysLog("failed to set empty emails to NULL: " + err.Error())
		return err
	}
	return nil
}

// detectDuplicateUserEmails 检测归一化后重复的邮箱，存在则返回列出冲突 user id 与邮箱的
// 错误（fail-closed，不自动挑一个）。软删除用户也占用唯一索引，故一并纳入检查。
func detectDuplicateUserEmails(tx *gorm.DB) error {
	type emailConflict struct {
		Email string `gorm:"column:email"`
		Ids   string `gorm:"column:ids"`
	}
	idsExpr := "GROUP_CONCAT(id)"
	if tx.Dialector.Name() == "postgres" {
		idsExpr = "STRING_AGG(CAST(id AS TEXT), ',')"
	}
	var conflicts []emailConflict
	if err := tx.Table("users").
		Select("email, " + idsExpr + " AS ids").
		Where("email IS NOT NULL").
		Group("email").
		Having("COUNT(*) > 1").
		Scan(&conflicts).Error; err != nil {
		logger.SysLog("failed to check for duplicate emails: " + err.Error())
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}
	details := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		details = append(details, c.Email+" -> user id ["+c.Ids+"]")
	}
	msg := "users.email contains duplicate emails after normalization, so the unique constraint cannot be created. Clean them up manually and restart:\n" +
		strings.Join(details, "\n")
	logger.SysLog(msg)
	return errors.New(msg)
}

// ensureUserEmailUniqueReady 必须在 AutoMigrate(&User{}) 之前调用。
//
// users.email 的唯一索引现在由 User 结构体的 gorm 标签声明，AutoMigrate 会直接建它
// （这也是让 MySQL driver 不再把它当冗余索引 DROP 掉的前提）。但脏库上直接建索引只会
// 抛出 1062 之类的原始错误，既看不出是哪些账号冲突，也没有先归一化的机会——历史库里
// 空邮箱是 ” 而非 NULL，多行 ” 本身就会撞唯一约束。所以这里先做归一化 + 重复检测
// fail-closed，把友好错误提前到建索引之前。
//
// 索引已存在时整体跳过（正常启动的常态路径），因此全表 UPDATE 只会在真正需要时执行一次；
// 索引缺失时（首次升级、或历史上被 MySQL driver 删掉的库）自动重跑，天然自愈且幂等。
func ensureUserEmailUniqueReady(db *gorm.DB) error {
	if !config.IsMasterNode {
		return nil
	}
	if !db.Migrator().HasTable("users") {
		return nil
	}
	if db.Migrator().HasIndex(&User{}, "idx_users_email_unique") {
		return nil
	}
	if err := normalizeUserEmails(db); err != nil {
		return err
	}
	return detectDuplicateUserEmails(db)
}

// migrateUserEmailUnique 让 users.email 成为可信的唯一登录凭证：
// ① 全表归一化（trim + 小写）并把空串统一写为 NULL；
// ② 检测归一化后重复的邮箱，存在则 fail-closed；
// ③ 建立唯一索引 idx_users_email_unique。
//
// 干净库上 ①②③ 实际都已由 ensureUserEmailUniqueReady + AutoMigrate 完成，这里是兜底：
// 保留它才能让「已记录 202609020001、但索引因故缺失」的库不出现语义空洞。
//
// 允许任意多个"无邮箱"用户共存的机制：空邮箱以 NULL 落库（列类型 NullableEmail 的
// Valuer 保证后续写入同样如此），MySQL / PostgreSQL / SQLite 的唯一索引均不约束 NULL，
// 因此无需部分索引等方言特化写法。
func migrateUserEmailUnique() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609020001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("users") {
				return nil
			}

			if err := normalizeUserEmails(tx); err != nil {
				return err
			}
			if err := detectDuplicateUserEmails(tx); err != nil {
				return err
			}

			// ③ 建立唯一索引
			if tx.Migrator().HasIndex(&User{}, "idx_users_email_unique") {
				return nil
			}
			stmt := "CREATE UNIQUE INDEX idx_users_email_unique ON users (email)"
			if tx.Dialector.Name() == "mysql" {
				stmt = "CREATE UNIQUE INDEX idx_users_email_unique ON `users` (`email`)"
			}
			if err := tx.Exec(stmt).Error; err != nil {
				logger.SysLog("failed to create unique index on users.email: " + err.Error())
				return err
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if !tx.Migrator().HasIndex(&User{}, "idx_users_email_unique") {
				return nil
			}
			return tx.Migrator().DropIndex(&User{}, "idx_users_email_unique")
		},
	}
}

// normalizeUserPhones 归一化 users.phone_number：trim；空串（含仅空白）统一写为 NULL。
// 不做号码格式改写（去连字符 / 补国家码等）：手机号来自 IdP 的已验证 claim，
// 本站无权重写它的写法，只保证"空 = NULL"这一唯一索引前提。
func normalizeUserPhones(tx *gorm.DB) error {
	// 与 User.Delete() 语义对齐：唯一索引不区分软删，历史软删行残留的手机号会白占号码
	if err := tx.Exec("UPDATE users SET phone_number = NULL WHERE deleted_at IS NOT NULL AND phone_number IS NOT NULL").Error; err != nil {
		logger.SysLog("failed to clear phone numbers of soft-deleted users: " + err.Error())
		return err
	}
	if err := tx.Exec("UPDATE users SET phone_number = TRIM(phone_number) WHERE phone_number IS NOT NULL").Error; err != nil {
		logger.SysLog("failed to normalize users.phone_number: " + err.Error())
		return err
	}
	if err := tx.Exec("UPDATE users SET phone_number = NULL WHERE phone_number = ''").Error; err != nil {
		logger.SysLog("failed to set empty phone numbers to NULL: " + err.Error())
		return err
	}
	return nil
}

// resolveDuplicateUserPhones 处理重复手机号：每组保留 id 最小的账号，其余账号的手机号
// 清空为 NULL，并用 SysError 列出被清空的 user id（手机号明文不入日志）。
//
// 与邮箱的 fail-closed 不同，这里不阻断启动：升级前 phone_number 只是展示字段、
// 既无唯一约束也不参与登录，重复很正常；而唯一索引由 gorm 标签声明、AutoMigrate 会直接建，
// 脏数据留着只会让整站起不来。被清空的号码不丢语义——这些账号下次经 IdP 登录时，
// syncOidcPhoneNumber 会在本地为空的前提下重新回填。
// 软删除用户也占用唯一索引，故一并纳入检查（normalizeUserPhones 已先清空软删行）。
func resolveDuplicateUserPhones(tx *gorm.DB) error {
	type phoneConflict struct {
		Ids string `gorm:"column:ids"`
	}
	idsExpr := "GROUP_CONCAT(id)"
	if tx.Dialector.Name() == "postgres" {
		idsExpr = "STRING_AGG(CAST(id AS TEXT), ',')"
	}
	var conflicts []phoneConflict
	if err := tx.Table("users").
		Select(idsExpr + " AS ids").
		Where("phone_number IS NOT NULL").
		Group("phone_number").
		Having("COUNT(*) > 1").
		Scan(&conflicts).Error; err != nil {
		logger.SysLog("failed to check for duplicate phone numbers: " + err.Error())
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}

	cleared := make([]int, 0, len(conflicts))
	for _, c := range conflicts {
		ids := make([]int, 0, 2)
		for _, raw := range strings.Split(c.Ids, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}
		if len(ids) < 2 {
			continue
		}
		// GROUP_CONCAT 不保证顺序，显式取最小 id 作为保留者
		keep := ids[0]
		for _, id := range ids {
			if id < keep {
				keep = id
			}
		}
		for _, id := range ids {
			if id != keep {
				cleared = append(cleared, id)
			}
		}
	}
	if len(cleared) == 0 {
		return nil
	}
	if err := tx.Exec("UPDATE users SET phone_number = NULL WHERE id IN ?", cleared).Error; err != nil {
		logger.SysLog("failed to clear duplicate phone numbers: " + err.Error())
		return err
	}
	details := make([]string, 0, len(cleared))
	for _, id := range cleared {
		details = append(details, strconv.Itoa(id))
	}
	logger.SysError("users.phone_number contained duplicates; kept the phone number on the account with the smallest id in each group and cleared it on the others " +
		"so the unique constraint can be created (the number now belongs to another account, so it will not be backfilled on these accounts' next IdP login): user id [" + strings.Join(details, ",") + "]")
	return nil
}

// ensureUserPhoneUniqueReady 必须在 AutoMigrate(&User{}) 之前调用，理由与
// ensureUserEmailUniqueReady 完全一致：唯一索引由 User 的 gorm 标签声明、AutoMigrate 直接建，
// 而历史库里手机号是 ” 而非 NULL，且升级前它只是展示字段、可能重复，
// 不先归一化 + 去重就只会拿到建索引时的原始错误。索引已存在时整体跳过。
func ensureUserPhoneUniqueReady(db *gorm.DB) error {
	if !config.IsMasterNode {
		return nil
	}
	if !db.Migrator().HasTable("users") {
		return nil
	}
	if !db.Migrator().HasColumn(&User{}, "phone_number") {
		return nil
	}
	if db.Migrator().HasIndex(&User{}, "idx_users_phone_unique") {
		return nil
	}
	if err := normalizeUserPhones(db); err != nil {
		return err
	}
	return resolveDuplicateUserPhones(db)
}

// migrateUserPhoneUnique 让 users.phone_number 从展示字段升级为可关联标识：
// ① 归一化（trim，空串写 NULL）；② 重复手机号按保留最小 id 的规则去重并告警；
// ③ 建唯一索引 idx_users_phone_unique。
// 与 migrateUserEmailUnique 同理，干净库上这三步已由 ensureUserPhoneUniqueReady + AutoMigrate
// 完成，这里是索引因故缺失时的兜底。空手机号以 NULL 落库（NullablePhone 的 Valuer 保证），
// 三种方言的唯一索引都不约束 NULL，故任意多个无手机号用户可共存。
func migrateUserPhoneUnique() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609120001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("users") {
				return nil
			}

			if err := normalizeUserPhones(tx); err != nil {
				return err
			}
			if err := resolveDuplicateUserPhones(tx); err != nil {
				return err
			}

			if tx.Migrator().HasIndex(&User{}, "idx_users_phone_unique") {
				return nil
			}
			stmt := "CREATE UNIQUE INDEX idx_users_phone_unique ON users (phone_number)"
			if tx.Dialector.Name() == "mysql" {
				stmt = "CREATE UNIQUE INDEX idx_users_phone_unique ON `users` (`phone_number`)"
			}
			if err := tx.Exec(stmt).Error; err != nil {
				logger.SysLog("failed to create unique index on users.phone_number: " + err.Error())
				return err
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if !tx.Migrator().HasIndex(&User{}, "idx_users_phone_unique") {
				return nil
			}
			return tx.Migrator().DropIndex(&User{}, "idx_users_phone_unique")
		},
	}
}

// normalizeModelOwnedBySlugs 归一化 model_owned_by.slug：trim + 小写；空串（含仅空白）写为 NULL。
// 空 slug 表示"该厂商没有公开标识"，必须是 NULL 才能与唯一索引共存（三种方言都不约束 NULL）。
func normalizeModelOwnedBySlugs(tx *gorm.DB) error {
	if err := tx.Exec("UPDATE model_owned_by SET slug = LOWER(TRIM(slug)) WHERE slug IS NOT NULL").Error; err != nil {
		logger.SysLog("failed to normalize model_owned_by.slug: " + err.Error())
		return err
	}
	if err := tx.Exec("UPDATE model_owned_by SET slug = NULL WHERE slug = ''").Error; err != nil {
		logger.SysLog("failed to set empty slugs to NULL: " + err.Error())
		return err
	}
	return nil
}

// resolveDuplicateModelOwnedBySlugs 处理重复 slug：每组保留 id 最小的厂商，其余厂商的 slug
// 追加 "-{id}" 后缀并记日志。不 fail-closed：slug 只是元数据同步时的映射线索，
// 升级前没有唯一约束、重复完全可能，而唯一索引由结构体标签声明、AutoMigrate 会直接建，
// 留着脏数据只会让整站起不来。被改写的厂商由管理员在后台重新填写正确 slug。
func resolveDuplicateModelOwnedBySlugs(tx *gorm.DB) error {
	type slugConflict struct {
		Ids string `gorm:"column:ids"`
	}
	idsExpr := "GROUP_CONCAT(id)"
	if tx.Dialector.Name() == "postgres" {
		idsExpr = "STRING_AGG(CAST(id AS TEXT), ',')"
	}
	var conflicts []slugConflict
	if err := tx.Table("model_owned_by").
		Select(idsExpr + " AS ids").
		Where("slug IS NOT NULL AND slug <> ''").
		Group("slug").
		Having("COUNT(*) > 1").
		Scan(&conflicts).Error; err != nil {
		logger.SysLog("failed to check for duplicate vendor slugs: " + err.Error())
		return err
	}

	renamed := make([]string, 0, len(conflicts))
	for _, c := range conflicts {
		ids := make([]int, 0, 2)
		for _, raw := range strings.Split(c.Ids, ",") {
			id, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}
		if len(ids) < 2 {
			continue
		}
		// GROUP_CONCAT 不保证顺序，显式取最小 id 作为保留者
		keep := ids[0]
		for _, id := range ids {
			if id < keep {
				keep = id
			}
		}
		for _, id := range ids {
			if id == keep {
				continue
			}
			suffix := "-" + strconv.Itoa(id)
			// MySQL 的 || 默认是逻辑或而非字符串拼接，会把 slug 静默写成 0，必须走 CONCAT
			stmt := "UPDATE model_owned_by SET slug = slug || ? WHERE id = ?"
			if tx.Dialector.Name() == "mysql" {
				stmt = "UPDATE model_owned_by SET slug = CONCAT(slug, ?) WHERE id = ?"
			}
			if err := tx.Exec(stmt, suffix, id).Error; err != nil {
				logger.SysLog("failed to rewrite duplicate vendor slugs: " + err.Error())
				return err
			}
			renamed = append(renamed, strconv.Itoa(id))
		}
	}
	if len(renamed) == 0 {
		return nil
	}
	logger.SysError("model_owned_by.slug contained duplicates; kept the vendor with the smallest id in each group and appended a " +
		"\"-{id}\" suffix to the slug of the others so the unique constraint can be created (set the correct slug for these vendors in the admin panel): vendor id [" +
		strings.Join(renamed, ",") + "]")
	return nil
}

// ensureModelOwnedBySlugUniqueReady 必须在 AutoMigrate(&ModelOwnedBy{}) 之前调用：
// 唯一索引由 ModelOwnedBy 的 gorm 标签声明、AutoMigrate 直接建，而历史库里 slug 是 ”
// 而非 NULL、且可能重复，不先归一化 + 去重就只会拿到建索引时的原始错误。索引已存在时整体跳过。
func ensureModelOwnedBySlugUniqueReady(db *gorm.DB) error {
	if !config.IsMasterNode {
		return nil
	}
	if !db.Migrator().HasTable("model_owned_by") {
		return nil
	}
	if !db.Migrator().HasColumn(&ModelOwnedBy{}, "slug") {
		return nil
	}
	if db.Migrator().HasIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug") {
		return nil
	}
	if err := normalizeModelOwnedBySlugs(db); err != nil {
		return err
	}
	return resolveDuplicateModelOwnedBySlugs(db)
}

// migrateModelOwnedBySlugUnique 让 model_owned_by.slug 成为可信的厂商标识：归一化、去重、
// 建唯一索引 idx_model_owned_by_slug。干净库上这三步已由 ensureModelOwnedBySlugUniqueReady +
// AutoMigrate 完成，这里是索引因故缺失时的兜底，与邮箱 / 手机号唯一索引的做法一致。
func migrateModelOwnedBySlugUnique() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202609220001",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasTable("model_owned_by") {
				return nil
			}

			if err := normalizeModelOwnedBySlugs(tx); err != nil {
				return err
			}
			if err := resolveDuplicateModelOwnedBySlugs(tx); err != nil {
				return err
			}

			if tx.Migrator().HasIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug") {
				return nil
			}
			stmt := "CREATE UNIQUE INDEX idx_model_owned_by_slug ON model_owned_by (slug)"
			if tx.Dialector.Name() == "mysql" {
				stmt = "CREATE UNIQUE INDEX idx_model_owned_by_slug ON `model_owned_by` (`slug`)"
			}
			if err := tx.Exec(stmt).Error; err != nil {
				logger.SysLog("failed to create unique index on model_owned_by.slug: " + err.Error())
				return err
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			if !tx.Migrator().HasIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug") {
				return nil
			}
			return tx.Migrator().DropIndex(&ModelOwnedBy{}, "idx_model_owned_by_slug")
		},
	}
}

// migrateTokenPeriodColumns 把令牌周期计数从 setting JSON 回填到专用列(SEC-5)。
// 列由 AutoMigrate 先建;此处仅一次性数据搬运,gormigrate 以 ID 保证幂等。
func migrateTokenPeriodColumns() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202606280001",
		Migrate: func(tx *gorm.DB) error {
			type tokenRaw struct {
				Id      int    `gorm:"column:id"`
				Setting string `gorm:"column:setting;type:json"`
			}
			var rows []tokenRaw
			if err := tx.Table("tokens").Select("id, setting").Find(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				if r.Setting == "" {
					continue
				}
				var s struct {
					PeriodUsed  int   `json:"period_used"`
					PeriodStart int64 `json:"period_start"`
				}
				if err := json.Unmarshal([]byte(r.Setting), &s); err != nil {
					continue
				}
				if s.PeriodUsed == 0 && s.PeriodStart == 0 {
					continue
				}
				if err := tx.Table("tokens").Where("id = ?", r.Id).
					Updates(map[string]interface{}{"period_used": s.PeriodUsed, "period_start": s.PeriodStart}).Error; err != nil {
					logger.SysLog("failed to backfill API key period usage columns: " + err.Error())
					continue
				}
			}
			logger.SysLog("migrated API key period usage period_used/period_start to dedicated columns")
			return nil
		},
	}
}

// migrateOrgMemberBudgetColumns 把组织成员周期预算计数从 setting JSON 回填到专用列(SEC-4)。
// 列由 AutoMigrate 先建;此处仅做一次性数据搬运,gormigrate 以 ID 保证幂等。
func migrateOrgMemberBudgetColumns() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202606270001",
		Migrate: func(tx *gorm.DB) error {
			type memberRaw struct {
				Id      int    `gorm:"column:id"`
				Setting string `gorm:"column:setting;type:json"`
			}
			var rows []memberRaw
			if err := tx.Table("organization_members").Select("id, setting").Find(&rows).Error; err != nil {
				return err
			}
			for _, r := range rows {
				if r.Setting == "" {
					continue
				}
				var s struct {
					BudgetUsed  int   `json:"budget_used"`
					BudgetStart int64 `json:"budget_start"`
				}
				if err := json.Unmarshal([]byte(r.Setting), &s); err != nil {
					continue
				}
				if s.BudgetUsed == 0 && s.BudgetStart == 0 {
					continue // 列默认即 0,无需写
				}
				if err := tx.Table("organization_members").Where("id = ?", r.Id).
					Updates(map[string]interface{}{"budget_used": s.BudgetUsed, "budget_start": s.BudgetStart}).Error; err != nil {
					logger.SysLog("failed to backfill organization member budget columns: " + err.Error())
					continue
				}
			}
			logger.SysLog("migrated organization member budget budget_used/budget_start to dedicated columns")
			return nil
		},
	}
}
