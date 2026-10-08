package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupOidcTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.OidcProvider{}, &model.UserOidcIdentity{}, &model.UserSession{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	// OIDC 登录只在外部账号体系下受理，测试统一切到该形态
	oldSystem := config.AccountSystem
	config.AccountSystem = config.AccountSystemExternal
	t.Cleanup(func() {
		model.DB = oldDB
		config.AccountSystem = oldSystem
	})
}

func seedOidcUser(t *testing.T, u *model.User) *model.User {
	t.Helper()
	if err := model.DB.Create(u).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	return u
}

// seedOidcProvider 建一个已启用的提供方；linkByEmail 控制是否允许按已验证邮箱关联。
func seedOidcProvider(t *testing.T, slug string, linkByEmail bool) *model.OidcProvider {
	t.Helper()
	provider := &model.OidcProvider{
		Slug:                slug,
		DisplayName:         slug,
		Issuer:              "https://idp.example.com",
		ClientId:            "client-" + slug,
		Scopes:              "openid,profile,email",
		UsernameClaim:       "preferred_username",
		DisplayNameClaim:    "name",
		AvatarClaim:         "picture",
		LinkByVerifiedEmail: linkByEmail,
		Enabled:             true,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("创建提供方失败: %v", err)
	}
	return provider
}

func seedOidcIdentity(t *testing.T, userId, providerId int, subject string) {
	t.Helper()
	identity := model.UserOidcIdentity{UserId: userId, ProviderId: providerId, Subject: subject}
	if err := identity.Insert(); err != nil {
		t.Fatalf("创建身份行失败: %v", err)
	}
}

// email_verified 需同时接受布尔 true 与字符串 "true"；缺失/false/其他类型视为未验证。
func TestOidcTrustedEmail(t *testing.T) {
	cases := []struct {
		name   string
		claims map[string]interface{}
		want   string
	}{
		{"布尔已验证", map[string]interface{}{"email": "Alice@Example.com", "email_verified": true}, "alice@example.com"},
		{"字符串已验证", map[string]interface{}{"email": "bob@example.com", "email_verified": "true"}, "bob@example.com"},
		{"未验证", map[string]interface{}{"email": "carol@example.com", "email_verified": false}, ""},
		{"缺 email_verified", map[string]interface{}{"email": "dave@example.com"}, ""},
		{"缺 email", map[string]interface{}{"email_verified": true}, ""},
		{"email 非字符串", map[string]interface{}{"email": 42, "email_verified": true}, ""},
		{"email 格式非法", map[string]interface{}{"email": "not-an-email", "email_verified": true}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := oidcTrustedEmail(tc.claims); got != tc.want {
				t.Fatalf("oidcTrustedEmail = %q，期望 %q", got, tc.want)
			}
		})
	}
}

// claim 值非字符串时必须返回空串而不是 panic：IdP 侧可下发任意类型。
func TestOidcStringClaim(t *testing.T) {
	claims := map[string]interface{}{
		"name":    "Alice",
		"picture": 42,
		"nilval":  nil,
	}
	cases := map[string]string{"name": "Alice", "picture": "", "nilval": "", "missing": "", "": ""}
	for name, want := range cases {
		if got := oidcStringClaim(claims, name); got != want {
			t.Fatalf("oidcStringClaim(%q) = %q，期望 %q", name, got, want)
		}
	}
}

// 身份行命中即登录，不受邮箱 claim 影响。
func TestResolveOidcUserByIdentity(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	user := seedOidcUser(t, &model.User{
		Username: "alice", Email: "alice@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-1", AffCode: "aff1",
	})
	seedOidcIdentity(t, user.Id, provider.Id, "sub-1")

	got, outcome, err := resolveOidcUser(provider, "sub-1", "", "")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("期望 oidcLinkLogin，实际 outcome=%d err=%v", outcome, err)
	}
	if got.Username != "alice" {
		t.Fatalf("命中用户应为 alice，实际 %q", got.Username)
	}
}

// 身份未命中但邮箱已验证且唯一命中已有账号：补写身份行后登录。
func TestResolveOidcUserLinksByVerifiedEmail(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	seeded := seedOidcUser(t, &model.User{
		Username: "bob", Email: "bob@example.com", EmailVerified: true,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-2", AffCode: "aff2",
	})

	user, outcome, err := resolveOidcUser(provider, "sub-2", "bob@example.com", "")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("期望 oidcLinkLogin，实际 outcome=%d err=%v", outcome, err)
	}
	if user.Id != seeded.Id {
		t.Fatalf("应关联到已有用户 %d，实际 %d", seeded.Id, user.Id)
	}

	identity, err := model.FindUserOidcIdentity(provider.Id, "sub-2")
	if err != nil {
		t.Fatalf("身份行应已落库: %v", err)
	}
	if identity.UserId != seeded.Id {
		t.Fatalf("身份行应归属 %d，实际 %d", seeded.Id, identity.UserId)
	}
}

// 目标账号在同一提供方下已绑定别的 subject：拒绝按邮箱自动关联，且不写入新身份行。
func TestResolveOidcUserRefusesLinkWhenProviderIdentityExists(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	seeded := seedOidcUser(t, &model.User{
		Username: "conflict", Email: "conflict@example.com", EmailVerified: true,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-c1", AffCode: "affc1",
	})
	seedOidcIdentity(t, seeded.Id, provider.Id, "sub-old")

	user, outcome, err := resolveOidcUser(provider, "sub-new", "conflict@example.com", "")
	if err != nil || outcome != oidcLinkConflict {
		t.Fatalf("期望 oidcLinkConflict，实际 outcome=%d err=%v", outcome, err)
	}
	if user == nil || user.Id != seeded.Id {
		t.Fatalf("应返回冲突的目标账号 %d，实际 %v", seeded.Id, user)
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-new"); err == nil {
		t.Fatal("冲突时不应写入新身份行")
	}
	old, err := model.FindUserOidcIdentity(provider.Id, "sub-old")
	if err != nil || old.UserId != seeded.Id {
		t.Fatalf("原身份行应保持不变，实际 %v err=%v", old, err)
	}
}

