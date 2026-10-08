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

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func setupLoginRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.POST("/api/user/login", Login)

	oldEnabled := config.PasswordLoginEnabled
	config.PasswordLoginEnabled = true
	t.Cleanup(func() { config.PasswordLoginEnabled = oldEnabled })
	return r
}

func postLogin(t *testing.T, r *gin.Engine, username, password string) map[string]interface{} {
	t.Helper()
	body := strings.NewReader(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/user/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("登录接口应返回 200，实际 %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	return resp
}

// 锁定期内的响应文案与状态码需与普通认证失败完全一致，避免泄露账号是否被锁。
func TestLoginLockedReturnsSameMessage(t *testing.T) {
	setupGetUserTestDB(t)
	r := setupLoginRouter(t)

	const wantMessage = "incorrect username or password, or the user is banned"

	resp := postLogin(t, r, "ghost", "wrong-password")
	if resp["success"] != false {
		t.Fatal("不存在的账号应登录失败")
	}
	if got := resp["message"]; got != wantMessage {
		t.Fatalf("首次失败文案 = %v，期望 %q", got, wantMessage)
	}

	ghostKey := common.LoginFailureKeyForIdentifier("ghost")
	for i := 0; i < config.LoginMaxFailures; i++ {
		postLogin(t, r, "ghost", "wrong-password")
	}
	if !common.IsLoginLocked(ghostKey) {
		t.Fatal("连续失败后账号应进入锁定")
	}
	t.Cleanup(func() { common.ClearLoginFailures(ghostKey) })

	locked := postLogin(t, r, "ghost", "wrong-password")
	if locked["success"] != false {
		t.Fatal("锁定期内应登录失败")
	}
	if got := locked["message"]; got != wantMessage {
		t.Fatalf("锁定期文案 = %v，期望与普通失败一致 %q", got, wantMessage)
	}
}

// 同一账号用「用户名」与「邮箱」两种写法失败时，计数必须收敛到同一 key(uid)，
// 否则换一种写法即可把可用尝试次数翻倍。
func TestLoginFailuresConvergeAcrossIdentifierForms(t *testing.T) {
	setupGetUserTestDB(t)
	r := setupLoginRouter(t)

	hash, err := common.Password2Hash("correct-password")
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	u := &model.User{
		Username:    "frank",
		Email:       model.NullableEmail("frank@example.com"),
		Password:    hash,
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeNormal,
		AccessToken: "test-token-frank",
		AffCode:     "afff",
	}
	if err := model.DB.Create(u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	uidKey := common.LoginFailureKeyForUser(u.Id)
	t.Cleanup(func() { common.ClearLoginFailures(uidKey) })

	// 一半失败走用户名，一半走邮箱；累加后应达到阈值
	for i := 0; i < config.LoginMaxFailures-1; i++ {
		ident := "frank"
		if i%2 == 1 {
			ident = "Frank@Example.com"
		}
		postLogin(t, r, ident, "wrong-password")
	}
	if common.IsLoginLocked(uidKey) {
		t.Fatal("未达阈值前不应锁定")
	}
	postLogin(t, r, "frank@example.com", "wrong-password")
	if !common.IsLoginLocked(uidKey) {
		t.Fatal("用户名与邮箱失败次数应累加到同一 uid key")
	}

	// 锁定期内即使密码正确也应拒绝，且文案与普通失败一致
	locked := postLogin(t, r, "frank", "correct-password")
	if locked["success"] != false {
		t.Fatal("锁定期内即使密码正确也应拒绝")
	}
	if got := locked["message"]; got != "incorrect username or password, or the user is banned" {
		t.Fatalf("锁定期文案 = %v，期望与普通失败一致", got)
	}
}

// 用户名恰好等于他人邮箱时不得跨列误命中：按用户名登录只查 username 列，
// 按邮箱登录只查 email 列。
func TestLoginIdentifierNoCrossColumnFallback(t *testing.T) {
	setupGetUserTestDB(t)
	r := setupLoginRouter(t)

	hash, err := common.Password2Hash("victim-password")
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	victim := &model.User{
		Username:    "victim",
		Email:       model.NullableEmail("shared@example.com"),
		Password:    hash,
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeNormal,
		AccessToken: "test-token-victim",
		AffCode:     "affv",
	}
	if err := model.DB.Create(victim).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	attackerHash, err := common.Password2Hash("attacker-password")
	if err != nil {
		t.Fatalf("生成密码哈希失败: %v", err)
	}
	attacker := &model.User{
		Username:    "shared@example.com",
		Password:    attackerHash,
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		Type:        config.UserTypeNormal,
		AccessToken: "test-token-attacker",
		AffCode:     "affa",
	}
	if err := model.DB.Create(attacker).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	t.Cleanup(func() {
		common.ClearLoginFailures(common.LoginFailureKeyForUser(victim.Id))
		common.ClearLoginFailures(common.LoginFailureKeyForUser(attacker.Id))
	})

	// 标识符含 @ → 按邮箱解析，命中 victim；attacker 的密码不应通过
	resp := postLogin(t, r, "shared@example.com", "attacker-password")
	if resp["success"] != false {
		t.Fatal("含 @ 的标识符应只按邮箱解析，不应命中同名用户名账号")
	}
	resp = postLogin(t, r, "shared@example.com", "victim-password")
	if resp["success"] != true {
		t.Fatalf("含 @ 的标识符应命中邮箱所属账号，实际: %v", resp)
	}
}
