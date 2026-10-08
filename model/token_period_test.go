package model

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

func utc(y int, m time.Month, d, hh, mm, ss int) time.Time {
	return time.Date(y, m, d, hh, mm, ss, 0, time.UTC)
}

func TestCurrentPeriodStart(t *testing.T) {
	tests := []struct {
		name   string
		period string
		now    time.Time
		want   time.Time
	}{
		// daily: UTC 0 点边界
		{"daily 边界前一秒归当日", TokenQuotaResetPeriodDaily, utc(2026, 6, 11, 23, 59, 59), utc(2026, 6, 11, 0, 0, 0)},
		{"daily 0 点整开启新周期", TokenQuotaResetPeriodDaily, utc(2026, 6, 12, 0, 0, 0), utc(2026, 6, 12, 0, 0, 0)},
		{"daily 非 UTC 时区输入按 UTC 计算", TokenQuotaResetPeriodDaily, time.Date(2026, 6, 12, 7, 0, 0, 0, time.FixedZone("UTC+8", 8*3600)), utc(2026, 6, 11, 0, 0, 0)},
		// weekly: 周一为起点（2026-06-08 是周一）
		{"weekly 周一当天", TokenQuotaResetPeriodWeekly, utc(2026, 6, 8, 0, 0, 0), utc(2026, 6, 8, 0, 0, 0)},
		{"weekly 周日归本周一", TokenQuotaResetPeriodWeekly, utc(2026, 6, 14, 23, 59, 59), utc(2026, 6, 8, 0, 0, 0)},
		{"weekly 下周一 0 点开启新周期", TokenQuotaResetPeriodWeekly, utc(2026, 6, 15, 0, 0, 0), utc(2026, 6, 15, 0, 0, 0)},
		{"weekly 跨月周（周一在上月）", TokenQuotaResetPeriodWeekly, utc(2026, 6, 3, 12, 0, 0), utc(2026, 6, 1, 0, 0, 0)},
		{"weekly 跨年周（2026-01-01 周四归 2025-12-29 周一）", TokenQuotaResetPeriodWeekly, utc(2026, 1, 1, 0, 0, 0), utc(2025, 12, 29, 0, 0, 0)},
		// monthly: 每月 1 号
		{"monthly 月末最后一秒归当月", TokenQuotaResetPeriodMonthly, utc(2026, 6, 30, 23, 59, 59), utc(2026, 6, 1, 0, 0, 0)},
		{"monthly 1 号 0 点开启新周期", TokenQuotaResetPeriodMonthly, utc(2026, 7, 1, 0, 0, 0), utc(2026, 7, 1, 0, 0, 0)},
		{"monthly 跨年", TokenQuotaResetPeriodMonthly, utc(2026, 1, 15, 8, 0, 0), utc(2026, 1, 1, 0, 0, 0)},
		// invalid
		{"未知 period 返回零值", "yearly", utc(2026, 6, 11, 0, 0, 0), time.Time{}},
		{"空 period 返回零值", "", utc(2026, 6, 11, 0, 0, 0), time.Time{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := currentPeriodStart(tt.period, tt.now)
			if !got.Equal(tt.want) {
				t.Errorf("currentPeriodStart(%q, %v) = %v, want %v", tt.period, tt.now, got, tt.want)
			}
		})
	}
}

