package model

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/utils"
)

// createGuardrailFixture 创建组织(含影子账户)与 Owner 成员,返回组织与 Owner 用户ID
func createGuardrailFixture(t *testing.T) (*Organization, int) {
	t.Helper()
	user := &User{
		Username:    "guardrail-" + utils.GetRandomString(8),
		Password:    "test-password",
		Email:       NullableEmail(strings.ToLower(utils.GetRandomString(8)) + "@example.com"),
		CreatedTime: utils.GetTimestamp(),
	}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	org, err := CreateOrganizationWithOwner("Guardrail Org", "", user.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	return org, user.Id
}

func TestOrgModelWhitelistAllowed(t *testing.T) {
	tests := []struct {
		name      string
		whitelist []string
		model     string
		want      bool
	}{
		{"空白名单不限制", nil, "gpt-4o", true},
		{"精确匹配", []string{"gpt-4o"}, "gpt-4o", true},
		{"大小写不敏感", []string{"GPT-4o"}, "gpt-4O", true},
		{"前缀通配符命中", []string{"claude-3*"}, "claude-3-opus", true},
		{"前缀通配符未命中", []string{"claude-3*"}, "gpt-4o", false},
		{"未列出模型拒绝", []string{"gpt-4o", "gpt-4o-mini"}, "o1-preview", false},
		{"空白条目跳过", []string{" ", "gpt-4o"}, "gpt-4o", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := orgModelWhitelistAllowed(tt.whitelist, tt.model); got != tt.want {
				t.Fatalf("orgModelWhitelistAllowed(%v, %q) = %v, want %v", tt.whitelist, tt.model, got, tt.want)
			}
		})
	}
}

func TestOrgMemberGuardrailBudgetExceeded(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	todayStart := time.Date(2026, 6, 12, 0, 0, 0, 0, time.UTC).Unix()
	tests := []struct {
		name      string
		guardrail *OrgMemberGuardrail
		want      bool
	}{
		{"无预算配置不限制", &OrgMemberGuardrail{}, false},
		{"limit<=0 不限制", &OrgMemberGuardrail{Budget: &QuotaResetSetting{Period: "daily", Limit: 0}}, false},
		{"当前周期内未超限", &OrgMemberGuardrail{Budget: &QuotaResetSetting{Period: "daily", Limit: 100}, BudgetUsed: 99, BudgetStart: todayStart}, false},
		{"当前周期内已达限额", &OrgMemberGuardrail{Budget: &QuotaResetSetting{Period: "daily", Limit: 100}, BudgetUsed: 100, BudgetStart: todayStart}, true},
		{"跨期后视为清零", &OrgMemberGuardrail{Budget: &QuotaResetSetting{Period: "daily", Limit: 100}, BudgetUsed: 500, BudgetStart: todayStart - 86400}, false},
		{"未知周期不限制", &OrgMemberGuardrail{Budget: &QuotaResetSetting{Period: "yearly", Limit: 100}, BudgetUsed: 500, BudgetStart: todayStart}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.guardrail.BudgetExceeded(now); got != tt.want {
				t.Fatalf("BudgetExceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOrgMemberGuardrailWhitelistFallback(t *testing.T) {
	setupOrgTestDB(t)
	org, ownerId := createGuardrailFixture(t)

	// 组织级缺省白名单
	orgSetting := org.Setting.Data()
	orgSetting.ModelWhitelist = []string{"gpt-4o"}
	org.Setting.Set(orgSetting)
	if err := DB.Model(&Organization{}).Where("id = ?", org.Id).Update("setting", org.Setting).Error; err != nil {
		t.Fatalf("更新组织设置失败: %v", err)
	}

	// 成员未配置白名单 → 回退组织级
	g, err := CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if err != nil {
		t.Fatalf("加载守护数据失败: %v", err)
	}
	if !g.ModelAllowed("gpt-4o") || g.ModelAllowed("claude-3-opus") {
		t.Fatalf("应回退组织级白名单: %v", g.ModelWhitelist)
	}

	// 成员配置自身白名单 → 成员级优先
	if _, err := UpdateOrgMemberLimits(org, ownerId, nil, []string{"claude-3*"}); err != nil {
		t.Fatalf("更新成员限制失败: %v", err)
	}
	g, err = CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if err != nil {
		t.Fatalf("加载守护数据失败: %v", err)
	}
	if !g.ModelAllowed("claude-3-opus") || g.ModelAllowed("gpt-4o") {
		t.Fatalf("成员级白名单应优先: %v", g.ModelWhitelist)
	}
}

func TestUpdateOrgMemberLimitsValidationAndReset(t *testing.T) {
	setupOrgTestDB(t)
	org, ownerId := createGuardrailFixture(t)

	if _, err := UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "daily", Limit: 0}, nil); err == nil {
		t.Fatal("limit<=0 应被拒绝")
	}
	if _, err := UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "yearly", Limit: 100}, nil); err == nil {
		t.Fatal("非法周期应被拒绝")
	}
	if _, err := UpdateOrgMemberLimits(org, ownerId+999, &QuotaResetSetting{Period: "daily", Limit: 100}, nil); err == nil {
		t.Fatal("非成员应被拒绝")
	}

	// 配置预算并产生用量
	member, err := UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "daily", Limit: 100}, nil)
	if err != nil {
		t.Fatalf("配置预算失败: %v", err)
	}
	if err := AccrueOrgMemberBudgetUsed(member.Id, 60); err != nil {
		t.Fatalf("累计预算用量失败: %v", err)
	}

	// 同周期更新(改 limit 不改 period)→ 保留计数
	member, err = UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "daily", Limit: 200}, nil)
	if err != nil {
		t.Fatalf("更新预算失败: %v", err)
	}
	if got := member.BudgetUsed; got != 60 {
		t.Fatalf("同周期更新应保留计数, got %d", got)
	}

	// 周期变化 → 清零重新计数
	member, err = UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "weekly", Limit: 200}, nil)
	if err != nil {
		t.Fatalf("更新预算失败: %v", err)
	}
	if got := member.BudgetUsed; got != 0 {
		t.Fatalf("周期变化应清零计数, got %d", got)
	}
}

