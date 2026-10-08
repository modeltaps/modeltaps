package controller

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupGetUserTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	// user_oidc_identities：删除用户会连带清理身份行，缺表会让删除用例直接报错
	if err := testDB.AutoMigrate(&model.User{}, &model.UserOidcIdentity{}, &model.UserSession{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
}

func callGetUser(t *testing.T, id string, role int) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/user/"+id, nil)
	c.Params = gin.Params{{Key: "id", Value: id}}
	c.Set("role", role)
	GetUser(c)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return resp
}

// root 后台按数字 ID 直查:影子记账账户应不可见(与列表/搜索的 ExcludeShadowUsers 语义一致),
// 普通用户应正常返回。
func TestGetUserShadowUserFiltered(t *testing.T) {
	setupGetUserTestDB(t)

	normal := &model.User{
		Username:    "alice",
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeNormal,
		AccessToken: "test-token-normal",
		AffCode:     "affn",
	}
	if err := model.DB.Create(normal).Error; err != nil {
		t.Fatalf("创建普通用户失败: %v", err)
	}
	shadow := &model.User{
		Username:    "org-acme",
		Role:        config.RoleGuestUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeOrgShadow,
		AccessToken: "test-token-shadow",
		AffCode:     "affs",
	}
	if err := model.DB.Create(shadow).Error; err != nil {
		t.Fatalf("创建影子用户失败: %v", err)
	}

	t.Run("直查普通用户正常", func(t *testing.T) {
		resp := callGetUser(t, "1", config.RoleRootUser)
		if resp["success"] != true {
			t.Fatalf("普通用户直查应成功,实际: %v", resp)
		}
		data, ok := resp["data"].(map[string]interface{})
		if !ok || data["username"] != "alice" {
			t.Fatalf("应返回普通用户数据,实际: %v", resp["data"])
		}
	})

	t.Run("直查影子用户被拒", func(t *testing.T) {
		resp := callGetUser(t, "2", config.RoleRootUser)
		if resp["success"] != false {
			t.Fatalf("影子用户直查应被拒,实际: %v", resp)
		}
		if resp["message"] != "User not found" {
			t.Fatalf("应返回用户不存在,实际: %v", resp["message"])
		}
	})
}

func newAdminTestContext(t *testing.T, method, path, body string, role int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", role)
	return c, w
}

func decodeResp(t *testing.T, w *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v (body=%q)", err, w.Body.String())
	}
	return resp
}

func createTestUser(t *testing.T, username string, role int, userType int) *model.User {
	t.Helper()
	u := &model.User{
		Username:    username,
		Role:        role,
		Status:      config.UserStatusEnabled,
		Type:        userType,
		AccessToken: "token-" + username,
		AffCode:     "aff-" + username,
	}
	if err := model.DB.Create(u).Error; err != nil {
		t.Fatalf("创建用户 %s 失败: %v", username, err)
	}
	return u
}

// DeleteUser 的成功/失败分支必须返回正确的 success 语义(此前失败分支误返 success:true、
// 成功分支无响应体)。
func TestDeleteUserResponseBranches(t *testing.T) {
	setupGetUserTestDB(t)
	target := createTestUser(t, "bob", config.RoleCommonUser, config.UserTypeNormal)

	t.Run("成功删除返回success true", func(t *testing.T) {
		c, w := newAdminTestContext(t, "DELETE", "/api/user/"+strconv.Itoa(target.Id), "", config.RoleRootUser)
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(target.Id)}}
		DeleteUser(c)
		resp := decodeResp(t, w)
		if resp["success"] != true {
			t.Fatalf("删除成功应返回 success:true,实际: %v", resp)
		}
	})

	t.Run("删除失败返回success false", func(t *testing.T) {
		victim := createTestUser(t, "carol", config.RoleCommonUser, config.UserTypeNormal)
		// 让 model.DeleteUserById 内部的写入失败，验证失败分支不再误返 success:true
		if err := model.DB.Callback().Update().Before("gorm:update").
			Register("test_fail_update", func(tx *gorm.DB) {
				tx.AddError(errors.New("模拟写入失败"))
			}); err != nil {
			t.Fatalf("注册测试回调失败: %v", err)
		}
		t.Cleanup(func() {
			_ = model.DB.Callback().Update().Remove("test_fail_update")
		})

		c, w := newAdminTestContext(t, "DELETE", "/api/user/"+strconv.Itoa(victim.Id), "", config.RoleRootUser)
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(victim.Id)}}
		DeleteUser(c)
		resp := decodeResp(t, w)
		if resp["success"] != false {
			t.Fatalf("删除失败应返回 success:false,实际: %v", resp)
		}
		if resp["message"] != "模拟写入失败" {
			t.Fatalf("删除失败应带错误 message,实际: %v", resp["message"])
		}
	})
}

