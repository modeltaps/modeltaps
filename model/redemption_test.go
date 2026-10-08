package model

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupRedeemTestDB 使用临时文件 SQLite(支持多连接共享同一库),
// 以便并发 goroutine 真正争用同一条兑换码记录。
func setupRedeemTestDB(t *testing.T) {
	t.Helper()
	if viper.GetString("user_token_secret") == "" {
		viper.Set("user_token_secret", "redeem-test-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatalf("初始化用户令牌编码器失败: %v", err)
		}
	}
	dsn := filepath.Join(t.TempDir(), "redeem_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&User{}, &Redemption{}, &UserGroup{}, &Log{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldUsingSQLite := common.UsingSQLite
	common.UsingSQLite = true
	oldDB := DB
	DB = testDB
	t.Cleanup(func() {
		DB = oldDB
		common.UsingSQLite = oldUsingSQLite
	})
}

// TestRedeemConcurrentSingleWinner 断言:N 个 goroutine 同时兑换同一码时
// 仅 1 次成功、用户额度只 +1 份、码终态为 used。
func TestRedeemConcurrentSingleWinner(t *testing.T) {
	setupRedeemTestDB(t)

	user := &User{Username: "redeem-user", Password: "x", AccessToken: "at-redeem", AffCode: "aff-redeem"}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	const codeQuota = 1000
	code := &Redemption{Key: "REDEEMCODE0000000000000000000001", Status: config.RedemptionCodeStatusEnabled, Quota: codeQuota}
	if err := DB.Create(code).Error; err != nil {
		t.Fatalf("创建兑换码失败: %v", err)
	}

	const workers = 16
	var (
		wg         sync.WaitGroup
		mu         sync.Mutex
		successes  int
		failures   int
		grantedSum int
		start      = make(chan struct{})
	)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			quota, err := Redeem(code.Key, user.Id, "127.0.0.1")
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successes++
				grantedSum += quota
			} else {
				failures++
			}
		}()
	}
	close(start)
	wg.Wait()

	if successes != 1 {
		t.Fatalf("应仅 1 次兑换成功,实际 %d 次(失败 %d 次)", successes, failures)
	}
	if grantedSum != codeQuota {
		t.Fatalf("成功返回的发放额度应为 %d,实际 %d", codeQuota, grantedSum)
	}

	var got User
	if err := DB.First(&got, user.Id).Error; err != nil {
		t.Fatalf("回读用户失败: %v", err)
	}
	if got.Quota != codeQuota {
		t.Fatalf("用户额度应只 +1 份(%d),实际 %d", codeQuota, got.Quota)
	}

	var gotCode Redemption
	if err := DB.First(&gotCode, code.Id).Error; err != nil {
		t.Fatalf("回读兑换码失败: %v", err)
	}
	if gotCode.Status != config.RedemptionCodeStatusUsed {
		t.Fatalf("兑换码终态应为 used(%d),实际 %d", config.RedemptionCodeStatusUsed, gotCode.Status)
	}
	if gotCode.RedeemedTime == 0 {
		t.Fatal("兑换码 redeemed_time 应被写入")
	}
}

// TestRedeemReusedCodeRejected 断言:已使用的码再次兑换被拒绝且不重复加额度。
func TestRedeemReusedCodeRejected(t *testing.T) {
	setupRedeemTestDB(t)

	user := &User{Username: "redeem-user-2", Password: "x", AccessToken: "at-redeem-2", AffCode: "aff-redeem-2"}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	const codeQuota = 500
	code := &Redemption{Key: "REDEEMCODE0000000000000000000002", Status: config.RedemptionCodeStatusEnabled, Quota: codeQuota}
	if err := DB.Create(code).Error; err != nil {
		t.Fatalf("创建兑换码失败: %v", err)
	}

	if _, err := Redeem(code.Key, user.Id, "127.0.0.1"); err != nil {
		t.Fatalf("首次兑换应成功,实际: %v", err)
	}
	if _, err := Redeem(code.Key, user.Id, "127.0.0.1"); err == nil {
		t.Fatal("二次兑换应失败")
	}

	var got User
	if err := DB.First(&got, user.Id).Error; err != nil {
		t.Fatalf("回读用户失败: %v", err)
	}
	if got.Quota != codeQuota {
		t.Fatalf("用户额度应只 +1 份(%d),实际 %d", codeQuota, got.Quota)
	}
}