// 目标账号只在别的提供方有身份行：邮箱关联照常放行。
func TestResolveOidcUserLinksByEmailAcrossProviders(t *testing.T) {
	setupOidcTestDB(t)
	providerP := seedOidcProvider(t, "oidc", true)
	providerQ := seedOidcProvider(t, "authgear", true)
	seeded := seedOidcUser(t, &model.User{
		Username: "cross", Email: "cross@example.com", EmailVerified: true,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-c2", AffCode: "affc2",
	})
	seedOidcIdentity(t, seeded.Id, providerQ.Id, "sub-q")

	user, outcome, err := resolveOidcUser(providerP, "sub-p", "cross@example.com", "")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("期望 oidcLinkLogin，实际 outcome=%d err=%v", outcome, err)
	}
	if user.Id != seeded.Id {
		t.Fatalf("应关联到已有用户 %d，实际 %d", seeded.Id, user.Id)
	}
	identity, err := model.FindUserOidcIdentity(providerP.Id, "sub-p")
	if err != nil || identity.UserId != seeded.Id {
		t.Fatalf("提供方 P 的身份行应已落库并归属 %d，实际 %v err=%v", seeded.Id, identity, err)
	}
}

// link_by_verified_email=false 时即使邮箱已验证也不得关联已有账号。
func TestResolveOidcUserLinkByEmailDisabled(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", false)
	seedOidcUser(t, &model.User{
		Username: "bella", Email: "bella@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-2b", AffCode: "aff2b",
	})

	if _, outcome, err := resolveOidcUser(provider, "sub-2b", "bella@example.com", ""); err != nil || outcome != oidcLinkNone {
		t.Fatalf("关闭邮箱关联应返回 oidcLinkNone，实际 outcome=%d err=%v", outcome, err)
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-2b"); err == nil {
		t.Fatal("关闭邮箱关联时不应写入身份行")
	}
}

// 同一 subject 在不同提供方下是两个互不相干的身份。
func TestResolveOidcUserIsolatedByProvider(t *testing.T) {
	setupOidcTestDB(t)
	first := seedOidcProvider(t, "oidc", false)
	second := seedOidcProvider(t, "authgear", false)
	userA := seedOidcUser(t, &model.User{
		Username: "ann", Email: "ann@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-7", AffCode: "aff7",
	})
	userB := seedOidcUser(t, &model.User{
		Username: "ben", Email: "ben@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-8", AffCode: "aff8",
	})
	seedOidcIdentity(t, userA.Id, first.Id, "same-sub")
	seedOidcIdentity(t, userB.Id, second.Id, "same-sub")

	got, outcome, err := resolveOidcUser(first, "same-sub", "", "")
	if err != nil || outcome != oidcLinkLogin || got.Id != userA.Id {
		t.Fatalf("提供方 oidc 应命中 userA，实际 id=%v outcome=%d err=%v", got, outcome, err)
	}
	got, outcome, err = resolveOidcUser(second, "same-sub", "", "")
	if err != nil || outcome != oidcLinkLogin || got.Id != userB.Id {
		t.Fatalf("提供方 authgear 应命中 userB，实际 id=%v outcome=%d err=%v", got, outcome, err)
	}
}

// 邮箱未验证（trustedEmail 为空）时不得关联已有账号，应走注册。
func TestResolveOidcUserUnverifiedEmailDoesNotLink(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	seedOidcUser(t, &model.User{
		Username: "carol", Email: "carol@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-3", AffCode: "aff3",
	})

	if _, outcome, err := resolveOidcUser(provider, "sub-3", "", ""); err != nil || outcome != oidcLinkNone {
		t.Fatalf("未验证邮箱应返回 oidcLinkNone，实际 outcome=%d err=%v", outcome, err)
	}
}

// username claim 同名不构成关联依据：邮箱不匹配时一律走注册。
func TestResolveOidcUserIgnoresUsernameMatch(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	seedOidcUser(t, &model.User{
		Username: "dave", Email: "dave@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-4", AffCode: "aff4",
	})

	if _, outcome, err := resolveOidcUser(provider, "sub-4", "other@example.com", ""); err != nil || outcome != oidcLinkNone {
		t.Fatalf("邮箱不匹配应返回 oidcLinkNone，实际 outcome=%d err=%v", outcome, err)
	}
}

// 影子记账账户即使邮箱匹配也不得被 OIDC 关联。
func TestResolveOidcUserExcludesShadowUser(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	seedOidcUser(t, &model.User{
		Username: "org-acme", Email: "shadow@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeOrgShadow, AccessToken: "tok-5", AffCode: "aff5",
	})

	if _, outcome, err := resolveOidcUser(provider, "sub-5", "shadow@example.com", ""); err != nil || outcome != oidcLinkNone {
		t.Fatalf("影子账户不应被关联，实际 outcome=%d err=%v", outcome, err)
	}
}

// 邮箱命中但账号被封禁：拒绝登录，不写入身份行。
func TestResolveOidcUserDisabledAccount(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	seedOidcUser(t, &model.User{
		Username: "erin", Email: "erin@example.com", EmailVerified: true,
		Role: config.RoleCommonUser, Status: config.UserStatusDisabled,
		Type: config.UserTypeNormal, AccessToken: "tok-6", AffCode: "aff6",
	})

	if _, outcome, err := resolveOidcUser(provider, "sub-6", "erin@example.com", ""); err != nil || outcome != oidcLinkDisabled {
		t.Fatalf("封禁账号应返回 oidcLinkDisabled，实际 outcome=%d err=%v", outcome, err)
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-6"); err == nil {
		t.Fatal("封禁账号不应写入身份行")
	}
}

// 登录态绑定：会话用户正常启用时，未占用的 (provider, subject) 应写入身份行。
func TestBindOidcIdentityNewBinding(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", false)
	user := seedOidcUser(t, &model.User{
		Username: "binder", Email: "binder@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b1", AffCode: "affb1",
	})

	outcome, err := bindOidcIdentity(provider.Id, user.Id, "sub-bind")
	if err != nil || outcome != oidcBindOK {
		t.Fatalf("期望 oidcBindOK，实际 outcome=%d err=%v", outcome, err)
	}
	identity, err := model.FindUserOidcIdentity(provider.Id, "sub-bind")
	if err != nil || identity.UserId != user.Id {
		t.Fatalf("身份行应归属 %d，实际 %v err=%v", user.Id, identity, err)
	}
}

