package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/database"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func intPtr(i int) *int { return &i }

func setupQuotaWarningSettingDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open memory db: %v", err)
	}
	if err := testDB.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

func createQuotaWarningUser(t *testing.T, id int, quota int, userType int, setting UserSetting) {
	t.Helper()
	var value database.JSONType[UserSetting]
	value.Set(setting)
	user := User{Id: id, Username: fmt.Sprintf("u%d", id), AccessToken: fmt.Sprintf("at%d", id), AffCode: fmt.Sprintf("af%d", id), Quota: quota, Type: userType, Setting: value}
	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
}

func TestUserSettingQuotaRemindUpdate(t *testing.T) {
	origThreshold := config.QuotaRemindThreshold
	config.QuotaRemindThreshold = 500_000
	t.Cleanup(func() { config.QuotaRemindThreshold = origThreshold })

	var s UserSetting
	if !s.QuotaRemindAllowed() || s.EffectiveQuotaRemindThreshold() != 500_000 {
		t.Fatalf("unset setting should be on with the site threshold, got %v %d", s.QuotaRemindAllowed(), s.EffectiveQuotaRemindThreshold())
	}
	if err := s.ApplyUpdate(UserSettingUpdate{QuotaRemindEnabled: boolPtr(false), QuotaRemindThreshold: intPtr(2_000_000)}); err != nil {
		t.Fatal(err)
	}
	if s.QuotaRemindAllowed() || s.EffectiveQuotaRemindThreshold() != 2_000_000 {
		t.Fatalf("update not applied: %+v", s)
	}
	// Other fields leave the reminder setting alone.
	if err := s.ApplyUpdate(UserSettingUpdate{Theme: strPtr("dark")}); err != nil {
		t.Fatal(err)
	}
	if s.QuotaRemindAllowed() || s.QuotaRemindThreshold == nil {
		t.Fatalf("partial update cleared the reminder setting: %+v", s)
	}
	// A negative threshold resets to the site default; zero is kept (only "used up" warnings).
	if err := s.ApplyUpdate(UserSettingUpdate{QuotaRemindThreshold: intPtr(-1)}); err != nil {
		t.Fatal(err)
	}
	if s.QuotaRemindThreshold != nil || s.EffectiveQuotaRemindThreshold() != 500_000 {
		t.Fatalf("negative threshold should reset to the site default, got %+v", s)
	}
	if err := s.ApplyUpdate(UserSettingUpdate{QuotaRemindThreshold: intPtr(0)}); err != nil {
		t.Fatal(err)
	}
	if s.EffectiveQuotaRemindThreshold() != 0 {
		t.Fatalf("zero threshold should be kept, got %d", s.EffectiveQuotaRemindThreshold())
	}

	if err := (&UserSetting{QuotaRemindThreshold: intPtr(maxQuotaRemindThreshold + 1)}).Validate(); err == nil {
		t.Fatal("an oversized threshold should be rejected")
	}
	if err := (&UserSetting{QuotaRemindThreshold: intPtr(maxQuotaRemindThreshold)}).Validate(); err != nil {
		t.Fatalf("the largest threshold should pass, got %v", err)
	}

	view := (&UserSetting{}).APIView()
	if view["quota_remind_enabled"] != true || view["quota_remind_threshold"] != (*int)(nil) || view["site_quota_remind_threshold"] != 500_000 {
		t.Fatalf("unexpected api view %+v", view)
	}
}

func TestLoadQuotaBeforeSettlement_UserSetting(t *testing.T) {
	setupQuotaWarningTest(t)
	setupQuotaWarningSettingDB(t)
	createQuotaWarningUser(t, 1, 900_000, 0, UserSetting{})
	createQuotaWarningUser(t, 2, 900_000, 0, UserSetting{QuotaRemindThreshold: intPtr(2_000_000)})
	createQuotaWarningUser(t, 3, 900_000, 0, UserSetting{QuotaRemindEnabled: boolPtr(false)})
	createQuotaWarningUser(t, 4, 900_000, config.UserTypeOrgShadow, UserSetting{})

	if q, th, ok := loadQuotaBeforeSettlement(1, true); !ok || q != 900_000 || th != 500_000 {
		t.Fatalf("unset user should use the site threshold, got %d %d %v", q, th, ok)
	}
	if _, th, ok := loadQuotaBeforeSettlement(2, true); !ok || th != 2_000_000 {
		t.Fatalf("personal threshold not used, got %d %v", th, ok)
	}
	if _, _, ok := loadQuotaBeforeSettlement(3, true); ok {
		t.Fatal("user who turned warnings off should be skipped")
	}
	if _, _, ok := loadQuotaBeforeSettlement(4, true); ok {
		t.Fatal("shadow account should be skipped")
	}
	if _, _, ok := loadQuotaBeforeSettlement(1, false); ok {
		t.Fatal("organization token should be skipped")
	}

	if th, ok := loadQuotaRemindThreshold(2); !ok || th != 2_000_000 {
		t.Fatalf("pre-charge path should read the personal threshold, got %d %v", th, ok)
	}
	if _, ok := loadQuotaRemindThreshold(3); ok {
		t.Fatal("pre-charge path should skip a user who turned warnings off")
	}

	config.QuotaRemindEnabled = false
	if _, _, ok := loadQuotaBeforeSettlement(2, true); ok {
		t.Fatal("the site switch should win over the personal setting")
	}
	if _, ok := loadQuotaRemindThreshold(2); ok {
		t.Fatal("the site switch should win on the pre-charge path")
	}
}

