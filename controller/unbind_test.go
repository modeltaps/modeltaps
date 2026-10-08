package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// setUnbindAuthFlags 临时改写站点登录开关，用例结束后自动还原。
func setUnbindAuthFlags(t *testing.T, passwordLogin, githubOAuth bool) {
	t.Helper()
	// 解绑保护的用例都按内置账号体系写：密码与 GitHub 只有在这个形态下才算登录方式
	oldSystem, oldPassword, oldGitHub := config.AccountSystem, config.PasswordLoginEnabled, config.GitHubOAuthEnabled
	config.AccountSystem = config.AccountSystemBuiltin
	config.PasswordLoginEnabled, config.GitHubOAuthEnabled = passwordLogin, githubOAuth
	t.Cleanup(func() {
		config.AccountSystem = oldSystem
		config.PasswordLoginEnabled, config.GitHubOAuthEnabled = oldPassword, oldGitHub
	})
}

// setUnbindSmtpConfigured 临时把站点 SMTP 设为已配置 / 未配置，用例结束后自动还原。
func setUnbindSmtpConfigured(t *testing.T, configured bool) {
	t.Helper()
	oldServer, oldPort := config.SMTPServer, config.SMTPPort
	oldAccount, oldToken := config.SMTPAccount, config.SMTPToken
	if configured {
		config.SMTPServer, config.SMTPPort = "smtp.example.com", 587
		config.SMTPAccount, config.SMTPToken = "noreply@example.com", "smtp-token"
	} else {
		config.SMTPServer, config.SMTPPort = "", 0
		config.SMTPAccount, config.SMTPToken = "", ""
	}
	t.Cleanup(func() {
		config.SMTPServer, config.SMTPPort = oldServer, oldPort
		config.SMTPAccount, config.SMTPToken = oldAccount, oldToken
	})
}

// seedUnbindUser 建一个测试用户；password / email 传空串即模拟三方注册用户。
func seedUnbindUser(t *testing.T, username, password, email string) *model.User {
	t.Helper()
	return seedOidcUser(t, &model.User{
		Username: username, Password: password, Email: model.NullableEmail(email),
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-" + username, AffCode: "aff-" + username,
	})
}

// seedDisabledOidcProvider 建一个已停用的提供方。
func seedDisabledOidcProvider(t *testing.T, slug string) *model.OidcProvider {
	t.Helper()
	provider := seedOidcProvider(t, slug, false)
	if err := model.DB.Model(provider).Update("enabled", false).Error; err != nil {
		t.Fatalf("停用提供方失败: %v", err)
	}
	return provider
}

func callUnbind(t *testing.T, userId int, body string) map[string]interface{} {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/api/user/unbind", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", userId)
	Unbind(c)
	if w.Code != 200 {
		t.Fatalf("Unbind 应返回 200，实际 %d", w.Code)
	}
	return decodeResp(t, w)
}

// countIdentities 该用户剩余的 OIDC 身份行数，用于验证被拒时事务已回滚。
func countIdentities(t *testing.T, userId int) int64 {
	t.Helper()
	var n int64
	if err := model.DB.Model(&model.UserOidcIdentity{}).Where("user_id = ?", userId).Count(&n).Error; err != nil {
		t.Fatalf("统计身份行失败: %v", err)
	}
	return n
}

func assertUnbindRejectedWith(t *testing.T, resp map[string]interface{}, message string) {
	t.Helper()
	if resp["success"] != false {
		t.Fatalf("解绑应被拒，实际: %v", resp)
	}
	if resp["message"] != message {
		t.Fatalf("拒绝文案应为 %q，实际: %v", message, resp["message"])
	}
}

func assertUnbindRejected(t *testing.T, resp map[string]interface{}) {
	t.Helper()
	assertUnbindRejectedWith(t, resp, unbindLastLoginMethodMessage)
}

// 三方注册用户（无密码无邮箱）只有一个 OIDC 身份时，解绑必须被拒且数据库无变化。
func TestUnbindRejectsLastOidcIdentity(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	provider := seedOidcProvider(t, "idp", false)
	user := seedUnbindUser(t, "alice", "", "")
	seedOidcIdentity(t, user.Id, provider.Id, "sub-1")

	assertUnbindRejected(t, callUnbind(t, user.Id, `{"type":"oidc","provider":"idp"}`))
	if got := countIdentities(t, user.Id); got != 1 {
		t.Fatalf("被拒后身份行应保留，实际剩余 %d", got)
	}
}