// TestCurrentPeriodStartTimezoneAndWeekStart 覆盖 TK8a:按配置时区/周起始日计算周期起点，
// 以及无效时区回退 UTC。测试结束恢复默认配置，避免污染其它依赖默认值的用例。
func TestCurrentPeriodStartTimezoneAndWeekStart(t *testing.T) {
	origTZ, origWS := config.QuotaResetTimezone, config.QuotaResetWeekStart
	t.Cleanup(func() {
		config.QuotaResetTimezone, config.QuotaResetWeekStart = origTZ, origWS
	})

	sh, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("加载时区失败: %v", err)
	}

	// daily:UTC 2026-06-11 20:00 = 上海 2026-06-12 04:00,日边界应为上海当日 0 点
	config.QuotaResetTimezone, config.QuotaResetWeekStart = "Asia/Shanghai", "monday"
	gotDaily := currentPeriodStart(TokenQuotaResetPeriodDaily, utc(2026, 6, 11, 20, 0, 0))
	wantDaily := time.Date(2026, 6, 12, 0, 0, 0, 0, sh)
	if !gotDaily.Equal(wantDaily) {
		t.Errorf("daily 上海时区 = %v, want %v", gotDaily, wantDaily)
	}

	// weekly + sunday 周起点:2026-06-11 是周四,周日起点应为上海 2026-06-07
	config.QuotaResetWeekStart = "sunday"
	gotWeekly := currentPeriodStart(TokenQuotaResetPeriodWeekly, time.Date(2026, 6, 11, 12, 0, 0, 0, sh))
	wantWeekly := time.Date(2026, 6, 7, 0, 0, 0, 0, sh)
	if !gotWeekly.Equal(wantWeekly) {
		t.Errorf("weekly sunday 起点 = %v, want %v", gotWeekly, wantWeekly)
	}

	// 无效时区回退 UTC
	config.QuotaResetTimezone, config.QuotaResetWeekStart = "Not/AZone", "monday"
	gotFallback := currentPeriodStart(TokenQuotaResetPeriodDaily, utc(2026, 6, 11, 20, 0, 0))
	if !gotFallback.Equal(utc(2026, 6, 11, 0, 0, 0)) {
		t.Errorf("无效时区应回退 UTC = %v, want %v", gotFallback, utc(2026, 6, 11, 0, 0, 0))
	}
}

// 周期计数现为 Token 专用列，校验函数按值传入 periodUsed/periodStart（SEC-5）。
func TestTokenPeriodQuotaExceeded(t *testing.T) {
	now := utc(2026, 6, 11, 12, 0, 0)
	todayStart := utc(2026, 6, 11, 0, 0, 0).Unix()
	yesterdayStart := utc(2026, 6, 10, 0, 0, 0).Unix()

	tests := []struct {
		name        string
		setting     TokenSetting
		periodUsed  int
		periodStart int64
		want        bool
	}{
		{"无 quota_reset 永不超限", TokenSetting{}, 999999, 0, false},
		{"limit<=0 不生效", TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 0}}, 100, todayStart, false},
		{"未知 period 不生效", TokenSetting{QuotaReset: &QuotaResetSetting{Period: "yearly", Limit: 10}}, 100, todayStart, false},
		{"本周期内未达限额", TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 100}}, 99, todayStart, false},
		{"本周期内达到限额", TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 100}}, 100, todayStart, true},
		{"已跨期视为清零（懒重置）", TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 100}}, 100, yesterdayStart, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenPeriodQuotaExceeded(&tt.setting, tt.periodUsed, tt.periodStart, now); got != tt.want {
				t.Errorf("tokenPeriodQuotaExceeded() = %v, want %v", got, tt.want)
			}
		})
	}
}

