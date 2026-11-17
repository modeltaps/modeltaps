package model

import (
	"sync"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/gorm"
)

const (
	rebateInviterId = 21
	rebateInviteeId = 22
)

// setInviterReward sets the rebate settings for one test and restores them afterwards.
func setInviterReward(t *testing.T, rewardType string, value int) {
	t.Helper()
	oldType, oldValue := config.InviterRewardType, config.InviterRewardValue
	config.InviterRewardType, config.InviterRewardValue = rewardType, value
	t.Cleanup(func() { config.InviterRewardType, config.InviterRewardValue = oldType, oldValue })
}

// seedRebateUsers writes an inviter and the user it invited, both with no quota.
func seedRebateUsers(t *testing.T) {
	t.Helper()
	for _, user := range []*User{
		{Id: rebateInviterId, Username: "inviter", Password: "placeholder", AccessToken: "rebate-inviter", AffCode: "inv1", Group: "default"},
		{Id: rebateInviteeId, Username: "invitee", Password: "placeholder", AccessToken: "rebate-invitee", AffCode: "inv2", Group: "default", InviterId: rebateInviterId},
	} {
		if err := DB.Create(user).Error; err != nil {
			t.Fatalf("create user %s: %v", user.Username, err)
		}
	}
}

func reloadUser(t *testing.T, id int) *User {
	t.Helper()
	user := &User{}
	if err := DB.First(user, id).Error; err != nil {
		t.Fatalf("load user %d: %v", id, err)
	}
	return user
}

func creditOrder(t *testing.T, order *Order) {
	t.Helper()
	credited, err := CreditPendingOrder(order, "gw-"+order.TradeNo)
	if err != nil || !credited {
		t.Fatalf("credit order %s: credited=%v err=%v", order.TradeNo, credited, err)
	}
}

func countRebateLogs(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := DB.Model(&Log{}).Where("user_id = ? AND type = ?", rebateInviterId, LogTypeSystem).Count(&count).Error; err != nil {
		t.Fatalf("count logs: %v", err)
	}
	return count
}

// Only the invitee's first paid order earns the inviter a rebate; the second earns nothing.
func TestInviterRebatePaidOnFirstPaidOrderOnly(t *testing.T) {
	cases := []struct {
		name       string
		rewardType string
		value      int
		wantRebate int
	}{
		{"fixed", "fixed", 500, 500},
		{"percentage", "percentage", 10, promotionQuota(10)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			setupPromotionTestDB(t)
			setInviterReward(t, tc.rewardType, tc.value)
			seedRebateUsers(t)

			creditOrder(t, seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusPending))
			inviter := reloadUser(t, rebateInviterId)
			if inviter.Quota != tc.wantRebate || inviter.AffQuota != tc.wantRebate {
				t.Fatalf("after the first order, inviter quota=%d aff_quota=%d, want %d", inviter.Quota, inviter.AffQuota, tc.wantRebate)
			}
			if got := countRebateLogs(t); got != 1 {
				t.Fatalf("rebate logs = %d, want 1", got)
			}
			if reloadUser(t, rebateInviteeId).FirstTopUpTime == 0 {
				t.Fatal("first_top_up_time should be set after the first paid order")
			}

			creditOrder(t, seedPromotionOrder(t, rebateInviteeId, 200, OrderStatusPending))
			inviter = reloadUser(t, rebateInviterId)
			if inviter.Quota != tc.wantRebate || inviter.AffQuota != tc.wantRebate {
				t.Fatalf("after the second order, inviter quota=%d aff_quota=%d, want %d", inviter.Quota, inviter.AffQuota, tc.wantRebate)
			}
			if got := countRebateLogs(t); got != 1 {
				t.Fatalf("rebate logs = %d, want 1", got)
			}
			if got := reloadUser(t, rebateInviteeId).Quota; got != promotionQuota(300) {
				t.Fatalf("invitee quota = %d, want %d", got, promotionQuota(300))
			}
		})
	}
}

