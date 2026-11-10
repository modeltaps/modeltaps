package controller

import (
	"net/http"
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
	// Token.AfterCreate derives the key with the token encoder.
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

	shadow := &model.User{Id: 11, Username: "shadow1", AccessToken: "t1", AffCode: "a1", Group: "default"}
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
	if stored.Key != data["key"] || float64(stored.Id) != data["id"] || stored.UserId != shadow.Id {
		t.Fatalf("response %v does not match stored key %+v", data, stored)
	}
}
