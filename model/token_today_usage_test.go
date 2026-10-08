package model

import (
	"testing"
	"time"
)

// TestSumUserUsageByTokenSince 验证「当日已用额度」聚合:跨零点只计入零点(含)之后的消费日志,
// 并按 token_name 正确分组;充值日志、无关用户日志均不计入,空名令牌归入空名桶。
func TestSumUserUsageByTokenSince(t *testing.T) {
	setupOrgTestDB(t)

	user := &User{Username: "today-usage-user", Password: "x", AccessToken: "at-today", AffCode: "aff-today"}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}

	// 固定零点边界(注入,避免依赖系统当前时间)
	midnight := utc(2026, time.June, 14, 0, 0, 0).Unix()

	logs := []struct {
		tokenName string
		createdAt int64
		quota     int
	}{
		{"tok-a", midnight - 1, 1000},   // 零点前一秒:不计入
		{"tok-a", midnight, 100},        // 恰为零点:计入(>= 边界)
		{"tok-a", midnight + 3600, 50},  // 零点后:计入
		{"tok-b", midnight + 7200, 200}, // 零点后:计入
		{"", midnight + 10, 7},          // 空名令牌:归入空名桶
	}
	for _, l := range logs {
		log := &Log{UserId: user.Id, Type: LogTypeConsume, TokenName: l.tokenName, CreatedAt: l.createdAt, Quota: l.quota}
		if err := DB.Create(log).Error; err != nil {
			t.Fatalf("插入消费日志失败: %v", err)
		}
	}
	// 当日充值日志与无关用户当日消费日志,均不应进入当日消费聚合
	if err := DB.Create(&Log{UserId: user.Id, Type: LogTypeTopup, TokenName: "tok-a", CreatedAt: midnight + 5, Quota: 9999}).Error; err != nil {
		t.Fatalf("插入充值日志失败: %v", err)
	}
	if err := DB.Create(&Log{UserId: 999, Type: LogTypeConsume, TokenName: "tok-a", CreatedAt: midnight + 5, Quota: 8888}).Error; err != nil {
		t.Fatalf("插入无关用户日志失败: %v", err)
	}

	usage, err := sumUserUsageByTokenSince(user.Id, midnight)
	if err != nil {
		t.Fatalf("当日聚合失败: %v", err)
	}

	if len(usage) != 3 {
		t.Fatalf("应有 3 个 token 分组,实际 %d: %+v", len(usage), usage)
	}
	if got := usage["tok-a"]; got != 150 {
		t.Fatalf("tok-a 当日应为 150(零点前 1000 不计入),实际 %d", got)
	}
	if got := usage["tok-b"]; got != 200 {
		t.Fatalf("tok-b 当日应为 200,实际 %d", got)
	}
	if got := usage[""]; got != 7 {
		t.Fatalf("空名令牌当日应为 7,实际 %d", got)
	}

	// 边界翻转:若零点推后到 midnight 之后,窗口外的日志不再计入
	usageNext, err := sumUserUsageByTokenSince(user.Id, midnight+8000)
	if err != nil {
		t.Fatalf("翻转后聚合失败: %v", err)
	}
	if len(usageNext) != 0 {
		t.Fatalf("零点推后至窗口外应无任何当日消费,实际 %+v", usageNext)
	}
}