// 四个后台管理端点均应拒绝对组织影子账户的操作。
func TestAdminEndpointsRejectShadowUser(t *testing.T) {
	setupGetUserTestDB(t)
	shadow := createTestUser(t, "org-acme", config.RoleGuestUser, config.UserTypeOrgShadow)
	shadowId := strconv.Itoa(shadow.Id)

	cases := []struct {
		name   string
		invoke func() *httptest.ResponseRecorder
	}{
		{"DeleteUser", func() *httptest.ResponseRecorder {
			c, w := newAdminTestContext(t, "DELETE", "/api/user/"+shadowId, "", config.RoleRootUser)
			c.Params = gin.Params{{Key: "id", Value: shadowId}}
			DeleteUser(c)
			return w
		}},
		{"ManageUser", func() *httptest.ResponseRecorder {
			body := `{"user_id":` + shadowId + `,"action":"disable"}`
			c, w := newAdminTestContext(t, "POST", "/api/user/manage", body, config.RoleRootUser)
			ManageUser(c)
			return w
		}},
		{"UpdateUser", func() *httptest.ResponseRecorder {
			body := `{"id":` + shadowId + `,"username":"org-acme","display_name":"x"}`
			c, w := newAdminTestContext(t, "PUT", "/api/user/", body, config.RoleRootUser)
			UpdateUser(c)
			return w
		}},
		{"ChangeUserQuota", func() *httptest.ResponseRecorder {
			c, w := newAdminTestContext(t, "PUT", "/api/user/quota/"+shadowId, `{"quota":100}`, config.RoleRootUser)
			c.Params = gin.Params{{Key: "id", Value: shadowId}}
			ChangeUserQuota(c)
			return w
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := decodeResp(t, tc.invoke())
			if resp["success"] != false {
				t.Fatalf("%s 对影子账户应被拒,实际: %v", tc.name, resp)
			}
			if resp["message"] != "Cannot operate on an organization shadow account" {
				t.Fatalf("%s 应返回影子账户拒绝提示,实际: %v", tc.name, resp["message"])
			}
		})
	}
}

// ChangeUserQuota 需与 UpdateUser 同语义：不能操作同级或更高等级用户。
func TestChangeUserQuotaRoleGuard(t *testing.T) {
	setupGetUserTestDB(t)
	peer := createTestUser(t, "admin2", config.RoleAdminUser, config.UserTypeNormal)
	peerId := strconv.Itoa(peer.Id)

	c, w := newAdminTestContext(t, "PUT", "/api/user/quota/"+peerId, `{"quota":100}`, config.RoleAdminUser)
	c.Params = gin.Params{{Key: "id", Value: peerId}}
	ChangeUserQuota(c)

	resp := decodeResp(t, w)
	if resp["success"] != false {
		t.Fatalf("同级用户改额度应被拒,实际: %v", resp)
	}
	if resp["message"] != "You are not allowed to update users of the same or higher role" {
		t.Fatalf("应返回角色层级拒绝提示,实际: %v", resp["message"])
	}
}

