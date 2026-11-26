package relay_util

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// A Venice cache hit priced at the channel's list price: the hit is taken off the input once, so
// the cost is the uncached input, the cache read and the output, not the 1-quota floor it fell to
// when the hit counted twice.
func TestChannelCostVeniceCacheHit(t *testing.T) {
	catalog := model.Price{Type: model.TokensPriceType, Input: 0.165, Output: 0.66}
	cp := &model.ChannelPrice{Model: "deepseek-v4-1-flash-uncensored", Input: 0.33, Output: 1.32, CacheRead: 0.00825}
	list, _, ok := model.SupplyListPrice(&catalog, &model.Channel{}, cp, cp.Model)
	if !ok {
		t.Fatal("no list price")
	}
	var usage types.Usage
	body := `{"prompt_tokens":3931,"completion_tokens":3,"total_tokens":3934,"prompt_tokens_details":{"cached_tokens":3840},"cache_read_input_tokens":3840}`
	if err := json.Unmarshal([]byte(body), &usage); err != nil {
		t.Fatal(err)
	}
	q := &Quota{price: catalog, costPrice: list, costSource: model.CostSourceChannelPrice}
	cost, _ := q.requestCost(&usage)
	// (3931 - 3840*0.975) input * 0.165 + 3 output * 0.66 = 32.8, rounded up.
	if cost != 33 {
		t.Fatalf("cost = %d quota, want 33", cost)
	}
}