// Concurrent callbacks for the invitee's first two orders (each delivered several times) credit
// both orders once and pay a single rebate.
func TestInviterRebatePaidOnceUnderConcurrentCallbacks(t *testing.T) {
	setupPromotionTestDB(t)
	setInviterReward(t, "fixed", 500)
	seedRebateUsers(t)
	orders := []*Order{
		seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusPending),
		seedPromotionOrder(t, rebateInviteeId, 50, OrderStatusPending),
	}

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		order := *orders[i%len(orders)]
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := CreditPendingOrder(&order, "gw-"+order.TradeNo); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("credit order: %v", err)
	}

	if got := reloadUser(t, rebateInviteeId).Quota; got != promotionQuota(150) {
		t.Fatalf("invitee quota = %d, want %d", got, promotionQuota(150))
	}
	inviter := reloadUser(t, rebateInviterId)
	if inviter.Quota != 500 || inviter.AffQuota != 500 {
		t.Fatalf("inviter quota=%d aff_quota=%d, want a single rebate of 500", inviter.Quota, inviter.AffQuota)
	}
	if got := countRebateLogs(t); got != 1 {
		t.Fatalf("rebate logs = %d, want 1", got)
	}
}

// The first paid order is the first one whatever the settings were: with the rebate off then,
// turning it on later pays nothing for later orders.
func TestInviterRebateNotPaidForLaterOrdersAfterEnabling(t *testing.T) {
	setupPromotionTestDB(t)
	setInviterReward(t, "fixed", 0)
	seedRebateUsers(t)

	creditOrder(t, seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusPending))
	config.InviterRewardValue = 500
	creditOrder(t, seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusPending))

	if got := reloadUser(t, rebateInviterId).Quota; got != 0 {
		t.Fatalf("inviter quota = %d, want 0", got)
	}
}

// Redemption codes pay no rebate and do not use up the first paid top-up.
func TestRedeemPaysNoInviterRebate(t *testing.T) {
	setupPromotionTestDB(t)
	setInviterReward(t, "fixed", 500)
	seedRebateUsers(t)

	code := &Redemption{Key: "REBATECODE0000000000000000000100", Status: config.RedemptionCodeStatusEnabled, Quota: promotionQuota(100)}
	if err := DB.Create(code).Error; err != nil {
		t.Fatalf("create redemption code: %v", err)
	}
	if _, err := Redeem(code.Key, rebateInviteeId, "127.0.0.1"); err != nil {
		t.Fatalf("redeem: %v", err)
	}
	if got := reloadUser(t, rebateInviterId).Quota; got != 0 {
		t.Fatalf("inviter quota after a redemption = %d, want 0", got)
	}

	creditOrder(t, seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusPending))
	if got := reloadUser(t, rebateInviterId).Quota; got != 500 {
		t.Fatalf("inviter quota after the first paid order = %d, want 500", got)
	}
}

// A user without an inviter still has the first paid top-up recorded, and nobody is paid.
func TestFirstTopUpRecordedWithoutInviter(t *testing.T) {
	setupPromotionTestDB(t)
	setInviterReward(t, "fixed", 500)
	seedPromotionUser(t, 30, 0, 0)

	creditOrder(t, seedPromotionOrder(t, 30, 100, OrderStatusPending))
	user := reloadUser(t, 30)
	if user.FirstTopUpTime == 0 || user.Quota != promotionQuota(100) {
		t.Fatalf("first_top_up_time=%d quota=%d, want a set time and %d", user.FirstTopUpTime, user.Quota, promotionQuota(100))
	}
	var logs int64
	DB.Model(&Log{}).Count(&logs)
	if logs != 0 {
		t.Fatalf("logs = %d, want none", logs)
	}
}

// Upgrading marks users that already have a credited order, so their next order pays no rebate;
// users with only unpaid orders keep a first top-up to come. Running it again changes nothing.
func TestBackfillUserFirstTopUp(t *testing.T) {
	setupPromotionTestDB(t)
	setInviterReward(t, "fixed", 500)
	seedRebateUsers(t)
	seedPromotionUser(t, 31, 0, 0)
	paid := seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusSuccess)
	seedPromotionOrder(t, 31, 100, OrderStatusPending)
	seedPromotionOrder(t, 31, 100, OrderStatusClosed)

	migration := backfillUserFirstTopUp()
	for round := 1; round <= 2; round++ {
		if err := migration.Migrate(DB); err != nil {
			t.Fatalf("migration round %d: %v", round, err)
		}
		if got := reloadUser(t, rebateInviteeId).FirstTopUpTime; got != int64(paid.UpdatedAt) {
			t.Fatalf("round %d: invitee first_top_up_time = %d, want %d", round, got, paid.UpdatedAt)
		}
		if got := reloadUser(t, 31).FirstTopUpTime; got != 0 {
			t.Fatalf("round %d: user without paid orders has first_top_up_time %d, want 0", round, got)
		}
		if got := reloadUser(t, rebateInviterId).FirstTopUpTime; got != 0 {
			t.Fatalf("round %d: inviter has first_top_up_time %d, want 0", round, got)
		}
	}

	creditOrder(t, seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusPending))
	if got := reloadUser(t, rebateInviterId).Quota; got != 0 {
		t.Fatalf("inviter quota after an order following the backfill = %d, want 0", got)
	}
}