// 有密码的用户解绑唯一 OIDC 身份应成功。
func TestUnbindAllowsOidcWhenPasswordExists(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	provider := seedOidcProvider(t, "idp", false)
	user := seedUnbindUser(t, "bob", "hashed-password", "")
	seedOidcIdentity(t, user.Id, provider.Id, "sub-1")

	resp := callUnbind(t, user.Id, `{"type":"oidc","provider":"idp"}`)
	if resp["success"] != true {
		t.Fatalf("有密码时解绑应成功，实际: %v", resp)
	}
	if got := countIdentities(t, user.Id); got != 0 {
		t.Fatalf("身份行应被删除，实际剩余 %d", got)
	}
}

// 绑定两个提供方时可解绑其一，剩下最后一个再解绑被拒。
func TestUnbindSecondOidcProviderRejected(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	first := seedOidcProvider(t, "idp-a", false)
	second := seedOidcProvider(t, "idp-b", false)
	user := seedUnbindUser(t, "carol", "", "")
	seedOidcIdentity(t, user.Id, first.Id, "sub-a")
	seedOidcIdentity(t, user.Id, second.Id, "sub-b")

	resp := callUnbind(t, user.Id, `{"type":"oidc","provider":"idp-a"}`)
	if resp["success"] != true {
		t.Fatalf("还剩一种方式时解绑应成功，实际: %v", resp)
	}
	assertUnbindRejected(t, callUnbind(t, user.Id, `{"type":"oidc","provider":"idp-b"}`))
	if got := countIdentities(t, user.Id); got != 1 {
		t.Fatalf("最后一个身份行应保留，实际剩余 %d", got)
	}
}

// 旧前端不带 provider 的全量 OIDC 解绑同样受保护。
func TestUnbindAllOidcIdentitiesRejected(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	first := seedOidcProvider(t, "idp-a", false)
	second := seedOidcProvider(t, "idp-b", false)
	user := seedUnbindUser(t, "dave", "", "")
	seedOidcIdentity(t, user.Id, first.Id, "sub-a")
	seedOidcIdentity(t, user.Id, second.Id, "sub-b")

	assertUnbindRejected(t, callUnbind(t, user.Id, `{"type":"oidc"}`))
	if got := countIdentities(t, user.Id); got != 2 {
		t.Fatalf("被拒后两行身份都应保留，实际剩余 %d", got)
	}
}

// 关闭密码登录后密码与邮箱都不算方式，仅剩 GitHub 的用户解绑 GitHub 被拒且列未被清空。
func TestUnbindGitHubRejectedWhenPasswordLoginDisabled(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, false, true)
	user := seedUnbindUser(t, "erin", "hashed-password", "erin@example.com")
	if err := model.DB.Model(user).Update("github_id", "gh-erin").Error; err != nil {
		t.Fatalf("写入 github_id 失败: %v", err)
	}

	assertUnbindRejected(t, callUnbind(t, user.Id, `{"type":"github"}`))
	var reloaded model.User
	if err := model.DB.First(&reloaded, "id = ?", user.Id).Error; err != nil {
		t.Fatalf("重新读取用户失败: %v", err)
	}
	if reloaded.GitHubId != "gh-erin" {
		t.Fatalf("被拒后 github_id 应保持不变，实际 %q", reloaded.GitHubId)
	}
}

// 提供方被停用后其身份不计入可用方式，解绑仅剩的启用提供方应被拒。
func TestUnbindIgnoresDisabledProviderIdentity(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	enabled := seedOidcProvider(t, "idp-on", false)
	disabled := seedDisabledOidcProvider(t, "idp-off")
	user := seedUnbindUser(t, "frank", "", "")
	seedOidcIdentity(t, user.Id, enabled.Id, "sub-on")
	seedOidcIdentity(t, user.Id, disabled.Id, "sub-off")

	assertUnbindRejected(t, callUnbind(t, user.Id, `{"type":"oidc","provider":"idp-on"}`))
	if got := countIdentities(t, user.Id); got != 2 {
		t.Fatalf("被拒后身份行应全部保留，实际剩余 %d", got)
	}
}

