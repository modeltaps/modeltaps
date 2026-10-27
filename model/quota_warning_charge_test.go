package model

import (
	"sync/atomic"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/gorm"
)

// countUserReads counts SELECTs on the users table from now on.
func countUserReads(t *testing.T) *atomic.Int32 {
	t.Helper()
	var n atomic.Int32
	name := "test:count_user_reads:" + t.Name()
	if err := DB.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "users" {
			n.Add(1)
		}
	}); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	t.Cleanup(func() { _ = DB.Callback().Query().Remove(name) })
	return &n
}

func setQuota(t *testing.T, userId, quota int) {
	t.Helper()
	if err := DB.Model(&User{}).Where("id = ?", userId).Update("quota", quota).Error; err != nil {
		t.Fatalf("set quota: %v", err)
	}
}

// The pre-charge warns only once the charge went through, with the balance after the charge.
func TestPreConsumeTokenQuota_WarnsAfterSuccessfulCharge(t *testing.T) {
	setupOrgBillingTestDB(t)
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	user := createOrgTestUser(t, "warn-pre-user")
	setQuota(t, user.Id, 500_200)

	// The token cannot pay: the charge fails and nothing is sent, although the balance would cross.
	poor := &Token{UserId: user.Id, Name: "poor", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10}
	DB.Create(poor)
	if err := PreConsumeTokenQuota(poor.Id, 300); err == nil {
		t.Fatal("pre-charge beyond the token quota should fail")
	}
	if got := sent(); len(got) != 0 {
		t.Fatalf("failed charge sent %+v", got)
	}

	tk := &Token{UserId: user.Id, Name: "ok", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 1_000_000}
	DB.Create(tk)
	if err := PreConsumeTokenQuota(tk.Id, 300); err != nil {
		t.Fatalf("pre-charge: %v", err)
	}
	if got := sent(); len(got) != 1 || got[0] != (sentWarning{user.Id, 499_900, false}) {
		t.Fatalf("sent %+v, want one running-low warning at 499900", got)
	}
}

// The final settlement is where the balance crosses when the pre-charge was skipped; it warns once,
// and further settlements below the threshold do not repeat it.
func TestPostConsumeTokenQuota_WarnsOnSettlement(t *testing.T) {
	setupOrgBillingTestDB(t)
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	user := createOrgTestUser(t, "warn-post-user")
	setQuota(t, user.Id, 520_000)
	tk := &Token{UserId: user.Id, Name: "k", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 1_000_000}
	DB.Create(tk)

	if err := PostConsumeTokenQuotaWithInfo(tk.Id, user.Id, false, 10_000, true); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got := sent(); len(got) != 0 {
		t.Fatalf("settlement above the threshold sent %+v", got)
	}
	for i := 0; i < 3; i++ {
		if err := PostConsumeTokenQuotaWithInfo(tk.Id, user.Id, false, 10_000, true); err != nil {
			t.Fatalf("settle: %v", err)
		}
	}
	if got := sent(); len(got) != 1 || got[0] != (sentWarning{user.Id, 500_000 - 10_000, false}) {
		t.Fatalf("sent %+v, want one running-low warning at 490000", got)
	}
	// A refund (negative settlement) never reads the balance for a warning.
	reads := countUserReads(t)
	if err := PostConsumeTokenQuotaWithInfo(tk.Id, user.Id, false, -500, true); err != nil {
		t.Fatalf("refund: %v", err)
	}
	if reads.Load() != 0 {
		t.Fatalf("refund read users %d times", reads.Load())
	}
}

// Organization tokens charge the shadow account, which has no mailbox: the charge paths skip the
// warning before any extra read or lock.
func TestQuotaWarning_SkipsOrganizationShadowAccount(t *testing.T) {
	setupOrgBillingTestDB(t)
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	owner := createOrgTestUser(t, "warn-org-owner")
	org, err := CreateOrganizationWithOwner("Warn Org", "", owner.Id)
	if err != nil {
		t.Fatalf("create org: %v", err)
	}
	setQuota(t, org.ShadowUserId, 500_200)
	tk := &Token{UserId: org.ShadowUserId, CreatedBy: owner.Id, Name: "org-key", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 1_000_000}
	DB.Create(tk)

	if err := PreConsumeTokenQuota(tk.Id, 300); err != nil {
		t.Fatalf("pre-charge: %v", err)
	}
	reads := countUserReads(t)
	if err := PostConsumeTokenQuotaWithInfo(tk.Id, org.ShadowUserId, false, 300, false); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if reads.Load() != 0 {
		t.Fatalf("organization settlement read users %d times, want 0", reads.Load())
	}
	// Even a caller that asks for the check finds the shadow account and sends nothing.
	if err := PostConsumeTokenQuotaWithInfo(tk.Id, org.ShadowUserId, false, 300, true); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got := sent(); len(got) != 0 {
		t.Fatalf("shadow account got warnings %+v", got)
	}
	quotaWarningLocalDedup.Lock()
	held := len(quotaWarningLocalDedup.expireAt)
	quotaWarningLocalDedup.Unlock()
	if held != 0 {
		t.Fatalf("shadow account took %d dedupe locks, want 0", held)
	}
}

// With quota warnings off, settlement does not read the balance at all.
func TestPostConsumeTokenQuota_RemindDisabledSkipsRead(t *testing.T) {
	setupOrgBillingTestDB(t)
	setupQuotaWarningTest(t)
	config.QuotaRemindEnabled = false
	sent := captureQuotaWarningEmails(t)
	user := createOrgTestUser(t, "warn-off-user")
	setQuota(t, user.Id, 500_200)
	tk := &Token{UserId: user.Id, Name: "k", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 1_000_000}
	DB.Create(tk)

	reads := countUserReads(t)
	if err := PostConsumeTokenQuotaWithInfo(tk.Id, user.Id, false, 300, true); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if reads.Load() != 0 || len(sent()) != 0 {
		t.Fatalf("reads=%d sent=%+v, want none", reads.Load(), sent())
	}
	// The counter does see the read once the switch is back on.
	config.QuotaRemindEnabled = true
	if err := PostConsumeTokenQuotaWithInfo(tk.Id, user.Id, false, 300, true); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if reads.Load() != 1 {
		t.Fatalf("settlement with warnings on read users %d times, want 1", reads.Load())
	}
}