func TestAccrueOrgMemberBudgetUsed(t *testing.T) {
	setupOrgTestDB(t)
	org, ownerId := createGuardrailFixture(t)
	member, err := UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "daily", Limit: 100}, nil)
	if err != nil {
		t.Fatalf("配置预算失败: %v", err)
	}

	// 正向累计
	if err := AccrueOrgMemberBudgetUsed(member.Id, 70); err != nil {
		t.Fatalf("累计失败: %v", err)
	}
	if err := AccrueOrgMemberBudgetUsed(member.Id, 40); err != nil {
		t.Fatalf("累计失败: %v", err)
	}
	g, err := CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if err != nil {
		t.Fatalf("加载守护数据失败: %v", err)
	}
	if g.BudgetUsed != 110 {
		t.Fatalf("BudgetUsed = %d, want 110", g.BudgetUsed)
	}
	if !g.BudgetExceeded(time.Now()) {
		t.Fatal("超过限额应判定为超限")
	}

	// 负向回冲(回退预扣)
	if err := AccrueOrgMemberBudgetUsed(member.Id, -40); err != nil {
		t.Fatalf("回冲失败: %v", err)
	}
	g, _ = CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if g.BudgetUsed != 70 {
		t.Fatalf("BudgetUsed = %d, want 70", g.BudgetUsed)
	}
	if g.BudgetExceeded(time.Now()) {
		t.Fatal("回冲后未达限额不应超限")
	}

	// 负向钳制到 0
	if err := AccrueOrgMemberBudgetUsed(member.Id, -999); err != nil {
		t.Fatalf("回冲失败: %v", err)
	}
	g, _ = CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if g.BudgetUsed != 0 {
		t.Fatalf("BudgetUsed = %d, want 0(钳制)", g.BudgetUsed)
	}

	// 未配置预算时 noop
	plain, err := UpdateOrgMemberLimits(org, ownerId, nil, nil)
	if err != nil {
		t.Fatalf("清除预算失败: %v", err)
	}
	if err := AccrueOrgMemberBudgetUsed(plain.Id, 50); err != nil {
		t.Fatalf("noop 累计不应报错: %v", err)
	}
	g, _ = CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if g.BudgetUsed != 0 {
		t.Fatalf("未配置预算不应累计, got %d", g.BudgetUsed)
	}
}

