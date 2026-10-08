package model

import (
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"

	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupOrgTestDB(t *testing.T) {
	t.Helper()
	// Token.AfterCreate 钩子依赖令牌编码器,测试环境需先初始化
	if viper.GetString("user_token_secret") == "" {
		viper.Set("user_token_secret", "org-test-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatalf("初始化用户令牌编码器失败: %v", err)
		}
	}
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	err = testDB.AutoMigrate(&User{}, &Token{}, &Log{}, &LogDetail{}, &Organization{}, &OrganizationMember{}, &OrganizationInvitation{}, &OrganizationAuditLog{})
	if err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

func TestGenerateOrgSlug(t *testing.T) {
	setupOrgTestDB(t)
	tests := []struct {
		name  string
		input string
	}{
		{"普通英文名", "My Team"},
		{"中文名回退到 org", "我的组织"},
		{"特殊字符清洗", "Foo!!@@Bar  Baz"},
		{"超长名称截断", strings.Repeat("a", 100)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slug := GenerateOrgSlug(tt.input)
			if slug == "" {
				t.Fatal("slug 不能为空")
			}
			if len(slug) > 64 {
				t.Fatalf("slug 超长: %d", len(slug))
			}
			for _, c := range slug {
				if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
					t.Fatalf("slug 含非法字符: %q in %q", c, slug)
				}
			}
		})
	}
}

func TestCreateShadowUserAndExclusion(t *testing.T) {
	setupOrgTestDB(t)

	shadow, err := CreateShadowUserForOrg(DB, "acme-x1y2z3")
	if err != nil {
		t.Fatalf("创建影子账户失败: %v", err)
	}
	if shadow.Id == 0 {
		t.Fatal("影子账户应有自增ID")
	}
	if !IsShadowUser(shadow) {
		t.Fatal("IsShadowUser 应为 true")
	}
	if !IsShadowUserId(shadow.Id) {
		t.Fatal("IsShadowUserId 应为 true")
	}

	// 普通用户对照
	hashed, _ := common.Password2Hash("test-password-123")
	normal := &User{Username: "alice", Password: hashed, Role: config.RoleCommonUser, Status: config.UserStatusEnabled}
	if err := DB.Create(normal).Error; err != nil {
		t.Fatalf("创建普通用户失败: %v", err)
	}
	if IsShadowUser(normal) || IsShadowUserId(normal.Id) {
		t.Fatal("普通用户不应被识别为影子账户")
	}

	// 影子账户禁止登录(即使密码碰巧正确也查不到)
	login := &User{Username: shadow.Username, Password: "anything"}
	if err := login.ValidateAndFill(); err == nil {
		t.Fatal("影子账户不应允许登录")
	}

	// FillUserByUsername(OIDC/WebAuthn 按用户名绑定路径)不应命中影子账户
	byUsername := &User{Username: shadow.Username}
	if err := byUsername.FillUserByUsername(); err == nil {
		t.Fatal("FillUserByUsername 不应查到影子账户")
	}
	byUsernameNormal := &User{Username: normal.Username}
	if err := byUsernameNormal.FillUserByUsername(); err != nil {
		t.Fatalf("FillUserByUsername 应能查到普通用户: %v", err)
	}

	// 用户列表排除影子账户
	result, err := GetUsersList(&SearchUserParams{})
	if err != nil {
		t.Fatalf("查询用户列表失败: %v", err)
	}
	for _, u := range *result.Data {
		if u.Type == config.UserTypeOrgShadow {
			t.Fatalf("用户列表不应包含影子账户: %s", u.Username)
		}
	}
}

func TestOrganizationUniqueConstraints(t *testing.T) {
	setupOrgTestDB(t)

	org := &Organization{Name: "Acme", Slug: "acme-test", ShadowUserId: 101, CreatedBy: 1}
	if err := org.Insert(); err != nil {
		t.Fatalf("插入组织失败: %v", err)
	}
	dup := &Organization{Name: "Acme2", Slug: "acme-test", ShadowUserId: 102, CreatedBy: 1}
	if err := dup.Insert(); err == nil {
		t.Fatal("slug 重复应失败")
	}

	member := &OrganizationMember{OrganizationId: org.Id, UserId: 1, Role: OrgRoleOwner}
	if err := member.Insert(); err != nil {
		t.Fatalf("插入成员失败: %v", err)
	}
	dupMember := &OrganizationMember{OrganizationId: org.Id, UserId: 1, Role: OrgRoleMember}
	if err := dupMember.Insert(); err == nil {
		t.Fatal("同组织同用户重复加入应失败")
	}

	inv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: 1, Role: OrgRoleMember, Token: "tok-abc"}
	if err := inv.Insert(); err != nil {
		t.Fatalf("插入邀请失败: %v", err)
	}
	dupInv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: 1, Role: OrgRoleMember, Token: "tok-abc"}
	if err := dupInv.Insert(); err == nil {
		t.Fatal("邀请 token 重复应失败")
	}
}

