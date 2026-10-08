package model

import (
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// setupOrgBillingTestDB 在通用组织测试库基础上限制单连接:
// sqlite :memory: 下多连接会各自得到独立空库,并发用例必须共享同一连接
func setupOrgBillingTestDB(t *testing.T) {
	t.Helper()
	setupOrgTestDB(t)
	sqlDB, err := DB.DB()
	if err != nil {
		t.Fatalf("获取底层连接失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
}

func TestTransferQuotaToOrganization(t *testing.T) {
	setupOrgBillingTestDB(t)
	owner := createOrgTestUser(t, "tr-owner")
	org, err := CreateOrganizationWithOwner("Transfer Quota Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", owner.Id).Update("quota", 1000).Error; err != nil {
		t.Fatalf("初始化个人积分失败: %v", err)
	}

	// 非法参数
	if err := TransferQuotaToOrganization(org, owner.Id, 0); err == nil {
		t.Fatal("转移 0 积分应失败")
	}
	if err := TransferQuotaToOrganization(org, owner.Id, -10); err == nil {
		t.Fatal("转移负数积分应失败")
	}

	// 正常转移
	if err := TransferQuotaToOrganization(org, owner.Id, 300); err != nil {
		t.Fatalf("转移失败: %v", err)
	}
	personalQuota, _ := GetUserQuota(owner.Id)
	orgQuota, _ := GetUserQuota(org.ShadowUserId)
	if personalQuota != 700 || orgQuota != 300 {
		t.Fatalf("转移后余额异常: personal=%d org=%d", personalQuota, orgQuota)
	}

	// 双侧 Log 记录可查
	var logCount int64
	DB.Model(&Log{}).Where("user_id IN ? AND type = ?", []int{owner.Id, org.ShadowUserId}, LogTypeManage).Count(&logCount)
	if logCount != 2 {
		t.Fatalf("应有双侧转移记录,实际 %d 条", logCount)
	}

	// 余额不足:原子拒绝,双方余额不变
	if err := TransferQuotaToOrganization(org, owner.Id, 800); err == nil {
		t.Fatal("余额不足应失败")
	}
	personalQuota, _ = GetUserQuota(owner.Id)
	orgQuota, _ = GetUserQuota(org.ShadowUserId)
	if personalQuota != 700 || orgQuota != 300 {
		t.Fatalf("失败转移不应改变余额: personal=%d org=%d", personalQuota, orgQuota)
	}
}

func TestTransferQuotaToOrganizationConcurrent(t *testing.T) {
	setupOrgBillingTestDB(t)
	owner := createOrgTestUser(t, "tr-conc-owner")
	org, err := CreateOrganizationWithOwner("Concurrent Transfer Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", owner.Id).Update("quota", 1000).Error; err != nil {
		t.Fatalf("初始化个人积分失败: %v", err)
	}

	const workers = 20
	const amount = 100 // 总申请 2000 > 余额 1000,应恰好成功 10 笔
	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := TransferQuotaToOrganization(org, owner.Id, amount); err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	personalQuota, _ := GetUserQuota(owner.Id)
	orgQuota, _ := GetUserQuota(org.ShadowUserId)
	if succeeded != 10 {
		t.Fatalf("并发转移应恰好成功 10 笔,实际 %d", succeeded)
	}
	if personalQuota != 0 || orgQuota != 1000 {
		t.Fatalf("并发转移后余额异常: personal=%d org=%d", personalQuota, orgQuota)
	}
	if personalQuota+orgQuota != 1000 {
		t.Fatalf("积分总量不守恒: %d", personalQuota+orgQuota)
	}
}

// TestOrgTokenPoolIsolation 组织池耗尽时拒绝预扣,且不触碰成员个人积分(规格 §3.3)
func TestOrgTokenPoolIsolation(t *testing.T) {
	setupOrgBillingTestDB(t)
	owner := createOrgTestUser(t, "pool-owner")
	org, err := CreateOrganizationWithOwner("Pool Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	// 个人池 500、组织池 50;组织令牌归属影子账户,created_by 记实际成员
	if err := DB.Model(&User{}).Where("id = ?", owner.Id).Update("quota", 500).Error; err != nil {
		t.Fatalf("初始化个人积分失败: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", org.ShadowUserId).Update("quota", 50).Error; err != nil {
		t.Fatalf("初始化组织积分失败: %v", err)
	}
	token := &Token{UserId: org.ShadowUserId, CreatedBy: owner.Id, Name: "org-key", Status: config.TokenStatusEnabled, RemainQuota: 1000}
	if err := DB.Create(token).Error; err != nil {
		t.Fatalf("创建组织令牌失败: %v", err)
	}

	// 组织池不足:预扣被拒,个人池与组织池均不变
	if err := PreConsumeTokenQuota(token.Id, 100); err == nil {
		t.Fatal("组织池耗尽时预扣应失败")
	}
	personalQuota, _ := GetUserQuota(owner.Id)
	orgQuota, _ := GetUserQuota(org.ShadowUserId)
	if personalQuota != 500 || orgQuota != 50 {
		t.Fatalf("池耗尽拒绝后余额不应变化: personal=%d org=%d", personalQuota, orgQuota)
	}

	// 组织池充足:预扣只动组织池,不动个人池
	if err := PreConsumeTokenQuota(token.Id, 30); err != nil {
		t.Fatalf("预扣失败: %v", err)
	}
	personalQuota, _ = GetUserQuota(owner.Id)
	orgQuota, _ = GetUserQuota(org.ShadowUserId)
	if personalQuota != 500 || orgQuota != 20 {
		t.Fatalf("预扣后余额异常: personal=%d org=%d", personalQuota, orgQuota)
	}

	// 回滚(Undo 路径):负数差额回冲组织池
	if err := PostConsumeTokenQuotaWithInfo(token.Id, org.ShadowUserId, false, -30); err != nil {
		t.Fatalf("回滚失败: %v", err)
	}
	orgQuota, _ = GetUserQuota(org.ShadowUserId)
	if orgQuota != 50 {
		t.Fatalf("回滚后组织池应恢复 50,实际 %d", orgQuota)
	}
	var gotToken Token
	DB.First(&gotToken, "id = ?", token.Id)
	if gotToken.RemainQuota != 1000 || gotToken.UsedQuota != 0 {
		t.Fatalf("回滚后令牌额度应复原: remain=%d used=%d", gotToken.RemainQuota, gotToken.UsedQuota)
	}
}

// TestOrgConsumeConcurrent 并发实扣(结算)一致性:组织池与令牌余额扣减总量准确
func TestOrgConsumeConcurrent(t *testing.T) {
	setupOrgBillingTestDB(t)
	owner := createOrgTestUser(t, "consume-owner")
	org, err := CreateOrganizationWithOwner("Consume Org", "", owner.Id)
	if err != nil {
		t.Fatalf("创建组织失败: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", org.ShadowUserId).Update("quota", 1000).Error; err != nil {
		t.Fatalf("初始化组织积分失败: %v", err)
	}
	token := &Token{UserId: org.ShadowUserId, CreatedBy: owner.Id, Name: "org-key-c", Status: config.TokenStatusEnabled, RemainQuota: 1000}
	if err := DB.Create(token).Error; err != nil {
		t.Fatalf("创建组织令牌失败: %v", err)
	}

	const workers = 10
	const each = 50
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := PostConsumeTokenQuotaWithInfo(token.Id, org.ShadowUserId, false, each); err != nil {
				t.Errorf("并发实扣失败: %v", err)
			}
		}()
	}
	wg.Wait()

	orgQuota, _ := GetUserQuota(org.ShadowUserId)
	if orgQuota != 1000-workers*each {
		t.Fatalf("并发实扣后组织池应为 %d,实际 %d", 1000-workers*each, orgQuota)
	}
	var gotToken Token
	DB.First(&gotToken, "id = ?", token.Id)
	if gotToken.RemainQuota != 1000-workers*each || gotToken.UsedQuota != workers*each {
		t.Fatalf("并发实扣后令牌额度异常: remain=%d used=%d", gotToken.RemainQuota, gotToken.UsedQuota)
	}
}
