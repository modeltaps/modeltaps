package relay_util

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// realtimeSessionQuota is a realtime session after PreQuotaConsumption reserved 600 in the database.
func realtimeSessionQuota(userId, tokenId int) *Quota {
	return &Quota{
		userId: userId, tokenId: tokenId, modelName: "gpt-realtime",
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, outputRatio: 1, groupRatio: 1,
		preConsumedQuota: 600, HandelStatus: true,
		startTime: time.Now(),
	}
}

// TestUpdateRealtimeSessionQuotaExtendsReservation checks SEC-75 without Redis: the reservation
// grows with the usage of the session and the session is stopped once the balance cannot cover it.
func TestUpdateRealtimeSessionQuotaExtendsReservation(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("Redis enabled in test env")
	}
	setupQuotaTestDB(t)
	// Balances after the entry reservation of 600.
	tokenId := seedUserAndToken(t, 1, 1000, 100000)
	q := realtimeSessionQuota(1, tokenId)
	usage := &types.UsageEvent{}

	// 500 used: still within the reservation, nothing more is taken.
	if err := q.UpdateRealtimeSessionQuota(usage, &types.UsageEvent{InputTokens: 300, OutputTokens: 200, TotalTokens: 500}); err != nil {
		t.Fatalf("usage within the reservation: %v", err)
	}
	if got := getUserQuota(t, 1); got != 1000 || q.preConsumedQuota != 600 {
		t.Fatalf("no extension expected: balance=%d reserved=%d", got, q.preConsumedQuota)
	}

	// 1200 used: the 600 above the reservation is reserved from the key and the balance.
	if err := q.UpdateRealtimeSessionQuota(usage, &types.UsageEvent{InputTokens: 400, OutputTokens: 300, TotalTokens: 700}); err != nil {
		t.Fatalf("extension within the balance: %v", err)
	}
	if got := getUserQuota(t, 1); got != 400 || q.preConsumedQuota != 1200 {
		t.Fatalf("extension of 600 expected: balance=%d reserved=%d", got, q.preConsumedQuota)
	}
	if tok := getToken(t, tokenId); tok.RemainQuota != 99400 {
		t.Fatalf("key should be charged the extension: remain=%d", tok.RemainQuota)
	}

	// 2200 used: the balance of 400 cannot cover 1000 more, so the session must stop.
	err := q.UpdateRealtimeSessionQuota(usage, &types.UsageEvent{InputTokens: 500, OutputTokens: 500, TotalTokens: 1000})
	if err == nil || !strings.Contains(err.Error(), "pre_consume_token_quota_failed") {
		t.Fatalf("exhausted balance should stop the session, got %v", err)
	}
	if got := getUserQuota(t, 1); got != 400 || q.preConsumedQuota != 1200 {
		t.Fatalf("a refused extension must take nothing: balance=%d reserved=%d", got, q.preConsumedQuota)
	}
}

// TestUpdateRealtimeSessionQuotaSettlesAgainstExtension checks that settlement reconciles against the
// grown reservation, so the extension is neither charged twice nor refunded.
func TestUpdateRealtimeSessionQuotaSettlesAgainstExtension(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("Redis enabled in test env")
	}
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 1, 1000, 100000)
	channel := &model.Channel{Name: "realtime-test-channel"}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatalf("create channel: %v", err)
	}
	q := realtimeSessionQuota(1, tokenId)
	q.channelId = channel.Id
	usage := &types.UsageEvent{}
	if err := q.UpdateRealtimeSessionQuota(usage, &types.UsageEvent{InputTokens: 500, OutputTokens: 400, TotalTokens: 900}); err != nil {
		t.Fatalf("extension: %v", err)
	}
	if err := q.completedQuotaConsumption(usage.ToChatUsage(), "tok", false, "127.0.0.1", context.Background(), nil); err != nil {
		t.Fatalf("settle: %v", err)
	}
	// 600 reserved at entry + 300 extension = 900 used: the balance ends at 1000 - 300.
	if got := getUserQuota(t, 1); got != 700 {
		t.Fatalf("balance after settlement = %d, want 700", got)
	}
	// After settlement a late usage event must not reserve anything.
	if err := q.UpdateRealtimeSessionQuota(usage, &types.UsageEvent{InputTokens: 500, OutputTokens: 500, TotalTokens: 1000}); err == nil {
		t.Fatal("a usage event after settlement should be refused")
	}
	if got := getUserQuota(t, 1); got != 700 {
		t.Fatalf("late usage event changed the balance: %d", got)
	}
}

// TestUpdateRealtimeSessionQuotaTeamBudget checks that a session of an organization key is stopped
// when the team budget cannot cover the usage, and that the pool is not charged.
func TestUpdateRealtimeSessionQuotaTeamBudget(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 900, 100000, 100000)
	var reserved []int
	orig := reserveOrganizationBudgetFn
	t.Cleanup(func() { reserveOrganizationBudgetFn = orig })
	reserveOrganizationBudgetFn = func(orgId, amount int) (bool, error) {
		reserved = append(reserved, amount)
		return false, nil
	}

	q := realtimeSessionQuota(900, tokenId)
	q.createdBy = 5
	q.orgGuardrail = &model.OrgMemberGuardrail{
		OrganizationId: 1,
		OrgBudget:      &model.QuotaResetSetting{Period: "daily", Limit: 1000},
	}
	err := q.UpdateRealtimeSessionQuota(&types.UsageEvent{}, &types.UsageEvent{InputTokens: 800, OutputTokens: 200, TotalTokens: 1000})
	if err == nil || !strings.Contains(err.Error(), "org_budget_exceeded") {
		t.Fatalf("team budget exceeded should stop the session, got %v", err)
	}
	if len(reserved) != 1 || reserved[0] != 400 {
		t.Fatalf("team budget should be asked for the 400 above the reservation, got %v", reserved)
	}
	if got := getUserQuota(t, 900); got != 100000 || q.preConsumedQuota != 600 {
		t.Fatalf("a refused extension must take nothing: balance=%d reserved=%d", got, q.preConsumedQuota)
	}
}

// TestUpdateRealtimeSessionQuotaWithoutReservation keeps the Redis live counter for sessions that hold
// no database reservation.
func TestUpdateRealtimeSessionQuotaWithoutReservation(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("Redis enabled in test env")
	}
	q := &Quota{price: model.Price{Type: model.TokensPriceType, Input: 1, Output: 1}, inputRatio: 1, outputRatio: 1, groupRatio: 1}
	usage := &types.UsageEvent{}
	if err := q.UpdateRealtimeSessionQuota(usage, &types.UsageEvent{InputTokens: 10, OutputTokens: 5, TotalTokens: 15}); err != nil {
		t.Fatalf("no reservation: %v", err)
	}
	if usage.TotalTokens != 15 || q.preConsumedQuota != 0 {
		t.Fatalf("usage should be merged without a reservation: usage=%+v reserved=%d", usage, q.preConsumedQuota)
	}
}