func createOrgTestUser(t *testing.T, username string) *User {
	t.Helper()
	hashed, _ := common.Password2Hash("test-password-123")
	user := &User{Username: username, Password: hashed, Email: NullableEmail(strings.ToLower(username) + "@test.local"), Role: config.RoleCommonUser, Status: config.UserStatusEnabled, AccessToken: utils.GetUUID(), AffCode: strings.ToLower(utils.GetRandomString(8))}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建测试用户失败: %v", err)
	}
	return user
}

func TestCreateOrganizationWithOwner(t *testing.T) {
	setupOrgTestDB(t)
	creator := createOrgTestUser(t, "creator")

	org, err := CreateOrganizationWithOwner("Acme Team", "", creator.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	if org.Id == 0 || org.ShadowUserId == 0 {
		t.Fatal("组织与影子账户应有自增ID")
	}
	if !IsShadowUserId(org.ShadowUserId) {
		t.Fatal("影子账户类型应为 UserTypeOrgShadow")
	}
	var shadow User
	if err := DB.First(&shadow, "id = ?", org.ShadowUserId).Error; err != nil {
		t.Fatalf("查询影子账户失败: %v", err)
	}
	if shadow.Quota != 0 {
		t.Fatalf("影子账户初始积分应为 0,实际 %d", shadow.Quota)
	}
	member, err := GetOrganizationMember(org.Id, creator.Id)
	if err != nil || member.Role != OrgRoleOwner {
		t.Fatalf("创建者应为 Owner,err=%v role=%v", err, member)
	}
	orgs, err := GetUserOrganizationsWithRole(creator.Id)
	if err != nil || len(orgs) != 1 || orgs[0].Role != OrgRoleOwner || orgs[0].Id != org.Id {
		t.Fatalf("GetUserOrganizationsWithRole 结果异常: err=%v orgs=%v", err, orgs)
	}

	// 创建者无效时应失败且不留脏数据
	if _, err := CreateOrganizationWithOwner("", "", creator.Id); err == nil {
		t.Fatal("空组织名应失败")
	}
	if _, err := CreateOrganizationWithOwner("X", "", 0); err == nil {
		t.Fatal("创建者ID为空应失败")
	}

	// 联系方式门槛:普通用户邮箱与手机号皆空则拒绝,任一非空放行,root 免检
	hashed, _ := common.Password2Hash("test-password-123")
	newUser := func(username string, role int, email NullableEmail, phone NullablePhone) *User {
		t.Helper()
		u := &User{Username: username, Password: hashed, Email: email, PhoneNumber: phone, Role: role, Status: config.UserStatusEnabled, AccessToken: utils.GetUUID(), AffCode: strings.ToLower(utils.GetRandomString(8))}
		if err := DB.Create(u).Error; err != nil {
			t.Fatalf("创建用户 %s 失败: %v", username, err)
		}
		return u
	}

	noContact := newUser("no-contact-u", config.RoleCommonUser, "", "")
	_, err = CreateOrganizationWithOwner("No Contact Org", "", noContact.Id)
	if err == nil {
		t.Fatal("邮箱与手机号皆空的普通用户创建组织应失败")
	}
	if strings.Contains(err.Error(), "link") || strings.Contains(err.Error(), "external") {
		t.Fatalf("错误文案不应出现「绑定」/「外部」: %v", err)
	}

	phoneOnly := newUser("phone-only-u", config.RoleCommonUser, "", "+8613800000000")
	if _, err := CreateOrganizationWithOwner("Phone Only Org", "", phoneOnly.Id); err != nil {
		t.Fatalf("仅有手机号的用户创建组织应成功: %v", err)
	}

	rootNoContact := newUser("root-no-contact-u", config.RoleRootUser, "", "")
	if _, err := CreateOrganizationWithOwner("Root Org", "", rootNoContact.Id); err != nil {
		t.Fatalf("root 无联系方式创建组织应成功: %v", err)
	}
}

func TestTransferOrganizationOwnership(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "owner-u")
	admin := createOrgTestUser(t, "admin-u")
	member := createOrgTestUser(t, "member-u")

	org, err := CreateOrganizationWithOwner("Transfer Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	for _, m := range []*OrganizationMember{
		{OrganizationId: org.Id, UserId: admin.Id, Role: OrgRoleAdmin},
		{OrganizationId: org.Id, UserId: member.Id, Role: OrgRoleMember},
	} {
		if err := m.Insert(); err != nil {
			t.Fatalf("插入成员失败: %v", err)
		}
	}

	if err := TransferOrganizationOwnership(org, owner.Id, owner.Id); err == nil {
		t.Fatal("转移给自己应失败")
	}
	if err := TransferOrganizationOwnership(org, owner.Id, member.Id); err == nil {
		t.Fatal("转移给 Member 应失败")
	}
	if err := TransferOrganizationOwnership(org, admin.Id, member.Id); err == nil {
		t.Fatal("非 Owner 发起转移应失败")
	}
	if err := TransferOrganizationOwnership(org, owner.Id, admin.Id); err != nil {
		t.Fatalf("Owner 转移给 Admin 应成功: %v", err)
	}
	newOwner, _ := GetOrganizationMember(org.Id, admin.Id)
	oldOwner, _ := GetOrganizationMember(org.Id, owner.Id)
	if newOwner.Role != OrgRoleOwner || oldOwner.Role != OrgRoleAdmin {
		t.Fatalf("转移后角色异常: new=%s old=%s", newOwner.Role, oldOwner.Role)
	}
}

