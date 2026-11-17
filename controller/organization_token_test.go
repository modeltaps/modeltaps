package controller

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Creating an organization API key returns its id and key, like the personal endpoint, so the
// console can show the new key right away.
func TestAddOrgTokenReturnsIdAndKey(t *testing.T) {
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
	if err := testDB.AutoMigrate(&model.User{}, &model.Organization{}, &model.OrganizationAuditLog{}, &model.Token{}); err != nil {
		t.Fatal(err)
	}
	prevDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = prevDB })

	shadow := &model.User{Id: 11, Username: "shadow1", AffCode: "a1", Group: "default"}
	if err := testDB.Create(shadow).Error; err != nil {
		t.Fatal(err)
	}
	org := &model.Organization{Id: 1, Name: "Org", Slug: "org1", Status: model.OrganizationStatusEnabled, ShadowUserId: shadow.Id, CreatedBy: 1}
	if err := testDB.Create(org).Error; err != nil {
		t.Fatal(err)
	}

	r := gin.New()
	r.POST("/api/org/:id/token", func(c *gin.Context) {
		c.Set("id", 1)
		c.Set(middleware.OrgContextKey, org)
		c.Next()
	}, AddOrgToken)

	code, body := orgRoutingCall(t, r, http.MethodPost, "/api/org/1/token", map[string]any{"name": "ci", "expired_time": -1, "unlimited_quota": true})
	if code != http.StatusOK || body["success"] != true {
		t.Fatalf("create failed: %d %v", code, body)
	}
	data, _ := body["data"].(map[string]any)
	if data == nil || data["id"] == nil || data["key"] == "" || data["key"] == nil {
		t.Fatalf("response has no id/key: %v", body)
	}
	var stored model.Token
	if err := testDB.First(&stored).Error; err != nil {
		t.Fatal(err)
	}
	key, _ := data["key"].(string)
	if stored.KeyHash != common.HashUserTokenKey(key) || float64(stored.Id) != data["id"] || stored.UserId != shadow.Id {
		t.Fatalf("response %v does not match stored key %+v", data, stored)
	}
}

// ORG-16: only owners and admins create keys owned by the organization; such a key is recorded with
// the shadow account as created_by, its secret can be reset by owners and admins, and a member's
// key can be made owned by the organization.
func TestOrganizationOwnedOrgTokens(t *testing.T) {
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
	if err := testDB.AutoMigrate(&model.User{}, &model.Organization{}, &model.OrganizationMember{}, &model.ModelAccessList{}, &model.OrganizationAuditLog{}, &model.Token{}, &model.AgentPairingCode{}, &model.AgentKeyClaim{}); err != nil {
		t.Fatal(err)
	}
	prevDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = prevDB })

	shadow := &model.User{Id: 11, Username: "shadow-oo", AffCode: "oo1", Group: "default"}
	if err := testDB.Create(shadow).Error; err != nil {
		t.Fatal(err)
	}
	org := &model.Organization{Id: 1, Name: "Org", Slug: "org-oo", Status: model.OrganizationStatusEnabled, ShadowUserId: shadow.Id, CreatedBy: 1}
	if err := testDB.Create(org).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range []*model.OrganizationMember{
		{OrganizationId: org.Id, UserId: 1, Role: model.OrgRoleAdmin},
		{OrganizationId: org.Id, UserId: 2, Role: model.OrgRoleMember},
	} {
		if err := testDB.Create(m).Error; err != nil {
			t.Fatal(err)
		}
	}

	as := func(userId int, role string) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("id", userId)
			c.Set(middleware.OrgContextKey, org)
			c.Set(middleware.OrgRoleContextKey, role)
			c.Next()
		}
	}
	r := gin.New()
	r.POST("/admin/token", as(1, model.OrgRoleAdmin), AddOrgToken)
	r.POST("/member/token", as(2, model.OrgRoleMember), AddOrgToken)
	r.POST("/admin/token/:token_id/reset_key", as(1, model.OrgRoleAdmin), ResetOrgTokenKey)
	r.POST("/admin/token/:token_id/transfer", as(1, model.OrgRoleAdmin), TransferOrgTokenToOrganization)
	r.POST("/member/token/:token_id/transfer", as(2, model.OrgRoleMember), TransferOrgTokenToOrganization)

	newKey := map[string]any{"name": "svc", "expired_time": -1, "unlimited_quota": true, "owner": "organization"}
	if _, body := orgRoutingCall(t, r, http.MethodPost, "/member/token", newKey); body["success"] == true || body["code"] != common.APICodeTokenOrgOwnedAdminOnly {
		t.Fatalf("a member must not create a key owned by the organization: %v", body)
	}
	if _, body := orgRoutingCall(t, r, http.MethodPost, "/admin/token", map[string]any{"name": "bad", "owner": "someone"}); body["success"] == true {
		t.Fatalf("an unknown owner must be refused: %v", body)
	}
	_, body := orgRoutingCall(t, r, http.MethodPost, "/admin/token", newKey)
	if body["success"] != true {
		t.Fatalf("admin create failed: %v", body)
	}
	ownedId := int(body["data"].(map[string]any)["id"].(float64))
	var owned model.Token
	testDB.First(&owned, ownedId)
	if owned.UserId != shadow.Id || owned.CreatedBy != shadow.Id {
		t.Fatalf("key owned by the organization must have created_by = shadow account: %+v", owned)
	}
	if _, body := orgRoutingCall(t, r, http.MethodPost, "/admin/token/"+strconv.Itoa(ownedId)+"/reset_key", nil); body["success"] != true {
		t.Fatalf("an admin may reset a key owned by the organization: %v", body)
	}

	// A member's key: the member cannot transfer it, an admin can.
	_, body = orgRoutingCall(t, r, http.MethodPost, "/member/token", map[string]any{"name": "mine", "expired_time": -1, "unlimited_quota": true})
	if body["success"] != true {
		t.Fatalf("member create failed: %v", body)
	}
	memberKeyId := strconv.Itoa(int(body["data"].(map[string]any)["id"].(float64)))
	if _, body := orgRoutingCall(t, r, http.MethodPost, "/admin/token/"+memberKeyId+"/reset_key", nil); body["success"] == true {
		t.Fatalf("an admin still cannot reset a member's key: %v", body)
	}
	if _, body := orgRoutingCall(t, r, http.MethodPost, "/member/token/"+memberKeyId+"/transfer", nil); body["success"] == true {
		t.Fatalf("transfer is for owners and admins: %v", body)
	}
	if _, body := orgRoutingCall(t, r, http.MethodPost, "/admin/token/"+memberKeyId+"/transfer", nil); body["success"] != true {
		t.Fatalf("admin transfer failed: %v", body)
	}
	var transferred model.Token
	testDB.First(&transferred, memberKeyId)
	if !model.TokenIsOrgOwned(&transferred) {
		t.Fatalf("transferred key must be owned by the organization: %+v", transferred)
	}
}
