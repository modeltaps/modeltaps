package model

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	crand "crypto/rand"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func SetupDB() {
	err := InitDB()
	if err != nil {
		logger.FatalLog("failed to initialize database: " + err.Error())
	}
	ChannelGroup.Load()
	GlobalUserGroupRatio.Load()
	config.RootUserEmail = GetRootUserEmail()
	NewModelOwnedBys()
	if err := ReloadModelInfoCache(); err != nil {
		logger.SysError("Failed to load model info cache: " + err.Error())
	}

	if viper.GetBool("batch_update_enabled") {
		config.BatchUpdateEnabled = true
		config.BatchUpdateInterval = utils.GetOrDefault("batch_update_interval", 5)
		logger.SysLog("batch update enabled with interval " + strconv.Itoa(config.BatchUpdateInterval) + "s")
		InitBatchUpdater()
	} else {
		// 同步化的 Quota.Consume 在 batch=false 时每请求多走 4 次真 DB 写，
		// 非流式 handler TTLB 增加 ~10-20ms。建议生产环境开启 batch_update_enabled=true
		// 把扣费 / 日志 / 统计走 in-memory 批量。
		logger.SysLog("batch_update_enabled=false: each request will sync-write 4 DB rows on Consume; set batch_update_enabled=true for lower TTLB")
	}
}

// rootPasswordChars 用于生成初始 root 随机密码（字母+数字，落在 User.Password 的 min=8,max=64 区间内）。
const rootPasswordChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// generateRootPassword 用 crypto/rand 生成 length 位强随机密码。
func generateRootPassword(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := crand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = rootPasswordChars[int(b)%len(rootPasswordChars)]
	}
	return string(buf), nil
}

func createRootAccountIfNeed() error {
	var user User
	//if user.Status != common.UserStatusEnabled {
	if err := DB.First(&user).Error; err != nil {
		var password string
		if envPassword := viper.GetString("root_password"); envPassword != "" {
			// root_password_allow_insecure 仅供本地开发（如 ROOT_PASSWORD=root），生产禁止开启。
			allowInsecure := viper.GetBool("root_password_allow_insecure")
			if allowInsecure {
				logger.SysLog("WARNING: ROOT_PASSWORD_ALLOW_INSECURE=true, ROOT_PASSWORD length check is skipped; this is for local development only, never enable it in production")
			} else if len(envPassword) < 8 || len(envPassword) > 64 {
				return fmt.Errorf("ROOT_PASSWORD length must be between 8 and 64 characters, got %d", len(envPassword))
			}
			password = envPassword
			logger.SysLog("no user exists, create a root user for you: username is root, password is set via ROOT_PASSWORD env")
		} else {
			randomPassword, err := generateRootPassword(16)
			if err != nil {
				return err
			}
			password = randomPassword
			logger.SysLog("no user exists, create a root user for you: username is root, initial root password: " + password)
		}
		hashedPassword, err := common.Password2Hash(password)
		if err != nil {
			return err
		}
		rootUser := User{
			Username:    "root",
			Password:    hashedPassword,
			Role:        config.RoleRootUser,
			Status:      config.UserStatusEnabled,
			DisplayName: "Root User",
			AccessToken: utils.GetUUID(),
			Quota:       100000000,
		}
		if err := DB.Create(&rootUser).Error; err != nil {
			return err
		}
	}
	return nil
}

func chooseDB() (*gorm.DB, error) {
	if viper.IsSet("sql_dsn") {
		dsn := viper.GetString("sql_dsn")
		localTimezone := utils.GetLocalTimezone()
		if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
			// Use PostgreSQL
			logger.SysLog("using PostgreSQL as database")
			common.UsingPostgreSQL = true
			dsn = dsnAddArg(dsn, "timezone", localTimezone)

			return gorm.Open(postgres.New(postgres.Config{
				DSN:                  dsn,
				PreferSimpleProtocol: true, // disables implicit prepared statement usage
			}), &gorm.Config{
				PrepareStmt: true, // precompile SQL
			})

		}
		// Use MySQL
		logger.SysLog("using MySQL as database")
		// mysql 时区设置
		dsn = dsnAddArg(dsn, "loc", localTimezone)
		// dsn = dsnAddArg(dsn, "parseTime", "true")
		return gorm.Open(mysql.Open(dsn), &gorm.Config{
			PrepareStmt: true, // precompile SQL
		})
	}
	// Use SQLite
	logger.SysLog("SQL_DSN not set, using SQLite as database")
	common.UsingSQLite = true
	config := fmt.Sprintf("?_busy_timeout=%d", utils.GetOrDefault("sqlite_busy_timeout", 3000))
	return gorm.Open(sqlite.Open(viper.GetString("sqlite_path")+config), &gorm.Config{
		PrepareStmt: true, // precompile SQL
	})
}