// 登录态绑定：身份已归属自己时幂等成功，不重复写行。
func TestBindOidcIdentityIdempotent(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", false)
	user := seedOidcUser(t, &model.User{
		Username: "rebinder", Email: "rebinder@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b2", AffCode: "affb2",
	})
	seedOidcIdentity(t, user.Id, provider.Id, "sub-again")

	outcome, err := bindOidcIdentity(provider.Id, user.Id, "sub-again")
	if err != nil || outcome != oidcBindOK {
		t.Fatalf("期望 oidcBindOK，实际 outcome=%d err=%v", outcome, err)
	}
	if got := countOidcIdentities(t, user.Id); got != 1 {
		t.Fatalf("幂等绑定不应重复写行，实际 %d 行", got)
	}
}

// 登录态绑定：会话用户已被封禁时拒绝，且不写入身份行。
func TestBindOidcIdentityRejectsDisabledUser(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", false)
	user := seedOidcUser(t, &model.User{
		Username: "banned", Email: "banned@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusDisabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b3", AffCode: "affb3",
	})

	outcome, err := bindOidcIdentity(provider.Id, user.Id, "sub-banned")
	if err != nil || outcome != oidcBindDenied {
		t.Fatalf("封禁用户应返回 oidcBindDenied，实际 outcome=%d err=%v", outcome, err)
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-banned"); err == nil {
		t.Fatal("封禁用户不应写入身份行")
	}
}

// 登录态绑定：会话用户已软删时拒绝（会话可能签发于删除之前），且不写入身份行。
func TestBindOidcIdentityRejectsDeletedUser(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", false)
	user := seedOidcUser(t, &model.User{
		Username: "gone", Email: "gone@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b4", AffCode: "affb4",
	})
	if err := user.Delete(); err != nil {
		t.Fatalf("软删用户失败: %v", err)
	}

	outcome, err := bindOidcIdentity(provider.Id, user.Id, "sub-gone")
	if err != nil || outcome != oidcBindDenied {
		t.Fatalf("软删用户应返回 oidcBindDenied，实际 outcome=%d err=%v", outcome, err)
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-gone"); err == nil {
		t.Fatal("软删用户不应写入身份行")
	}
}

// 登录态绑定：身份已归属他人时拒绝，且不改动原身份行。
func TestBindOidcIdentityRejectsTakenSubject(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", false)
	owner := seedOidcUser(t, &model.User{
		Username: "owner", Email: "owner@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b5", AffCode: "affb5",
	})
	other := seedOidcUser(t, &model.User{
		Username: "other", Email: "other@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b6", AffCode: "affb6",
	})
	seedOidcIdentity(t, owner.Id, provider.Id, "sub-taken")

	outcome, err := bindOidcIdentity(provider.Id, other.Id, "sub-taken")
	if err != nil || outcome != oidcBindTaken {
		t.Fatalf("期望 oidcBindTaken，实际 outcome=%d err=%v", outcome, err)
	}
	identity, err := model.FindUserOidcIdentity(provider.Id, "sub-taken")
	if err != nil || identity.UserId != owner.Id {
		t.Fatalf("原身份行应保持归属 %d，实际 %v err=%v", owner.Id, identity, err)
	}
}

func countOidcIdentities(t *testing.T, userId int) int64 {
	t.Helper()
	var count int64
	if err := model.DB.Model(&model.UserOidcIdentity{}).Where("user_id = ?", userId).Count(&count).Error; err != nil {
		t.Fatalf("统计身份行失败: %v", err)
	}
	return count
}

// (provider, subject) 已被他人占用时，第二个用户再绑定必须被唯一索引拒绝。
func TestOidcIdentityBindConflict(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", false)
	userA := seedOidcUser(t, &model.User{
		Username: "ownera", Email: "ownera@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-9", AffCode: "aff9",
	})
	userB := seedOidcUser(t, &model.User{
		Username: "ownerb", Email: "ownerb@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-10", AffCode: "aff10",
	})
	seedOidcIdentity(t, userA.Id, provider.Id, "taken-sub")

	identity, err := model.FindUserOidcIdentity(provider.Id, "taken-sub")
	if err != nil || identity.UserId == userB.Id {
		t.Fatalf("该身份应归属 userA，实际 %v err=%v", identity, err)
	}
	conflict := model.UserOidcIdentity{UserId: userB.Id, ProviderId: provider.Id, Subject: "taken-sub"}
	if err := conflict.Insert(); err == nil {
		t.Fatal("重复 (provider, subject) 应被唯一索引拒绝")
	}
}

// 未启用的提供方不出现在 /api/status 的登录入口列表里。
func TestLoginOidcProviderStatus(t *testing.T) {
	setupOidcTestDB(t)
	seedOidcProvider(t, "oidc", true)
	disabled := seedOidcProvider(t, "authgear", true)
	disabled.Enabled = false
	if err := disabled.Update(); err != nil {
		t.Fatalf("停用提供方失败: %v", err)
	}

	got := loginOidcProviderStatus()
	if len(got) != 1 || got[0].Slug != "oidc" {
		t.Fatalf("只应下发唯一启用的 oidc，实际 %+v", got)
	}

	// 内置账号模式下提供方不是登录入口，不下发
	config.AccountSystem = config.AccountSystemBuiltin
	if got := loginOidcProviderStatus(); len(got) != 0 {
		t.Fatalf("内置账号模式不应下发提供方，实际 %+v", got)
	}
}

