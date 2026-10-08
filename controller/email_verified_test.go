package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// users.email_verified 记的是「这个邮箱的来源是否经过验证」（AUTH-4）：账号安全页的
// 「已验证」角标与按已验证邮箱自动关联都读它，故每条写 users.email 的路径都得说清自己写什么。

// getUserEmailVerified 从库里读回标记，避免被内存中的结构体掩盖真实落库结果。
func getUserEmailVerified(t *testing.T, userId int) bool {
	t.Helper()
	user, err := model.GetUserById(userId, false)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	return user.EmailVerified
}

// seedEmailUser 建一个测试用户。
func seedEmailUser(t *testing.T, u *model.User) *model.User {
	t.Helper()
	if err := model.DB.Create(u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return u
}

// 验证码绑定邮箱：地址与「已验证」一并落库。
func TestEmailBindMarksEmailVerified(t *testing.T) {
	setupGetUserTestDB(t)
	user := seedEmailUser(t, &model.User{
		Username: "binder", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-ev1", AffCode: "affev1",
	})

	const email = "binder@example.com"
	common.RegisterVerificationCodeWithKey(email, "123456", common.EmailVerificationPurpose)
	t.Cleanup(func() { common.DeleteKey(email, common.EmailVerificationPurpose) })

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/user/email_bind?email="+email+"&code=123456", nil)
	c.Set("id", user.Id)
	EmailBind(c)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("绑定应成功，实际: %v", resp)
	}
	stored, err := model.GetUserById(user.Id, false)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if string(stored.Email) != email {
		t.Fatalf("邮箱应已写入，实际 %q", stored.Email)
	}
	if !stored.EmailVerified {
		t.Fatal("验证码绑定的邮箱应记为已验证")
	}
}

// callUpdateUser 以管理员身份提交一次后台用户编辑。
func callUpdateUser(t *testing.T, body string, myRole int) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/user/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("role", myRole)
	UpdateUser(c)
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return resp
}

// 管理员后台改邮箱不发验证码，新地址没有任何来源可以证明属于本人：一律降级为未验证。
// 同一地址重复提交不降级；请求体里伪造的 email_verified 一概不采信。
func TestUpdateUserResetsEmailVerified(t *testing.T) {
	setupGetUserTestDB(t)
	user := seedEmailUser(t, &model.User{
		Username: "target", Email: "target@example.com", EmailVerified: true,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-ev2", AffCode: "affev2",
	})

	t.Run("换邮箱降级为未验证", func(t *testing.T) {
		resp := callUpdateUser(t, `{"id":1,"username":"target","email":"admin-typed@example.com"}`, config.RoleRootUser)
		if resp["success"] != true {
			t.Fatalf("更新应成功，实际: %v", resp)
		}
		stored, err := model.GetUserById(user.Id, false)
		if err != nil {
			t.Fatalf("读取用户失败: %v", err)
		}
		if string(stored.Email) != "admin-typed@example.com" {
			t.Fatalf("邮箱应已改写，实际 %q", stored.Email)
		}
		if stored.EmailVerified {
			t.Fatal("管理员代填的邮箱不应保留已验证")
		}
	})

	t.Run("伪造 email_verified 不被采信", func(t *testing.T) {
		resp := callUpdateUser(t, `{"id":1,"username":"target","email":"admin-typed@example.com","email_verified":true}`, config.RoleRootUser)
		if resp["success"] != true {
			t.Fatalf("更新应成功，实际: %v", resp)
		}
		if getUserEmailVerified(t, user.Id) {
			t.Fatal("请求体里的 email_verified 不得提升为已验证")
		}
	})

	t.Run("邮箱未变时不动已验证标记", func(t *testing.T) {
		if err := model.DB.Model(&model.User{}).Where("id = ?", user.Id).
			Update("email_verified", true).Error; err != nil {
			t.Fatalf("准备数据失败: %v", err)
		}
		resp := callUpdateUser(t, `{"id":1,"username":"target","email":"admin-typed@example.com","display_name":"改个名"}`, config.RoleRootUser)
		if resp["success"] != true {
			t.Fatalf("更新应成功，实际: %v", resp)
		}
		if !getUserEmailVerified(t, user.Id) {
			t.Fatal("邮箱没变就不该降级")
		}
	})
}

