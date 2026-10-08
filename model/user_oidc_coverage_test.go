package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// createCoverageUser 造一个可落库的用户：access_token / aff_code 都有唯一索引，必须各不相同。
func createCoverageUser(t *testing.T, username string, status int, userType int) *User {
	t.Helper()
	user := User{
		Username:    username,
		Password:    "test-password",
		Status:      status,
		Type:        userType,
		AccessToken: "token-" + username,
		AffCode:     "aff-" + username,
	}
	if err := DB.Create(&user).Error; err != nil {
		t.Fatalf("创建用户 %s 失败: %v", username, err)
	}
	return &user
}

// setupCoverageFixture 造一套覆盖五类样本的数据：
// 无身份 / 绑启用提供方 / 只绑已停用提供方 / 软删 / 封禁，另加一个影子账户。
func setupCoverageFixture(t *testing.T) (enabled *OidcProvider, disabled *OidcProvider) {
	t.Helper()
	setupOidcTestDB(t)

	enabled = &OidcProvider{Slug: "idp-on", DisplayName: "启用中", Enabled: true}
	disabled = &OidcProvider{Slug: "idp-off", DisplayName: "已停用", Enabled: false}
	for _, provider := range []*OidcProvider{enabled, disabled} {
		if err := provider.Insert(); err != nil {
			t.Fatalf("插入提供方 %s 失败: %v", provider.Slug, err)
		}
	}

	createCoverageUser(t, "none", config.UserStatusEnabled, config.UserTypeNormal)
	bound := createCoverageUser(t, "bound", config.UserStatusEnabled, config.UserTypeNormal)
	offOnly := createCoverageUser(t, "offonly", config.UserStatusEnabled, config.UserTypeNormal)
	deleted := createCoverageUser(t, "deleted", config.UserStatusEnabled, config.UserTypeNormal)
	createCoverageUser(t, "banned", config.UserStatusDisabled, config.UserTypeNormal)
	createCoverageUser(t, "shadow", config.UserStatusEnabled, config.UserTypeOrgShadow)

	identities := []UserOidcIdentity{
		{UserId: bound.Id, ProviderId: enabled.Id, Subject: "sub-bound"},
		{UserId: offOnly.Id, ProviderId: disabled.Id, Subject: "sub-offonly"},
		{UserId: deleted.Id, ProviderId: enabled.Id, Subject: "sub-deleted"},
	}
	for i := range identities {
		if err := identities[i].Insert(); err != nil {
			t.Fatalf("绑定身份失败: %v", err)
		}
	}

	if err := deleted.Delete(); err != nil {
		t.Fatalf("软删用户失败: %v", err)
	}
	return enabled, disabled
}

// TestGetOidcCoverage 只绑已停用提供方的算未绑；软删、封禁、影子账户都不算活跃用户。
func TestGetOidcCoverage(t *testing.T) {
	enabled, _ := setupCoverageFixture(t)

	coverage, err := GetOidcCoverage()
	if err != nil {
		t.Fatalf("统计覆盖情况失败: %v", err)
	}
	// 活跃：none、bound、offonly
	if coverage.TotalActive != 3 {
		t.Fatalf("活跃用户应为 3，实际 %d", coverage.TotalActive)
	}
	if coverage.WithOidc != 1 {
		t.Fatalf("已绑用户应为 1，实际 %d", coverage.WithOidc)
	}
	if coverage.WithoutOidc != 2 {
		t.Fatalf("未绑用户应为 2，实际 %d", coverage.WithoutOidc)
	}
	if len(coverage.ByProvider) != 1 {
		t.Fatalf("只应统计启用中的提供方，实际 %+v", coverage.ByProvider)
	}
	item := coverage.ByProvider[0]
	if item.Id != enabled.Id || item.Slug != "idp-on" || item.DisplayName != "启用中" {
		t.Fatalf("提供方信息不符: %+v", item)
	}
	if item.BoundUsers != 1 {
		t.Fatalf("启用提供方下应有 1 个活跃用户，实际 %d", item.BoundUsers)
	}

	// 页头「未绑 OIDC」chip 的计数必须等于点击后(已启用 + no_oidc)的列表总条数。
	chip, err := GetUsersList(&SearchUserParams{Status: config.UserStatusEnabled, LoginMethod: LoginMethodFilterNoOidc})
	if err != nil {
		t.Fatalf("未绑 chip 列表失败: %v", err)
	}
	if chip.TotalCount != coverage.WithoutOidc {
		t.Fatalf("未绑 chip 计数 %d 与筛选结果 %d 不一致", coverage.WithoutOidc, chip.TotalCount)
	}
}

// TestGetUsersListNoOidcFilter no_oidc 筛选只排掉绑了启用提供方的用户，
// 影子账户仍由既有逻辑排除，且可与关键字搜索叠加。
func TestGetUsersListNoOidcFilter(t *testing.T) {
	setupCoverageFixture(t)

	all, err := GetUsersList(&SearchUserParams{})
	if err != nil {
		t.Fatalf("查询用户列表失败: %v", err)
	}
	if len(*all.Data) != 4 {
		t.Fatalf("不带筛选应返回 4 个非影子用户，实际 %d", len(*all.Data))
	}

	filtered, err := GetUsersList(&SearchUserParams{LoginMethod: LoginMethodFilterNoOidc})
	if err != nil {
		t.Fatalf("按未绑筛选失败: %v", err)
	}
	got := map[string]bool{}
	for _, user := range *filtered.Data {
		got[user.Username] = true
	}
	want := []string{"none", "offonly", "banned"}
	if len(got) != len(want) {
		t.Fatalf("未绑用户应为 %v，实际 %v", want, got)
	}
	for _, username := range want {
		if !got[username] {
			t.Fatalf("未绑用户应包含 %s，实际 %v", username, got)
		}
	}
	if filtered.TotalCount != 3 {
		t.Fatalf("未绑总数应为 3，实际 %d", filtered.TotalCount)
	}

	keyword, err := GetUsersList(&SearchUserParams{
		GenericParams: GenericParams{Keyword: "offonly"},
		LoginMethod:   LoginMethodFilterNoOidc,
	})
	if err != nil {
		t.Fatalf("关键字叠加筛选失败: %v", err)
	}
	if len(*keyword.Data) != 1 || (*keyword.Data)[0].Username != "offonly" {
		t.Fatalf("关键字叠加筛选应只命中 offonly，实际 %+v", *keyword.Data)
	}
}