func TestResetOrgMemberBudget(t *testing.T) {
	setupOrgTestDB(t)
	org, ownerId := createGuardrailFixture(t)
	member, err := UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "daily", Limit: 100}, nil)
	if err != nil {
		t.Fatalf("配置预算失败: %v", err)
	}

	// 伪造上一周期的用量(写专用列)
	if err := DB.Model(&OrganizationMember{}).Where("id = ?", member.Id).UpdateColumns(map[string]interface{}{
		"budget_used":  80,
		"budget_start": time.Now().UTC().AddDate(0, 0, -1).Unix(),
	}).Error; err != nil {
		t.Fatalf("写入伪造数据失败: %v", err)
	}

	ResetOrgMemberBudget()

	g, err := CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if err != nil {
		t.Fatalf("加载守护数据失败: %v", err)
	}
	if g.BudgetUsed != 0 {
		t.Fatalf("跨期清扫后 BudgetUsed = %d, want 0", g.BudgetUsed)
	}
	if g.BudgetStart != currentPeriodStart(TokenQuotaResetPeriodDaily, time.Now()).Unix() {
		t.Fatalf("BudgetStart 应滚动到当前周期起点, got %d", g.BudgetStart)
	}

	// 幂等:再次执行无变化
	ResetOrgMemberBudget()
	g, _ = CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if g.BudgetUsed != 0 {
		t.Fatalf("幂等清扫后 BudgetUsed = %d, want 0", g.BudgetUsed)
	}
}

// TestAccrueOrgMemberBudgetUsedConcurrent 同一成员预算在并发累计下必须无丢失更新(金钱正确性,SEC-4)。
// accrueOrgMemberBudgetUsed 用单条原子 UPDATE(CASE 懒重置+自增+钳制)替代读-改-写;本测试守护该不变量,
// 防止回归到非原子实现(默认 BatchUpdateEnabled=false 时 relay 每请求直接调用)。
func TestAccrueOrgMemberBudgetUsedConcurrent(t *testing.T) {
	setupOrgBillingTestDB(t) // 单连接共享内存库,确保并发用例命中同一份数据
	org, ownerId := createGuardrailFixture(t)
	// limit 取足够大,只测累计准确性而非超限闸门
	member, err := UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "monthly", Limit: 1_000_000}, nil)
	if err != nil {
		t.Fatalf("配置预算失败: %v", err)
	}

	const workers = 50
	const delta = 7
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AccrueOrgMemberBudgetUsed(member.Id, delta); err != nil {
				t.Errorf("并发累计失败: %v", err)
			}
		}()
	}
	wg.Wait()

	g, err := CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if err != nil {
		t.Fatalf("加载守护数据失败: %v", err)
	}
	if g.BudgetUsed != workers*delta {
		t.Fatalf("并发累计丢失更新: BudgetUsed = %d, want %d", g.BudgetUsed, workers*delta)
	}
}

