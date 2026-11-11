package controller

import (
	"net/http"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// API key names are unique per owner and cannot take the reserved sys_ prefix; saving a key
// without renaming it is always allowed, so keys that already share a name keep working.
func TestTokenNamesUniquePerOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if viper.GetString("user_token_secret") == "" {
		viper.Set("user_token_secret", "token-name-test-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatal(err)
		}
	}
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.Token{}); err != nil {
		t.Fatal(err)
	}
	prevDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = prevDB })

	asUser := func(id int) gin.HandlerFunc {
		return func(c *gin.Context) {
			c.Set("id", id)
			c.Set("role", 1)
			c.Next()
		}
	}
	r := gin.New()
	r.POST("/u1/token", asUser(1), AddToken)
	r.PUT("/u1/token", asUser(1), UpdateToken)
	r.POST("/u2/token", asUser(2), AddToken)

	create := func(path, name string) map[string]any {
		_, body := orgRoutingCall(t, r, http.MethodPost, path, map[string]any{"name": name, "expired_time": -1, "unlimited_quota": true})
		return body
	}
	code := func(body map[string]any) any { return body["code"] }

	first := create("/u1/token", "ci")
	if first["success"] != true {
		t.Fatalf("first create failed: %v", first)
	}
	if body := create("/u1/token", "ci"); body["success"] == true || code(body) != common.APICodeTokenNameTaken {
		t.Fatalf("duplicate name for the same user was accepted: %v", body)
	}
	if body := create("/u2/token", "ci"); body["success"] != true {
		t.Fatalf("another user could not use the same name: %v", body)
	}
	if body := create("/u1/token", "sys_playground"); body["success"] == true || code(body) != common.APICodeTokenNameReserved {
		t.Fatalf("reserved name was accepted: %v", body)
	}

	// A legacy duplicate (inserted directly) can still be saved without renaming, but not renamed
	// onto another key's name.
	legacy := &model.Token{UserId: 1, Name: "ci", ExpiredTime: -1, UnlimitedQuota: true}
	if err := testDB.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	update := func(name string) map[string]any {
		_, body := orgRoutingCall(t, r, http.MethodPut, "/u1/token", map[string]any{"id": legacy.Id, "name": name, "expired_time": -1, "unlimited_quota": true})
		return body
	}
	if body := update("ci"); body["success"] != true {
		t.Fatalf("saving a legacy duplicate without renaming failed: %v", body)
	}
	if body := update("prod"); body["success"] != true {
		t.Fatalf("renaming to a free name failed: %v", body)
	}
	if body := update("ci"); body["success"] == true || code(body) != common.APICodeTokenNameTaken {
		t.Fatalf("renaming onto a taken name was accepted: %v", body)
	}
}