func TestDissolveOrganization(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "dis-owner")

	org, err := CreateOrganizationWithOwner("Dissolve Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	// 影子账户充值 + 组织令牌 + 待处理邀请
	if err := DB.Model(&User{}).Where("id = ?", org.ShadowUserId).Update("quota", 500).Error; err != nil {
		t.Fatalf("充值失败: %v", err)
	}
	token := &Token{UserId: org.ShadowUserId, Key: "org-token-key-1", Name: "org-token", Status: config.TokenStatusEnabled}
	if err := DB.Create(token).Error; err != nil {
		t.Fatalf("创建组织令牌失败: %v", err)
	}
	inv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, Role: OrgRoleMember, Token: "tok-dissolve"}
	if err := inv.Insert(); err != nil {
		t.Fatalf("创建邀请失败: %v", err)
	}

	oldRefund := config.OrganizationDissolveQuotaRefund
	config.OrganizationDissolveQuotaRefund = "owner"
	t.Cleanup(func() { config.OrganizationDissolveQuotaRefund = oldRefund })

	if err := DissolveOrganization(org, owner.Id); err != nil {
		t.Fatalf("解散组织失败: %v", err)
	}

	// 令牌禁用
	var gotToken Token
	DB.First(&gotToken, "id = ?", token.Id)
	if gotToken.Status != config.TokenStatusDisabled {
		t.Fatalf("组织令牌应被禁用,实际 status=%d", gotToken.Status)
	}
	// 剩余积分退回 Owner
	ownerQuota, _ := GetUserQuota(owner.Id)
	if ownerQuota != owner.Quota+500 {
		t.Fatalf("Owner 应收到退回积分 500,实际增加 %d", ownerQuota-owner.Quota)
	}
	// 影子账户停用并清零
	var shadow User
	DB.First(&shadow, "id = ?", org.ShadowUserId)
	if shadow.Status != config.UserStatusDisabled || shadow.Quota != 0 {
		t.Fatalf("影子账户应停用且清零: status=%d quota=%d", shadow.Status, shadow.Quota)
	}
	// 成员清空
	count, _ := CountOrganizationMembers(org.Id)
	if count != 0 {
		t.Fatalf("成员应清空,实际 %d", count)
	}
	// 邀请撤销
	var gotInv OrganizationInvitation
	DB.First(&gotInv, "id = ?", inv.Id)
	if gotInv.Status != OrgInvitationStatusRevoked {
		t.Fatalf("待处理邀请应被撤销,实际 status=%d", gotInv.Status)
	}
	// 组织软删除
	if _, err := GetOrganizationById(org.Id); err == nil {
		t.Fatal("解散后组织不应可查")
	}
}

// TestDissolveOrganizationRefundNoOverRefund 锁定 ORG-4:解散退款不得存在
// "读余额 → 退款 → 清零" 的读写窗口。若在读余额之后余额被并发改动,
// 条件更新的 RowsAffected 会为 0,整笔事务回滚,而不是按旧值多退。
func TestDissolveOrganizationRefundNoOverRefund(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "norefund-owner")

	org, err := CreateOrganizationWithOwner("NoOverRefund Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", org.ShadowUserId).Update("quota", 500).Error; err != nil {
		t.Fatalf("充值失败: %v", err)
	}

	oldRefund := config.OrganizationDissolveQuotaRefund
	config.OrganizationDissolveQuotaRefund = "owner"
	t.Cleanup(func() { config.OrganizationDissolveQuotaRefund = oldRefund })

	// 模拟在途结算:余额在解散读取之后、清零之前被消耗。
	// SQLite 内存库无法真并发(第二条连接是另一个空库),这里用 gorm 回调
	// 在锁读之后、于同一连接上注入一次扣减,等价制造 read-then-write 窗口。
	consumed := false
	cbName := "test:consume_shadow_quota"
	if err := DB.Callback().Query().After("gorm:query").Register(cbName, func(tx *gorm.DB) {
		if consumed || tx.Statement.Table != "users" {
			return
		}
		if _, ok := tx.Statement.Clauses["FOR"]; !ok {
			return
		}
		consumed = true
		if err := tx.Session(&gorm.Session{NewDB: true}).
			Model(&User{}).Where("id = ?", org.ShadowUserId).Update("quota", 100).Error; err != nil {
			t.Errorf("注入扣减失败: %v", err)
		}
	}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}
	t.Cleanup(func() { DB.Callback().Query().Remove(cbName) })

	err = DissolveOrganization(org, owner.Id)
	DB.Callback().Query().Remove(cbName)
	if !consumed {
		t.Fatal("测试回调未触发,说明未走行锁读取路径")
	}
	if err == nil {
		t.Fatal("余额在读后被改动,解散应失败而非按旧值退款")
	}

	// Owner 不得收到任何退款,组织也不应被解散
	ownerQuota, _ := GetUserQuota(owner.Id)
	if ownerQuota != owner.Quota {
		t.Fatalf("Owner 不应收到退款,实际增加 %d", ownerQuota-owner.Quota)
	}
	if _, err := GetOrganizationById(org.Id); err != nil {
		t.Fatalf("事务应整体回滚,组织仍应存在: %v", err)
	}
}