// newSelfTestContext 构造已登录的自助请求上下文（鉴权中间件只往上下文写 id）。
func newSelfTestContext(t *testing.T, method, path, body string, userId int) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, path, strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userId)
	return c, w
}

// 密码登录关闭时本站不该再持有第二套凭据:自助改密必须被拒且密码列不变,
// 不带 password 的资料更新仍要成功;开启后改密行为与现状一致。
func TestUpdateSelfPasswordGuardedByPasswordLogin(t *testing.T) {
	setupGetUserTestDB(t)
	oldEnabled := config.PasswordLoginEnabled
	t.Cleanup(func() { config.PasswordLoginEnabled = oldEnabled })

	storedHash := mustHashPassword(t, "old-password-1")
	user := createTestUser(t, "dave", config.RoleCommonUser, config.UserTypeNormal)
	if err := model.DB.Model(&model.User{}).Where("id = ?", user.Id).
		Updates(map[string]interface{}{"password": storedHash, "display_name": "Dave"}).Error; err != nil {
		t.Fatalf("准备密码列失败: %v", err)
	}

	readUser := func() *model.User {
		t.Helper()
		got, err := model.GetUserById(user.Id, true)
		if err != nil {
			t.Fatalf("读取用户失败: %v", err)
		}
		return got
	}

	t.Run("关闭密码登录时改密被拒", func(t *testing.T) {
		config.PasswordLoginEnabled = false
		body := `{"username":"dave","password":"new-password-1","display_name":"Changed"}`
		c, w := newSelfTestContext(t, "PUT", "/api/user/self", body, user.Id)
		UpdateSelf(c)
		resp := decodeResp(t, w)
		if resp["success"] != false {
			t.Fatalf("改密应被拒,实际: %v", resp)
		}
		message, _ := resp["message"].(string)
		if strings.Contains(message, "OIDC") {
			t.Fatalf("拒绝文案不应出现 OIDC: %q", message)
		}
		got := readUser()
		if got.Password != storedHash {
			t.Fatalf("密码列不应变化,实际 %q", got.Password)
		}
		if got.DisplayName != "Dave" {
			t.Fatalf("被拒的请求不应改动其余字段,实际 %q", got.DisplayName)
		}
	})

	t.Run("关闭密码登录时不带密码仍可改资料", func(t *testing.T) {
		config.PasswordLoginEnabled = false
		body := `{"username":"dave","display_name":"Renamed"}`
		c, w := newSelfTestContext(t, "PUT", "/api/user/self", body, user.Id)
		UpdateSelf(c)
		if resp := decodeResp(t, w); resp["success"] != true {
			t.Fatalf("不带密码的更新应成功,实际: %v", resp)
		}
		got := readUser()
		if got.DisplayName != "Renamed" {
			t.Fatalf("display_name 应被更新,实际 %q", got.DisplayName)
		}
		if got.Password != storedHash {
			t.Fatalf("密码列不应变化,实际 %q", got.Password)
		}
	})

	t.Run("开启密码登录时改密成功", func(t *testing.T) {
		config.PasswordLoginEnabled = true
		body := `{"username":"dave","password":"new-password-1","original_password":"old-password-1","display_name":"Renamed"}`
		c, w := newSelfTestContext(t, "PUT", "/api/user/self", body, user.Id)
		UpdateSelf(c)
		if resp := decodeResp(t, w); resp["success"] != true {
			t.Fatalf("改密应成功,实际: %v", resp)
		}
		got := readUser()
		if got.Password == storedHash || got.Password == "" {
			t.Fatalf("密码列应被写入新哈希,实际 %q", got.Password)
		}
		if got.Password == "new-password-1" {
			t.Fatal("密码必须哈希后落库")
		}
	})
}

