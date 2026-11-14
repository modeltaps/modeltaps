package model

import "testing"

func TestReplaceChannelPrices(t *testing.T) {
	useSupplyTestDB(t)
	if err := DB.AutoMigrate(&ChannelPrice{}, &ChannelModelAlias{}); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceChannelPrices(1, []*ChannelPrice{
		{Model: "b/model", Input: 3, Output: 15},
		{Model: "a/model", Input: 1, Output: 2},
		{Model: "a/model", Input: 9}, // a repeated name keeps the first
	}); err != nil {
		t.Fatal(err)
	}
	if err := ReplaceChannelPrices(2, []*ChannelPrice{{Model: "a/model", Input: 5}}); err != nil {
		t.Fatal(err)
	}

	list, err := GetChannelPrices(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Prices) != 2 || list.Prices[0].Model != "a/model" || list.Prices[0].Input != 1 || list.SyncedAt == 0 {
		t.Fatalf("channel 1 = %+v", list)
	}

	// A new read replaces the list: models the upstream dropped go away.
	if err := ReplaceChannelPrices(1, []*ChannelPrice{{Model: "b/model", Input: 4, Output: 16}}); err != nil {
		t.Fatal(err)
	}
	list, _ = GetChannelPrices(1)
	if len(list.Prices) != 1 || list.Prices[0].Input != 4 {
		t.Fatalf("after replace = %+v", list.Prices)
	}

	// An empty read keeps what was stored.
	if err := ReplaceChannelPrices(1, nil); err != nil {
		t.Fatal(err)
	}
	if list, _ = GetChannelPrices(1); len(list.Prices) != 1 {
		t.Fatalf("empty read wiped the list: %+v", list.Prices)
	}

	// Other channels are untouched, and deleting a channel drops its prices.
	if list, _ = GetChannelPrices(2); len(list.Prices) != 1 || list.Prices[0].Input != 5 {
		t.Fatalf("channel 2 = %+v", list.Prices)
	}
	deleteChannelPrices([]int{2})
	if list, _ = GetChannelPrices(2); len(list.Prices) != 0 || list.SyncedAt != 0 {
		t.Fatalf("channel 2 after delete = %+v", list)
	}
}
