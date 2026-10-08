package model

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// reloadOrg 重新读取组织行,用于断言预算计数列
func reloadOrg(t *testing.T, orgId int) *Organization {
	t.Helper()
	org, err := GetOrganizationById(orgId)
	if err != nil {
		t.Fatalf("读取组织失败: %v", err)
	}
	return org
}

// TestUpdateOrganizationBudget 校验参数校验、清除限制与周期变化清零重计
func TestUpdateOrganizationBudget(t *testing.T) {
	setupOrgTestDB(t)
	org, _ := createGuardrailFixture(t)

	if err := UpdateOrganizationBudget(org, &QuotaResetSetting{Period: "daily", Limit: 0}); err == nil {
		t.Fatal("limit<=0 应校验失败")
	}
	if err := UpdateOrganizationBudget(org, &QuotaResetSetting{Period: "yearly", Limit: 100}); err == nil {
		t.Fatal("非法周期应校验失败")
	}
	if err := UpdateOrganizationBudget(org, &QuotaResetSetting{Period: "daily", Limit: 100}); err != nil {
		t.Fatalf("配置日预算失败: %v", err)
	}
	if got := reloadOrg(t, org.Id).Setting.Data().Budget; got == nil || got.Limit != 100 {
		t.Fatalf("预算未落库: %+v", got)
	}

	if ok, err := ReserveOrganizationBudget(org.Id, 60); err != nil || !ok {
		t.Fatalf("预留 60 应成功: ok=%v err=%v", ok, err)
	}
	// 周期不变时只改额度,计数保留
	if err := UpdateOrganizationBudget(org, &QuotaResetSetting{Period: "daily", Limit: 200}); err != nil {
		t.Fatalf("调整额度失败: %v", err)
	}
	if got := reloadOrg(t, org.Id).BudgetUsed; got != 60 {
		t.Fatalf("同周期改额度不应清零: budget_used=%d", got)
	}
	// 周期变化 → 清零重计
	if err := UpdateOrganizationBudget(org, &QuotaResetSetting{Period: "monthly", Limit: 200}); err != nil {
		t.Fatalf("切换周期失败: %v", err)
	}
	reloaded := reloadOrg(t, org.Id)
	if reloaded.BudgetUsed != 0 || reloaded.BudgetStart != 0 {
		t.Fatalf("周期变化应清零重计: used=%d start=%d", reloaded.BudgetUsed, reloaded.BudgetStart)
	}
	// 清除限制
	if err := UpdateOrganizationBudget(org, nil); err != nil {
		t.Fatalf("清除预算失败: %v", err)
	}
	if got := reloadOrg(t, org.Id).Setting.Data().Budget; got != nil {
		t.Fatalf("预算应已清除: %+v", got)
	}
	if ok, err := ReserveOrganizationBudget(org.Id, 999999); err != nil || !ok {
		t.Fatalf("未配置团队预算应放行: ok=%v err=%v", ok, err)
	}
}

// TestReserveOrganizationBudget 校验团队硬闸边界:在期内自增判上限、超限拒绝且不改计数、跨期清零重计
func TestReserveOrganizationBudget(t *testing.T) {
	setupOrgTestDB(t)
	org, _ := createGuardrailFixture(t)
	if err := UpdateOrganizationBudget(org, &QuotaResetSetting{Period: "daily", Limit: 100}); err != nil {
		t.Fatalf("配置预算失败: %v", err)
	}
	assertUsed := func(want int) {
		t.Helper()
		if got := reloadOrg(t, org.Id).BudgetUsed; got != want {
			t.Fatalf("BudgetUsed = %d, want %d", got, want)
		}
	}

	if ok, err := ReserveOrganizationBudget(org.Id, 60); err != nil || !ok {
		t.Fatalf("预留 60 应成功: ok=%v err=%v", ok, err)
	}
	assertUsed(60)
	if ok, err := ReserveOrganizationBudget(org.Id, 60); err != nil || ok {
		t.Fatalf("预留 60 应因超限被拒: ok=%v err=%v", ok, err)
	}
	assertUsed(60)
	if ok, err := ReserveOrganizationBudget(org.Id, 40); err != nil || !ok {
		t.Fatalf("预留 40(到上限)应成功: ok=%v err=%v", ok, err)
	}
	assertUsed(100)
	if ok, _ := ReserveOrganizationBudget(org.Id, 1); ok {
		t.Fatal("已达上限后预留应被拒")
	}
	assertUsed(100)

	// 跨期:退回上一周期起点并置高计数,预留应清零重计并成功
	if err := DB.Model(&Organization{}).Where("id = ?", org.Id).UpdateColumns(map[string]interface{}{
		"budget_used":  500,
		"budget_start": time.Now().Add(-48 * time.Hour).Unix(),
	}).Error; err != nil {
		t.Fatalf("构造跨期状态失败: %v", err)
	}
	if ok, err := ReserveOrganizationBudget(org.Id, 30); err != nil || !ok {
		t.Fatalf("跨期预留 30 应成功(清零重计): ok=%v err=%v", ok, err)
	}
	assertUsed(30)

	// 回冲(结算/预留失败路径):可负、钳制 ≥0
	if err := AccrueOrganizationBudgetUsed(org.Id, -10); err != nil {
		t.Fatalf("回冲失败: %v", err)
	}
	assertUsed(20)
	if err := AccrueOrganizationBudgetUsed(org.Id, -999); err != nil {
		t.Fatalf("回冲失败: %v", err)
	}
	assertUsed(0)
}

// TestReserveOrganizationBudgetConcurrent 团队硬闸核心守护:N 个并发预留同一组织预算,
// 合计绝不超过周期上限(跨全体成员共享同一行,由 DB 行级串行 + WHERE 上限判据保证)。
func TestReserveOrganizationBudgetConcurrent(t *testing.T) {
	setupOrgBillingTestDB(t) // 单连接共享内存库,确保并发命中同一份数据
	org, _ := createGuardrailFixture(t)
	const limit = 1000
	const amount = 100
	const workers = 50
	const wantSucceed = limit / amount

	if err := UpdateOrganizationBudget(org, &QuotaResetSetting{Period: "monthly", Limit: limit}); err != nil {
		t.Fatalf("配置预算失败: %v", err)
	}

	var success, rejected int64
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := ReserveOrganizationBudget(org.Id, amount)
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
	if got := reloadOrg(t, org.Id).BudgetUsed; got != wantSucceed*amount {
		t.Fatalf("并发预留超支: BudgetUsed = %d, want %d(=上限)", got, wantSucceed*amount)
	}
}
