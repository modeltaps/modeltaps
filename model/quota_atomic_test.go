package model

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// runConcurrentDecrease 并发调用 fn(decAmount) goroutines 次,统计成功/拒绝/异常。
// 仅当返回 wantErr(余额不足守卫)时计入 rejected;其余非 nil 视为异常,测试失败。
func runConcurrentDecrease(t *testing.T, goroutines int, wantErr error, fn func() error) (success, rejected int64) {
	t.Helper()
	var unexpected atomic.Value
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			switch err := fn(); {
			case err == nil:
				atomic.AddInt64(&success, 1)
			case errors.Is(err, wantErr):
				atomic.AddInt64(&rejected, 1)
			default:
				unexpected.Store(err)
			}
		}()
	}
	wg.Wait()
	if v := unexpected.Load(); v != nil {
		t.Fatalf("并发扣减出现异常错误: %v", v.(error))
	}
	return success, rejected
}

// TestPreDecreaseUserQuotaAtomic 验证用户额度预扣的 check-then-act 原子守卫:
// 高并发下扣减总额不超过初始余额、余额不为负、超额请求被明确拒绝。
func TestPreDecreaseUserQuotaAtomic(t *testing.T) {
	t.Run("单位扣减_并发竞态", func(t *testing.T) {
		setupOrgBillingTestDB(t)
		user := createOrgTestUser(t, "quota-atomic-unit")
		const initial = 100
		if err := DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", initial).Error; err != nil {
			t.Fatalf("初始化额度失败: %v", err)
		}

		const goroutines = 200
		success, rejected := runConcurrentDecrease(t, goroutines, ErrUserQuotaNotEnough, func() error {
			return PreDecreaseUserQuota(user.Id, 1)
		})

		final, err := GetUserQuota(user.Id)
		if err != nil {
			t.Fatalf("读取额度失败: %v", err)
		}
		if final < 0 {
			t.Fatalf("额度透支为负: %d", final)
		}
		if success != initial {
			t.Fatalf("成功扣减次数应为 %d,实际 %d", initial, success)
		}
		if final != 0 {
			t.Fatalf("初始 %d 全部扣完后余额应为 0,实际 %d", initial, final)
		}
		if rejected != goroutines-initial {
			t.Fatalf("被拒绝次数应为 %d,实际 %d", goroutines-initial, rejected)
		}
	})

	t.Run("批量扣减_超额拒绝", func(t *testing.T) {
		setupOrgBillingTestDB(t)
		user := createOrgTestUser(t, "quota-atomic-batch")
		const initial = 50
		const dec = 10
		if err := DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", initial).Error; err != nil {
			t.Fatalf("初始化额度失败: %v", err)
		}

		const goroutines = 20
		success, rejected := runConcurrentDecrease(t, goroutines, ErrUserQuotaNotEnough, func() error {
			return PreDecreaseUserQuota(user.Id, dec)
		})

		final, err := GetUserQuota(user.Id)
		if err != nil {
			t.Fatalf("读取额度失败: %v", err)
		}
		if final < 0 {
			t.Fatalf("额度透支为负: %d", final)
		}
		if want := int64(initial / dec); success != want {
			t.Fatalf("成功扣减次数应为 %d,实际 %d", want, success)
		}
		if final != initial-int(success)*dec || final != 0 {
			t.Fatalf("结余异常: final=%d success=%d", final, success)
		}
		if success+rejected != goroutines {
			t.Fatalf("成功+拒绝应等于总数 %d,实际 %d", goroutines, success+rejected)
		}
	})
}

// TestPreDecreaseTokenQuotaAtomic 验证令牌额度预扣的原子守卫,语义同用户额度。
func TestPreDecreaseTokenQuotaAtomic(t *testing.T) {
	setupOrgBillingTestDB(t)
	user := createOrgTestUser(t, "token-atomic-user")
	const initial = 100
	token := &Token{UserId: user.Id, Name: "atomic-tok", Status: 1, RemainQuota: initial}
	if err := DB.Create(token).Error; err != nil {
		t.Fatalf("创建令牌失败: %v", err)
	}
	if err := DB.Model(&Token{}).Where("id = ?", token.Id).
		Updates(map[string]interface{}{"remain_quota": initial, "used_quota": 0}).Error; err != nil {
		t.Fatalf("初始化令牌额度失败: %v", err)
	}

	const goroutines = 200
	success, rejected := runConcurrentDecrease(t, goroutines, ErrTokenQuotaNotEnough, func() error {
		return PreDecreaseTokenQuota(token.Id, 1)
	})

	var got Token
	if err := DB.First(&got, "id = ?", token.Id).Error; err != nil {
		t.Fatalf("读取令牌失败: %v", err)
	}
	if got.RemainQuota < 0 {
		t.Fatalf("令牌额度透支为负: %d", got.RemainQuota)
	}
	if success != initial {
		t.Fatalf("成功扣减次数应为 %d,实际 %d", initial, success)
	}
	if got.RemainQuota != 0 || got.UsedQuota != initial {
		t.Fatalf("扣减后 remain/used 异常: remain=%d used=%d", got.RemainQuota, got.UsedQuota)
	}
	if rejected != goroutines-initial {
		t.Fatalf("被拒绝次数应为 %d,实际 %d", goroutines-initial, rejected)
	}
}
