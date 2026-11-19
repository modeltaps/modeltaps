package controller

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Users of the SEC-51 tests: the owner, an admin and a member of organization 1, whose shadow
// account is orgLogIOShadow.
const (
	orgLogIOOwner  = 1
	orgLogIOAdmin  = 5
	orgLogIOMember = 7
	orgLogIOShadow = 11
)

func setupOrgLogIOTest(t *testing.T) (*gorm.DB, *model.Organization) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if viper.GetString("user_token_secret") == "" {
		viper.Set("user_token_secret", "org-token-test-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatal(err)
		}
	}
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.Organization{}, &model.OrganizationAuditLog{}, &model.Token{},
		&model.Log{}, &model.LogDetail{}); err != nil {
		t.Fatal(err)
	}
	prevDB, prevEnabled := model.DB, config.LogIOEnabled
	model.DB = testDB
	config.LogIOEnabled = true
	t.Cleanup(func() {
		model.DB, config.LogIOEnabled = prevDB, prevEnabled
	})

	for _, u := range []*model.User{
		{Id: orgLogIOOwner, Username: "olivia", AffCode: "a1", Group: "default"},
		{Id: orgLogIOAdmin, Username: "alice", AffCode: "a5", Group: "default"},
		{Id: orgLogIOMember, Username: "max", AffCode: "a7", Group: "default"},
		{Id: orgLogIOShadow, Username: "shadow1", AffCode: "a11", Group: "default"},
	} {
		if err := testDB.Create(u).Error; err != nil {
			t.Fatal(err)
		}
	}
	org := &model.Organization{Id: 1, Name: "Org", Slug: "org1", Status: model.OrganizationStatusEnabled, ShadowUserId: orgLogIOShadow, CreatedBy: orgLogIOOwner}
	if err := testDB.Create(org).Error; err != nil {
		t.Fatal(err)
	}
	return testDB, org
}

// orgLogIOAs runs the handler as userId with the given organization role.
func orgLogIOAs(org *model.Organization, userId int, role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("id", userId)
		c.Set("role", config.RoleCommonUser)
		c.Set(middleware.OrgContextKey, org)
		c.Set(middleware.OrgRoleContextKey, role)
		c.Next()
	}
}

func orgLogIOCount(t *testing.T, db *gorm.DB, action string) int64 {
	t.Helper()
	var n int64
	if err := db.Model(&model.OrganizationAuditLog{}).Where("action = ?", action).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

// Turning on request and response retention for the organization is recorded under its own audit
// action, and every member's API keys page then says the bodies are stored, by whom and since
// when (SEC-51).
func TestOrgLogIODefaultIsDisclosedToMembers(t *testing.T) {
	db, org := setupOrgLogIOTest(t)
	r := gin.New()
	r.PUT("/api/org/:id", orgLogIOAs(org, orgLogIOAdmin, model.OrgRoleAdmin), UpdateOrganization)
	r.GET("/api/org/:id/token", orgLogIOAs(org, orgLogIOMember, model.OrgRoleMember), GetOrgTokensList)

	disclosure := func() map[string]any {
		t.Helper()
		code, body := orgRoutingCall(t, r, http.MethodGet, "/api/org/1/token?p=1&size=10", nil)
		if code != http.StatusOK || body["success"] != true {
			t.Fatalf("list failed: %d %v", code, body)
		}
		status, _ := body["log_io"].(map[string]any)
		if status == nil {
			t.Fatalf("list has no log_io disclosure: %v", body)
		}
		return status
	}

	if got := disclosure(); got["stored"] != false {
		t.Fatalf("retention is off, disclosure says %v", got)
	}

	before := time.Now().Unix()
	code, body := orgRoutingCall(t, r, http.MethodPut, "/api/org/1", map[string]any{"name": "Org", "log_io_default": "on"})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("update failed: %d %v", code, body)
	}
	if n := orgLogIOCount(t, db, model.OrgAuditActionLogIOUpdate); n != 1 {
		t.Fatalf("turning retention on recorded %d %s entries, want 1", n, model.OrgAuditActionLogIOUpdate)
	}
	got := disclosure()
	if got["stored"] != true || got["source"] != "organization" || got["changed_by"] != "alice" ||
		got["changed_by_id"] != float64(orgLogIOAdmin) || got["retention_days"] != float64(model.LogIORetentionDays) {
		t.Fatalf("disclosure after turning retention on = %v", got)
	}
	if at, _ := got["changed_at"].(float64); int64(at) < before {
		t.Fatalf("changed_at %v is before the change at %d", got["changed_at"], before)
	}

	// The site switch off stores nothing, whatever the organization chose.
	config.LogIOEnabled = false
	if got := disclosure(); got["stored"] != false {
		t.Fatalf("site switch off, disclosure says %v", got)
	}
	config.LogIOEnabled = true

	code, body = orgRoutingCall(t, r, http.MethodPut, "/api/org/1", map[string]any{"name": "Org", "log_io_default": "off"})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("update failed: %d %v", code, body)
	}
	if got := disclosure(); got["stored"] != false {
		t.Fatalf("retention turned off, disclosure says %v", got)
	}

	// Clearing the choice keeps no bodies: there is no site default to follow.
	code, _ = orgRoutingCall(t, r, http.MethodPut, "/api/org/1", map[string]any{"name": "Org", "log_io_default": "inherit"})
	if code != http.StatusOK {
		t.Fatalf("update failed: %d", code)
	}
	if got := disclosure(); got["stored"] != false {
		t.Fatalf("no choice, disclosure says %v", got)
	}
}

