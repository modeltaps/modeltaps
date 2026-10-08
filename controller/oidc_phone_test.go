package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

// newOidcPhoneRouter 建一个带会话中间件的测试路由，供直接驱动 oidcRegister / oidcBind。
func newOidcPhoneRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	return r
}

func getUserPhone(t *testing.T, userId int) string {
	t.Helper()
	user, err := model.GetUserById(userId, false)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	return string(user.PhoneNumber)
}

// phone_number_verified 需同时接受布尔 true 与字符串 "true"；缺失 / false / 非字符串号码 /
// 超过列宽的号码一律不采信。
func TestOidcVerifiedPhoneNumber(t *testing.T) {
	cases := []struct {
		name   string
		claims map[string]interface{}
		want   string
	}{
		{"布尔已验证", map[string]interface{}{"phone_number": "+85251234567", "phone_number_verified": true}, "+85251234567"},
		{"字符串已验证", map[string]interface{}{"phone_number": " +85251234568 ", "phone_number_verified": "true"}, "+85251234568"},
		{"未验证", map[string]interface{}{"phone_number": "+85251234569", "phone_number_verified": false}, ""},
		{"缺 phone_number_verified", map[string]interface{}{"phone_number": "+85251234570"}, ""},
		{"缺 phone_number", map[string]interface{}{"phone_number_verified": true}, ""},
		{"phone_number 非字符串", map[string]interface{}{"phone_number": 8525123, "phone_number_verified": true}, ""},
		{"超长号码", map[string]interface{}{"phone_number": strings.Repeat("9", 33), "phone_number_verified": true}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := oidcVerifiedPhoneNumber(tc.claims); got != tc.want {
				t.Fatalf("oidcVerifiedPhoneNumber = %q，期望 %q", got, tc.want)
			}
		})
	}
}

// 首登注册：已验证手机号随新账号落库；未验证时新账号不带手机号。
func TestOidcRegisterWritesVerifiedPhone(t *testing.T) {
	setupOidcTestDB(t)
	oldRegister := config.RegisterEnabled
	config.RegisterEnabled = true
	t.Cleanup(func() { config.RegisterEnabled = oldRegister })

	provider := seedOidcProvider(t, "authgear", false)
	r := newOidcPhoneRouter(t)
	r.GET("/register", func(c *gin.Context) {
		claims := map[string]interface{}{
			"preferred_username":    c.Query("name"),
			"phone_number":          c.Query("phone"),
			"phone_number_verified": c.Query("verified") == "1",
		}
		oidcRegister(c, provider, "", c.Query("sub"), claims, "")
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/register?name=pu1&sub=sub-p1&phone=%2B85251234567&verified=1", nil))
	var verified model.User
	if err := model.DB.First(&verified, "username = ?", "pu1").Error; err != nil {
		t.Fatalf("注册用户应已落库: %v", err)
	}
	if verified.PhoneNumber != "+85251234567" {
		t.Fatalf("已验证手机号应写入，实际 %q", verified.PhoneNumber)
	}

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/register?name=pu2&sub=sub-p2&phone=%2B85251234568&verified=0", nil))
	var unverified model.User
	if err := model.DB.First(&unverified, "username = ?", "pu2").Error; err != nil {
		t.Fatalf("注册用户应已落库: %v", err)
	}
	if unverified.PhoneNumber != "" {
		t.Fatalf("未验证手机号不应写入，实际 %q", unverified.PhoneNumber)
	}
}

// 第三方提供方的登录路径：按已验证邮箱关联登录时本地手机号为空则回填，
// 此后身份行命中的再登录一律不覆盖已有值（第三方不得静默改本站可关联标识），
// 未验证时同样不写。
func TestSyncOidcPhoneOnLoginPaths(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", true)
	seeded := seedOidcUser(t, &model.User{
		Username: "linker", Email: "linker@example.com", EmailVerified: true,
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-p1", AffCode: "affp1",
	})

	linked, outcome, err := resolveOidcUser(provider, "sub-link", "linker@example.com", "")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("期望 oidcLinkLogin，实际 outcome=%d err=%v", outcome, err)
	}
	syncOidcPhoneNumber(linked, oidcVerifiedPhoneNumber(map[string]interface{}{
		"phone_number": "+85251234567", "phone_number_verified": true,
	}), provider)
	if got := getUserPhone(t, seeded.Id); got != "+85251234567" {
		t.Fatalf("邮箱关联登录应写入手机号，实际 %q", got)
	}

	again, outcome, err := resolveOidcUser(provider, "sub-link", "", "")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("再登录期望 oidcLinkLogin，实际 outcome=%d err=%v", outcome, err)
	}
	syncOidcPhoneNumber(again, oidcVerifiedPhoneNumber(map[string]interface{}{
		"phone_number": "+85259999999", "phone_number_verified": "true",
	}), provider)
	if got := getUserPhone(t, seeded.Id); got != "+85251234567" {
		t.Fatalf("第三方提供方不应覆盖本地已有手机号，实际 %q", got)
	}

	syncOidcPhoneNumber(again, oidcVerifiedPhoneNumber(map[string]interface{}{
		"phone_number": "+85250000000", "phone_number_verified": false,
	}), provider)
	if got := getUserPhone(t, seeded.Id); got != "+85251234567" {
		t.Fatalf("未验证手机号不应写入，实际 %q", got)
	}
}

