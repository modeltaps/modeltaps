package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// GitHub 登录只取 GitHub 侧 primary + verified 的邮箱（见 getGithubEmail），所以写进
// users.email 的地址与 OIDC 可信邮箱同级，一并记 email_verified = true；反过来按邮箱
// 关联已有账号时，也要求目标账号自己的邮箱已验证（AUTH-8，与 lookupOidcLinkTarget 一致）。

// stubGitHubUserInfo 替换取 GitHub 用户信息的实现，避免单测打真实网络。
func stubGitHubUserInfo(t *testing.T, githubUser *GitHubUser) {
	t.Helper()
	old := getGitHubUserInfoByCode
	getGitHubUserInfoByCode = func(code string) (*GitHubUser, error) {
		return githubUser, nil
	}
	t.Cleanup(func() { getGitHubUserInfoByCode = old })
}

// enableGitHubLogin 打开 GitHub 登录与注册（内置账号体系）。
func enableGitHubLogin(t *testing.T) {
	t.Helper()
	oldSystem, oldOAuth, oldRegister := config.AccountSystem, config.GitHubOAuthEnabled, config.RegisterEnabled
	config.AccountSystem = config.AccountSystemBuiltin
	config.GitHubOAuthEnabled = true
	config.RegisterEnabled = true
	t.Cleanup(func() {
		config.AccountSystem = oldSystem
		config.GitHubOAuthEnabled = oldOAuth
		config.RegisterEnabled = oldRegister
	})
}

// newGitHubRouter 带 session 中间件，GitHubOAuth 的 state 校验与 GitHubBind 的登录态都要它。
func newGitHubRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.GET("/api/oauth/state", GenerateOAuthCode)
	r.GET("/api/oauth/github", GitHubOAuth)
	return r
}