// PeriodConfigChanged 仅判周期(period)是否变化；计数保留/清零由列层面处理（SEC-5）。
func TestPeriodConfigChanged(t *testing.T) {
	daily := &QuotaResetSetting{Period: "daily", Limit: 100}
	weekly := &QuotaResetSetting{Period: "weekly", Limit: 100}
	cases := []struct {
		name string
		newS TokenSetting
		oldS TokenSetting
		want bool
	}{
		{"周期不变", TokenSetting{QuotaReset: daily}, TokenSetting{QuotaReset: daily}, false},
		{"仅 limit 变化不算周期变化", TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 200}}, TokenSetting{QuotaReset: daily}, false},
		{"周期变化", TokenSetting{QuotaReset: weekly}, TokenSetting{QuotaReset: daily}, true},
		{"移除 quota_reset 算变化", TokenSetting{}, TokenSetting{QuotaReset: daily}, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := PeriodConfigChanged(&tt.newS, &tt.oldS); got != tt.want {
				t.Errorf("PeriodConfigChanged() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTokenInPeriodMode(t *testing.T) {
	cases := []struct {
		name    string
		setting TokenSetting
		want    bool
	}{
		{"无 quota_reset 非 period", TokenSetting{}, false},
		{"limit<=0 非 period", TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 0}}, false},
		{"limit>0 为 period 模式", TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 100}}, true},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tokenInPeriodMode(&tt.setting); got != tt.want {
				t.Errorf("tokenInPeriodMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestValidateUserTokenPeriodMode 验证 period 模式下 remain_quota 耗尽不再使 Key 失效，
// 仅受周期上限约束；非 period 模式 remain_quota 耗尽仍判 ErrTokenQuotaExhausted。
func TestValidateUserTokenPeriodMode(t *testing.T) {
	setupOrgTestDB(t)
	user := createOrgTestUser(t, "period-validate-user")
	todayStart := currentPeriodStart(TokenQuotaResetPeriodDaily, time.Now()).Unix()

	mkKey := func(name string, remain int, setting TokenSetting, periodUsed int, periodStart int64) string {
		tk := &Token{UserId: user.Id, Name: name, Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: remain, PeriodUsed: periodUsed, PeriodStart: periodStart}
		tk.Setting.Set(setting)
		if err := DB.Create(tk).Error; err != nil {
			t.Fatalf("创建令牌失败: %v", err)
		}
		got, err := GetTokenById(tk.Id)
		if err != nil {
			t.Fatalf("回读令牌失败: %v", err)
		}
		return got.Key
	}

	t.Run("period 模式 remain=0 周期未超放行", func(t *testing.T) {
		key := mkKey("p-ok", 0, TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 100}}, 0, todayStart)
		if _, err := ValidateUserToken(key); err != nil {
			t.Fatalf("period 模式 remain=0 应放行，实际: %v", err)
		}
	})
	t.Run("period 模式周期已超拒绝", func(t *testing.T) {
		key := mkKey("p-exceed", 0, TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 100}}, 100, todayStart)
		if _, err := ValidateUserToken(key); !errors.Is(err, ErrTokenPeriodQuotaExhausted) {
			t.Fatalf("周期已超应返回 ErrTokenPeriodQuotaExhausted，实际: %v", err)
		}
	})
	t.Run("非 period 模式 remain=0 仍判耗尽", func(t *testing.T) {
		key := mkKey("np-exhausted", 0, TokenSetting{}, 0, 0)
		if _, err := ValidateUserToken(key); !errors.Is(err, ErrTokenQuotaExhausted) {
			t.Fatalf("非 period remain=0 应返回 ErrTokenQuotaExhausted，实际: %v", err)
		}
	})
}