// account_settings_url 要同时出现在 /api/status 的登录入口与 /api/user/self 的身份列表里；
// 已停用的提供方也得下发，账号安全页仍要能给出改密出口。
func TestOidcAccountSettingsUrlDownstream(t *testing.T) {
	setupOidcTestDB(t)
	const settingsUrl = "https://idp.example.com/settings"
	enabled := seedOidcProvider(t, "oidc", true)
	enabled.AccountSettingsUrl = settingsUrl
	if err := enabled.Update(); err != nil {
		t.Fatalf("更新提供方失败: %v", err)
	}
	disabled := seedOidcProvider(t, "authgear", true)
	disabled.AccountSettingsUrl = settingsUrl
	disabled.Enabled = false
	if err := disabled.Update(); err != nil {
		t.Fatalf("停用提供方失败: %v", err)
	}

	got := loginOidcProviderStatus()
	if len(got) != 1 || got[0].AccountSettingsUrl != settingsUrl {
		t.Fatalf("/api/status 应下发 account_settings_url，实际 %+v", got)
	}

	user := seedOidcUser(t, &model.User{Username: "grace", AccessToken: "token-grace", AffCode: "aff-grace"})
	for _, provider := range []*model.OidcProvider{enabled, disabled} {
		identity := &model.UserOidcIdentity{UserId: user.Id, ProviderId: provider.Id, Subject: "sub-" + provider.Slug}
		if err := identity.Insert(); err != nil {
			t.Fatalf("创建身份失败: %v", err)
		}
	}

	identities := selfOidcIdentities(user.Id)
	if len(identities) != 2 {
		t.Fatalf("应下发两条身份（含已停用提供方），实际 %+v", identities)
	}
	for _, identity := range identities {
		if identity.AccountSettingsUrl != settingsUrl {
			t.Fatalf("%s 身份缺少 account_settings_url，实际 %+v", identity.ProviderSlug, identity)
		}
	}
}

// /api/status 只下发承担本站身份的那一个提供方：多个启用时取标记为 first_party 的；
// 都没标记时配置不完整，什么都不下发。
func TestLoginOidcProviderStatusFirstParty(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "oidc", true)
	seedOidcProvider(t, "authgear", true)

	if got := loginOidcProviderStatus(); len(got) != 0 {
		t.Fatalf("两个启用且都未标记本站身份时不应下发，实际 %+v", got)
	}

	provider.FirstParty = true
	if err := provider.Update(); err != nil {
		t.Fatalf("更新提供方失败: %v", err)
	}
	got := loginOidcProviderStatus()
	if len(got) != 1 {
		t.Fatalf("应只下发本站身份提供方，实际 %+v", got)
	}
	for _, item := range got {
		if item.Slug != "oidc" || !item.FirstParty {
			t.Fatalf("应下发 oidc 且 first_party=true，实际 %+v", item)
		}
		if item.Slug == "authgear" {
			t.Fatalf("authgear 不应出现，实际 %+v", item)
		}
	}
}

// callOidcRegisterOrReject 用带 session 的路由跑一次「未关联到已有账号」的收口分支。
func callOidcRegisterOrReject(t *testing.T, provider *model.OidcProvider, subject string, claims map[string]interface{}) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	r.GET("/oauth/oidc", func(c *gin.Context) {
		oidcRegisterOrReject(c, provider, "raw-id-token", subject, claims, oidcTrustedEmail(claims))
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/oauth/oidc", nil))
	resp := map[string]any{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v (body=%s)", err, w.Body.String())
	}
	return resp
}

// disable_auto_register=true 时未关联账号的首登被拒，且不建用户、不写身份行。
func TestOidcRegisterRejectedWhenAutoRegisterDisabled(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	provider.DisplayName = "Authgear"
	provider.DisableAutoRegister = true
	if err := provider.Update(); err != nil {
		t.Fatalf("更新提供方失败: %v", err)
	}

	claims := map[string]interface{}{"preferred_username": "newbie", "email": "newbie@example.com", "email_verified": true}
	resp := callOidcRegisterOrReject(t, provider, "sub-blocked", claims)
	if resp["success"] != false {
		t.Fatalf("禁止建号时应返回失败，实际 %v", resp)
	}
	message, _ := resp["message"].(string)
	if !strings.HasPrefix(message, "OIDC_REGISTER_DISABLED:") {
		t.Fatalf("消息应以 OIDC_REGISTER_DISABLED: 开头，实际 %q", message)
	}
	if !strings.Contains(message, "Authgear") {
		t.Fatalf("消息应包含提供方展示名，实际 %q", message)
	}

	var userCount, identityCount int64
	model.DB.Model(&model.User{}).Count(&userCount)
	model.DB.Model(&model.UserOidcIdentity{}).Count(&identityCount)
	if userCount != 0 || identityCount != 0 {
		t.Fatalf("被拒的首登不应建号，实际 user=%d identity=%d", userCount, identityCount)
	}
}

// disable_auto_register=false 时首登照常建号并写身份行。
func TestOidcRegisterAllowedWhenAutoRegisterEnabled(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)

	oldRegister := config.RegisterEnabled
	oldQuota := config.QuotaForNewUser
	config.RegisterEnabled = true
	config.QuotaForNewUser = 0
	t.Cleanup(func() {
		config.RegisterEnabled = oldRegister
		config.QuotaForNewUser = oldQuota
	})

	claims := map[string]interface{}{"preferred_username": "newbie", "email": "newbie@example.com", "email_verified": true}
	resp := callOidcRegisterOrReject(t, provider, "sub-allowed", claims)
	if resp["success"] != true {
		t.Fatalf("未开启禁止建号时首登应成功，实际 %v", resp)
	}
	identity, err := model.FindUserOidcIdentity(provider.Id, "sub-allowed")
	if err != nil {
		t.Fatalf("身份行应已落库: %v", err)
	}
	if identity.UserId == 0 {
		t.Fatal("身份行应归属新建用户")
	}
}

// 提供方未配 display_name 时，拒绝消息退回 slug 文案。
func TestOidcRegisterRejectedMessageFallsBackToSlug(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	provider.DisplayName = ""
	provider.DisableAutoRegister = true

	resp := callOidcRegisterOrReject(t, provider, "sub-noname", map[string]interface{}{})
	message, _ := resp["message"].(string)
	if !strings.Contains(message, "authgear") {
		t.Fatalf("消息应退回 slug，实际 %q", message)
	}
}