func setInviteRewardEnabled(t *testing.T, enabled bool) {
	t.Helper()
	old := config.InviteRewardEnabled
	config.InviteRewardEnabled = enabled
	t.Cleanup(func() { config.InviteRewardEnabled = old })
}

// With invite rewards switched off the first paid order pays no rebate, and switching them back
// on afterwards does not pay it later: the first top-up has been used.
func TestInviterRebateNotPaidWhenInviteRewardsOff(t *testing.T) {
	setupPromotionTestDB(t)
	setInviterReward(t, "fixed", 500)
	setInviteRewardEnabled(t, false)
	seedRebateUsers(t)

	creditOrder(t, seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusPending))
	if inviter := reloadUser(t, rebateInviterId); inviter.Quota != 0 || inviter.AffQuota != 0 {
		t.Fatalf("inviter quota=%d aff_quota=%d with invite rewards off, want 0", inviter.Quota, inviter.AffQuota)
	}
	if got := countRebateLogs(t); got != 0 {
		t.Fatalf("rebate logs = %d, want 0", got)
	}

	config.InviteRewardEnabled = true
	creditOrder(t, seedPromotionOrder(t, rebateInviteeId, 100, OrderStatusPending))
	if got := reloadUser(t, rebateInviterId).Quota; got != 0 {
		t.Fatalf("inviter quota = %d after re-enabling, want 0 (no rebate for a later order)", got)
	}
}

// Sign-up bonuses for both sides follow the switch; the inviter (set by the caller) is kept.
func TestInviteSignupBonusFollowsSwitch(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		name := "on"
		if !enabled {
			name = "off"
		}
		t.Run(name, func(t *testing.T) {
			setupPromotionTestDB(t)
			setInviteRewardEnabled(t, enabled)
			oldInviter, oldInvitee, oldNew := config.QuotaForInviter, config.QuotaForInvitee, config.QuotaForNewUser
			config.QuotaForInviter, config.QuotaForInvitee, config.QuotaForNewUser = 300, 200, 0
			t.Cleanup(func() {
				config.QuotaForInviter, config.QuotaForInvitee, config.QuotaForNewUser = oldInviter, oldInvitee, oldNew
			})
			inviter := &User{Id: rebateInviterId, Username: "inviter", Password: "placeholder", AccessToken: "rebate-inviter", AffCode: "inv1", Group: "default"}
			if err := DB.Create(inviter).Error; err != nil {
				t.Fatal(err)
			}

			viaInsert := &User{Username: "invitee-a", Password: "placeholder-password", Group: "default", InviterId: rebateInviterId}
			if err := viaInsert.Insert(rebateInviterId); err != nil {
				t.Fatalf("Insert: %v", err)
			}
			viaTx := &User{Username: "invitee-b", Password: "placeholder-password", Group: "default", InviterId: rebateInviterId}
			if err := DB.Transaction(func(tx *gorm.DB) error { return viaTx.InsertWithTx(tx, rebateInviterId) }); err != nil {
				t.Fatalf("InsertWithTx: %v", err)
			}

			wantInvitee, wantInviter := 0, 0
			if enabled {
				wantInvitee, wantInviter = 200, 600
			}
			for _, u := range []*User{viaInsert, viaTx} {
				got := reloadUser(t, u.Id)
				if got.Quota != wantInvitee {
					t.Fatalf("%s quota = %d, want %d", u.Username, got.Quota, wantInvitee)
				}
				if got.InviterId != rebateInviterId {
					t.Fatalf("%s inviter_id = %d, want %d", u.Username, got.InviterId, rebateInviterId)
				}
			}
			if got := reloadUser(t, rebateInviterId).Quota; got != wantInviter {
				t.Fatalf("inviter quota = %d, want %d", got, wantInviter)
			}
		})
	}
}