// 身份属停用提供方、账号零可用方式时被拒，文案不应称「唯一的登录方式」。
func TestUnbindDisabledProviderIdentityUsesNoMethodMessage(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	disabled := seedDisabledOidcProvider(t, "idp-off")
	user := seedUnbindUser(t, "grace", "", "")
	seedOidcIdentity(t, user.Id, disabled.Id, "sub-off")

	assertUnbindRejectedWith(t, callUnbind(t, user.Id, `{"type":"oidc","provider":"idp-off"}`), unbindNoLoginMethodMessage)
	if got := countIdentities(t, user.Id); got != 1 {
		t.Fatalf("被拒后身份行应保留，实际剩余 %d", got)
	}
}

// root 在站点关闭密码登录时仍可走 /login?local=1，密码算一种方式，解绑唯一 OIDC 身份应成功。
func TestUnbindAllowsRootEscapePasswordWhenPasswordLoginDisabled(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, false, false)
	provider := seedOidcProvider(t, "idp", false)
	user := seedUnbindUser(t, "ivy", "hashed-password", "")
	if err := model.DB.Model(user).Update("role", config.RoleRootUser).Error; err != nil {
		t.Fatalf("提升为 root 失败: %v", err)
	}
	seedOidcIdentity(t, user.Id, provider.Id, "sub-1")

	resp := callUnbind(t, user.Id, `{"type":"oidc","provider":"idp"}`)
	if resp["success"] != true {
		t.Fatalf("root 有逃生口密码时解绑应成功，实际: %v", resp)
	}
	if got := countIdentities(t, user.Id); got != 0 {
		t.Fatalf("身份行应被删除，实际剩余 %d", got)
	}
}

// 普通用户同场景没有逃生口，密码不算方式，解绑唯一 OIDC 身份仍被拒。
func TestUnbindRejectsNonRootWhenPasswordLoginDisabled(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, false, false)
	provider := seedOidcProvider(t, "idp", false)
	user := seedUnbindUser(t, "jack", "hashed-password", "")
	seedOidcIdentity(t, user.Id, provider.Id, "sub-1")

	assertUnbindRejected(t, callUnbind(t, user.Id, `{"type":"oidc","provider":"idp"}`))
	if got := countIdentities(t, user.Id); got != 1 {
		t.Fatalf("被拒后身份行应保留，实际剩余 %d", got)
	}
}

// 同一提供方下两个 subject 只算一种方式，解绑它仍会被拒。
func TestUnbindCountsMultipleSubjectsOfSameProviderOnce(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	provider := seedOidcProvider(t, "idp", false)
	user := seedUnbindUser(t, "kate", "", "")
	seedOidcIdentity(t, user.Id, provider.Id, "sub-1")
	seedOidcIdentity(t, user.Id, provider.Id, "sub-2")

	assertUnbindRejected(t, callUnbind(t, user.Id, `{"type":"oidc","provider":"idp"}`))
	if got := countIdentities(t, user.Id); got != 2 {
		t.Fatalf("被拒后两行身份都应保留，实际剩余 %d", got)
	}
}

// 有密码的用户解绑 GitHub 成功后，只清空 github 两列，password / email 不受影响。
func TestUnbindGitHubClearsOnlyGitHubColumns(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, true)
	user := seedUnbindUser(t, "henry", "hashed-password", "henry@example.com")
	if err := model.DB.Model(user).Updates(map[string]interface{}{"github_id": "gh-henry", "github_id_new": 4242}).Error; err != nil {
		t.Fatalf("写入 github 列失败: %v", err)
	}

	resp := callUnbind(t, user.Id, `{"type":"github"}`)
	if resp["success"] != true {
		t.Fatalf("有密码时解绑 GitHub 应成功，实际: %v", resp)
	}
	var reloaded model.User
	if err := model.DB.First(&reloaded, "id = ?", user.Id).Error; err != nil {
		t.Fatalf("重新读取用户失败: %v", err)
	}
	if reloaded.GitHubId != "" {
		t.Fatalf("github_id 应被清空，实际 %q", reloaded.GitHubId)
	}
	var nullCount int64
	if err := model.DB.Model(&model.User{}).Where("id = ? AND github_id_new IS NULL", user.Id).Count(&nullCount).Error; err != nil {
		t.Fatalf("检查 github_id_new 失败: %v", err)
	}
	if nullCount != 1 {
		t.Fatalf("github_id_new 应被置为 NULL")
	}
	if reloaded.Password != "hashed-password" {
		t.Fatalf("password 不应被改动，实际 %q", reloaded.Password)
	}
	if string(reloaded.Email) != "henry@example.com" {
		t.Fatalf("email 不应被改动，实际 %q", string(reloaded.Email))
	}
}

