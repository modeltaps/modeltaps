package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/gorm"
)

// recordLockedOrgReads 注册查询回调,记录执行过的语句表名以及组织行是否被加锁读取,
// 用于断言"成员上限统计"发生在组织行锁之后(ORG-5)。返回读取记录的函数。
func recordLockedOrgReads(t *testing.T, cbName string) func() []string {
	t.Helper()
	var seq []string
	if err := DB.Callback().Query().After("gorm:query").Register(cbName, func(tx *gorm.DB) {
		switch tx.Statement.Table {
		case "organizations":
			if _, locked := tx.Statement.Clauses["FOR"]; locked {
				seq = append(seq, "lock:organizations")
			}
		case "organization_members":
			seq = append(seq, "read:organization_members")
		}
	}); err != nil {
		t.Fatalf("注册回调失败: %v", err)
	}
	t.Cleanup(func() { DB.Callback().Query().Remove(cbName) })
	return func() []string { return seq }
}

// TestCreateOrgMemberAccountMemberCapLocked 锁定 ORG-5:代建账号的
// "统计成员数 → 插入成员" 必须在组织行锁之内,且成员上限按库内最新值判定。
func TestCreateOrgMemberAccountMemberCapLocked(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "cap-owner")
	org, err := CreateOrganizationWithOwner("Cap Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	// 上限设为 1(仅 Owner),后续代建应被拒
	if err := DB.Model(&Organization{}).Where("id = ?", org.Id).Update("max_members", 1).Error; err != nil {
		t.Fatalf("设置成员上限失败: %v", err)
	}

	readSeq := recordLockedOrgReads(t, "test:org_member_cap_lock")

	// 传入的 org 结构体仍是旧值(MaxMembers=0),上限须以锁内读到的组织行为准
	if _, _, err := CreateOrgMemberAccount(org, "cap-u1", "secret123", "", "", OrgRoleMember, false); err == nil {
		t.Fatal("成员已达上限,代建应被拒绝")
	}

	seq := readSeq()
	if len(seq) < 2 || seq[0] != "lock:organizations" {
		t.Fatalf("应先行锁组织行再统计成员,实际顺序: %v", seq)
	}

	// 拒绝路径整体回滚:不应残留用户
	var users int64
	DB.Model(&User{}).Where("username = ?", "cap-u1").Count(&users)
	if users != 0 {
		t.Fatalf("超限时不应残留用户,实际 %d 条", users)
	}
}

// TestAcceptInvitationMemberCapLocked 锁定 ORG-5:接受邀请的
// "统计成员数 → 插入成员" 同样必须在组织行锁之内。
func TestAcceptInvitationMemberCapLocked(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "invcap-owner")
	bob := createOrgTestUser(t, "invcap-bob")
	org, err := CreateOrganizationWithOwner("InvCap Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	if err := DB.Model(&Organization{}).Where("id = ?", org.Id).Update("max_members", 1).Error; err != nil {
		t.Fatalf("设置成员上限失败: %v", err)
	}
	inv := &OrganizationInvitation{OrganizationId: org.Id, InviterId: owner.Id, InviteeId: bob.Id, Email: string(bob.Email), Role: OrgRoleMember, Token: "tok-invcap", MaxUses: 1}
	if err := inv.Insert(); err != nil {
		t.Fatalf("创建邀请失败: %v", err)
	}

	readSeq := recordLockedOrgReads(t, "test:invitation_cap_lock")

	if _, err := AcceptOrganizationInvitation("tok-invcap", bob.Id); err == nil {
		t.Fatal("成员已达上限,接受邀请应被拒绝")
	}

	seq := readSeq()
	if len(seq) < 2 || seq[0] != "lock:organizations" {
		t.Fatalf("应先行锁组织行再统计成员,实际顺序: %v", seq)
	}
	if _, err := GetOrganizationMember(org.Id, bob.Id); err == nil {
		t.Fatal("超限时不应写入成员关系")
	}
}

