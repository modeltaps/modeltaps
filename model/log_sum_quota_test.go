package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// The user's own usage total is filtered by user_id, so logs written under a previous username
// still count and another user's logs never do.
func TestSumUsedQuota_FiltersByUserId(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&Log{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })

	seed := []*Log{
		{UserId: 7, Username: "alice", Type: LogTypeConsume, Quota: 100, CreatedAt: 1000},
		{UserId: 7, Username: "alice-renamed", Type: LogTypeConsume, Quota: 20, CreatedAt: 2000},
		{UserId: 7, Username: "alice-renamed", Type: LogTypeTopup, Quota: 5000, CreatedAt: 2000},
		{UserId: 8, Username: "alice", Type: LogTypeConsume, Quota: 3, CreatedAt: 2000},
	}
	if err := DB.Create(seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	if got := SumUsedQuota(&LogsListParams{UserId: 7}); got != 120 {
		t.Fatalf("SumUsedQuota(user 7) = %d, want 120", got)
	}
	if got := SumUsedQuota(&LogsListParams{UserId: 7, StartTimestamp: 1500}); got != 20 {
		t.Fatalf("SumUsedQuota(user 7, from 1500) = %d, want 20", got)
	}
	if got := SumUsedQuota(&LogsListParams{UserId: 8}); got != 3 {
		t.Fatalf("SumUsedQuota(user 8) = %d, want 3", got)
	}
}