// 逃生口密码必须可轮换:密码登录关闭时 root 仍能自助改密,其余角色维持拒绝。
func TestUpdateSelfRootPasswordExemptWhenPasswordLoginDisabled(t *testing.T) {
	setupGetUserTestDB(t)
	oldEnabled := config.PasswordLoginEnabled
	t.Cleanup(func() { config.PasswordLoginEnabled = oldEnabled })
	config.PasswordLoginEnabled = false

	storedHash := mustHashPassword(t, "old-root-password")
	root := createTestUser(t, "rootself", config.RoleRootUser, config.UserTypeNormal)
	if err := model.DB.Model(&model.User{}).Where("id = ?", root.Id).
		Update("password", storedHash).Error; err != nil {
		t.Fatalf("准备密码列失败: %v", err)
	}

	body := `{"username":"rootself","password":"new-root-password","original_password":"old-root-password"}`
	c, w := newSelfTestContext(t, "PUT", "/api/user/self", body, root.Id)
	c.Set("role", config.RoleRootUser)
	UpdateSelf(c)
	if resp := decodeResp(t, w); resp["success"] != true {
		t.Fatalf("root 改密应成功,实际: %v", resp)
	}
	got, err := model.GetUserById(root.Id, true)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if got.Password == storedHash || got.Password == "" {
		t.Fatalf("密码列应被写入新哈希,实际 %q", got.Password)
	}
	if got.Password == "new-root-password" {
		t.Fatal("密码必须哈希后落库")
	}
}

func mustHashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := common.Password2Hash(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}
	return hash
}

// Changing one's own password requires the current password; an account without a stored
// password (third-party sign-in only) sets its first password without one, and a password-only
// request needs no username.
func TestUpdateSelfRequiresOriginalPassword(t *testing.T) {
	setupGetUserTestDB(t)
	oldEnabled := config.PasswordLoginEnabled
	t.Cleanup(func() { config.PasswordLoginEnabled = oldEnabled })
	config.PasswordLoginEnabled = true

	storedHash := mustHashPassword(t, "old-password-1")
	user := createTestUser(t, "grace", config.RoleCommonUser, config.UserTypeNormal)
	if err := model.DB.Model(&model.User{}).Where("id = ?", user.Id).
		Update("password", storedHash).Error; err != nil {
		t.Fatalf("failed to seed password: %v", err)
	}

	update := func(userId int, body string) map[string]interface{} {
		t.Helper()
		c, w := newSelfTestContext(t, "PUT", "/api/user/self", body, userId)
		UpdateSelf(c)
		return decodeResp(t, w)
	}
	storedPassword := func(userId int) string {
		t.Helper()
		got, err := model.GetUserById(userId, true)
		if err != nil {
			t.Fatalf("failed to read user: %v", err)
		}
		return got.Password
	}

	t.Run("missing original password", func(t *testing.T) {
		resp := update(user.Id, `{"password":"new-password-1"}`)
		if resp["success"] != false || resp["message"] != "Current password is required to change the password" {
			t.Fatalf("expected a missing-password rejection, got: %v", resp)
		}
		if storedPassword(user.Id) != storedHash {
			t.Fatal("password must not change")
		}
	})

	t.Run("wrong original password", func(t *testing.T) {
		resp := update(user.Id, `{"password":"new-password-1","original_password":"wrong-password"}`)
		if resp["success"] != false || resp["message"] != "Current password is incorrect" {
			t.Fatalf("expected a wrong-password rejection, got: %v", resp)
		}
		if storedPassword(user.Id) != storedHash {
			t.Fatal("password must not change")
		}
	})

	t.Run("correct original password without username", func(t *testing.T) {
		resp := update(user.Id, `{"password":"new-password-1","original_password":"old-password-1"}`)
		if resp["success"] != true {
			t.Fatalf("password change should succeed, got: %v", resp)
		}
		if !common.ValidatePasswordAndHash("new-password-1", storedPassword(user.Id)) {
			t.Fatal("new password should be stored")
		}
	})

	t.Run("third-party account sets its first password", func(t *testing.T) {
		social := createTestUser(t, "henry", config.RoleCommonUser, config.UserTypeNormal)
		if storedPassword(social.Id) != "" {
			t.Fatal("test account should start without a password")
		}
		resp := update(social.Id, `{"password":"first-password-1"}`)
		if resp["success"] != true {
			t.Fatalf("first password should be set without the original password, got: %v", resp)
		}
		if !common.ValidatePasswordAndHash("first-password-1", storedPassword(social.Id)) {
			t.Fatal("first password should be stored")
		}
	})
}