// assertNoProviderWording 断言错误文案里没有泄漏提供方名称与 OIDC 字样（错误码前缀除外）。
func assertNoProviderWording(t *testing.T, message string, prefix string, provider *model.OidcProvider) {
	t.Helper()
	if !strings.HasPrefix(message, prefix) {
		t.Fatalf("消息应以 %s 开头，实际 %q", prefix, message)
	}
	detail := strings.TrimPrefix(message, prefix)
	for _, word := range []string{provider.DisplayName, provider.Slug, "OIDC"} {
		if word != "" && strings.Contains(detail, word) {
			t.Fatalf("first_party 文案不应出现 %q，实际 %q", word, detail)
		}
	}
}

// first_party 提供方的邮箱冲突文案不得出现供应商名称 / slug / OIDC 字样。
func TestOidcLinkConflictMessageFirstParty(t *testing.T) {
	provider := &model.OidcProvider{Slug: "authgear", DisplayName: "Authgear", FirstParty: true}
	assertNoProviderWording(t, oidcLinkConflictMessage(provider), "OIDC_LINK_CONFLICT:", provider)
}

// 非 first_party 提供方的邮箱冲突文案保留展示名，便于用户分辨用了哪个第三方账号。
func TestOidcLinkConflictMessageThirdParty(t *testing.T) {
	provider := &model.OidcProvider{Slug: "authgear", DisplayName: "Authgear"}
	message := oidcLinkConflictMessage(provider)
	if !strings.HasPrefix(message, "OIDC_LINK_CONFLICT:") || !strings.Contains(message, "Authgear") {
		t.Fatalf("非 first_party 消息应带前缀并包含展示名，实际 %q", message)
	}
}

// first_party 提供方的禁止建号文案同样不得泄漏供应商名称。
func TestOidcRegisterDisabledMessageFirstParty(t *testing.T) {
	provider := &model.OidcProvider{Slug: "authgear", DisplayName: "Authgear", FirstParty: true}
	assertNoProviderWording(t, oidcRegisterDisabledMessage(provider), "OIDC_REGISTER_DISABLED:", provider)
}

// first_party 提供方的绑定占用文案不得出现供应商名称 / slug / OIDC 字样（该文案无错误码前缀）。
func TestOidcBindTakenMessageFirstParty(t *testing.T) {
	provider := &model.OidcProvider{Slug: "authgear", DisplayName: "Authgear", FirstParty: true}
	assertNoProviderWording(t, oidcBindTakenMessage(provider), "", provider)
}

// 非 first_party 提供方的绑定占用文案保留展示名，未配展示名时退回 slug。
func TestOidcBindTakenMessageThirdParty(t *testing.T) {
	provider := &model.OidcProvider{Slug: "authgear", DisplayName: "Authgear"}
	if message := oidcBindTakenMessage(provider); !strings.Contains(message, "Authgear") {
		t.Fatalf("非 first_party 消息应包含展示名，实际 %q", message)
	}
	provider.DisplayName = ""
	if message := oidcBindTakenMessage(provider); !strings.Contains(message, "authgear") {
		t.Fatalf("未配展示名时应退回 slug，实际 %q", message)
	}
}

// 密码登录关闭时不再出现「原方式登录」措辞，改为引导联系管理员。
func TestOidcFailureAdviceFollowsPasswordLogin(t *testing.T) {
	old := config.PasswordLoginEnabled
	t.Cleanup(func() { config.PasswordLoginEnabled = old })

	config.PasswordLoginEnabled = true
	if !strings.Contains(oidcFailureAdvice(), "original method") {
		t.Fatalf("密码登录开启时应引导原方式登录，实际 %q", oidcFailureAdvice())
	}
	config.PasswordLoginEnabled = false
	advice := oidcFailureAdvice()
	if strings.Contains(advice, "original method") || !strings.Contains(advice, "admin") {
		t.Fatalf("密码登录关闭时应引导联系管理员，实际 %q", advice)
	}

	// 外部账号体系下密码登录对普通用户恒关闭，哪怕开关还是开的
	oldSystem := config.AccountSystem
	t.Cleanup(func() { config.AccountSystem = oldSystem })
	config.AccountSystem = config.AccountSystemExternal
	config.PasswordLoginEnabled = true
	if advice := oidcFailureAdvice(); strings.Contains(advice, "original method") {
		t.Fatalf("外部账号体系下应引导联系管理员，实际 %q", advice)
	}
}

// 站点关闭注册时首登以 OIDC_REGISTER_CLOSED 前缀返回，文案不含供应商名称。
func TestOidcRegisterClosedWhenRegisterDisabled(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	provider.DisplayName = "Authgear"
	provider.FirstParty = true
	if err := provider.Update(); err != nil {
		t.Fatalf("更新提供方失败: %v", err)
	}

	oldRegister := config.RegisterEnabled
	config.RegisterEnabled = false
	t.Cleanup(func() { config.RegisterEnabled = oldRegister })

	resp := callOidcRegisterOrReject(t, provider, "sub-closed", map[string]interface{}{})
	if resp["success"] != false {
		t.Fatalf("关闭注册时应返回失败，实际 %v", resp)
	}
	message, _ := resp["message"].(string)
	assertNoProviderWording(t, message, "OIDC_REGISTER_CLOSED:", provider)

	var userCount int64
	model.DB.Model(&model.User{}).Count(&userCount)
	if userCount != 0 {
		t.Fatalf("关闭注册时不应建号，实际 %d", userCount)
	}
}

// username claim 常见形态就是邮箱：注册出的用户名必须只取 @ 之前的部分，且不超过 12 个 rune。
func TestSanitizeOAuthUsername(t *testing.T) {
	cases := map[string]string{
		"foo@x.com":   "foo",
		"  bar  ":     "bar",
		" baz@x.com ": "baz",
		"@x.com":      "",
		"@":           "",
		"":            "",
		"plain":       "plain",
		// 超长输入按 rune 截到 12
		"sc59116longusername":         "sc59116longu",
		"averyveryverylongname@x.com": "averyveryver",
		"一二三四五六七八九十甲乙丙":               "一二三四五六七八九十甲乙",
		// 截断点落在空格上时仍需去掉尾部空白
		"abcdefghijk lmnop": "abcdefghijk",
		// 恰好 12 个 rune 不受影响
		"abcdefghijkl": "abcdefghijkl",
	}
	for in, want := range cases {
		if got := sanitizeOAuthUsername(in); got != want {
			t.Fatalf("sanitizeOAuthUsername(%q) = %q，期望 %q", in, got, want)
		}
	}
}