// The detail of one organization API key carries the same disclosure.
func TestOrgTokenDetailDisclosesLogIO(t *testing.T) {
	db, org := setupOrgLogIOTest(t)
	on := true
	org.Setting.Set(model.OrganizationSetting{LogIODefault: &on})
	if err := org.UpdateSetting(); err != nil {
		t.Fatal(err)
	}
	model.RecordOrgAudit(org.Id, orgLogIOOwner, model.OrgAuditActionLogIOUpdate, "Set request and response retention of organization API keys to on")
	token := &model.Token{Id: 3, UserId: orgLogIOShadow, CreatedBy: orgLogIOMember, Name: "k", KeyHash: "h3"}
	if err := db.Create(token).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/api/org/:id/token/:token_id", orgLogIOAs(org, orgLogIOMember, model.OrgRoleMember), GetOrgToken)
	code, body := orgRoutingCall(t, r, http.MethodGet, "/api/org/1/token/3", nil)
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("detail failed: %d %v", code, body)
	}
	status, _ := body["log_io"].(map[string]any)
	if status == nil || status["stored"] != true || status["changed_by"] != "olivia" {
		t.Fatalf("detail disclosure = %v", body["log_io"])
	}
}

// The key-level log_io is not taken from requests: retention follows the organization default
// only, so an owner or admin cannot turn it on for one member's key without the disclosure.
func TestOrgTokenIgnoresKeyLevelLogIO(t *testing.T) {
	db, org := setupOrgLogIOTest(t)
	token := &model.Token{Id: 3, UserId: orgLogIOShadow, CreatedBy: orgLogIOMember, Name: "k", KeyHash: "h3", ExpiredTime: -1, UnlimitedQuota: true, Status: config.TokenStatusEnabled}
	if err := db.Create(token).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.PUT("/api/org/:id/token", orgLogIOAs(org, orgLogIOOwner, model.OrgRoleOwner), UpdateOrgToken)
	r.POST("/api/org/:id/token", orgLogIOAs(org, orgLogIOOwner, model.OrgRoleOwner), AddOrgToken)

	code, body := orgRoutingCall(t, r, http.MethodPut, "/api/org/1/token", map[string]any{
		"id": 3, "name": "k", "expired_time": -1, "unlimited_quota": true, "status": config.TokenStatusEnabled, "log_io": true,
	})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("update failed: %d %v", code, body)
	}
	var stored model.Token
	if err := db.First(&stored, 3).Error; err != nil {
		t.Fatal(err)
	}
	if stored.LogIO != nil {
		t.Fatalf("update stored log_io = %v, want it ignored", *stored.LogIO)
	}

	code, body = orgRoutingCall(t, r, http.MethodPost, "/api/org/1/token", map[string]any{"name": "new", "expired_time": -1, "unlimited_quota": true, "log_io": true})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("create failed: %d %v", code, body)
	}
	var created model.Token
	if err := db.Where("name = ?", "new").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	if created.LogIO != nil {
		t.Fatalf("create stored log_io = %v, want it ignored", *created.LogIO)
	}
}

// Only the owner reads a member's stored request and response, and each read is audited; an admin
// is refused (SEC-51).
func TestGetOrgLogDetailOwnerOnlyAndAudited(t *testing.T) {
	db, org := setupOrgLogIOTest(t)
	details := []*model.LogDetail{
		{LogId: 1, TokenId: 3, UserId: orgLogIOShadow, CreatedBy: orgLogIOMember, RequestBody: "req", ResponseBody: "resp", CreatedAt: time.Now().Unix()},
		{LogId: 2, TokenId: 4, UserId: orgLogIOShadow, CreatedBy: orgLogIOOwner, RequestBody: "own", ResponseBody: "own", CreatedAt: time.Now().Unix()},
	}
	if err := db.Create(&details).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.GET("/owner/:id/logs/detail/:log_id", orgLogIOAs(org, orgLogIOOwner, model.OrgRoleOwner), GetOrgLogDetail)
	r.GET("/admin/:id/logs/detail/:log_id", orgLogIOAs(org, orgLogIOAdmin, model.OrgRoleAdmin), GetOrgLogDetail)

	code, body := orgRoutingCall(t, r, http.MethodGet, "/admin/1/logs/detail/1", nil)
	if code != http.StatusForbidden {
		t.Fatalf("admin read a member's body: %d %v", code, body)
	}
	if n := orgLogIOCount(t, db, model.OrgAuditActionLogIOView); n != 0 {
		t.Fatalf("a refused read was audited (%d entries)", n)
	}

	code, body = orgRoutingCall(t, r, http.MethodGet, "/owner/1/logs/detail/1", nil)
	data, _ := body["data"].(map[string]any)
	if code != http.StatusOK || data["request_body"] != "req" {
		t.Fatalf("owner could not read the body: %d %v", code, body)
	}
	var entry model.OrganizationAuditLog
	if err := db.Where("action = ?", model.OrgAuditActionLogIOView).First(&entry).Error; err != nil {
		t.Fatalf("owner's read was not audited: %v", err)
	}
	if entry.ActorId != orgLogIOOwner || entry.OrganizationId != org.Id || entry.Content != fmt.Sprintf("Read the stored request and response of log 1 (API key ID: 3, created by user ID: %d)", orgLogIOMember) {
		t.Fatalf("audit entry = %+v", entry)
	}

	// Reading one's own request is not someone else's prompt: no entry.
	if code, _ = orgRoutingCall(t, r, http.MethodGet, "/owner/1/logs/detail/2", nil); code != http.StatusOK {
		t.Fatalf("owner could not read their own body: %d", code)
	}
	if n := orgLogIOCount(t, db, model.OrgAuditActionLogIOView); n != 1 {
		t.Fatalf("reading one's own body was audited (%d entries)", n)
	}
}