func TestOrgInvitationLifecycle(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "inv-owner")
	bob := createOrgTestUser(t, "inv-bob")
	carol := createOrgTestUser(t, "inv-carol")
	dave := createOrgTestUser(t, "inv-dave")

	org, err := CreateOrganizationWithOwner("Invite Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}

	// 邀请角色不能为 owner
	badInv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, Role: OrgRoleOwner, Token: "tok-bad"}
	if err := badInv.Insert(); err == nil {
		t.Fatal("owner 角色邀请应被拒绝")
	}

	// 定向邀请:接受(仅被邀请人本人)
	dirInv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, InviteeId: bob.Id, Email: string(bob.Email), Role: OrgRoleMember, Token: "tok-dir-1", MaxUses: 1}
	if err := dirInv.Insert(); err != nil {
		t.Fatalf("创建定向邀请失败: %v", err)
	}
	if !HasPendingDirectedInvitation(org.Id, bob.Id) {
		t.Fatal("应存在发给 bob 的待处理定向邀请")
	}
	if _, err := AcceptOrganizationInvitation("tok-dir-1", carol.Id); err == nil {
		t.Fatal("非被邀请人接受定向邀请应失败")
	}
	accepted, err := AcceptOrganizationInvitation("tok-dir-1", bob.Id)
	if err != nil {
		t.Fatalf("被邀请人接受应成功: %v", err)
	}
	if accepted.Status != OrgInvitationStatusAccepted {
		t.Fatalf("接受后状态应为 accepted,实际 %d", accepted.Status)
	}
	if m, err := GetOrganizationMember(org.Id, bob.Id); err != nil || m.Role != OrgRoleMember {
		t.Fatalf("bob 应成为 member,err=%v", err)
	}
	if _, err := AcceptOrganizationInvitation("tok-dir-1", bob.Id); err == nil {
		t.Fatal("重复接受应失败")
	}

	// 定向邀请:拒绝(仅本人;拒绝后不可再接受)
	rejInv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, InviteeId: carol.Id, Role: OrgRoleMember, Token: "tok-rej-1", MaxUses: 1}
	if err := rejInv.Insert(); err != nil {
		t.Fatalf("创建邀请失败: %v", err)
	}
	if _, err := RejectOrganizationInvitation("tok-rej-1", dave.Id); err == nil {
		t.Fatal("非被邀请人拒绝应失败")
	}
	if _, err := RejectOrganizationInvitation("tok-rej-1", carol.Id); err != nil {
		t.Fatalf("被邀请人拒绝应成功: %v", err)
	}
	if _, err := AcceptOrganizationInvitation("tok-rej-1", carol.Id); err == nil {
		t.Fatal("拒绝后再接受应失败")
	}

	// 撤销:pending → revoked,撤销后不可接受
	revInv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, InviteeId: dave.Id, Role: OrgRoleMember, Token: "tok-rev-1", MaxUses: 1}
	if err := revInv.Insert(); err != nil {
		t.Fatalf("创建邀请失败: %v", err)
	}
	if _, err := RevokeOrganizationInvitation(org.Id, revInv.Id); err != nil {
		t.Fatalf("撤销应成功: %v", err)
	}
	if _, err := RevokeOrganizationInvitation(org.Id, revInv.Id); err == nil {
		t.Fatal("重复撤销应失败")
	}
	if _, err := AcceptOrganizationInvitation("tok-rev-1", dave.Id); err == nil {
		t.Fatal("撤销后再接受应失败")
	}

	// 过期:接受时惰性标记 expired
	expInv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, InviteeId: dave.Id, Role: OrgRoleMember, Token: "tok-exp-1", MaxUses: 1, ExpiredTime: utils.GetTimestamp() - 10}
	if err := expInv.Insert(); err != nil {
		t.Fatalf("创建邀请失败: %v", err)
	}
	if _, err := AcceptOrganizationInvitation("tok-exp-1", dave.Id); err == nil {
		t.Fatal("过期邀请接受应失败")
	}
	var gotExp OrganizationInvitation
	DB.First(&gotExp, "id = ?", expInv.Id)
	if gotExp.Status != OrgInvitationStatusExpired {
		t.Fatalf("过期邀请应被惰性标记 expired,实际 %d", gotExp.Status)
	}

	// 链接邀请:max_uses 限次;已是成员不可重复加入
	linkInv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, Role: OrgRoleMember, Token: "tok-link-1", MaxUses: 1}
	if err := linkInv.Insert(); err != nil {
		t.Fatalf("创建链接邀请失败: %v", err)
	}
	if _, err := AcceptOrganizationInvitation("tok-link-1", bob.Id); err == nil {
		t.Fatal("已是成员通过链接加入应失败")
	}
	if _, err := AcceptOrganizationInvitation("tok-link-1", carol.Id); err != nil {
		t.Fatalf("链接邀请首次使用应成功: %v", err)
	}
	if _, err := AcceptOrganizationInvitation("tok-link-1", dave.Id); err == nil {
		t.Fatal("超过 max_uses 应失败")
	}

	// 成员上限:达到上限后接受失败
	oldMax := config.OrganizationDefaultMaxMembers
	config.OrganizationDefaultMaxMembers = 3 // owner + bob + carol
	t.Cleanup(func() { config.OrganizationDefaultMaxMembers = oldMax })
	limitInv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, InviteeId: dave.Id, Role: OrgRoleMember, Token: "tok-limit-1", MaxUses: 1}
	if err := limitInv.Insert(); err != nil {
		t.Fatalf("创建邀请失败: %v", err)
	}
	if _, err := AcceptOrganizationInvitation("tok-limit-1", dave.Id); err == nil {
		t.Fatal("成员达上限后接受应失败")
	}

	// 我的待处理邀请:只含未过期 pending,且附组织名/邀请人
	mine, err := GetUserPendingOrgInvitations(dave.Id)
	if err != nil {
		t.Fatalf("查询我的邀请失败: %v", err)
	}
	if len(mine) != 1 || mine[0].Token != "tok-limit-1" {
		t.Fatalf("dave 应只有 1 条待处理邀请,实际 %d", len(mine))
	}
	if mine[0].OrganizationName != org.Name || mine[0].InviterUsername != owner.Username {
		t.Fatalf("邀请应附组织名与邀请人: %+v", mine[0])
	}
}