// TestRedeemScopeMatrix 断言 3 种用途 × 2 类账户的兑换结果:
// any 均可兑换,personal 仅个人,org 仅组织影子账户;被拒时兑换码保持可用。
func TestRedeemScopeMatrix(t *testing.T) {
	cases := []struct {
		name     string
		scope    string
		userType int
		wantErr  bool
	}{
		{"any-个人可兑换", RedemptionScopeAny, 0, false},
		{"any-组织可兑换", RedemptionScopeAny, config.UserTypeOrgShadow, false},
		{"personal-个人可兑换", RedemptionScopePersonal, 0, false},
		{"personal-组织被拒", RedemptionScopePersonal, config.UserTypeOrgShadow, true},
		{"org-个人被拒", RedemptionScopeOrg, 0, true},
		{"org-组织可兑换", RedemptionScopeOrg, config.UserTypeOrgShadow, false},
	}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupRedeemTestDB(t)
			user := &User{
				Username:    fmt.Sprintf("scope-user-%d", i),
				Password:    "x",
				AccessToken: fmt.Sprintf("at-scope-%d", i),
				AffCode:     fmt.Sprintf("aff-scope-%d", i),
				Type:        tc.userType,
			}
			if err := DB.Create(user).Error; err != nil {
				t.Fatalf("创建用户失败: %v", err)
			}
			const codeQuota = 300
			code := &Redemption{
				Key:    fmt.Sprintf("SCOPECODE%023d", i),
				Status: config.RedemptionCodeStatusEnabled,
				Quota:  codeQuota,
				Scope:  tc.scope,
			}
			if err := DB.Create(code).Error; err != nil {
				t.Fatalf("创建兑换码失败: %v", err)
			}

			_, err := Redeem(code.Key, user.Id, "127.0.0.1")
			if tc.wantErr && err == nil {
				t.Fatal("该用途应拒绝此账户类型")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("该用途应允许兑换,实际: %v", err)
			}

			var gotUser User
			if err := DB.First(&gotUser, user.Id).Error; err != nil {
				t.Fatalf("回读用户失败: %v", err)
			}
			var gotCode Redemption
			if err := DB.First(&gotCode, code.Id).Error; err != nil {
				t.Fatalf("回读兑换码失败: %v", err)
			}
			if tc.wantErr {
				if gotUser.Quota != 0 {
					t.Fatalf("被拒时不应加额度,实际 %d", gotUser.Quota)
				}
				if gotCode.Status != config.RedemptionCodeStatusEnabled {
					t.Fatalf("被拒时兑换码应保持可用,实际状态 %d", gotCode.Status)
				}
			} else {
				if gotUser.Quota != codeQuota {
					t.Fatalf("额度应为 %d,实际 %d", codeQuota, gotUser.Quota)
				}
				if gotCode.Status != config.RedemptionCodeStatusUsed {
					t.Fatalf("兑换码应为 used,实际状态 %d", gotCode.Status)
				}
			}
		})
	}
}

// TestRedeemEmptyScopeTreatedAsAny 断言存量数据(scope 为空)不受限制。
func TestRedeemEmptyScopeTreatedAsAny(t *testing.T) {
	setupRedeemTestDB(t)
	user := &User{Username: "legacy-scope", Password: "x", AccessToken: "at-legacy", AffCode: "aff-legacy", Type: config.UserTypeOrgShadow}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	code := &Redemption{Key: "LEGACYSCOPECODE00000000000000001", Status: config.RedemptionCodeStatusEnabled, Quota: 200}
	if err := DB.Create(code).Error; err != nil {
		t.Fatalf("创建兑换码失败: %v", err)
	}
	if err := DB.Model(&Redemption{}).Where("id = ?", code.Id).Update("scope", "").Error; err != nil {
		t.Fatalf("重置 scope 失败: %v", err)
	}
	if _, err := Redeem(code.Key, user.Id, "127.0.0.1"); err != nil {
		t.Fatalf("空 scope 应视为不限,实际: %v", err)
	}
}

func setupRedemptionTestDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&Redemption{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	t.Cleanup(func() { DB = oldDB })
}

func countRedemptions(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := DB.Model(&Redemption{}).Count(&n).Error; err != nil {
		t.Fatalf("统计兑换码失败: %v", err)
	}
	return n
}

// 全部成功路径:落库数量与传入数量一致。
func TestBatchInsertRedemptionsAllSucceed(t *testing.T) {
	setupRedemptionTestDB(t)
	redemptions := []*Redemption{
		{UserId: 1, Name: "batch", Key: "k1", Quota: 100, CreatedTime: 1},
		{UserId: 1, Name: "batch", Key: "k2", Quota: 100, CreatedTime: 1},
		{UserId: 1, Name: "batch", Key: "k3", Quota: 100, CreatedTime: 1},
	}
	if err := BatchInsertRedemptions(redemptions); err != nil {
		t.Fatalf("全部成功路径不应报错: %v", err)
	}
	if got := countRedemptions(t); got != 3 {
		t.Fatalf("落库数量应为 3,实际 %d", got)
	}
}

// 中途失败(重复 key 触发唯一约束)时整体回滚,不留任何部分码。
func TestBatchInsertRedemptionsRollbackOnFailure(t *testing.T) {
	setupRedemptionTestDB(t)
	redemptions := []*Redemption{
		{UserId: 1, Name: "batch", Key: "dup", Quota: 100, CreatedTime: 1},
		{UserId: 1, Name: "batch", Key: "ok", Quota: 100, CreatedTime: 1},
		{UserId: 1, Name: "batch", Key: "dup", Quota: 100, CreatedTime: 1},
	}
	if err := BatchInsertRedemptions(redemptions); err == nil {
		t.Fatal("含重复 key 应返回错误")
	}
	if got := countRedemptions(t); got != 0 {
		t.Fatalf("失败后应整体回滚,落库数量应为 0,实际 %d", got)
	}
}

// 空切片视为成功且不落库。
func TestBatchInsertRedemptionsEmpty(t *testing.T) {
	setupRedemptionTestDB(t)
	if err := BatchInsertRedemptions(nil); err != nil {
		t.Fatalf("空切片不应报错: %v", err)
	}
	if got := countRedemptions(t); got != 0 {
		t.Fatalf("空切片不应落库,实际 %d", got)
	}
}