// 密码登录关闭时只给 root 留逃生口:root 走完整验证链(含锁定与失败计数),
// 其余请求(非 root、账号不存在、请求体非法)响应逐字一致且不累计失败计数。
func TestLoginRootEscapeHatchWhenPasswordLoginDisabled(t *testing.T) {
	setupGetUserTestDB(t)
	r := setupLoginRouter(t)
	config.PasswordLoginEnabled = false

	const disabledMessage = "The admin has disabled password sign-in"
	const rootPassword = "root-escape-password"

	newUserWithPassword := func(username string, role int, password string) *model.User {
		t.Helper()
		u := createTestUser(t, username, role, config.UserTypeNormal)
		hash, err := common.Password2Hash(password)
		if err != nil {
			t.Fatalf("生成密码哈希失败: %v", err)
		}
		if err := model.DB.Model(&model.User{}).Where("id = ?", u.Id).
			Update("password", hash).Error; err != nil {
			t.Fatalf("准备密码列失败: %v", err)
		}
		t.Cleanup(func() { common.ClearLoginFailures(common.LoginFailureKeyForUser(u.Id)) })
		return u
	}

	root := newUserWithPassword("rootlogin", config.RoleRootUser, rootPassword)
	member := newUserWithPassword("memberlogin", config.RoleCommonUser, "member-password")

	t.Run("root 正确密码可登录", func(t *testing.T) {
		resp := postLogin(t, r, "rootlogin", rootPassword)
		if resp["success"] != true {
			t.Fatalf("root 应可通过逃生口登录,实际: %v", resp)
		}
	})

	t.Run("非 root 被拒且不记失败计数", func(t *testing.T) {
		memberKey := common.LoginFailureKeyForUser(member.Id)
		for i := 0; i < config.LoginMaxFailures+1; i++ {
			resp := postLogin(t, r, "memberlogin", "member-password")
			if resp["success"] != false || resp["message"] != disabledMessage {
				t.Fatalf("非 root 应返回 %q,实际: %v", disabledMessage, resp)
			}
		}
		if common.IsLoginLocked(memberKey) {
			t.Fatal("被拒的非 root 请求不应累计失败计数")
		}
	})

	t.Run("账号不存在返回同一文案", func(t *testing.T) {
		resp := postLogin(t, r, "ghostlogin", "whatever")
		if resp["success"] != false || resp["message"] != disabledMessage {
			t.Fatalf("不存在的账号应返回 %q,实际: %v", disabledMessage, resp)
		}
	})

	t.Run("请求体非法返回同一文案", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		resp := decodeResp(t, w)
		if resp["success"] != false || resp["message"] != disabledMessage {
			t.Fatalf("非法请求体应返回 %q,实际: %v", disabledMessage, resp)
		}
	})

	t.Run("root 错密码走失败计数并可锁定", func(t *testing.T) {
		rootKey := common.LoginFailureKeyForUser(root.Id)
		for i := 0; i < config.LoginMaxFailures; i++ {
			resp := postLogin(t, r, "rootlogin", "wrong-password")
			if resp["success"] != false || resp["message"] != model.ErrLoginFailed.Error() {
				t.Fatalf("root 错密码应返回通用失败文案,实际: %v", resp)
			}
		}
		if !common.IsLoginLocked(rootKey) {
			t.Fatal("root 连续失败后应进入锁定")
		}
		locked := postLogin(t, r, "rootlogin", rootPassword)
		if locked["success"] != false || locked["message"] != model.ErrLoginFailed.Error() {
			t.Fatalf("锁定期内即使密码正确也应拒绝,实际: %v", locked)
		}
	})
}