func TestOrganizationMemberManagement(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "mm-owner")
	admin := createOrgTestUser(t, "mm-admin")
	bob := createOrgTestUser(t, "mm-bob")

	org, err := CreateOrganizationWithOwner("Member Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	for _, m := range []*OrganizationMember{
		{OrganizationId: org.Id, UserId: admin.Id, Role: OrgRoleAdmin},
		{OrganizationId: org.Id, UserId: bob.Id, Role: OrgRoleMember},
	} {
		if err := m.Insert(); err != nil {
			t.Fatalf("插入成员失败: %v", err)
		}
	}

	// 成员列表附用户名
	list, err := GetOrganizationMembersList(org.Id, owner.Id, true, &PaginationParams{})
	if err != nil || list.TotalCount != 3 {
		t.Fatalf("成员列表异常: err=%v total=%d", err, list.TotalCount)
	}
	names := map[string]bool{}
	for _, info := range *list.Data {
		names[info.Username] = true
	}
	if !names[owner.Username] || !names[admin.Username] || !names[bob.Username] {
		t.Fatalf("成员列表应包含用户名: %v", names)
	}

	// 预算可见性(T21c):Admin+ 可见全员预算;Member 仅见自己的
	if _, err := UpdateOrgMemberLimits(org, admin.Id, &QuotaResetSetting{Period: TokenQuotaResetPeriodMonthly, Limit: 300}, nil); err != nil {
		t.Fatalf("设置 admin 预算失败: %v", err)
	}
	if _, err := UpdateOrgMemberLimits(org, bob.Id, &QuotaResetSetting{Period: TokenQuotaResetPeriodMonthly, Limit: 500}, nil); err != nil {
		t.Fatalf("设置 bob 预算失败: %v", err)
	}
	adminView, err := GetOrganizationMembersList(org.Id, admin.Id, true, &PaginationParams{})
	if err != nil {
		t.Fatalf("成员列表异常: %v", err)
	}
	budgetByUser := map[int]*QuotaResetSetting{}
	for _, info := range *adminView.Data {
		budgetByUser[info.UserId] = info.Budget
	}
	if budgetByUser[bob.Id] == nil || budgetByUser[bob.Id].Limit != 500 {
		t.Fatalf("Admin 视角应可见 bob 的预算: %+v", budgetByUser[bob.Id])
	}
	memberView, err := GetOrganizationMembersList(org.Id, bob.Id, false, &PaginationParams{})
	if err != nil {
		t.Fatalf("成员列表异常: %v", err)
	}
	for _, info := range *memberView.Data {
		if info.UserId == bob.Id {
			if info.Budget == nil || info.Budget.Limit != 500 {
				t.Fatalf("Member 视角应可见自己的预算: %+v", info.Budget)
			}
		} else if info.Budget != nil {
			t.Fatalf("Member 视角不应可见他人预算: user=%d budget=%+v", info.UserId, info.Budget)
		}
	}

	// 角色修改:owner 不可改;非法角色不可设;member → admin 成功
	if _, err := UpdateOrganizationMemberRole(org.Id, owner.Id, OrgRoleMember); err == nil {
		t.Fatal("修改 owner 角色应失败")
	}
	if _, err := UpdateOrganizationMemberRole(org.Id, bob.Id, OrgRoleOwner); err == nil {
		t.Fatal("角色设为 owner 应失败")
	}
	if _, err := UpdateOrganizationMemberRole(org.Id, bob.Id, "super"); err == nil {
		t.Fatal("非法角色应失败")
	}
	updated, err := UpdateOrganizationMemberRole(org.Id, bob.Id, OrgRoleAdmin)
	if err != nil || updated.Role != OrgRoleAdmin {
		t.Fatalf("member 升 admin 应成功: err=%v", err)
	}
	if got, _ := GetOrganizationMember(org.Id, bob.Id); got.Role != OrgRoleAdmin {
		t.Fatalf("角色未持久化: %s", got.Role)
	}

	// 移除成员:owner 不可移除;移除后其组织令牌禁用,他人令牌不受影响
	if err := RemoveOrganizationMember(org, owner.Id); err == nil {
		t.Fatal("移除 owner 应失败")
	}
	bobToken := &Token{UserId: org.ShadowUserId, CreatedBy: bob.Id, Key: "mm-bob-token-key", Name: "bob-org-token", Status: config.TokenStatusEnabled}
	adminToken := &Token{UserId: org.ShadowUserId, CreatedBy: admin.Id, Key: "mm-admin-token-key", Name: "admin-org-token", Status: config.TokenStatusEnabled}
	for _, tk := range []*Token{bobToken, adminToken} {
		if err := DB.Create(tk).Error; err != nil {
			t.Fatalf("创建组织令牌失败: %v", err)
		}
	}
	if err := RemoveOrganizationMember(org, bob.Id); err != nil {
		t.Fatalf("移除成员应成功: %v", err)
	}
	if _, err := GetOrganizationMember(org.Id, bob.Id); err == nil {
		t.Fatal("移除后成员关系应不存在")
	}
	var gotBobToken, gotAdminToken Token
	DB.First(&gotBobToken, "id = ?", bobToken.Id)
	DB.First(&gotAdminToken, "id = ?", adminToken.Id)
	if gotBobToken.Status != config.TokenStatusDisabled {
		t.Fatalf("被移除成员的组织令牌应禁用,实际 %d", gotBobToken.Status)
	}
	if gotAdminToken.Status != config.TokenStatusEnabled {
		t.Fatalf("其他成员令牌不应受影响,实际 %d", gotAdminToken.Status)
	}
	if err := RemoveOrganizationMember(org, bob.Id); err == nil {
		t.Fatal("重复移除应失败")
	}
}

