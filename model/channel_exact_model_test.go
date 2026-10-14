package model

import (
	"os"
	"slices"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// exactModelDialects returns the databases to run the exact_model search on. SQLite always runs;
// MySQL and PostgreSQL run when MODELTAPS_TEST_MYSQL_DSN / MODELTAPS_TEST_POSTGRES_DSN point to a
// scratch database (the channels table is dropped and recreated).
func exactModelDialects(t *testing.T) map[string]func() (*gorm.DB, error) {
	t.Helper()
	silent := &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)}
	dialects := map[string]func() (*gorm.DB, error){
		"sqlite": func() (*gorm.DB, error) { return gorm.Open(sqlite.Open(":memory:"), silent) },
	}
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

func TestGetChannelsListExactModel(t *testing.T) {
	for name, open := range exactModelDialects(t) {
		t.Run(name, func(t *testing.T) {
			testDB, err := open()
			if err != nil {
				t.Fatal(err)
			}
			if sqlDB, err := testDB.DB(); err == nil {
				sqlDB.SetMaxOpenConns(1)
				t.Cleanup(func() { sqlDB.Close() })
			}
			if err := testDB.Migrator().DropTable(&Channel{}); err != nil {
				t.Fatal(err)
			}
			if err := testDB.AutoMigrate(&Channel{}); err != nil {
				t.Fatal(err)
			}
			oldDB, oldPG := DB, common.UsingPostgreSQL
			DB, common.UsingPostgreSQL = testDB, name == "postgres"
			t.Cleanup(func() { DB, common.UsingPostgreSQL = oldDB, oldPG })

			models := map[string]string{
				"only":         "gemini-2.5-flash",
				"longer-name":  "gemini-2.5-flash-image,gpt-4o",
				"last":         "gpt-4o,gemini-2.5-flash",
				"middle":       "a,gemini-2.5-flash,b",
				"first":        "gemini-2.5-flash,b",
				"dash":         "mj-imagine,z",
				"underscore":   "mj_imagine,x",
				"percent":      "a%b",
				"percent-fake": "axyzb,a-b",
				"bang":         "x!y,z",
				"bang-fake":    "xy,z",
			}
			for channelName, list := range models {
				if err := testDB.Create(&Channel{Name: channelName, Models: list, Key: "k-" + channelName, Status: 1}).Error; err != nil {
					t.Fatal(err)
				}
			}

			cases := map[string][]string{
				"gemini-2.5-flash": {"first", "last", "middle", "only"},
				"mj_imagine":       {"underscore"},
				"a%b":              {"percent"},
				"x!y":              {"bang"},
				"gemini-2.5":       {},
			}
			for search, want := range cases {
				result, err := GetChannelsList(&SearchChannelsParams{
					ExactModel:       search,
					PaginationParams: PaginationParams{Page: 1, Size: 100},
				})
				if err != nil {
					t.Fatalf("%s: %v", search, err)
				}
				got := []string{}
				for _, ch := range *result.Data {
					got = append(got, ch.Name)
				}
				slices.Sort(got)
				if !slices.Equal(got, want) || result.TotalCount != int64(len(want)) {
					t.Errorf("exact_model=%q: got %v (total %d), want %v", search, got, result.TotalCount, want)
				}
			}
		})
	}
}