// GET /api/user/self 需下发 has_password 供前端区分「设置密码」与「修改密码」,
// 且响应体任何位置都不得出现密码哈希。
func TestGetSelfHasPassword(t *testing.T) {
	setupGetUserTestDB(t)
	const storedHash = "self-password-hash"
	withPassword := createTestUser(t, "erin", config.RoleCommonUser, config.UserTypeNormal)
	if err := model.DB.Model(&model.User{}).Where("id = ?", withPassword.Id).
		Update("password", storedHash).Error; err != nil {
		t.Fatalf("准备密码列失败: %v", err)
	}
	withoutPassword := createTestUser(t, "frank", config.RoleCommonUser, config.UserTypeNormal)

	callSelf := func(userId int) (map[string]interface{}, string) {
		t.Helper()
		c, w := newSelfTestContext(t, "GET", "/api/user/self", "", userId)
		GetSelf(c)
		return decodeResp(t, w), w.Body.String()
	}

	resp, raw := callSelf(withPassword.Id)
	data, ok := resp["data"].(map[string]interface{})
	if !ok || data["has_password"] != true {
		t.Fatalf("有密码用户应下发 has_password=true,实际: %v", resp["data"])
	}
	if strings.Contains(raw, storedHash) {
		t.Fatalf("响应体泄露了密码哈希: %s", raw)
	}
	if _, exists := data["password"]; exists {
		t.Fatalf("响应体不应出现 password 键: %s", raw)
	}

	resp, _ = callSelf(withoutPassword.Id)
	data, ok = resp["data"].(map[string]interface{})
	if !ok || data["has_password"] != false {
		t.Fatalf("空密码用户应下发 has_password=false,实际: %v", resp["data"])
	}
}