func TestGetOrganizationAuditLogsFilters(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "audit-owner")
	other := createOrgTestUser(t, "audit-other")
	org, err := CreateOrganizationWithOwner("Audit Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	logs := []*OrganizationAuditLog{
		{OrganizationId: org.Id, ActorId: owner.Id, Action: "org.update", Content: "a", CreatedTime: 1000},
		{OrganizationId: org.Id, ActorId: owner.Id, Action: "token.create", Content: "b", CreatedTime: 2000},
		{OrganizationId: org.Id, ActorId: other.Id, Action: "org.update", Content: "c", CreatedTime: 3000},
		{OrganizationId: org.Id + 999, ActorId: owner.Id, Action: "org.update", Content: "外组织", CreatedTime: 1500},
	}
	for _, l := range logs {
		if err := DB.Create(l).Error; err != nil {
			t.Fatalf("写入审计日志失败: %v", err)
		}
	}

	// 无过滤:只返回本组织的日志,不串组织
	all, err := GetOrganizationAuditLogs(org.Id, &OrgAuditLogsParams{})
	if err != nil || all.TotalCount != 3 {
		t.Fatalf("无过滤应返回本组织 3 条,err=%v got=%d", err, all.TotalCount)
	}
	// 按操作者过滤
	byActor, err := GetOrganizationAuditLogs(org.Id, &OrgAuditLogsParams{ActorId: other.Id})
	if err != nil || byActor.TotalCount != 1 || (*byActor.Data)[0].Content != "c" {
		t.Fatalf("按操作者过滤异常: err=%v result=%+v", err, byActor)
	}
	// 按动作过滤
	byAction, err := GetOrganizationAuditLogs(org.Id, &OrgAuditLogsParams{Action: "token.create"})
	if err != nil || byAction.TotalCount != 1 || (*byAction.Data)[0].Content != "b" {
		t.Fatalf("按动作过滤异常: err=%v", err)
	}
	// 按时间窗口过滤
	byTime, err := GetOrganizationAuditLogs(org.Id, &OrgAuditLogsParams{StartTimestamp: 1500, EndTimestamp: 2500})
	if err != nil || byTime.TotalCount != 1 || (*byTime.Data)[0].Content != "b" {
		t.Fatalf("按时间过滤异常: err=%v", err)
	}
	// 组合过滤 + 分页(默认 id 倒序)
	combo, err := GetOrganizationAuditLogs(org.Id, &OrgAuditLogsParams{
		ActorId:          owner.Id,
		Action:           "org.update",
		PaginationParams: PaginationParams{Page: 1, Size: 1},
	})
	if err != nil || combo.TotalCount != 1 || (*combo.Data)[0].Content != "a" {
		t.Fatalf("组合过滤异常: err=%v", err)
	}
	// 不允许的排序字段应报错
	if _, err := GetOrganizationAuditLogs(org.Id, &OrgAuditLogsParams{
		PaginationParams: PaginationParams{Order: "action"},
	}); err == nil {
		t.Fatal("不在白名单的排序字段应报错")
	}
}

