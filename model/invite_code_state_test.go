package model

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestInviteCodeStateSegments checks that the state filter splits codes into non-overlapping
// segments and that the counts add up.
func TestInviteCodeStateSegments(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "invite.db")), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&InviteCode{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })

	now := time.Now().Unix()
	codes := map[string]*InviteCode{
		"usable":    {Code: "USABLE", Name: "seg", Status: InviteCodeStatusEnabled},
		"pending":   {Code: "PENDING", Name: "seg", Status: InviteCodeStatusEnabled, StartsAt: now + 3600},
		"expired":   {Code: "EXPIRED", Name: "seg", Status: InviteCodeStatusEnabled, ExpiresAt: now - 3600},
		"exhausted": {Code: "EXHAUSTED", Name: "seg", Status: InviteCodeStatusEnabled, MaxUses: 2, UsedCount: 2},
		"disabled":  {Code: "DISABLED", Name: "seg", Status: InviteCodeStatusDisabled, ExpiresAt: now - 3600},
	}
	for _, c := range codes {
		c.CreatedTime, c.UpdatedTime = now, now
		if err := DB.Create(c).Error; err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	counts, err := GetInviteCodeStateCounts("seg")
	if err != nil {
		t.Fatalf("counts: %v", err)
	}
	want := InviteCodeStateCounts{All: 5, Usable: 1, Pending: 1, Expired: 1, Exhausted: 1, Disabled: 1}
	if *counts != want {
		t.Fatalf("counts = %+v, want %+v", *counts, want)
	}

	for state, code := range codes {
		result, err := GetInviteCodesList(&InviteCodeSearchParams{PaginationParams: PaginationParams{Page: 1, Size: 10}, State: state})
		if err != nil {
			t.Fatalf("list %s: %v", state, err)
		}
		if len(*result.Data) != 1 || (*result.Data)[0].Code != code.Code {
			t.Fatalf("state %s returned %d rows", state, len(*result.Data))
		}
	}
}