// 邮箱找回也算一种方式：开启密码登录且 SMTP 已配置时，无密码但有邮箱的用户可解绑唯一 OIDC 身份；
// 关闭密码登录后邮箱不再计入，同样的解绑被拒。
func TestUnbindCountsEmailRecovery(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "idp", false)

	setUnbindAuthFlags(t, true, false)
	setUnbindSmtpConfigured(t, true)
	mona := seedUnbindUser(t, "mona", "", "mona@example.com")
	seedOidcIdentity(t, mona.Id, provider.Id, "sub-mona")
	resp := callUnbind(t, mona.Id, `{"type":"oidc","provider":"idp"}`)
	if resp["success"] != true {
		t.Fatalf("有邮箱找回时解绑应成功，实际: %v", resp)
	}
	if got := countIdentities(t, mona.Id); got != 0 {
		t.Fatalf("身份行应被删除，实际剩余 %d", got)
	}

	setUnbindAuthFlags(t, false, false)
	nora := seedUnbindUser(t, "nora", "", "nora@example.com")
	seedOidcIdentity(t, nora.Id, provider.Id, "sub-nora")
	assertUnbindRejected(t, callUnbind(t, nora.Id, `{"type":"oidc","provider":"idp"}`))
	if got := countIdentities(t, nora.Id); got != 1 {
		t.Fatalf("被拒后身份行应保留，实际剩余 %d", got)
	}
}

// SMTP 未配置时发不出重置邮件，邮箱找回不算一种方式：无密码、只有一条 OIDC 身份的用户解绑被拒。
func TestUnbindRejectsWhenSmtpUnconfigured(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	setUnbindSmtpConfigured(t, false)
	provider := seedOidcProvider(t, "idp", false)
	user := seedUnbindUser(t, "pete", "", "pete@example.com")
	seedOidcIdentity(t, user.Id, provider.Id, "sub-pete")

	assertUnbindRejected(t, callUnbind(t, user.Id, `{"type":"oidc","provider":"idp"}`))
	if got := countIdentities(t, user.Id); got != 1 {
		t.Fatalf("被拒后身份行应保留，实际剩余 %d", got)
	}
}

// 解绑类型不在白名单内一律拒绝，不做任何写库。
func TestUnbindRejectsUnknownType(t *testing.T) {
	setupOidcTestDB(t)
	setUnbindAuthFlags(t, true, false)
	provider := seedOidcProvider(t, "idp", false)
	user := seedUnbindUser(t, "olga", "hashed-password", "olga@example.com")
	seedOidcIdentity(t, user.Id, provider.Id, "sub-olga")

	assertUnbindRejectedWith(t, callUnbind(t, user.Id, `{"type":"password"}`), "Unknown link type")
	assertUnbindRejectedWith(t, callUnbind(t, user.Id, `{"type":"email"}`), "Unknown link type")
	var reloaded model.User
	if err := model.DB.First(&reloaded, "id = ?", user.Id).Error; err != nil {
		t.Fatalf("重新读取用户失败: %v", err)
	}
	if reloaded.Password != "hashed-password" || string(reloaded.Email) != "olga@example.com" {
		t.Fatalf("未知类型不应清空任何列，实际 password=%q email=%q", reloaded.Password, string(reloaded.Email))
	}
	if got := countIdentities(t, user.Id); got != 1 {
		t.Fatalf("未知类型不应删除身份行，实际剩余 %d", got)
	}
}