// @ 前缀为空或已被占用时退回 "<prefix>_<下一个用户 id>"，任何情况下都不含 @。
func TestOAuthUsernameFallback(t *testing.T) {
	setupOidcTestDB(t)
	seedOidcUser(t, &model.User{
		Username: "taken", Email: "taken@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-a", AffCode: "affa",
	})

	if got := oauthUsername("foo@x.com", "oidc"); got != "foo" {
		t.Fatalf("未占用的 @ 前缀应直接使用，实际 %q", got)
	}
	for _, in := range []string{"taken@example.com", "taken", "@x.com", ""} {
		got := oauthUsername(in, "oidc")
		if got != "oidc_2" {
			t.Fatalf("输入 %q 应走兜底 oidc_2，实际 %q", in, got)
		}
	}
}

// 超长外部登录名截断后落库，撞上已有用户名时照旧走 "<prefix>_<下一个用户 id>" 兜底。
func TestOAuthUsernameTruncation(t *testing.T) {
	setupOidcTestDB(t)
	seedOidcUser(t, &model.User{
		Username: "abcdefghijkl", Email: "abcdefghijkl@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b", AffCode: "affb",
	})

	if got := oauthUsername("sc59116longusername", "oidc"); got != "sc59116longu" {
		t.Fatalf("超长登录名应截到 12 个 rune，实际 %q", got)
	}
	if got := oauthUsername("abcdefghijklmnop", "oidc"); got != "oidc_2" {
		t.Fatalf("截断后撞名应走兜底 oidc_2，实际 %q", got)
	}
}

