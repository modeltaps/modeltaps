package model

import (
	"errors"
	"fmt"
	"testing"
)

func usePriceSourceTestDB(t *testing.T) *int64 {
	t.Helper()
	useAlertTestDB(t)
	if err := DB.AutoMigrate(&PriceSourceState{}); err != nil {
		t.Fatal(err)
	}
	now := int64(1_000_000)
	prev := priceSourceNow
	priceSourceNow = func() int64 { return now }
	t.Cleanup(func() { priceSourceNow = prev })
	return &now
}

func TestPriceSourceStateCountsFailuresUntilASuccess(t *testing.T) {
	now := usePriceSourceTestDB(t)
	subject := ChannelPriceSubject(4)
	RecordPriceSourceSuccess(subject, 30)
	*now += 100
	RecordPriceSourceFailure(subject, "", errors.New("upstream said 502"))
	*now += 100
	RecordPriceSourceFailure(subject, PriceSourceReasonShrank, errors.New("went from 30 to 3"))
	s := GetPriceSourceState(subject)
	if s == nil || s.Failures != 2 || s.FailingSince != 1_000_100 || s.LastSuccessAt != 1_000_000 ||
		s.LastReason != PriceSourceReasonShrank || s.LastError != "went from 30 to 3" || s.LastCount != 30 {
		t.Fatalf("state = %+v", s)
	}
	*now += 100
	RecordPriceSourceSuccess(subject, 31)
	s = GetPriceSourceState(subject)
	if s.Failures != 0 || s.FailingSince != 0 || s.LastError != "" || s.LastReason != "" || s.LastCount != 31 {
		t.Fatalf("after success = %+v", s)
	}
}

func TestReviewPriceSourcesTellsOnceAfterADayOfFailures(t *testing.T) {
	now := usePriceSourceTestDB(t)
	subject := ChannelPriceSubject(7)
	review := func() []FailingPriceSource {
		t.Helper()
		fresh, err := ReviewPriceSources(nil)
		if err != nil {
			t.Fatal(err)
		}
		return fresh
	}
	RecordPriceSourceSuccess(subject, 30)
	RecordPriceSourceFailure(subject, "", errors.New("timeout"))
	*now += priceSourceAlertAfter
	if got := review(); len(got) != 0 {
		t.Fatalf("one failure told: %+v", got)
	}
	RecordPriceSourceFailure(subject, "", errors.New("timeout again"))
	got := review()
	if len(got) != 1 || got[0].Subject != subject || got[0].Item != fmt.Sprintf("read|%d|%d", 1_000_000, 1_000_000) {
		t.Fatalf("review = %+v", got)
	}
	if again := review(); len(again) != 0 {
		t.Fatalf("told twice: %+v", again)
	}
	RecordPriceSourceSuccess(subject, 30)
	if open, _ := OpenAlerts(PriceSourceAlertKind); len(open) != 0 {
		t.Fatalf("still open after a good read: %+v", open)
	}
}

func TestReviewPriceSourcesWaitsForADay(t *testing.T) {
	now := usePriceSourceTestDB(t)
	subject := ChannelPriceSubject(8)
	for i := 0; i < 5; i++ {
		RecordPriceSourceFailure(subject, "", errors.New("timeout"))
		*now += 60
	}
	if got, _ := ReviewPriceSources(nil); len(got) != 0 {
		t.Fatalf("quick retries told: %+v", got)
	}
}

func TestReviewPriceSourcesSkipsUnwatchedSources(t *testing.T) {
	now := usePriceSourceTestDB(t)
	subject := ChannelPriceSubject(9)
	RecordPriceSourceFailure(subject, "", errors.New("timeout"))
	*now += priceSourceAlertAfter
	RecordPriceSourceFailure(subject, "", errors.New("timeout"))
	got, err := ReviewPriceSources(func(string) bool { return false })
	if err != nil || len(got) != 0 {
		t.Fatalf("unwatched told: %+v, %v", got, err)
	}
}

func TestDeletePriceSourceStatesClearsStateAndAlert(t *testing.T) {
	now := usePriceSourceTestDB(t)
	subject := ChannelPriceSubject(10)
	RecordPriceSourceFailure(subject, "", errors.New("timeout"))
	*now += priceSourceAlertAfter
	RecordPriceSourceFailure(subject, "", errors.New("timeout"))
	if got, _ := ReviewPriceSources(nil); len(got) != 1 {
		t.Fatalf("review = %+v", got)
	}
	deletePriceSourceStates([]int{10})
	if s := GetPriceSourceState(subject); s != nil {
		t.Fatalf("state kept: %+v", s)
	}
	if open, _ := OpenAlerts(PriceSourceAlertKind); len(open) != 0 {
		t.Fatalf("alert kept: %+v", open)
	}
}

func priceList(n, unpriced int) []*ChannelPrice {
	out := make([]*ChannelPrice, n)
	for i := range out {
		out[i] = &ChannelPrice{Model: fmt.Sprintf("m%d", i), Input: 1, Output: 2}
		if i < unpriced {
			out[i].Input, out[i].Output = 0, 0
		}
	}
	return out
}

func TestCheckChannelPriceList(t *testing.T) {
	var suspicious *SuspiciousPriceListError
	cases := []struct {
		name         string
		stored, next []*ChannelPrice
		reason       string
	}{
		{"normal change", priceList(40, 2), priceList(38, 3), ""},
		{"first read", nil, priceList(5, 5), ""},
		{"small stored list", priceList(19, 0), priceList(2, 2), ""},
		{"shrank", priceList(40, 0), priceList(19, 0), PriceSourceReasonShrank},
		{"half is fine", priceList(40, 0), priceList(20, 0), ""},
		{"zeroed", priceList(40, 2), priceList(40, 12), PriceSourceReasonZeroed},
		{"few more free", priceList(40, 2), priceList(40, 9), ""},
	}
	for _, c := range cases {
		err := CheckChannelPriceList(c.stored, c.next)
		if c.reason == "" {
			if err != nil {
				t.Errorf("%s: %v", c.name, err)
			}
			continue
		}
		if !errors.As(err, &suspicious) || suspicious.Reason != c.reason {
			t.Errorf("%s: err = %v, want %s", c.name, err, c.reason)
		}
	}
}

func TestUnpricedCountKeepsOtherPrices(t *testing.T) {
	list := []*ChannelPrice{{Model: "a"}, {Model: "b", Image: 0.04}, {Model: "c", Other: true}, {Model: "d", Request: 0.01}}
	if n := unpricedCount(list); n != 1 {
		t.Fatalf("unpriced = %d, want 1", n)
	}
}
