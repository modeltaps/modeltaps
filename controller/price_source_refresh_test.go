package controller

import (
	"errors"
	"fmt"
	"testing"

	"github.com/modeltaps/modeltaps/model"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
)

type fakePriceLister struct {
	providersBase.ProviderInterface
	prices []*model.ChannelPrice
	err    error
}

func (f *fakePriceLister) GetChannelPrices() ([]*model.ChannelPrice, error) { return f.prices, f.err }

func pricedModels(n int) []*model.ChannelPrice {
	out := make([]*model.ChannelPrice, n)
	for i := range out {
		out[i] = &model.ChannelPrice{Model: fmt.Sprintf("m%02d", i), Input: 1, Output: 2}
	}
	return out
}

// A list that lost most of its models is not stored unless forced, and each read is recorded.
func TestRefreshChannelPricesHoldsASuspiciousList(t *testing.T) {
	setupAggregatorSyncDB(t)
	if err := model.DB.AutoMigrate(&model.ChannelPrice{}, &model.PriceSourceState{}, &model.Alert{}); err != nil {
		t.Fatal(err)
	}
	ch := &model.Channel{Id: 3}
	subject := model.ChannelPriceSubject(3)
	if n, err := refreshChannelPrices(ch, &fakePriceLister{prices: pricedModels(30)}, false); err != nil || n != 30 {
		t.Fatalf("first read = %d, %v", n, err)
	}

	_, err := refreshChannelPrices(ch, &fakePriceLister{prices: pricedModels(5)}, false)
	var suspicious *model.SuspiciousPriceListError
	if !errors.As(err, &suspicious) || suspicious.Reason != model.PriceSourceReasonShrank {
		t.Fatalf("shrunk list: err = %v", err)
	}
	if list, _ := model.GetChannelPrices(3); len(list.Prices) != 30 {
		t.Fatalf("stored list replaced: %d models", len(list.Prices))
	}
	if s := model.GetPriceSourceState(subject); s == nil || s.Failures != 1 || s.LastReason != model.PriceSourceReasonShrank {
		t.Fatalf("state = %+v", s)
	}

	if _, err := refreshChannelPrices(ch, &fakePriceLister{err: errors.New("502")}, false); err == nil {
		t.Fatal("read error swallowed")
	}
	if s := model.GetPriceSourceState(subject); s.Failures != 2 || s.LastReason != model.PriceSourceReasonRead {
		t.Fatalf("state = %+v", s)
	}

	if n, err := refreshChannelPrices(ch, &fakePriceLister{prices: pricedModels(5)}, true); err != nil || n != 5 {
		t.Fatalf("forced read = %d, %v", n, err)
	}
	if s := model.GetPriceSourceState(subject); s.Failures != 0 || s.LastCount != 5 {
		t.Fatalf("state after forced read = %+v", s)
	}
}