// 授权地址只在 ui_locales 合法时携带该参数：缺省与非法值下与不带时一致。
func TestOIDCEndpointUILocales(t *testing.T) {
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"合法", "?ui_locales=zh-HK", "zh-HK"},
		{"缺省", "", ""},
		{"非法", "?ui_locales=fr", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupOidcTestDB(t)
			idp := newFakeIdPForLogout(t, false)

			provider := seedOidcProvider(t, "keycloak", false)
			provider.Issuer = idp.URL
			if err := provider.Update(); err != nil {
				t.Fatalf("更新提供方失败: %v", err)
			}

			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
			r.GET("/api/oauth/endpoint/:slug", OIDCEndpoint)

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/oauth/endpoint/keycloak"+tc.query, nil))

			var resp struct {
				Success bool   `json:"success"`
				Data    string `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("解析响应失败: %v (body=%s)", err, w.Body.String())
			}
			if !resp.Success {
				t.Fatalf("取授权地址应成功，实际 %s", w.Body.String())
			}
			parsed, err := url.Parse(resp.Data)
			if err != nil {
				t.Fatalf("授权地址应可解析: %v (%s)", err, resp.Data)
			}
			if got := parsed.Query().Get("ui_locales"); got != tc.want {
				t.Fatalf("ui_locales 期望 %q 实际 %q", tc.want, got)
			}
		})
	}
}

func getUserEmail(t *testing.T, userId int) model.NullableEmail {
	t.Helper()
	user, err := model.GetUserById(userId, false)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	return user.Email
}

// syncOidcEmail 第三方提供方的三条分支：本地为空则回填；本地已有邮箱不覆盖；
// 邮箱已被他人占用则跳过。
func TestSyncOidcEmail(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", true)

	blank := seedOidcUser(t, &model.User{
		Username: "blankmail", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-e1", AffCode: "affe1",
	})
	syncOidcEmail(blank, "idp@example.com", provider)
	if blank.Email != "idp@example.com" {
		t.Fatalf("内存中的邮箱应同步更新，实际 %q", blank.Email)
	}
	if got := getUserEmail(t, blank.Id); got != "idp@example.com" {
		t.Fatalf("本地邮箱为空时应回填，实际 %q", got)
	}

	existing := seedOidcUser(t, &model.User{
		Username: "hasmail", Email: "local@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-e2", AffCode: "affe2",
	})
	syncOidcEmail(existing, "other@example.com", provider)
	if got := getUserEmail(t, existing.Id); got != "local@example.com" {
		t.Fatalf("本地已有邮箱不应被覆盖，实际 %q", got)
	}

	taken := seedOidcUser(t, &model.User{
		Username: "takenmail", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-e3", AffCode: "affe3",
	})
	syncOidcEmail(taken, "local@example.com", provider)
	if got := getUserEmail(t, taken.Id); got != "" {
		t.Fatalf("邮箱已被占用时不应写入，实际 %q", got)
	}
}

// first_party 提供方是身份权威：本地已有邮箱与 IdP 不同时跟随覆盖；目标邮箱已被其它账号
// （含影子账户）占用则跳过；大小写 / 空白归一化后相同视作无变化，不产生多余写入。
func TestSyncOidcEmailFirstParty(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", true)
	provider.FirstParty = true

	changed := seedOidcUser(t, &model.User{
		Username: "fpchanged", Email: "old@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-fp1", AffCode: "afffp1",
	})
	syncOidcEmail(changed, "new@example.com", provider)
	if changed.Email != "new@example.com" {
		t.Fatalf("内存中的邮箱应跟随 IdP 更新，实际 %q", changed.Email)
	}
	if got := getUserEmail(t, changed.Id); got != "new@example.com" {
		t.Fatalf("first_party 提供方应覆盖本地已有邮箱，实际 %q", got)
	}

	shadow := seedOidcUser(t, &model.User{
		Username: "fpshadow", Email: "occupied@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeOrgShadow, AccessToken: "tok-fp2", AffCode: "afffp2",
	})
	if shadow.Id == 0 {
		t.Fatal("影子账户应已落库")
	}
	blocked := seedOidcUser(t, &model.User{
		Username: "fpblocked", Email: "mine@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-fp3", AffCode: "afffp3",
	})
	syncOidcEmail(blocked, "occupied@example.com", provider)
	if got := getUserEmail(t, blocked.Id); got != "mine@example.com" {
		t.Fatalf("邮箱已被其它账号占用时不应覆盖，实际 %q", got)
	}

	same := seedOidcUser(t, &model.User{
		Username: "fpsame", Email: "same@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-fp4", AffCode: "afffp4",
	})
	syncOidcEmail(same, " Same@Example.com ", provider)
	if got := getUserEmail(t, same.Id); got != "same@example.com" {
		t.Fatalf("归一化后相同的邮箱不应改写，实际 %q", got)
	}
}

// getOidcIdentitySnapshot 读回身份行上的展示快照。
func getOidcIdentitySnapshot(t *testing.T, providerId int, subject string) (string, string) {
	t.Helper()
	identity, err := model.FindUserOidcIdentity(providerId, subject)
	if err != nil {
		t.Fatalf("读取身份行失败: %v", err)
	}
	return identity.IdpUsername, identity.IdpEmail
}

// 展示快照的取值规则：用户名优先 preferred_username、缺失退回 name；邮箱取 email claim
// 原样（未验证也展示，因为它不参与任何匹配）；超过列宽的值一律不展示。
func TestOidcIdentitySnapshot(t *testing.T) {
	cases := []struct {
		name         string
		claims       map[string]interface{}
		wantUsername string
		wantEmail    string
	}{
		{"preferred_username 优先", map[string]interface{}{"preferred_username": "alice", "name": "Alice Doe"}, "alice", ""},
		{"缺 preferred_username 退回 name", map[string]interface{}{"name": "Alice Doe"}, "Alice Doe", ""},
		{"未验证邮箱同样展示", map[string]interface{}{"email": "alice@idp.com", "email_verified": false}, "", "alice@idp.com"},
		{"非字符串 claim 忽略", map[string]interface{}{"preferred_username": 42, "email": nil}, "", ""},
		{"超长值不展示", map[string]interface{}{"preferred_username": strings.Repeat("a", 256), "email": strings.Repeat("b", 256)}, "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			username, email := oidcIdentitySnapshot(&model.OidcProvider{}, tc.claims)
			if username != tc.wantUsername || email != tc.wantEmail {
				t.Fatalf("oidcIdentitySnapshot = (%q, %q)，期望 (%q, %q)", username, email, tc.wantUsername, tc.wantEmail)
			}
		})
	}
}

// 登录态绑定时写入快照，此后每次经该提供方登录都刷新为最新 claim。
func TestOidcBindWritesAndLoginRefreshesSnapshot(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	user := seedOidcUser(t, &model.User{
		Username: "snapuser", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-s1", AffCode: "affs1",
	})

	r := newOidcPhoneRouter(t)
	r.GET("/bind", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", user.Id)
		_ = session.Save()
		oidcBind(c, provider, "sub-snap", map[string]interface{}{
			"preferred_username": "alice",
			"email":              "alice@idp.com",
		})
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/bind", nil))

	if username, email := getOidcIdentitySnapshot(t, provider.Id, "sub-snap"); username != "alice" || email != "alice@idp.com" {
		t.Fatalf("绑定应写入快照，实际 (%q, %q)", username, email)
	}

	// 再登录：IdP 侧改名换邮箱后，身份行刷新为最新值。
	refreshOidcIdentitySnapshot(provider, "sub-snap", map[string]interface{}{
		"preferred_username": "alice2",
		"email":              "alice2@idp.com",
	})
	if username, email := getOidcIdentitySnapshot(t, provider.Id, "sub-snap"); username != "alice2" || email != "alice2@idp.com" {
		t.Fatalf("再登录应刷新快照，实际 (%q, %q)", username, email)
	}

	// claim 缺失时清空，避免继续展示早已不存在的账户信息。
	refreshOidcIdentitySnapshot(provider, "sub-snap", map[string]interface{}{})
	if username, email := getOidcIdentitySnapshot(t, provider.Id, "sub-snap"); username != "" || email != "" {
		t.Fatalf("claim 缺失时快照应清空，实际 (%q, %q)", username, email)
	}
}

// 首登注册：快照随身份行一起落库。
func TestOidcRegisterWritesIdentitySnapshot(t *testing.T) {
	setupOidcTestDB(t)
	oldRegister := config.RegisterEnabled
	config.RegisterEnabled = true
	t.Cleanup(func() { config.RegisterEnabled = oldRegister })

	provider := seedOidcProvider(t, "authgear", false)
	r := newOidcPhoneRouter(t)
	r.GET("/register", func(c *gin.Context) {
		oidcRegister(c, provider, "", "sub-reg", map[string]interface{}{
			"preferred_username": "bob",
			"email":              "bob@idp.com",
		}, "")
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/register", nil))

	if username, email := getOidcIdentitySnapshot(t, provider.Id, "sub-reg"); username != "bob" || email != "bob@idp.com" {
		t.Fatalf("注册应写入快照，实际 (%q, %q)", username, email)
	}
}

// /api/user/self 下发快照与绑定时间，subject 始终不下发。
func TestSelfOidcIdentitiesExposesSnapshot(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	user := seedOidcUser(t, &model.User{
		Username: "selfsnap", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-s2", AffCode: "affs2",
	})
	identity := &model.UserOidcIdentity{
		UserId: user.Id, ProviderId: provider.Id, Subject: "sub-self",
		IdpUsername: "carol", IdpEmail: "carol@idp.com",
	}
	if err := identity.Insert(); err != nil {
		t.Fatalf("创建身份失败: %v", err)
	}

	identities := selfOidcIdentities(user.Id)
	if len(identities) != 1 {
		t.Fatalf("应下发一条身份，实际 %+v", identities)
	}
	got := identities[0]
	if got.IdpUsername != "carol" || got.IdpEmail != "carol@idp.com" {
		t.Fatalf("应下发 IdP 账户快照，实际 %+v", got)
	}
	if got.CreatedTime == 0 {
		t.Fatalf("应下发绑定时间，实际 %+v", got)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if strings.Contains(string(body), "sub-self") || strings.Contains(string(body), "subject") {
		t.Fatalf("响应体不应包含 subject，实际 %s", body)
	}
}

// callOidcBind 以 userId 作为会话用户驱动登录态绑定分支。
func callOidcBind(t *testing.T, provider *model.OidcProvider, userId int, subject string, claims map[string]interface{}) map[string]interface{} {
	t.Helper()
	r := newOidcPhoneRouter(t)
	r.GET("/bind", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Set("id", userId)
		_ = session.Save()
		oidcBind(c, provider, subject, claims)
	})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/bind", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("oidcBind 应返回 200，实际 %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v (body=%q)", err, w.Body.String())
	}
	return resp
}

// 该 subject 已归属其它账号：绑定被拒，身份行仍归原主，当前账号不落新行。
func TestOidcBindRejectsSubjectOwnedByOthers(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	owner := seedOidcUser(t, &model.User{
		Username: "bindowner", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b1", AffCode: "affb1",
	})
	seedOidcIdentity(t, owner.Id, provider.Id, "sub-taken")
	taker := seedOidcUser(t, &model.User{
		Username: "bindtaker", Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b2", AffCode: "affb2",
	})

	resp := callOidcBind(t, provider, taker.Id, "sub-taken", map[string]interface{}{"preferred_username": "taker"})
	if resp["success"] != false {
		t.Fatalf("身份已归属他人时绑定应被拒，实际: %v", resp)
	}
	if resp["message"] != oidcBindTakenMessage(provider) {
		t.Fatalf("拒绝文案应为 %q，实际: %v", oidcBindTakenMessage(provider), resp["message"])
	}
	identity, err := model.FindUserOidcIdentity(provider.Id, "sub-taken")
	if err != nil || identity.UserId != owner.Id {
		t.Fatalf("身份行应仍归属 %d，实际 %v err=%v", owner.Id, identity, err)
	}
	if got := countOidcIdentities(t, taker.Id); got != 0 {
		t.Fatalf("被拒账号不应有身份行，实际 %d", got)
	}
}

// 会话用户已被封禁：绑定被拒且不写身份行（会话可能签发于封禁之前）。
func TestOidcBindRejectsBannedUser(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	banned := seedOidcUser(t, &model.User{
		Username: "bindbanned", Role: config.RoleCommonUser, Status: config.UserStatusDisabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b3", AffCode: "affb3",
	})

	resp := callOidcBind(t, provider, banned.Id, "sub-banned", map[string]interface{}{"preferred_username": "banned"})
	if resp["success"] != false {
		t.Fatalf("封禁用户绑定应被拒，实际: %v", resp)
	}
	if resp["message"] != "User is banned or does not exist" {
		t.Fatalf("拒绝文案应为「用户已被封禁或不存在」，实际: %v", resp["message"])
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-banned"); err == nil {
		t.Fatal("被拒时不应写入身份行")
	}
	if got := countOidcIdentities(t, banned.Id); got != 0 {
		t.Fatalf("被拒账号不应有身份行，实际 %d", got)
	}
}

// 登录态绑定成功：写身份行 + 快照，并按 first_party 语义把 IdP 已验证邮箱同步到 users.email。
func TestOidcBindFirstPartySyncsEmail(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	provider.FirstParty = true
	user := seedOidcUser(t, &model.User{
		Username: "bindmail", Email: "old@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b4", AffCode: "affb4",
	})

	resp := callOidcBind(t, provider, user.Id, "sub-bindmail", map[string]interface{}{
		"preferred_username": "bindmail",
		"email":              "new@example.com",
		"email_verified":     true,
	})
	if resp["success"] != true {
		t.Fatalf("绑定应成功，实际: %v", resp)
	}
	identity, err := model.FindUserOidcIdentity(provider.Id, "sub-bindmail")
	if err != nil || identity.UserId != user.Id {
		t.Fatalf("身份行应落库并归属 %d，实际 %v err=%v", user.Id, identity, err)
	}
	if identity.IdpUsername != "bindmail" || identity.IdpEmail != "new@example.com" {
		t.Fatalf("绑定应写入快照，实际 (%q, %q)", identity.IdpUsername, identity.IdpEmail)
	}
	if got := getUserEmail(t, user.Id); got != "new@example.com" {
		t.Fatalf("first_party 绑定应同步 users.email，实际 %q", got)
	}
}

// 登录路径上的 first_party 邮箱刷新（对齐 OIDCAuth 的 oidcLinkLogin 分支）：
// 身份行命中登录后，快照与 users.email 都跟随 IdP 的已验证邮箱更新。
func TestOidcLoginPathFirstPartyRefreshesEmail(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", false)
	provider.FirstParty = true
	user := seedOidcUser(t, &model.User{
		Username: "loginmail", Email: "stale@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-b5", AffCode: "affb5",
	})
	seedOidcIdentity(t, user.Id, provider.Id, "sub-loginmail")

	claims := map[string]interface{}{
		"preferred_username": "loginmail",
		"email":              "fresh@example.com",
		"email_verified":     true,
	}
	existing, outcome, err := resolveOidcUser(provider, "sub-loginmail", oidcTrustedEmail(claims), "")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("期望 oidcLinkLogin，实际 outcome=%d err=%v", outcome, err)
	}
	if existing.Id != user.Id {
		t.Fatalf("应命中已有账号 %d，实际 %d", user.Id, existing.Id)
	}
	refreshOidcIdentitySnapshot(provider, "sub-loginmail", claims)
	syncOidcEmail(existing, oidcTrustedEmail(claims), provider)

	if got := getUserEmail(t, user.Id); got != "fresh@example.com" {
		t.Fatalf("登录应把 users.email 刷新为 IdP 邮箱，实际 %q", got)
	}
	if _, email := getOidcIdentitySnapshot(t, provider.Id, "sub-loginmail"); email != "fresh@example.com" {
		t.Fatalf("登录应刷新身份行快照邮箱，实际 %q", email)
	}
}
