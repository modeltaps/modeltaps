package controller

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	billingTestPersonalUser = 1
	billingTestShadowUser   = 2
	billingTestMember       = 3
	billingTestOrgId        = 1
)

func setupBillingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.Token{}, &model.Organization{}, &model.OrganizationMember{}); err != nil {
		t.Fatalf("migrate test tables: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	oldDisplayTokenStat, oldDisplayInCurrency := config.DisplayTokenStatEnabled, config.DisplayInCurrencyEnabled
	config.DisplayInCurrencyEnabled = false
	t.Cleanup(func() {
		model.DB = oldDB
		config.DisplayTokenStatEnabled, config.DisplayInCurrencyEnabled = oldDisplayTokenStat, oldDisplayInCurrency
	})

	users := []model.User{
		{Id: billingTestPersonalUser, Username: "alice", AffCode: "aff1", Quota: 5000, UsedQuota: 700},
		// The shadow account holds the organization pool and the whole organization's usage.
		{Id: billingTestShadowUser, Username: "org-acme", AffCode: "aff2", Type: config.UserTypeOrgShadow, Quota: 900000, UsedQuota: 400000},
		{Id: billingTestMember, Username: "bob", AffCode: "aff3", Quota: 10, UsedQuota: 0},
	}
	for i := range users {
		if err := testDB.Create(&users[i]).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	if err := testDB.Create(&model.Organization{Id: billingTestOrgId, Name: "Acme", Slug: "acme", ShadowUserId: billingTestShadowUser}).Error; err != nil {
		t.Fatalf("create organization: %v", err)
	}
	if err := testDB.Create(&model.OrganizationMember{OrganizationId: billingTestOrgId, UserId: billingTestMember, Role: model.OrgRoleMember}).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	return testDB
}

func createBillingTestToken(t *testing.T, token model.Token) *model.Token {
	t.Helper()
	if err := model.DB.Create(&token).Error; err != nil {
		t.Fatalf("create token: %v", err)
	}
	return &token
}

func callBilling(t *testing.T, handler gin.HandlerFunc, token *model.Token) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/v1/dashboard/billing/subscription", nil)
	// The same keys middleware.OpenaiAuth sets from the API key.
	c.Set("id", token.UserId)
	c.Set("token_id", token.Id)
	handler(c)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response %q: %v", w.Body.String(), err)
	}
	return resp
}

func billingFigures(t *testing.T, token *model.Token) (hardLimit, totalUsage float64) {
	t.Helper()
	sub := callBilling(t, GetSubscription, token)
	usage := callBilling(t, GetUsage, token)
	hardLimit, ok := sub["hard_limit_usd"].(float64)
	if !ok {
		t.Fatalf("subscription response has no hard_limit_usd: %v", sub)
	}
	totalUsage, ok = usage["total_usage"].(float64)
	if !ok {
		t.Fatalf("usage response has no total_usage: %v", usage)
	}
	return hardLimit, totalUsage
}

// SEC-74: an organization API key must not read the organization pool balance or the whole
// organization's usage (the shadow account's quota and used_quota) through the billing endpoints.
func TestBillingOrgKeyDoesNotExposePool(t *testing.T) {
	for _, displayTokenStat := range []bool{true, false} {
		setupBillingTestDB(t)
		config.DisplayTokenStatEnabled = displayTokenStat

		unlimited := createBillingTestToken(t, model.Token{UserId: billingTestShadowUser, CreatedBy: billingTestMember,
			Key: "unlimited", Name: "unlimited", UnlimitedQuota: true, UsedQuota: 120})
		limit, used := billingFigures(t, unlimited)
		if limit != 120 || used != 120*100 {
			t.Fatalf("display_token_stat=%v: unlimited org key without a member budget should report only its own usage, got limit %v usage %v",
				displayTokenStat, limit, used)
		}

		limited := createBillingTestToken(t, model.Token{UserId: billingTestShadowUser, CreatedBy: billingTestMember,
			Key: "limited", Name: "limited", RemainQuota: 300, UsedQuota: 50, ExpiredTime: 1900000000})
		limit, used = billingFigures(t, limited)
		if limit != 350 || used != 50*100 {
			t.Fatalf("display_token_stat=%v: limited org key should report its own limit, got limit %v usage %v",
				displayTokenStat, limit, used)
		}
		if sub := callBilling(t, GetSubscription, limited); sub["access_until"].(float64) != 1900000000 {
			t.Fatalf("limited org key should report its expiry, got %v", sub["access_until"])
		}
	}
}

// An unlimited organization API key reports the creating member's own period budget, the same
// figures the member sees in the console.
func TestBillingOrgKeyReportsMemberBudget(t *testing.T) {
	db := setupBillingTestDB(t)
	member := model.OrganizationMember{}
	if err := db.First(&member, "organization_id = ? AND user_id = ?", billingTestOrgId, billingTestMember).Error; err != nil {
		t.Fatalf("load member: %v", err)
	}
	member.Setting.Set(model.OrgMemberSetting{Budget: &model.QuotaResetSetting{Period: model.TokenQuotaResetPeriodMonthly, Limit: 1000}})
	member.BudgetUsed = 250
	member.BudgetStart = time.Now().Unix()
	if err := db.Save(&member).Error; err != nil {
		t.Fatalf("save member budget: %v", err)
	}

	token := createBillingTestToken(t, model.Token{UserId: billingTestShadowUser, CreatedBy: billingTestMember,
		Key: "unlimited", Name: "unlimited", UnlimitedQuota: true, UsedQuota: 120})
	limit, used := billingFigures(t, token)
	if limit != 1000 || used != 250*100 {
		t.Fatalf("unlimited org key should report the member budget, got limit %v usage %v", limit, used)
	}
}

// Personal keys keep the previous behaviour: an unlimited key, or any key while per-key stats are
// off, reports the account; a limited key with per-key stats on reports the key.
func TestBillingPersonalKeyUnchanged(t *testing.T) {
	setupBillingTestDB(t)
	unlimited := createBillingTestToken(t, model.Token{UserId: billingTestPersonalUser, Key: "p-unlimited", Name: "p-unlimited",
		UnlimitedQuota: true, UsedQuota: 30})
	limited := createBillingTestToken(t, model.Token{UserId: billingTestPersonalUser, Key: "p-limited", Name: "p-limited",
		RemainQuota: 80, UsedQuota: 20})

	config.DisplayTokenStatEnabled = true
	if limit, used := billingFigures(t, unlimited); limit != 5700 || used != 700*100 {
		t.Fatalf("unlimited personal key should report the account, got limit %v usage %v", limit, used)
	}
	if limit, used := billingFigures(t, limited); limit != 100 || used != 20*100 {
		t.Fatalf("limited personal key should report the key, got limit %v usage %v", limit, used)
	}

	config.DisplayTokenStatEnabled = false
	if limit, used := billingFigures(t, limited); limit != 5700 || used != 700*100 {
		t.Fatalf("with per-key stats off a personal key should report the account, got limit %v usage %v", limit, used)
	}
}