// githubOAuthLogin 走一遍「取 state → 携带 state 回调」的完整前端流程。
func githubOAuthLogin(t *testing.T, r *gin.Engine, extraCookie string) map[string]interface{} {
	t.Helper()
	w1 := httptest.NewRecorder()
	req1 := httptest.NewRequest(http.MethodGet, "/api/oauth/state", nil)
	if extraCookie != "" {
		req1.Header.Set("Cookie", extraCookie)
	}
	r.ServeHTTP(w1, req1)
	var stateResp map[string]interface{}
	if err := json.Unmarshal(w1.Body.Bytes(), &stateResp); err != nil {
		t.Fatalf("解析 state 响应失败: %v", err)
	}
	state, _ := stateResp["data"].(string)
	if state == "" {
		t.Fatal("未获取到 state")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/oauth/github?code=gh-code&state="+state, nil)
	req2.Header.Set("Cookie", w1.Header().Get("Set-Cookie"))
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var resp map[string]interface{}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return resp
}

// 按邮箱关联已有账号只命中自己邮箱也已验证的账号：管理员代填的邮箱不得成为登录入口，
// 未验证时按「未命中」处理。把同一账号标为已验证后即可命中，证明拦截点确实是 email_verified。
func TestGetUserByGitHubSkipsUnverifiedEmail(t *testing.T) {
	setupGetUserTestDB(t)
	seeded := seedEmailUser(t, &model.User{
		Username: "ghvictim", Email: "victim@example.com", EmailVerified: false,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-gh1", AffCode: "affgh1",
	})

	githubUser := &GitHubUser{Id: 9001, Login: "attacker", Email: "victim@example.com"}
	matched, err := getUserByGitHub(githubUser)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if matched != nil {
		t.Fatalf("未验证邮箱不应命中，实际关联到 user_id=%d", matched.Id)
	}

	if err := model.DB.Model(&model.User{}).Where("id = ?", seeded.Id).
		Update("email_verified", true).Error; err != nil {
		t.Fatalf("准备数据失败: %v", err)
	}
	linked, err := getUserByGitHub(githubUser)
	if err != nil {
		t.Fatalf("不应报错: %v", err)
	}
	if linked == nil || linked.Id != seeded.Id {
		t.Fatalf("已验证邮箱应命中 user_id=%d，实际 %v", seeded.Id, linked)
	}
}

// 首次用 GitHub 登录新建账号：随账号落库的 GitHub 邮箱记为已验证。
func TestGitHubOAuthMarksNewUserEmailVerified(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	stubGitHubUserInfo(t, &GitHubUser{Id: 9100, Login: "newcomer", Name: "New Comer", Email: "newcomer@example.com"})

	resp := githubOAuthLogin(t, newGitHubRouter(t), "")
	if resp["success"] != true {
		t.Fatalf("登录应成功，实际: %v", resp)
	}

	var created model.User
	if err := model.DB.First(&created, "github_id_new = ?", 9100).Error; err != nil {
		t.Fatalf("新账号应已落库: %v", err)
	}
	if string(created.Email) != "newcomer@example.com" {
		t.Fatalf("GitHub 邮箱应写入，实际 %q", created.Email)
	}
	if !created.EmailVerified {
		t.Fatal("GitHub 的 primary+verified 邮箱应记为已验证")
	}
}

// 邮箱已归属一个未验证账号时：关联被拦下，注册照常进行，但新账号不带这个邮箱——
// 否则 Insert 会撞唯一索引，这个 GitHub 账号永远注册不进来。
func TestGitHubOAuthRegistersWithoutTakenEmail(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	seedEmailUser(t, &model.User{
		Username: "ghholder", Email: "held@example.com", EmailVerified: false,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-gh2", AffCode: "affgh2",
	})
	stubGitHubUserInfo(t, &GitHubUser{Id: 9200, Login: "stranger", Email: "held@example.com"})

	resp := githubOAuthLogin(t, newGitHubRouter(t), "")
	if resp["success"] != true {
		t.Fatalf("注册应成功，实际: %v", resp)
	}

	var created model.User
	if err := model.DB.First(&created, "github_id_new = ?", 9200).Error; err != nil {
		t.Fatalf("新账号应已落库: %v", err)
	}
	if created.Email != "" {
		t.Fatalf("被占用的邮箱不应落到新账号，实际 %q", created.Email)
	}
	if created.EmailVerified {
		t.Fatal("没写邮箱就不该记已验证")
	}
}

// 已有账号用 GitHub 登录且本地邮箱为空：回填的 GitHub 邮箱记为已验证。
func TestGitHubOAuthBackfillMarksEmailVerified(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	seeded := seedEmailUser(t, &model.User{
		Username: "ghold", GitHubIdNew: 9300,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-gh3", AffCode: "affgh3",
	})
	stubGitHubUserInfo(t, &GitHubUser{Id: 9300, Login: "ghold", Email: "ghold@example.com"})

	resp := githubOAuthLogin(t, newGitHubRouter(t), "")
	if resp["success"] != true {
		t.Fatalf("登录应成功，实际: %v", resp)
	}

	stored, err := model.GetUserById(seeded.Id, false)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if string(stored.Email) != "ghold@example.com" {
		t.Fatalf("邮箱应回填，实际 %q", stored.Email)
	}
	if !stored.EmailVerified {
		t.Fatal("登录回填的 GitHub 邮箱应记为已验证")
	}
}

// 已登录用户绑定 GitHub 且本地邮箱为空：回填的 GitHub 邮箱同样记为已验证。
func TestGitHubBindMarksEmailVerified(t *testing.T) {
	setupGetUserTestDB(t)
	enableGitHubLogin(t)
	seeded := seedEmailUser(t, &model.User{
		Username: "ghbinder", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-gh4", AffCode: "affgh4",
	})
	stubGitHubUserInfo(t, &GitHubUser{Id: 9400, Login: "ghbinder-gh", Email: "ghbinder@example.com"})

	r := newGitHubRouter(t)
	// GitHubOAuth 见到 session 里有 username 就转交 GitHubBind，模拟「已登录后去绑定」
	r.GET("/test/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", seeded.Id)
		session.Set("username", seeded.Username)
		_ = session.Save()
		c.Status(http.StatusOK)
	})
	w0 := httptest.NewRecorder()
	r.ServeHTTP(w0, httptest.NewRequest(http.MethodGet, "/test/login", nil))

	resp := githubOAuthLogin(t, r, w0.Header().Get("Set-Cookie"))
	if resp["success"] != true {
		t.Fatalf("绑定应成功，实际: %v", resp)
	}

	stored, err := model.GetUserById(seeded.Id, false)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if string(stored.Email) != "ghbinder@example.com" {
		t.Fatalf("邮箱应回填，实际 %q", stored.Email)
	}
	if !stored.EmailVerified {
		t.Fatal("绑定回填的 GitHub 邮箱应记为已验证")
	}
}