// first_party 提供方是身份权威：本地已有手机号与 IdP 不同时跟随覆盖；
// claim 缺失（空号码）时不清空本地值；两侧仅差首尾空白视作无变化。
func TestSyncOidcPhoneFirstParty(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", true)
	provider.FirstParty = true

	changed := seedOidcUser(t, &model.User{
		Username: "fpphone", Email: "fpphone@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-fpp1", AffCode: "afffpp1",
		PhoneNumber: model.NullablePhone("+85251234567"),
	})
	syncOidcPhoneNumber(changed, oidcVerifiedPhoneNumber(map[string]interface{}{
		"phone_number": "+85259999999", "phone_number_verified": true,
	}), provider)
	if changed.PhoneNumber != "+85259999999" {
		t.Fatalf("内存中的手机号应跟随 IdP 更新，实际 %q", changed.PhoneNumber)
	}
	if got := getUserPhone(t, changed.Id); got != "+85259999999" {
		t.Fatalf("first_party 提供方应覆盖本地已有手机号，实际 %q", got)
	}

	syncOidcPhoneNumber(changed, oidcVerifiedPhoneNumber(map[string]interface{}{
		"phone_number_verified": true,
	}), provider)
	if got := getUserPhone(t, changed.Id); got != "+85259999999" {
		t.Fatalf("IdP 未下发手机号时不应清空本地值，实际 %q", got)
	}

	syncOidcPhoneNumber(changed, " +85259999999 ", provider)
	if got := getUserPhone(t, changed.Id); got != "+85259999999" {
		t.Fatalf("归一化后相同的手机号不应改写，实际 %q", got)
	}
}

// 该手机号已归属其它账号时不得回填 / 覆盖：抢占既是账号接管路径，也会撞唯一索引。
// 两档提供方同一口径。
func TestSyncOidcPhoneSkipsTakenNumber(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", true)
	owner := seedOidcUser(t, &model.User{
		Username: "owner", Email: "owner@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-p3", AffCode: "affp3",
		PhoneNumber: model.NullablePhone("+85251234567"),
	})
	other := seedOidcUser(t, &model.User{
		Username: "other", Email: "other@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-p4", AffCode: "affp4",
	})

	syncOidcPhoneNumber(other, "+85251234567", provider)
	if got := getUserPhone(t, other.Id); got != "" {
		t.Fatalf("已被占用的手机号不应回填，实际 %q", got)
	}
	if got := getUserPhone(t, owner.Id); got != "+85251234567" {
		t.Fatalf("原持有者的手机号不应变化，实际 %q", got)
	}

	provider.FirstParty = true
	holder := seedOidcUser(t, &model.User{
		Username: "fpholder", Email: "fpholder@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-p8", AffCode: "affp8",
		PhoneNumber: model.NullablePhone("+85258888888"),
	})
	syncOidcPhoneNumber(holder, "+85251234567", provider)
	if got := getUserPhone(t, holder.Id); got != "+85258888888" {
		t.Fatalf("first_party 也不得抢已被占用的手机号，实际 %q", got)
	}
}

// seedOidcPhoneLinkProvider 建一个只开启手机号关联的提供方。
func seedOidcPhoneLinkProvider(t *testing.T, slug string) *model.OidcProvider {
	t.Helper()
	provider := &model.OidcProvider{
		Slug:                slug,
		DisplayName:         slug,
		Issuer:              "https://idp.example.com",
		ClientId:            "client-" + slug,
		Scopes:              "openid,profile,phone",
		UsernameClaim:       "preferred_username",
		LinkByVerifiedPhone: true,
		Enabled:             true,
	}
	if err := provider.Insert(); err != nil {
		t.Fatalf("创建提供方失败: %v", err)
	}
	return provider
}

