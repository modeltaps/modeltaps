package model

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// A channel's real fee comes from what its top-ups cost: the cash paid (card and currency fees
// included) for the credit the provider added. The default fee of a channel type is what the
// provider's documentation states; once top-ups are recorded, the fee is derived from them and
// replaces it, so the costs the router and the price check use follow the bills.

// channelTopupFeeWindow is how many of the latest top-ups the fee is derived from, so a changed
// fee shows within a few top-ups.
const channelTopupFeeWindow = 10

// ChannelTopup is one top-up of a channel's provider account, in USD.
type ChannelTopup struct {
	Id        int     `json:"id"`
	ChannelId int     `json:"channel_id" gorm:"index"`
	Paid      float64 `json:"paid" gorm:"type:decimal(14,4)"`     // cash paid, every fee included
	Credited  float64 `json:"credited" gorm:"type:decimal(14,4)"` // credit the provider added
	Note      string  `json:"note" gorm:"type:varchar(255)"`
	CreatedBy int     `json:"created_by"`
	CreatedAt int64   `json:"created_at" gorm:"autoCreateTime"`
}

var ErrChannelTopupInvalid = errors.New("paid and credited must be positive amounts")

// ChannelTopupSummary is a channel's recorded top-ups and the fee derived from them.
type ChannelTopupSummary struct {
	Topups []*ChannelTopup `json:"topups"`
	// FeeRate is paid / credited - 1 over the latest top-ups (at least 0); Derived is false when
	// there is none and the channel keeps the fee it has.
	FeeRate float64 `json:"fee_rate"`
	Derived bool    `json:"derived"`
	Window  int     `json:"window"`
}

// GetChannelTopups lists a channel's top-ups, newest first, with the fee they give.
func GetChannelTopups(channelId int) (*ChannelTopupSummary, error) {
	var topups []*ChannelTopup
	if err := DB.Where("channel_id = ?", channelId).Order("created_at DESC, id DESC").Find(&topups).Error; err != nil {
		return nil, err
	}
	out := &ChannelTopupSummary{Topups: topups, Window: channelTopupFeeWindow}
	out.FeeRate, out.Derived = topupFeeRate(topups)
	return out, nil
}

// topupFeeRate derives the fee from the latest top-ups (newest first).
func topupFeeRate(topups []*ChannelTopup) (float64, bool) {
	paid, credited := 0.0, 0.0
	for i, t := range topups {
		if i >= channelTopupFeeWindow {
			break
		}
		paid += t.Paid
		credited += t.Credited
	}
	if credited <= 0 {
		return 0, false
	}
	// Bonus credit (paid below credited) is not a negative fee: costs never go below list price.
	rate := math.Max(0, paid/credited-1)
	return math.Round(rate*10000) / 10000, true
}

// AddChannelTopup records a top-up and applies the derived fee to the channel.
func AddChannelTopup(t *ChannelTopup) (*ChannelTopupSummary, error) {
	if t.Paid <= 0 || t.Credited <= 0 || math.IsInf(t.Paid, 0) || math.IsInf(t.Credited, 0) {
		return nil, ErrChannelTopupInvalid
	}
	if _, err := GetChannelById(t.ChannelId); err != nil {
		return nil, err
	}
	t.Id = 0
	if err := DB.Create(t).Error; err != nil {
		return nil, err
	}
	return applyTopupFee(t.ChannelId)
}

// DeleteChannelTopup removes a top-up of the channel and applies the fee the others give. With
// none left the channel keeps its fee.
func DeleteChannelTopup(channelId, id int) (*ChannelTopupSummary, error) {
	result := DB.Where("id = ? AND channel_id = ?", id, channelId).Delete(&ChannelTopup{})
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, errors.New("top-up not found")
	}
	return applyTopupFee(channelId)
}

func applyTopupFee(channelId int) (*ChannelTopupSummary, error) {
	summary, err := GetChannelTopups(channelId)
	if err != nil || !summary.Derived {
		return summary, err
	}
	n := min(len(summary.Topups), channelTopupFeeWindow)
	paid, credited := 0.0, 0.0
	for _, t := range summary.Topups[:n] {
		paid += t.Paid
		credited += t.Credited
	}
	note := fmt.Sprintf("Derived from the latest %d top-up(s): paid $%.2f for $%.2f of credit (%s).",
		n, paid, credited, time.Now().UTC().Format("2006-01-02"))
	if err := DB.Model(&Channel{}).Where("id = ?", channelId).
		Updates(map[string]any{"fee_rate": summary.FeeRate, "fee_note": note}).Error; err != nil {
		return nil, err
	}
	// The router and the price check read the new fee.
	InvalidateChannelPriceCache()
	ChannelGroup.Load()
	return summary, nil
}
