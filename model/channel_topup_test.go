package model

import (
	"math"
	"testing"
)

// Recorded top-ups set the channel's fee to paid / credited - 1 over the latest ones.
func TestChannelTopupDerivesFee(t *testing.T) {
	useAllocationTestDB(t)
	if err := DB.AutoMigrate(&ChannelTopup{}); err != nil {
		t.Fatal(err)
	}
	ch := makeAllocationChannel(t, testChannel{"vercel", 1, "https://v.example", 0, 1})
	if _, err := AddChannelTopup(&ChannelTopup{ChannelId: ch.Id, Paid: 0, Credited: 100}); err == nil {
		t.Fatal("a top-up with nothing paid was accepted")
	}
	sum, err := AddChannelTopup(&ChannelTopup{ChannelId: ch.Id, Paid: 102.5, Credited: 100})
	if err != nil || !sum.Derived || math.Abs(sum.FeeRate-0.025) > 1e-9 {
		t.Fatalf("one top-up: %+v, %v", sum, err)
	}
	got, _ := GetChannelById(ch.Id)
	if math.Abs(got.GetFeeRate()-0.025) > 1e-9 || got.FeeNote == "" {
		t.Fatalf("channel fee = %v note %q, want 2.5%% with a note", got.GetFeeRate(), got.FeeNote)
	}
	// (102.5 + 210) / (100 + 200) - 1 = 4.1667%.
	sum, _ = AddChannelTopup(&ChannelTopup{ChannelId: ch.Id, Paid: 210, Credited: 200})
	if math.Abs(sum.FeeRate-0.0417) > 1e-9 || len(sum.Topups) != 2 {
		t.Fatalf("two top-ups: %+v", sum)
	}
	// Bonus credit does not make a negative fee.
	sum, _ = AddChannelTopup(&ChannelTopup{ChannelId: ch.Id, Paid: 100, Credited: 400})
	if sum.FeeRate != 0 {
		t.Fatalf("bonus credit: fee %v, want 0", sum.FeeRate)
	}
	sum, err = DeleteChannelTopup(ch.Id, sum.Topups[0].Id)
	if err != nil || math.Abs(sum.FeeRate-0.0417) > 1e-9 {
		t.Fatalf("after delete: %+v, %v", sum, err)
	}
}
