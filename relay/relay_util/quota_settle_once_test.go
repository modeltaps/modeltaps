package relay_util

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// SEC-55: one reservation is refunded or settled exactly once, however many times Undo and
// Consume are called on the same Quota (retry loops, deferred cleanup).

const (
	settleUserQuota  = 10000
	settleTokenQuota = 5000
)

func newSettleTestQuota(t *testing.T) (*Quota, *gin.Context) {
	t.Helper()
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 1, settleUserQuota, settleTokenQuota)
	channel := &model.Channel{Name: "settle-test-channel"}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)

	q := &Quota{
		userId: 1, tokenId: tokenId, channelId: channel.Id, promptTokens: 100,
		modelName:  "settle-test-model",
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, outputRatio: 1, groupRatio: 1,
	}
	// pre = 100 prompt + 0 output (PreConsumedOutputTokens = 0) + PreConsumedQuota (500) = 600
	orig := config.PreConsumedOutputTokens
	config.PreConsumedOutputTokens = 0
	t.Cleanup(func() { config.PreConsumedOutputTokens = orig })
	if errResp := q.PreQuotaConsumption(); errResp != nil {
		t.Fatalf("pre-consumption: %+v", errResp)
	}
	if q.preConsumedQuota != 600 || !q.HandelStatus {
		t.Fatalf("pre=%d handel=%v, want a 600 reservation", q.preConsumedQuota, q.HandelStatus)
	}
	return q, c
}

func assertCharged(t *testing.T, q *Quota, charged int) {
	t.Helper()
	if got := getUserQuota(t, q.userId); got != settleUserQuota-charged {
		t.Fatalf("user quota = %d, want %d (charged %d)", got, settleUserQuota-charged, charged)
	}
	if tok := getToken(t, q.tokenId); tok.RemainQuota != settleTokenQuota-charged {
		t.Fatalf("token remain = %d, want %d (charged %d)", tok.RemainQuota, settleTokenQuota-charged, charged)
	}
}

func settle(q *Quota, usage *types.Usage) {
	q.ConsumeWithSnapshot(ConsumeSnapshot{Ctx: context.Background(), StartTime: time.Now()}, usage, false)
}

// 200 tokens at ratio 1.
var settleUsage = &types.Usage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}

func TestUndoRefundsOnce(t *testing.T) {
	q, c := newSettleTestQuota(t)
	assertCharged(t, q, 600)

	for i := 0; i < 3; i++ {
		q.Undo(c)
	}
	assertCharged(t, q, 0)
	if q.HandelStatus || q.preConsumedQuota != 0 {
		t.Fatalf("reservation should be cleared: handel=%v pre=%d", q.HandelStatus, q.preConsumedQuota)
	}
}

// The old task retry loop: Undo after each failed attempt, then settle on the attempt that
// succeeded. The request must be charged in full, not settled against a refunded reservation.
func TestUndoUndoConsumeChargesFullUsage(t *testing.T) {
	q, c := newSettleTestQuota(t)
	q.Undo(c)
	q.Undo(c)
	settle(q, settleUsage)

	assertCharged(t, q, 200)
	if q.ChargedQuota() != 200 {
		t.Fatalf("ChargedQuota = %d, want 200", q.ChargedQuota())
	}
}

func TestUndoAfterConsumeIsNoop(t *testing.T) {
	q, c := newSettleTestQuota(t)
	settle(q, settleUsage)
	q.Undo(c)
	q.Undo(c)

	assertCharged(t, q, 200)
}

func TestConsumeTwiceChargesOnce(t *testing.T) {
	q, _ := newSettleTestQuota(t)
	settle(q, settleUsage)
	settle(q, settleUsage)

	assertCharged(t, q, 200)
	var logs int64
	if err := model.DB.Model(&model.Log{}).Where("type = ?", model.LogTypeConsume).Count(&logs).Error; err != nil {
		t.Fatal(err)
	}
	if logs != 1 {
		t.Fatalf("consume logs = %d, want 1", logs)
	}
}

// The MJ relax fallback: the fast reservation is refunded, a relax reservation is taken, the
// relax submission fails and the deferred settlement refunds it. Neither is refunded twice.
func TestRelaxFallbackRefundsEachReservationOnce(t *testing.T) {
	fast, c := newSettleTestQuota(t)
	fast.Undo(c) // switching to relax

	relax := &Quota{
		userId: fast.userId, tokenId: fast.tokenId, channelId: fast.channelId, promptTokens: 100,
		modelName: fast.modelName, price: fast.price, inputRatio: 1, outputRatio: 1, groupRatio: 1,
	}
	if errResp := relax.PreQuotaConsumption(); errResp != nil {
		t.Fatalf("relax pre-consumption: %+v", errResp)
	}
	assertCharged(t, fast, 600)

	relax.Undo(c) // relax submission failed
	relax.Undo(c) // deferred settlement
	fast.Undo(c)  // a stale reference to the fast reservation
	assertCharged(t, fast, 0)
}

func TestRequestedMaxOutputTokensTakesLargest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := map[string]int{
		`{"max_completion_tokens":1,"max_tokens":64000}`:                   64000,
		`{"max_tokens":1,"max_completion_tokens":64000}`:                   64000,
		`{"max_completion_tokens":1}`:                                      1,
		`{"max_completion_tokens":0,"max_tokens":300}`:                     300,
		`{"max_output_tokens":5,"generationConfig":{"maxOutputTokens":9}}`: 9,
		`{"max_tokens":-5}`:                                                0,
		`{"max_tokens":99999999999}`:                                       2147483647,
	}
	for body, want := range cases {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Set(config.GinRequestBodyKey, []byte(body))
		if got := requestedMaxOutputTokens(c); got != want {
			t.Fatalf("%s: got %d want %d", body, got, want)
		}
	}
}

// max_completion_tokens:1 next to max_tokens:64000 reserves for the capped default, not for one
// output token (SEC-56).
func TestPreQuotaConsumptionSmallestLimitDoesNotShrinkEstimate(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 1, 100000, 100000)
	orig := config.PreConsumedOutputTokens
	t.Cleanup(func() { config.PreConsumedOutputTokens = orig })
	config.PreConsumedOutputTokens = 4096

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(config.GinRequestBodyKey, []byte(`{"max_completion_tokens":1,"max_tokens":64000}`))
	q := &Quota{
		userId: 1, tokenId: tokenId, promptTokens: 100, requestedOutputTokens: requestedMaxOutputTokens(c),
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 2},
		inputRatio: 1, outputRatio: 2, groupRatio: 1,
	}
	if errResp := q.PreQuotaConsumption(); errResp != nil {
		t.Fatalf("pre-consumption: %+v", errResp)
	}
	if want := 100 + 4096*2 + 500; q.preConsumedQuota != want {
		t.Fatalf("pre=%d want %d", q.preConsumedQuota, want)
	}
}
