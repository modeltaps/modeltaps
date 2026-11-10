package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func useOptionTestDB(t *testing.T, migrate bool) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if migrate {
		if err := testDB.AutoMigrate(&Option{}); err != nil {
			t.Fatal(err)
		}
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

var updateOptionTestValue = 1

func init() {
	config.GlobalOption.RegisterInt("UpdateOptionTestValue", &updateOptionTestValue, config.ScopeAdmin)
}

// A failed database write is reported and leaves the running value untouched, so an option never
// looks saved on one node while the database still holds the old value.
func TestUpdateOptionKeepsRunningValueWhenSaveFails(t *testing.T) {
	useOptionTestDB(t, false) // no options table: every write fails
	updateOptionTestValue = 1
	t.Cleanup(func() { updateOptionTestValue = 1 })

	if err := UpdateOption("UpdateOptionTestValue", "7"); err == nil {
		t.Fatal("UpdateOption returned nil although the database write failed")
	}
	if updateOptionTestValue != 1 {
		t.Fatalf("running value = %d after a failed save, want 1", updateOptionTestValue)
	}
}

func TestUpdateOptionSavesAndApplies(t *testing.T) {
	useOptionTestDB(t, true)
	updateOptionTestValue = 1
	t.Cleanup(func() { updateOptionTestValue = 1 })

	if err := UpdateOption("UpdateOptionTestValue", "7"); err != nil {
		t.Fatal(err)
	}
	if updateOptionTestValue != 7 {
		t.Fatalf("running value = %d, want 7", updateOptionTestValue)
	}
	var stored Option
	if err := DB.First(&stored, "key = ?", "UpdateOptionTestValue").Error; err != nil {
		t.Fatal(err)
	}
	if stored.Value != "7" {
		t.Fatalf("stored value = %q, want 7", stored.Value)
	}
}