func TestOrganizationAdminFunctions(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "adm-owner")
	member := createOrgTestUser(t, "adm-member")
	org, err := CreateOrganizationWithOwner("Admin Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	org2, err := CreateOrganizationWithOwner("Other Org", "", member.Id)
	if err != nil {
		t.Fatalf("创建组织2失败: %v", err)
	}
	m := &OrganizationMember{OrganizationId: org.Id, UserId: member.Id, Role: OrgRoleMember}
	if err := m.Insert(); err != nil {
		t.Fatalf("插入成员失败: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", org.ShadowUserId).
		Updates(map[string]interface{}{"quota": 500, "used_quota": 120}).Error; err != nil {
		t.Fatalf("设置影子账户积分失败: %v", err)
	}

	// 列表:全量与 keyword 搜索
	list, err := GetOrganizationsAdminList(&GenericParams{})
	if err != nil || list.TotalCount != 2 {
		t.Fatalf("组织列表应有 2 条,err=%v got=%d", err, list.TotalCount)
	}
	search, err := GetOrganizationsAdminList(&GenericParams{Keyword: "Admin"})
	if err != nil || search.TotalCount != 1 || (*search.Data)[0].Id != org.Id {
		t.Fatalf("keyword 搜索异常: err=%v", err)
	}

	// 详情:组织池余额、成员数、Owner
	info, err := GetOrganizationAdminInfo(org.Id)
	if err != nil {
		t.Fatalf("查询组织详情失败: %v", err)
	}
	if info.Quota != 500 || info.UsedQuota != 120 || info.MemberCount != 2 || info.OwnerId != owner.Id {
		t.Fatalf("详情数据异常: %+v", info)
	}

	// Owner 查询
	ownerId, err := GetOrganizationOwnerId(org2.Id)
	if err != nil || ownerId != member.Id {
		t.Fatalf("Owner 查询异常: err=%v got=%d", err, ownerId)
	}

	// 启停:同步影子账户状态
	if err := SetOrganizationStatus(org, 0); err == nil {
		t.Fatal("无效状态应报错")
	}
	if err := SetOrganizationStatus(org, OrganizationStatusDisabled); err != nil {
		t.Fatalf("禁用组织失败: %v", err)
	}
	var shadow User
	DB.First(&shadow, "id = ?", org.ShadowUserId)
	if org.Status != OrganizationStatusDisabled || shadow.Status != config.UserStatusDisabled {
		t.Fatalf("禁用后状态异常: org=%d shadow=%d", org.Status, shadow.Status)
	}
	if err := SetOrganizationStatus(org, OrganizationStatusEnabled); err != nil {
		t.Fatalf("启用组织失败: %v", err)
	}
	DB.First(&shadow, "id = ?", org.ShadowUserId)
	if org.Status != OrganizationStatusEnabled || shadow.Status != config.UserStatusEnabled {
		t.Fatalf("启用后状态异常: org=%d shadow=%d", org.Status, shadow.Status)
	}
}

func TestShadowUserExclusionHardening(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "hard-owner")
	org, err := CreateOrganizationWithOwner("Harden Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	var shadow User
	if err := DB.First(&shadow, "id = ?", org.ShadowUserId).Error; err != nil {
		t.Fatalf("查询影子账户失败: %v", err)
	}

	// ValidateAccessToken:影子账户 access_token 不可登录,真实用户不受影响
	if got := ValidateAccessToken(shadow.AccessToken); got != nil {
		t.Fatal("影子账户的 access_token 不应通过校验")
	}
	if got := ValidateAccessToken(owner.AccessToken); got == nil || got.Id != owner.Id {
		t.Fatal("真实用户的 access_token 应通过校验")
	}

	// GetUserIdByAffCode:影子账户 aff_code 不可用作邀请码
	if _, err := GetUserIdByAffCode(shadow.AffCode); err == nil {
		t.Fatal("影子账户的 aff_code 不应可用")
	}
	if id, err := GetUserIdByAffCode(owner.AffCode); err != nil || id != owner.Id {
		t.Fatalf("真实用户的 aff_code 应可用: err=%v id=%d", err, id)
	}
}

// TestGetUsersListOrgFieldsAndStats 用户列表行带所属组织与影子账户标记；按 org_id 筛选时返回
// 该组织成员与其影子账户；页头统计口径与默认列表一致（排除影子账户与软删用户）。
func TestGetUsersListOrgFieldsAndStats(t *testing.T) {
	setupOrgTestDB(t)

	root := createCoverageUser(t, "root", config.UserStatusEnabled, config.UserTypeNormal)
	admin := createCoverageUser(t, "admin", config.UserStatusEnabled, config.UserTypeNormal)
	alice := createCoverageUser(t, "alice", config.UserStatusEnabled, config.UserTypeNormal)
	bob := createCoverageUser(t, "bob", config.UserStatusDisabled, config.UserTypeNormal)
	gone := createCoverageUser(t, "gone", config.UserStatusEnabled, config.UserTypeNormal)
	shadow := createCoverageUser(t, "org-shadow", config.UserStatusEnabled, config.UserTypeOrgShadow)
	DB.Model(root).Update("role", config.RoleRootUser)
	DB.Model(admin).Update("role", config.RoleAdminUser)
	DB.Model(alice).Update("role", config.RoleCommonUser)
	DB.Model(bob).Update("role", config.RoleCommonUser)
	if err := DB.Delete(gone).Error; err != nil {
		t.Fatalf("软删用户失败: %v", err)
	}

	acme := &Organization{Name: "Acme", Slug: "acme-list", ShadowUserId: shadow.Id, CreatedBy: alice.Id}
	other := &Organization{Name: "Other", Slug: "other-list", ShadowUserId: 9999, CreatedBy: bob.Id}
	for _, org := range []*Organization{acme, other} {
		if err := org.Insert(); err != nil {
			t.Fatalf("插入组织失败: %v", err)
		}
	}
	members := []*OrganizationMember{
		{OrganizationId: other.Id, UserId: alice.Id, Role: OrgRoleMember},
		{OrganizationId: acme.Id, UserId: alice.Id, Role: OrgRoleOwner},
		{OrganizationId: other.Id, UserId: bob.Id, Role: OrgRoleOwner},
	}
	for _, m := range members {
		if err := m.Insert(); err != nil {
			t.Fatalf("插入成员失败: %v", err)
		}
	}

	all, err := GetUsersList(&SearchUserParams{})
	if err != nil {
		t.Fatalf("查询用户列表失败: %v", err)
	}
	rows := map[string]*UserListItem{}
	for _, item := range *all.Data {
		rows[item.Username] = item
	}
	if len(rows) != 4 || rows["org-shadow"] != nil {
		t.Fatalf("默认列表应只含 4 个非影子用户，实际 %v", rows)
	}
	if r := rows["alice"]; r.OrgId != other.Id || r.OrgName != "Other" || r.IsShadow {
		t.Fatalf("alice 默认应取最早加入的组织 Other: %+v", r)
	}
	if r := rows["root"]; r.OrgId != 0 || r.OrgName != "" {
		t.Fatalf("root 不属于任何组织: %+v", r)
	}

	filtered, err := GetUsersList(&SearchUserParams{
		GenericParams: GenericParams{PaginationParams: PaginationParams{Order: "-quota"}},
		OrgId:         acme.Id,
	})
	if err != nil {
		t.Fatalf("按组织筛选失败: %v", err)
	}
	if filtered.TotalCount != 2 {
		t.Fatalf("Acme 应含 alice 与影子账户，实际 %d", filtered.TotalCount)
	}
	for _, item := range *filtered.Data {
		switch item.Username {
		case "alice":
			if item.OrgId != acme.Id || item.OrgName != "Acme" || item.IsShadow {
				t.Fatalf("筛选 Acme 时 alice 应显示 Acme: %+v", item)
			}
		case "org-shadow":
			if item.OrgId != acme.Id || item.OrgName != "Acme" || !item.IsShadow {
				t.Fatalf("影子账户应标记 is_shadow 且归属 Acme: %+v", item)
			}
		default:
			t.Fatalf("筛选 Acme 不应返回 %s", item.Username)
		}
	}

	disabled, err := GetUsersList(&SearchUserParams{Status: config.UserStatusDisabled})
	if err != nil || disabled.TotalCount != 1 || (*disabled.Data)[0].Username != "bob" {
		t.Fatalf("按状态筛选应只返回 bob: %v %+v", err, disabled)
	}
	admins, err := GetUsersList(&SearchUserParams{MinRole: config.RoleAdminUser})
	if err != nil || admins.TotalCount != 2 {
		t.Fatalf("按最低角色筛选应返回 root 与 admin: %v %+v", err, admins)
	}

	stats, err := GetUserStats()
	if err != nil {
		t.Fatalf("统计失败: %v", err)
	}
	want := UserStats{Total: 4, Enabled: 3, Disabled: 1, Root: 1, Admin: 1, Common: 2}
	if *stats != want {
		t.Fatalf("统计不符: got %+v want %+v", *stats, want)
	}

	// 页头 chip 计数必须等于点击该 chip 后列表的总条数。
	chips := []struct {
		name   string
		params SearchUserParams
		count  int64
	}{
		{"all", SearchUserParams{}, stats.Total},
		{"enabled", SearchUserParams{Status: config.UserStatusEnabled}, stats.Enabled},
		{"disabled", SearchUserParams{Status: config.UserStatusDisabled}, stats.Disabled},
		{"admin", SearchUserParams{MinRole: config.RoleAdminUser}, stats.Admin + stats.Root},
	}
	for _, chip := range chips {
		res, err := GetUsersList(&chip.params)
		if err != nil {
			t.Fatalf("chip %s 列表失败: %v", chip.name, err)
		}
		if res.TotalCount != chip.count {
			t.Fatalf("chip %s 计数 %d 与筛选结果 %d 不一致", chip.name, chip.count, res.TotalCount)
		}
	}
}