// TestPreConsumeTokenQuotaPeriodMode 验证 period 模式预扣跳过 remain_quota 约束（仅累计 used/period 用量），
// 周期已超拒绝且不动额度；非 period 模式 remain_quota 约束行为不变。
func TestPreConsumeTokenQuotaPeriodMode(t *testing.T) {
	setupOrgBillingTestDB(t)
	user := createOrgTestUser(t, "period-preconsume-user")
	if err := DB.Model(&User{}).Where("id = ?", user.Id).Update("quota", 1000000).Error; err != nil {
		t.Fatalf("初始化用户额度失败: %v", err)
	}
	todayStart := currentPeriodStart(TokenQuotaResetPeriodDaily, time.Now()).Unix()

	mkToken := func(name string, remain int, setting TokenSetting, periodUsed int, periodStart int64) *Token {
		tk := &Token{UserId: user.Id, Name: name, Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: remain, PeriodUsed: periodUsed, PeriodStart: periodStart}
		tk.Setting.Set(setting)
		if err := DB.Create(tk).Error; err != nil {
			t.Fatalf("创建令牌失败: %v", err)
		}
		return tk
	}

	t.Run("period 模式 remain=0 放行且累计用量", func(t *testing.T) {
		tk := mkToken("p-ok", 0, TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 100}}, 0, todayStart)
		if err := PreConsumeTokenQuota(tk.Id, 50); err != nil {
			t.Fatalf("period 模式 remain=0 预扣应放行，实际: %v", err)
		}
		var got Token
		if err := DB.First(&got, "id = ?", tk.Id).Error; err != nil {
			t.Fatalf("回读令牌失败: %v", err)
		}
		if got.RemainQuota != 0 {
			t.Fatalf("period 模式不应扣减 remain_quota，实际 remain=%d", got.RemainQuota)
		}
		if got.UsedQuota != 50 {
			t.Fatalf("used_quota 应累计 50，实际 %d", got.UsedQuota)
		}
		if got.PeriodUsed != 50 {
			t.Fatalf("period_used 应累计 50，实际 %d", got.PeriodUsed)
		}
	})

	t.Run("period 模式周期已超拒绝且不动额度", func(t *testing.T) {
		tk := mkToken("p-exceed", 0, TokenSetting{QuotaReset: &QuotaResetSetting{Period: "daily", Limit: 100}}, 100, todayStart)
		before, _ := GetUserQuota(user.Id)
		if err := PreConsumeTokenQuota(tk.Id, 50); !errors.Is(err, ErrTokenPeriodQuotaExhausted) {
			t.Fatalf("周期已超应返回 ErrTokenPeriodQuotaExhausted，实际: %v", err)
		}
		if after, _ := GetUserQuota(user.Id); before != after {
			t.Fatalf("周期已超拒绝不应扣用户额度: before=%d after=%d", before, after)
		}
	})

	t.Run("非 period 模式额度不足仍拒绝", func(t *testing.T) {
		tk := mkToken("np-low", 10, TokenSetting{}, 0, 0)
		if err := PreConsumeTokenQuota(tk.Id, 50); err == nil {
			t.Fatal("非 period 额度不足应拒绝")
		}
	})

	t.Run("非 period 模式额度充足正常扣减", func(t *testing.T) {
		tk := mkToken("np-ok", 100, TokenSetting{}, 0, 0)
		if err := PreConsumeTokenQuota(tk.Id, 50); err != nil {
			t.Fatalf("非 period 充足应放行，实际: %v", err)
		}
		var got Token
		if err := DB.First(&got, "id = ?", tk.Id).Error; err != nil {
			t.Fatalf("回读令牌失败: %v", err)
		}
		if got.RemainQuota != 50 || got.UsedQuota != 50 {
			t.Fatalf("非 period 应扣 remain_quota: remain=%d used=%d", got.RemainQuota, got.UsedQuota)
		}
	})
}

// TestAccrueTokenPeriodUsedConcurrent 同一令牌周期计数在并发累计下必须无丢失更新(金钱正确性,SEC-5)。
// accrueTokenPeriodUsed 用单条原子 UPDATE(CASE 懒重置+自增+钳制)替代读-改-写;本测试守护该不变量。
func TestAccrueTokenPeriodUsedConcurrent(t *testing.T) {
	setupOrgBillingTestDB(t)
	user := createOrgTestUser(t, "period-accrue-conc")
	monthStart := currentPeriodStart(TokenQuotaResetPeriodMonthly, time.Now()).Unix()
	tk := &Token{UserId: user.Id, Name: "p-conc", Status: config.TokenStatusEnabled, ExpiredTime: -1, RemainQuota: 0, PeriodStart: monthStart}
	tk.Setting.Set(TokenSetting{QuotaReset: &QuotaResetSetting{Period: "monthly", Limit: 1_000_000}})
	if err := DB.Create(tk).Error; err != nil {
		t.Fatalf("创建令牌失败: %v", err)
	}

	const workers = 50
	const delta = 7
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := AccrueTokenPeriodUsed(tk.Id, delta); err != nil {
				t.Errorf("并发累计失败: %v", err)
			}
		}()
	}
	wg.Wait()

	var got Token
	if err := DB.First(&got, "id = ?", tk.Id).Error; err != nil {
		t.Fatalf("回读令牌失败: %v", err)
	}
	if got.PeriodUsed != workers*delta {
		t.Fatalf("并发累计丢失更新: period_used = %d, want %d", got.PeriodUsed, workers*delta)
	}
}
