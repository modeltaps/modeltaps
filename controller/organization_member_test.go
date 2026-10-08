package controller

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupOrgMemberAccountTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.Organization{},
		&model.OrganizationMember{}, &model.OrganizationAuditLog{}, &model.Log{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
	// 代建路径会在事务内行锁组织记录(ORG-5),需真实存在该行
	if err := testDB.Create(&model.Organization{Id: 1, Name: "Acme", Slug: "acme", ShadowUserId: 2}).Error; err != nil {
		t.Fatalf("创建测试组织失败: %v", err)
	}
}

func callCreateOrgMemberAccount(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/org/1/member/account", bytes.NewBufferString(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 1)
	c.Set(middleware.OrgContextKey, &model.Organization{Id: 1, Name: "Acme", ShadowUserId: 2})
	c.Set(middleware.OrgRoleContextKey, model.OrgRoleOwner)
	CreateOrgMemberAccount(c)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return resp
}

func countUsersByUsername(t *testing.T, username string) int64 {
	t.Helper()
	var n int64
	if err := model.DB.Model(&model.User{}).Where("username = ?", username).Count(&n).Error; err != nil {
		t.Fatalf("统计用户失败: %v", err)
	}
	return n
}

// SEC-16:代建成员账号必须复用注册路径的入参校验,弱密码/超长用户名等
// 非法入参应被拒绝并给出明确提示,且不得落库。
func TestCreateOrgMemberAccountValidation(t *testing.T) {
	cases := []struct {
		name     string
		body     string
		username string
		wantMsg  string
	}{
		{
			name:     "密码过短被拒",
			body:     `{"username":"shortpw","password":"1","role":"member"}`,
			username: "shortpw",
			wantMsg:  "Password must be at least 8 characters",
		},
		{
			name:     "用户名超长被拒",
			body:     `{"username":"averyveryverylongusername","password":"secret123","role":"member"}`,
			username: "averyveryverylongusername",
			wantMsg:  "Username must not exceed 12 characters",
		},
		{
			name:     "显示名超长被拒",
			body:     `{"username":"dispname","password":"secret123","display_name":"` + strings.Repeat("d", 21) + `","role":"member"}`,
			username: "dispname",
			wantMsg:  "Display name must not exceed 20 characters",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupOrgMemberAccountTestDB(t)
			resp := callCreateOrgMemberAccount(t, tc.body)
			if resp["success"] != false {
				t.Fatalf("非法入参应被拒绝, got %v", resp)
			}
			if msg, _ := resp["message"].(string); msg != tc.wantMsg {
				t.Fatalf("提示信息应为 %q, got %q", tc.wantMsg, msg)
			}
			if n := countUsersByUsername(t, tc.username); n != 0 {
				t.Fatalf("被拒的入参不应落库, 实际 %d 条", n)
			}
		})
	}

	t.Run("合法入参正常创建", func(t *testing.T) {
		setupOrgMemberAccountTestDB(t)
		resp := callCreateOrgMemberAccount(t, `{"username":"goodname","password":"secret123","role":"member"}`)
		if resp["success"] != true {
			t.Fatalf("合法入参应创建成功, got %v", resp)
		}
		if n := countUsersByUsername(t, "goodname"); n != 1 {
			t.Fatalf("应创建 1 个用户, 实际 %d 条", n)
		}
	})
}