func InitDB() (err error) {
	db, err := chooseDB()
	if err == nil {
		if config.Debug {
			db = db.Debug()
		}
		DB = db
		sqlDB, err := DB.DB()
		if err != nil {
			return err
		}

		sqlDB.SetMaxIdleConns(utils.GetOrDefault("SQL_MAX_IDLE_CONNS", 100))
		sqlDB.SetMaxOpenConns(utils.GetOrDefault("SQL_MAX_OPEN_CONNS", 1000))
		sqlDB.SetConnMaxLifetime(time.Second * time.Duration(utils.GetOrDefault("SQL_MAX_LIFETIME", 60)))

		if !config.IsMasterNode {
			return nil
		}
		logger.SysLog("database migration started")

		migrationBefore(DB)

		// users.email 的唯一索引由 gorm 标签声明、AutoMigrate 直接建；脏库必须先归一化并
		// 查重 fail-closed，否则只会拿到建索引时的原始 1062 错误，看不出是哪些账号冲突。
		if err = ensureUserEmailUniqueReady(DB); err != nil {
			return err
		}

		// users.phone_number 同理：升级前它只是展示字段，可能存在 '' 与重复号码
		if err = ensureUserPhoneUniqueReady(DB); err != nil {
			return err
		}

		// model_owned_by.slug 同理：升级前无唯一约束，存量行是 '' 且可能重复
		if err = ensureModelOwnedBySlugUniqueReady(DB); err != nil {
			return err
		}

		err = db.AutoMigrate(&Channel{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Token{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&User{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Option{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Redemption{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Log{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&LogDetail{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&TelegramMenu{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Price{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Midjourney{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&Payment{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Order{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Task{})
		if err != nil {
			return err
		}
		err = db.AutoMigrate(&Statistics{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&UserGroup{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&ModelOwnedBy{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&InviteCode{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&ModelInfo{})
		if err != nil {
			return err
		}

		err = DB.AutoMigrate(&WebAuthnCredential{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&StatisticsMonthGeneratedHistory{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&StatisticsMonth{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&Organization{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&OrganizationMember{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&OrganizationInvitation{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&OrganizationAuditLog{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&ModelCatalogAuditLog{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&OidcProvider{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&UserOidcIdentity{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&UserSession{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&UserSession{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&BrandIconCache{})
		if err != nil {
			return err
		}

		err = db.AutoMigrate(&BrandIconUpload{})
		if err != nil {
			return err
		}

		// 迁移失败必须终止启动：邮箱唯一约束等 fail-closed 迁移一旦被吞掉，
		// 进程会带着未建立的约束继续跑，等于约束形同虚设。
		if err = migrationAfter(DB); err != nil {
			return err
		}

		logger.SysLog("database migrated")
		err = createRootAccountIfNeed()
		return err
	} else {
		logger.FatalLog(err)
	}
	return err
}

// func MigrateDB(db *gorm.DB) error {
// 	if DB.Migrator().HasConstraint(&Price{}, "model") {
// 		fmt.Println("----Price model has constraint----")
// 		// 如果是主键，移除主键约束
// 		err := db.Migrator().DropConstraint(&Price{}, "model")
// 		if err != nil {
// 			return err
// 		}
// 		// 修改字段长度
// 		err = db.Migrator().AlterColumn(&Price{}, "model")
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	return nil
// }

func CloseDB() error {
	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}
	err = sqlDB.Close()
	return err
}

func dsnAddArg(dsn string, arg string, value string) string {
	// 如果是MySQL 需要转义
	if !common.UsingPostgreSQL {
		value = url.QueryEscape(value)
	}

	if !strings.Contains(dsn, arg+"=") {
		if strings.Contains(dsn, "?") {
			dsn += "&" + arg + "=" + value
		} else {
			dsn += "?" + arg + "=" + value
		}
	}
	return dsn
}
