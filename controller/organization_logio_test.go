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

// Locking every key on is recorded under its own audit action, and every member's API keys page
// then says the bodies are stored, by whom and since when (SEC-51).
func TestOrgLogIOPolicyIsDisclosedToMembers(t *testing.T) {
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

	if got := disclosure(); got["stored"] != false || got["policy"] != model.LogIOPolicyOff {
		t.Fatalf("members choose, disclosure says %v", got)
	}

	before := time.Now().Unix()
	code, body := orgRoutingCall(t, r, http.MethodPut, "/api/org/1", map[string]any{"name": "Org", "log_io_policy": "on"})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("update failed: %d %v", code, body)
	}
	if n := orgLogIOCount(t, db, model.OrgAuditActionLogIOUpdate); n != 1 {
		t.Fatalf("locking retention on recorded %d %s entries, want 1", n, model.OrgAuditActionLogIOUpdate)
	}
	got := disclosure()
	if got["stored"] != true || got["policy"] != "on" || got["source"] != "organization" || got["changed_by"] != "alice" ||
		got["changed_by_id"] != float64(orgLogIOAdmin) || got["retention_days"] != float64(model.LogIORetentionDays) {
		t.Fatalf("disclosure after locking retention on = %v", got)
	}
	if at, _ := got["changed_at"].(float64); int64(at) < before {
		t.Fatalf("changed_at %v is before the change at %d", got["changed_at"], before)
	}

	// The site switch off stores nothing, whatever the organization chose.
	config.LogIOEnabled = false
	if got := disclosure(); got["stored"] != false || got["policy"] != nil {
		t.Fatalf("site switch off, disclosure says %v", got)
	}
	config.LogIOEnabled = true

	code, body = orgRoutingCall(t, r, http.MethodPut, "/api/org/1", map[string]any{"name": "Org", "log_io_policy": "off"})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("update failed: %d %v", code, body)
	}
	if got := disclosure(); got["stored"] != false || got["policy"] != "off" {
		t.Fatalf("retention off, disclosure says %v", got)
	}

	code, body = orgRoutingCall(t, r, http.MethodPut, "/api/org/1", map[string]any{"name": "Org", "log_io_policy": "inherit"})
	if code != http.StatusOK || body["success"] == true {
		t.Fatalf("an unknown policy was accepted: %d %v", code, body)
	}
}

// The detail of a key carries the policy's disclosure while it locks every key on.
func TestOrgTokenDetailDisclosesLockedPolicy(t *testing.T) {
	db, org := setupOrgLogIOTest(t)
	org.Setting.Set(model.OrganizationSetting{LogIOPolicy: model.LogIOPolicyOn})
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
	if status == nil || status["stored"] != true || status["source"] != "organization" || status["changed_by"] != "olivia" {
		t.Fatalf("detail disclosure = %v", body["log_io"])
	}
}

// Under "selected", the keys an admin ticks in the organization settings keep their bodies: each
// change is audited, and the key's page tells its member who selected it and when (ORG-10, SEC-51).
// Key creation and edits never change the selection.
func TestOrgLogIOSelection(t *testing.T) {
	db, org := setupOrgLogIOTest(t)
	org.Setting.Set(model.OrganizationSetting{LogIOPolicy: model.LogIOPolicySelected})
	if err := org.UpdateSetting(); err != nil {
		t.Fatal(err)
	}
	token := &model.Token{Id: 3, UserId: orgLogIOShadow, CreatedBy: orgLogIOMember, Name: "k", KeyHash: "h3", ExpiredTime: -1, UnlimitedQuota: true, Status: config.TokenStatusEnabled}
	if err := db.Create(token).Error; err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.PUT("/api/org/:id", orgLogIOAs(org, orgLogIOAdmin, model.OrgRoleAdmin), UpdateOrganization)
	r.PUT("/api/org/:id/token", orgLogIOAs(org, orgLogIOMember, model.OrgRoleMember), UpdateOrgToken)
	r.POST("/api/org/:id/token", orgLogIOAs(org, orgLogIOMember, model.OrgRoleMember), AddOrgToken)
	r.GET("/api/org/:id/token/:token_id", orgLogIOAs(org, orgLogIOMember, model.OrgRoleMember), GetOrgToken)
	selected := func(id int) bool {
		t.Helper()
		var stored model.Token
		if err := db.First(&stored, id).Error; err != nil {
			t.Fatal(err)
		}
		return stored.LogIO != nil && *stored.LogIO
	}
	selectKeys := func(ids ...int) {
		t.Helper()
		code, body := orgRoutingCall(t, r, http.MethodPut, "/api/org/1", map[string]any{"name": "Org", "log_io_key_ids": append([]int{}, ids...)})
		if code != http.StatusOK || body["success"] != true {
			t.Fatalf("selection failed: %d %v", code, body)
		}
	}

	selectKeys(3)
	if !selected(3) {
		t.Fatal("the admin's selection was not saved")
	}
	selectKeys(3) // unchanged: no new entry
	if n := orgLogIOCount(t, db, model.OrgAuditActionTokenLogIOUpdate); n != 1 {
		t.Fatalf("selecting the key recorded %d entries, want 1", n)
	}
	code, body := orgRoutingCall(t, r, http.MethodGet, "/api/org/1/token/3", nil)
	status, _ := body["log_io"].(map[string]any)
	if code != http.StatusOK || status == nil || status["stored"] != true || status["source"] != "key" || status["changed_by"] != "alice" {
		t.Fatalf("key disclosure = %v", body["log_io"])
	}

	// The member editing the key cannot take it out, and a new key is not selected.
	code, body = orgRoutingCall(t, r, http.MethodPut, "/api/org/1/token", map[string]any{
		"id": 3, "name": "k", "expired_time": -1, "unlimited_quota": true, "status": config.TokenStatusEnabled, "log_io": false,
	})
	if code != http.StatusOK || body["success"] != true || !selected(3) {
		t.Fatalf("a member's edit changed the selection: %d %v", code, body)
	}
	code, body = orgRoutingCall(t, r, http.MethodPost, "/api/org/1/token", map[string]any{"name": "new", "expired_time": -1, "unlimited_quota": true, "log_io": true})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("create failed: %d %v", code, body)
	}
	var created model.Token
	if err := db.Where("name = ?", "new").First(&created).Error; err != nil {
		t.Fatal(err)
	}
	if created.LogIO != nil && *created.LogIO {
		t.Fatal("a new key was selected by its creator")
	}

	selectKeys()
	if selected(3) {
		t.Fatal("an empty selection kept the key")
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
