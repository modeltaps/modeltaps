package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestMigrateMailTemplateLanguages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Option{}); err != nil {
		t.Fatal(err)
	}
	db.Create(&Option{Key: "DefaultLanguage", Value: "zh_CN"})
	db.Create(&Option{Key: "MailTemplates", Value: `{"verification":{"subject":"S","body":""}}`})

	for round := 0; round < 2; round++ {
		if err := migrateMailTemplateLanguages().Migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	var got Option
	db.First(&got, "key = ?", "MailTemplates")
	if got.Value != `{"verification":{"zh_CN":{"subject":"S","body":""}}}` {
		t.Fatalf("MailTemplates = %s", got.Value)
	}
}