// TestReserveOrgMemberBudget 校验原子预留闸(SEC-9)的边界语义:在期内自增判上限、超限拒绝且不改计数、
// 跨期清零重计、未配置预算放行。与 AccrueOrgMemberBudgetUsed(计数器,永不拒绝)形成对照。
func TestReserveOrgMemberBudget(t *testing.T) {
	setupOrgTestDB(t)
	org, ownerId := createGuardrailFixture(t)
	member, err := UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "daily", Limit: 100}, nil)
	if err != nil {
		t.Fatalf("配置预算失败: %v", err)
	}

	assertUsed := func(want int) {
		t.Helper()
		g, err := CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
		if err != nil {
			t.Fatalf("加载守护数据失败: %v", err)
		}
		if g.BudgetUsed != want {
			t.Fatalf("BudgetUsed = %d, want %d", g.BudgetUsed, want)
		}
	}

	// 在期内自增,未超上限 → 成功
	if ok, err := ReserveOrgMemberBudget(member.Id, 60); err != nil || !ok {
		t.Fatalf("预留 60 应成功: ok=%v err=%v", ok, err)
	}
	assertUsed(60)

	// 自增后会超上限(60+60>100)→ 拒绝且计数不变
	if ok, err := ReserveOrgMemberBudget(member.Id, 60); err != nil || ok {
		t.Fatalf("预留 60 应因超限被拒: ok=%v err=%v", ok, err)
	}
	assertUsed(60)

	// 正好到上限(60+40=100)→ 成功
	if ok, err := ReserveOrgMemberBudget(member.Id, 40); err != nil || !ok {
		t.Fatalf("预留 40(到上限)应成功: ok=%v err=%v", ok, err)
	}
	assertUsed(100)

	// 已到上限,再预留任意正值 → 拒绝
	if ok, _ := ReserveOrgMemberBudget(member.Id, 1); ok {
		t.Fatal("已达上限后预留应被拒")
	}
	assertUsed(100)

	// 跨期:把 budget_start 退到上一周期、budget_used 置高,预留应清零重计并成功
	if err := DB.Model(&OrganizationMember{}).Where("id = ?", member.Id).UpdateColumns(map[string]interface{}{
		"budget_used":  500,
		"budget_start": time.Now().Add(-48 * time.Hour).Unix(),
	}).Error; err != nil {
		t.Fatalf("构造跨期状态失败: %v", err)
	}
	if ok, err := ReserveOrgMemberBudget(member.Id, 30); err != nil || !ok {
		t.Fatalf("跨期预留 30 应成功(清零重计): ok=%v err=%v", ok, err)
	}
	assertUsed(30)

	// 未配置预算的成员 → 直接放行(不记账)
	_, plain, err := CreateOrgMemberAccount(org, "plain-"+utils.GetRandomString(6), "secret123", "", "", OrgRoleMember, false)
	if err != nil {
		t.Fatalf("代建成员失败: %v", err)
	}
	if ok, err := ReserveOrgMemberBudget(plain.Id, 999); err != nil || !ok {
		t.Fatalf("未配置预算应放行: ok=%v err=%v", ok, err)
	}
}

// TestReserveOrgMemberBudgetConcurrent 是 SEC-9 的核心守护:N 个并发预留同一成员预算,
// 合计绝不超过周期上限。修复前(入场只做 advisory 检查 / 100x 跳过导致零入场记账)会全部放行并超支;
// 修复后原子 UPDATE 的 WHERE 上限判据 + 行级串行,使恰好 floor(limit/amount) 个成功,budget_used 精确等于上限。
func TestReserveOrgMemberBudgetConcurrent(t *testing.T) {
	setupOrgBillingTestDB(t) // 单连接共享内存库,确保并发命中同一份数据
	org, ownerId := createGuardrailFixture(t)
	const limit = 1000
	const amount = 100
	const workers = 50
	const wantSucceed = limit / amount // 10

	member, err := UpdateOrgMemberLimits(org, ownerId, &QuotaResetSetting{Period: "monthly", Limit: limit}, nil)
	if err != nil {
		t.Fatalf("配置预算失败: %v", err)
	}

	var success, rejected int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := ReserveOrgMemberBudget(member.Id, amount)
			if err != nil {
				t.Errorf("并发预留出错: %v", err)
				return
			}
			if ok {
				atomic.AddInt64(&success, 1)
			} else {
				atomic.AddInt64(&rejected, 1)
			}
		}()
	}
	wg.Wait()

	if success != wantSucceed {
		t.Fatalf("成功预留数 = %d, want %d(合计不得超上限)", success, wantSucceed)
	}
	if success+rejected != workers {
		t.Fatalf("成功+拒绝 = %d, want %d(不得丢失)", success+rejected, workers)
	}
	g, err := CacheGetOrgMemberGuardrail(org.ShadowUserId, ownerId)
	if err != nil {
		t.Fatalf("加载守护数据失败: %v", err)
	}
	if g.BudgetUsed != wantSucceed*amount {
		t.Fatalf("并发预留超支: BudgetUsed = %d, want %d(=上限)", g.BudgetUsed, wantSucceed*amount)
	}
	if g.BudgetUsed > limit {
		t.Fatalf("BudgetUsed=%d 超过周期上限 %d(硬闸失效)", g.BudgetUsed, limit)
	}
}
