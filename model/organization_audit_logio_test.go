package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GetLatestOrgLogIOUpdate finds the newest change of the LogIO default, recorded under its own
// action or, before SEC-51, as an org.update_setting entry, and ignores other settings and other
// organizations.
func TestGetLatestOrgLogIOUpdate(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&User{}, &OrganizationAuditLog{}); err != nil {
		t.Fatal(err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
	for _, u := range []*User{{Id: 1, Username: "olivia", AffCode: "a1"}, {Id: 2, Username: "alice", AffCode: "a2"}} {
		if err := testDB.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}

	if got, err := GetLatestOrgLogIOUpdate(1); err != nil || got != nil {
		t.Fatalf("no entries: got %+v, %v", got, err)
	}

	entries := []*OrganizationAuditLog{
		{OrganizationId: 1, ActorId: 1, Action: "org.update_setting", Content: "Updated organization API key LogIO default: on", CreatedTime: 100},
		{OrganizationId: 1, ActorId: 2, Action: "org.update_setting", Content: "Updated member usage visibility: true", CreatedTime: 200},
		{OrganizationId: 2, ActorId: 2, Action: OrgAuditActionLogIOUpdate, Content: "other organization", CreatedTime: 300},
	}
	if err := testDB.Create(&entries).Error; err != nil {
		t.Fatal(err)
	}
	got, err := GetLatestOrgLogIOUpdate(1)
	if err != nil || got == nil || got.ActorId != 1 || got.ActorName != "olivia" || got.CreatedTime != 100 {
		t.Fatalf("legacy entry: got %+v, %v", got, err)
	}

	RecordOrgAudit(1, 2, OrgAuditActionLogIOUpdate, "Set request and response retention of organization API keys to on (bodies stored: true)")
	got, err = GetLatestOrgLogIOUpdate(1)
	if err != nil || got == nil || got.ActorId != 2 || got.ActorName != "alice" || got.Action != OrgAuditActionLogIOUpdate {
		t.Fatalf("newest entry: got %+v, %v", got, err)
	}
}
