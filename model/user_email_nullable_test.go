package model

import (
	"os"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// The MySQL ALTER must keep the column's own type, character set and collation: MODIFY replaces
// the whole definition, and GORM's default type (or the table defaults) could truncate values.
func TestUserEmailNullableSQL(t *testing.T) {
	cases := []struct {
		name   string
		column mysqlColumnInfo
		want   string
	}{
		{
			name:   "string column",
			column: mysqlColumnInfo{IsNullable: "NO", ColumnType: "varchar(255)", CharacterSetName: strPtr("utf8mb4"), CollationName: strPtr("utf8mb4_unicode_ci")},
			want:   "ALTER TABLE `users` MODIFY COLUMN `email` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NULL",
		},
		{
			name:   "no character set",
			column: mysqlColumnInfo{IsNullable: "NO", ColumnType: "varbinary(191)"},
			want:   "ALTER TABLE `users` MODIFY COLUMN `email` varbinary(191) NULL",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := userEmailNullableSQL(tc.column); got != tc.want {
				t.Fatalf("userEmailNullableSQL = %q, want %q", got, tc.want)
			}
		})
	}
}

// SQLite has no hand-edit path to fix; the pre-check must not touch it or fail.
func TestEnsureUserEmailNullableSkipsSQLite(t *testing.T) {
	db := setupEmailTestDB(t, true)
	if err := ensureUserEmailNullable(db); err != nil {
		t.Fatalf("ensureUserEmailNullable on SQLite: %v", err)
	}
}

// notNullEmailDialects returns the server databases the NOT NULL scenario runs on. They run only
// when MODELTAPS_TEST_MYSQL_DSN / MODELTAPS_TEST_POSTGRES_DSN point to a scratch database (the
// users table is dropped and recreated).
func notNullEmailDialects(t *testing.T) map[string]func() (*gorm.DB, error) {
	t.Helper()
	silent := &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)}
	dialects := map[string]func() (*gorm.DB, error){}
	if dsn := os.Getenv("MODELTAPS_TEST_MYSQL_DSN"); dsn != "" {
		dialects["mysql"] = func() (*gorm.DB, error) { return gorm.Open(mysql.Open(dsn), silent) }
	}
	if dsn := os.Getenv("MODELTAPS_TEST_POSTGRES_DSN"); dsn != "" {
		dialects["postgres"] = func() (*gorm.DB, error) {
			return gorm.Open(postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true}), silent)
		}
	}
	return dialects
}

// SEC-22: an operator made users.email NOT NULL by hand on an upgraded database (empty emails are
// '' and the unique index is not there yet). Startup normalization writes NULL for them, which
// used to stop startup with Error 1048. The pre-check now makes the column nullable again, keeps
// its type, and the normalization and AutoMigrate then complete.
func TestEnsureUserEmailUniqueReadyRelaxesNotNullEmail(t *testing.T) {
	dialects := notNullEmailDialects(t)
	if len(dialects) == 0 {
		t.Skip("set MODELTAPS_TEST_MYSQL_DSN or MODELTAPS_TEST_POSTGRES_DSN to run against a server database")
	}
	for name, open := range dialects {
		t.Run(name, func(t *testing.T) {
			db, err := open()
			if err != nil {
				t.Fatal(err)
			}
			if sqlDB, err := db.DB(); err == nil {
				sqlDB.SetMaxOpenConns(1)
				t.Cleanup(func() { sqlDB.Close() })
			}
			if err := db.Migrator().DropTable(&User{}); err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&User{}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { db.Migrator().DropTable(&User{}) })
			if err := db.Migrator().DropIndex(&User{}, "idx_users_email_unique"); err != nil {
				t.Fatal(err)
			}

			notNull := "ALTER TABLE `users` MODIFY COLUMN `email` varchar(120) NOT NULL DEFAULT ''"
			if name == "postgres" {
				notNull = "ALTER TABLE users ALTER COLUMN email SET DEFAULT '', ALTER COLUMN email SET NOT NULL"
			}
			if err := db.Exec(notNull).Error; err != nil {
				t.Fatalf("make users.email NOT NULL: %v", err)
			}
			for _, username := range []string{"empty1", "empty2"} {
				if err := db.Exec("INSERT INTO users (username, password, email, aff_code) VALUES (?, ?, ?, ?)",
					username, "test-password", "", utils.GetRandomString(8)).Error; err != nil {
					t.Fatalf("seed legacy row: %v", err)
				}
			}

			if err := ensureUserEmailUniqueReady(db); err != nil {
				t.Fatalf("ensureUserEmailUniqueReady with a NOT NULL email column: %v", err)
			}
			if name == "mysql" {
				var columnType string
				if err := db.Raw("SELECT COLUMN_TYPE FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = DATABASE() " +
					"AND TABLE_NAME = 'users' AND COLUMN_NAME = 'email'").Scan(&columnType).Error; err != nil {
					t.Fatal(err)
				}
				if columnType != "varchar(120)" {
					t.Fatalf("the ALTER should keep the column type, got %q", columnType)
				}
			}
			if err := db.AutoMigrate(&User{}); err != nil {
				t.Fatalf("AutoMigrate after the pre-check: %v", err)
			}

			columnTypes, err := db.Migrator().ColumnTypes(&User{})
			if err != nil {
				t.Fatal(err)
			}
			for _, column := range columnTypes {
				if column.Name() != "email" {
					continue
				}
				if nullable, ok := column.Nullable(); !ok || !nullable {
					t.Fatalf("users.email should be nullable again, nullable=%v ok=%v", nullable, ok)
				}
			}
			var nullCount int64
			if err := db.Model(&User{}).Where("email IS NULL").Count(&nullCount).Error; err != nil {
				t.Fatal(err)
			}
			if nullCount != 2 {
				t.Fatalf("both empty emails should be NULL, got %d", nullCount)
			}
			if !db.Migrator().HasIndex(&User{}, "idx_users_email_unique") {
				t.Fatal("the unique index should exist after AutoMigrate")
			}
		})
	}
}

// When the ALTER cannot run, startup stops with the exact SQL for the operator instead of the
// bare driver error.
func TestMakeUserEmailNullableReportsSQL(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	stmt := "ALTER TABLE `missing_users` MODIFY COLUMN `email` varchar(255) NULL"
	err = makeUserEmailNullable(db, stmt)
	if err == nil {
		t.Fatal("a failing ALTER must stop startup")
	}
	if !strings.Contains(err.Error(), stmt) || !strings.Contains(err.Error(), "ALTER privilege") {
		t.Fatalf("the error should carry the SQL to run, got: %s", err.Error())
	}
}