// IdP 下发的已验证邮箱：回填 / 覆盖时一并记为已验证；地址没变但本地还记着未验证时就地升级。
func TestSyncOidcEmailMarksVerified(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", true)

	blank := seedOidcUser(t, &model.User{
		Username: "evblank", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-ev3", AffCode: "affev3",
	})
	syncOidcEmail(blank, "idp@example.com", provider)
	if !getUserEmailVerified(t, blank.Id) {
		t.Fatal("IdP 已验证邮箱回填后应记为已验证")
	}
	if !blank.EmailVerified {
		t.Fatal("内存中的标记也应同步更新")
	}

	provider.FirstParty = true
	stale := seedOidcUser(t, &model.User{
		Username: "evstale", Email: "stale@example.com", EmailVerified: false,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-ev4", AffCode: "affev4",
	})
	syncOidcEmail(stale, " Stale@Example.com ", provider)
	if got := getUserEmail(t, stale.Id); got != "stale@example.com" {
		t.Fatalf("地址不应被改写，实际 %q", got)
	}
	if !getUserEmailVerified(t, stale.Id) {
		t.Fatal("IdP 证实同一地址已验证后，本地未验证标记应升级")
	}
}

// OIDC 首登注册：随新账号落库的可信邮箱记为已验证。
func TestOidcRegisterMarksEmailVerified(t *testing.T) {
	setupOidcTestDB(t)
	oldRegister := config.RegisterEnabled
	config.RegisterEnabled = true
	t.Cleanup(func() { config.RegisterEnabled = oldRegister })

	provider := seedOidcProvider(t, "authgear", false)
	r := newOidcPhoneRouter(t)
	r.GET("/register", func(c *gin.Context) {
		oidcRegister(c, provider, "", "sub-ev", map[string]interface{}{"preferred_username": "evreg"}, "evreg@example.com")
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/register", nil))

	var created model.User
	if err := model.DB.First(&created, "username = ?", "evreg").Error; err != nil {
		t.Fatalf("注册用户应已落库: %v", err)
	}
	if string(created.Email) != "evreg@example.com" {
		t.Fatalf("可信邮箱应写入，实际 %q", created.Email)
	}
	if !created.EmailVerified {
		t.Fatal("注册时回填的可信邮箱应记为已验证")
	}
}

// 按已验证邮箱自动关联只命中自己邮箱也已验证的账号：管理员代填的邮箱不得成为登录入口，
// 未命中时继续走后续规则（这里是注册），不写身份行。
func TestResolveOidcUserSkipsUnverifiedEmail(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	seeded := seedOidcUser(t, &model.User{
		Username: "unverified", Email: "victim@example.com", EmailVerified: false,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-ev5", AffCode: "affev5",
	})

	user, outcome, err := resolveOidcUser(provider, "sub-ev-skip", "victim@example.com", "")
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if outcome != oidcLinkNone || user != nil {
		t.Fatalf("未验证邮箱不应命中，实际 outcome=%d user=%v", outcome, user)
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-ev-skip"); err == nil {
		t.Fatal("未关联就不该写身份行")
	}

	// 同一账号的邮箱被标为已验证后即可关联，证明拦截点确实是 email_verified。
	if err := model.DB.Model(&model.User{}).Where("id = ?", seeded.Id).
		Update("email_verified", true).Error; err != nil {
		t.Fatalf("准备数据失败: %v", err)
	}
	linked, outcome, err := resolveOidcUser(provider, "sub-ev-link", "victim@example.com", "")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("期望 oidcLinkLogin，实际 outcome=%d err=%v", outcome, err)
	}
	if linked.Id != seeded.Id {
		t.Fatalf("应关联到 %d，实际 %d", seeded.Id, linked.Id)
	}
}