// 登录标识符含 @ 一律按邮箱解析,含 @ 的用户名会永远登录不上,
// 因此注册 / 自助改名 / 管理员建改用户 / 组织代建共用的 common.Validate 必须拒绝,
// 且五处都经 getFriendlyValidationMessage 输出同一条文案。
func TestUsernameRejectsAtSign(t *testing.T) {
	cases := []struct {
		name     string
		username string
		want     string
	}{
		{"邮箱形态", "a@b.co", "Username must not contain the @ symbol"},
		{"仅一个 @", "@", "Username must not contain the @ symbol"},
		{"末尾带 @", "alice@", "Username must not contain the @ symbol"},
		{"正常用户名", "alice", ""},
		{"空用户名", "", "Username must not be empty"},
		{"超长用户名", "0123456789abc", "Username must not exceed 12 characters"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := common.Validate.Struct(&model.User{
				Username: tc.username,
				Password: "password123",
			})
			if tc.want == "" {
				if err != nil {
					t.Fatalf("用户名 %q 应通过校验,实际: %v", tc.username, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("用户名 %q 应被拒绝", tc.username)
			}
			if got := getFriendlyValidationMessage(err); got != tc.want {
				t.Fatalf("文案 = %q,期望 %q", got, tc.want)
			}
		})
	}
}

// callEmailBind 驱动 /api/user/email_bind：邮箱与验证码走 query，身份由鉴权中间件写入的 id 提供。
func callEmailBind(t *testing.T, userId int, email, code string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	target := "/api/user/email_bind?email=" + url.QueryEscape(email) + "&code=" + url.QueryEscape(code)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	c.Set("id", userId)
	EmailBind(c)
	if w.Code != http.StatusOK {
		t.Fatalf("EmailBind 应返回 200，实际 %d", w.Code)
	}
	return decodeResp(t, w)
}

// 验证码正确时邮箱落库；大小写不同的输入按归一化后的邮箱校验并写入。
func TestEmailBindWritesVerifiedEmail(t *testing.T) {
	setupGetUserTestDB(t)
	user := createTestUser(t, "binder", config.RoleCommonUser, config.UserTypeNormal)
	common.RegisterVerificationCodeWithKey("binder@example.com", "123456", common.EmailVerificationPurpose)
	t.Cleanup(func() { common.DeleteKey("binder@example.com", common.EmailVerificationPurpose) })

	resp := callEmailBind(t, user.Id, "Binder@Example.com", "123456")
	if resp["success"] != true {
		t.Fatalf("验证码正确时应绑定成功，实际: %v", resp)
	}
	if got := getUserEmail(t, user.Id); got != "binder@example.com" {
		t.Fatalf("users.email 应更新为归一化后的邮箱，实际 %q", got)
	}
}

// 验证码错误 / 未申请，以及邮箱格式不合要求时一律被拒，且不写库。
func TestEmailBindRejectsBadCodeAndEmail(t *testing.T) {
	setupGetUserTestDB(t)
	user := createTestUser(t, "badbind", config.RoleCommonUser, config.UserTypeNormal)
	common.RegisterVerificationCodeWithKey("badbind@example.com", "123456", common.EmailVerificationPurpose)
	t.Cleanup(func() { common.DeleteKey("badbind@example.com", common.EmailVerificationPurpose) })

	cases := []struct {
		name    string
		email   string
		code    string
		message string
	}{
		{"验证码不匹配", "badbind@example.com", "654321", "Verification code is incorrect or has expired"},
		{"邮箱未申请验证码", "never@example.com", "123456", "Verification code is incorrect or has expired"},
		{"邮箱格式不合要求", "not-an-email", "123456", "Invalid email format"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := callEmailBind(t, user.Id, tc.email, tc.code)
			if resp["success"] != false {
				t.Fatalf("绑定应被拒，实际: %v", resp)
			}
			if resp["message"] != tc.message {
				t.Fatalf("拒绝文案应为 %q，实际: %v", tc.message, resp["message"])
			}
			if got := getUserEmail(t, user.Id); got != "" {
				t.Fatalf("被拒后 users.email 应保持为空，实际 %q", got)
			}
		})
	}
}

// 目标邮箱已被其它账号占用：撞唯一索引前先由 Update 拦下，报「该邮箱已被使用！」且两边都不变。
func TestEmailBindRejectsTakenEmail(t *testing.T) {
	setupGetUserTestDB(t)
	owner := createTestUser(t, "mailowner", config.RoleCommonUser, config.UserTypeNormal)
	if err := model.DB.Model(owner).Update("email", "taken@example.com").Error; err != nil {
		t.Fatalf("写入占用邮箱失败: %v", err)
	}
	user := createTestUser(t, "mailtaker", config.RoleCommonUser, config.UserTypeNormal)
	common.RegisterVerificationCodeWithKey("taken@example.com", "123456", common.EmailVerificationPurpose)
	t.Cleanup(func() { common.DeleteKey("taken@example.com", common.EmailVerificationPurpose) })

	resp := callEmailBind(t, user.Id, "taken@example.com", "123456")
	if resp["success"] != false {
		t.Fatalf("邮箱已被占用时应被拒，实际: %v", resp)
	}
	if resp["message"] != "this email is already in use" {
		t.Fatalf("拒绝文案应为「该邮箱已被使用！」，实际: %v", resp["message"])
	}
	if got := getUserEmail(t, user.Id); got != "" {
		t.Fatalf("被拒后 users.email 应保持为空，实际 %q", got)
	}
	if got := getUserEmail(t, owner.Id); got != "taken@example.com" {
		t.Fatalf("原持有者的邮箱不应变化，实际 %q", got)
	}
}