// A personal threshold above the site one sends "running low" where the site threshold would not.
func TestNotifyQuotaWarning_PersonalThreshold(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)

	notifyQuotaWarningIfNeeded(20, 1_000_200, 300, config.QuotaRemindThreshold)
	if got := sent(); len(got) != 0 {
		t.Fatalf("site threshold should not fire at 999900, got %+v", got)
	}
	notifyQuotaWarningIfNeeded(20, 1_000_200, 300, 1_000_000)
	if got := sent(); len(got) != 1 || got[0].noMoreQuota || got[0].quota != 999_900 {
		t.Fatalf("personal threshold should send one low warning, got %+v", got)
	}
}

// Changing the setting releases the "sent" lock, so the new threshold can warn at once.
func TestResetQuotaWarning(t *testing.T) {
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)
	advance := fakeQuotaWarningClock(t)

	notifyQuotaWarningIfNeeded(21, 500_200, 300, 500_000)
	advance(11 * time.Minute)
	notifyQuotaWarningIfNeeded(21, 499_900, 300, 1_000_000)
	if got := sent(); len(got) != 1 {
		t.Fatalf("held lock should suppress the second warning, got %+v", got)
	}
	ResetQuotaWarning(21)
	notifyQuotaWarningIfNeeded(21, 499_600, 300, 1_000_000)
	if got := sent(); len(got) != 2 {
		t.Fatalf("reset should allow a new warning, got %+v", got)
	}
}

// On the charge paths a personal threshold applies, the settlement still reads users once, and a
// user who turned warnings off gets nothing from either path.
func TestChargePaths_UsePersonalQuotaRemindSetting(t *testing.T) {
	setupOrgBillingTestDB(t)
	setupQuotaWarningTest(t)
	sent := captureQuotaWarningEmails(t)

	user := createOrgTestUser(t, "warn-personal-user")
	setQuota(t, user.Id, 1_020_000)
	if err := UpdateUserSettingById(user.Id, UserSetting{QuotaRemindThreshold: intPtr(1_000_000)}); err != nil {
		t.Fatalf("save setting: %v", err)
	}
	tk := &Token{UserId: user.Id, Name: "k", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10_000_000}
	DB.Create(tk)
	reads := countUserReads(t)
	if err := PostConsumeTokenQuotaWithInfo(tk.Id, user.Id, false, 30_000, true); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got := sent(); len(got) != 1 || got[0] != (sentWarning{user.Id, 990_000, false}) {
		t.Fatalf("sent %+v, want one running-low warning at 990000 from the personal threshold", got)
	}
	if reads.Load() != 1 {
		t.Fatalf("settlement read users %d times, want 1", reads.Load())
	}

	off := createOrgTestUser(t, "warn-off-user")
	setQuota(t, off.Id, 500_200)
	if err := UpdateUserSettingById(off.Id, UserSetting{QuotaRemindEnabled: boolPtr(false)}); err != nil {
		t.Fatalf("save setting: %v", err)
	}
	offToken := &Token{UserId: off.Id, Name: "off", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 10_000_000}
	DB.Create(offToken)
	if err := PreConsumeTokenQuota(offToken.Id, 300); err != nil {
		t.Fatalf("pre-charge: %v", err)
	}
	if err := PostConsumeTokenQuotaWithInfo(offToken.Id, off.Id, false, 500_000, true); err != nil {
		t.Fatalf("settle: %v", err)
	}
	if got := sent(); len(got) != 1 {
		t.Fatalf("user with warnings off got %+v", got[1:])
	}
}