// 开启 link_by_verified_phone：已验证手机号命中唯一账号即登录并补写身份行。
func TestResolveOidcUserLinksByVerifiedPhone(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcPhoneLinkProvider(t, "authgear")
	seeded := seedOidcUser(t, &model.User{
		Username: "phonelink", Email: "phonelink@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-p5", AffCode: "affp5",
		PhoneNumber: model.NullablePhone("+85251234567"),
	})

	user, outcome, err := resolveOidcUser(provider, "sub-phone", "", "+85251234567")
	if err != nil || outcome != oidcLinkLogin {
		t.Fatalf("期望 oidcLinkLogin，实际 outcome=%d err=%v", outcome, err)
	}
	if user.Id != seeded.Id {
		t.Fatalf("应关联到已有用户 %d，实际 %d", seeded.Id, user.Id)
	}
	identity, err := model.FindUserOidcIdentity(provider.Id, "sub-phone")
	if err != nil || identity.UserId != seeded.Id {
		t.Fatalf("身份行应已落库并归属 %d，实际 %v err=%v", seeded.Id, identity, err)
	}
}

// 手机号命中的账号在同一提供方下已绑定别的 subject：拒绝自动关联，不写新身份行。
func TestResolveOidcUserPhoneLinkConflict(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcPhoneLinkProvider(t, "authgear")
	seeded := seedOidcUser(t, &model.User{
		Username: "phoneconflict", Email: "phoneconflict@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-p6", AffCode: "affp6",
		PhoneNumber: model.NullablePhone("+85251234568"),
	})
	seedOidcIdentity(t, seeded.Id, provider.Id, "sub-old")

	user, outcome, err := resolveOidcUser(provider, "sub-new", "", "+85251234568")
	if err != nil || outcome != oidcLinkConflict {
		t.Fatalf("期望 oidcLinkConflict，实际 outcome=%d err=%v", outcome, err)
	}
	if user == nil || user.Id != seeded.Id {
		t.Fatalf("应返回冲突的目标账号 %d，实际 %v", seeded.Id, user)
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-new"); err == nil {
		t.Fatal("冲突时不应写入新身份行")
	}
}

// link_by_verified_phone=false 时即使手机号已验证也不得关联已有账号。
func TestResolveOidcUserPhoneLinkDisabled(t *testing.T) {
	setupOidcTestDB(t)
	provider := seedOidcProvider(t, "authgear", true)
	seedOidcUser(t, &model.User{
		Username: "phoneoff", Email: "phoneoff@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-p7", AffCode: "affp7",
		PhoneNumber: model.NullablePhone("+85251234569"),
	})

	if _, outcome, err := resolveOidcUser(provider, "sub-off", "", "+85251234569"); err != nil || outcome != oidcLinkNone {
		t.Fatalf("关闭手机号关联应返回 oidcLinkNone，实际 outcome=%d err=%v", outcome, err)
	}
	if _, err := model.FindUserOidcIdentity(provider.Id, "sub-off"); err == nil {
		t.Fatal("关闭手机号关联时不应写入身份行")
	}
}

// 用户自助更新资料只走白名单字段：请求体里带 phone_number 也不得改写库里的展示手机号。
func TestUpdateSelfIgnoresPhoneNumber(t *testing.T) {
	setupOidcTestDB(t)
	user := seedOidcUser(t, &model.User{
		Username: "selfpu", DisplayName: "old", Email: "selfpu@example.com",
		Role: config.RoleCommonUser, Status: config.UserStatusEnabled,
		Type: config.UserTypeNormal, AccessToken: "tok-p2", AffCode: "affp2",
		PhoneNumber: model.NullablePhone("+85251234567"),
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.PUT("/api/user/self", func(c *gin.Context) {
		c.Set("id", user.Id)
		UpdateSelf(c)
	})

	body := strings.NewReader(`{"username":"selfpu","display_name":"new","phone_number":"+85259999999"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/user/self", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v", err)
	}
	if resp["success"] != true {
		t.Fatalf("自助更新应成功，实际 %v", resp)
	}
	updated, err := model.GetUserById(user.Id, false)
	if err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if updated.PhoneNumber != "+85251234567" {
		t.Fatalf("手机号不应被自助更新改写，实际 %q", updated.PhoneNumber)
	}
	if updated.DisplayName != "new" {
		t.Fatalf("显示名应被更新，实际 %q", updated.DisplayName)
	}
}