// TestCreateOrgMemberAccount 代建成员账号(ORG-1):创建用户 + 入组,角色/密码校验,事务回滚。
func TestCreateOrgMemberAccount(t *testing.T) {
	setupOrgTestDB(t)
	owner := createOrgTestUser(t, "acct-owner")
	org, err := CreateOrganizationWithOwner("Acct Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}

	// owner 角色不可代建
	if _, _, err := CreateOrgMemberAccount(org, "u1", "pw", "", "", OrgRoleOwner, false); err == nil {
		t.Fatal("owner 角色应被拒绝")
	}
	// 空密码拒绝
	if _, _, err := CreateOrgMemberAccount(org, "u1", "", "", "", OrgRoleMember, false); err == nil {
		t.Fatal("空密码应被拒绝")
	}

	// 正常创建
	user, member, err := CreateOrgMemberAccount(org, "newbie", "secret123", "", "", OrgRoleMember, false)
	if err != nil {
		t.Fatalf("代建成员失败: %v", err)
	}
	if user.Id == 0 || member.Id == 0 {
		t.Fatal("用户/成员应已创建")
	}
	if member.Role != OrgRoleMember || member.OrganizationId != org.Id || member.UserId != user.Id {
		t.Fatalf("成员关系异常: %+v", member)
	}
	if user.DisplayName != "newbie" {
		t.Fatalf("displayName 应回退用户名, got %q", user.DisplayName)
	}
	if user.Password == "secret123" {
		t.Fatal("密码应已哈希存储")
	}

	// 确实成为成员
	got, err := GetOrganizationMember(org.Id, user.Id)
	if err != nil || got.Role != OrgRoleMember {
		t.Fatalf("应可查到成员: %v", err)
	}

	// 用户名重复 → 事务回滚,不应残留成员
	if _, _, err := CreateOrgMemberAccount(org, "newbie", "x", "", "", OrgRoleMember, false); err == nil {
		t.Fatal("重复用户名应失败")
	}
}

// TestCreateOrgMemberAccountSignupBonus 代建账号注册奖励开关(SEC-11):
// 默认不发 QuotaForNewUser 与赠送日志,显式开启才发;自助注册路径(InsertWithTx)行为不变。
func TestCreateOrgMemberAccountSignupBonus(t *testing.T) {
	setupOrgTestDB(t)
	oldQuota := config.QuotaForNewUser
	config.QuotaForNewUser = 500
	t.Cleanup(func() { config.QuotaForNewUser = oldQuota })

	owner := createOrgTestUser(t, "bonus-owner")
	org, err := CreateOrganizationWithOwner("Bonus Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}

	countBonusLogs := func(userId int) int64 {
		t.Helper()
		var count int64
		if err := DB.Model(&Log{}).Where("user_id = ? AND type = ?", userId, LogTypeSystem).Count(&count).Error; err != nil {
			t.Fatalf("查询日志失败: %v", err)
		}
		return count
	}

	// 代建默认不发奖励:Quota 为 0,无赠送日志
	noBonus, _, err := CreateOrgMemberAccount(org, "no-bonus", "secret123", "", "", OrgRoleMember, false)
	if err != nil {
		t.Fatalf("代建成员失败: %v", err)
	}
	if noBonus.Quota != 0 {
		t.Fatalf("代建账号 Quota 应为 0, got %d", noBonus.Quota)
	}
	if n := countBonusLogs(noBonus.Id); n != 0 {
		t.Fatalf("代建账号不应有赠送日志, got %d 条", n)
	}

	// 显式开启时才发奖励
	withBonus, _, err := CreateOrgMemberAccount(org, "with-bonus", "secret123", "", "", OrgRoleMember, true)
	if err != nil {
		t.Fatalf("代建成员失败: %v", err)
	}
	if withBonus.Quota != 500 {
		t.Fatalf("显式开启应发放奖励 500, got %d", withBonus.Quota)
	}
	if n := countBonusLogs(withBonus.Id); n != 1 {
		t.Fatalf("显式开启应有 1 条赠送日志, got %d 条", n)
	}

	// 自助注册路径(InsertWithTx)仍发奖励
	selfReg := &User{Username: "self-reg", Password: "secret123"}
	if err := DB.Transaction(func(tx *gorm.DB) error {
		return selfReg.InsertWithTx(tx, 0)
	}); err != nil {
		t.Fatalf("自助注册失败: %v", err)
	}
	if selfReg.Quota != 500 {
		t.Fatalf("自助注册应发放奖励 500, got %d", selfReg.Quota)
	}
	if n := countBonusLogs(selfReg.Id); n != 1 {
		t.Fatalf("自助注册应有 1 条赠送日志, got %d 条", n)
	}
}
