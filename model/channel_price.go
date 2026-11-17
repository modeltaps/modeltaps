package model

import (
	"sort"
	"time"

	"gorm.io/gorm"
)

// ChannelPrice is what one channel's upstream charges for one model, as the upstream's own price
// list reports it. It is the channel's cost, kept apart from the catalog price (Price), which is
// what Modeltaps charges its users and holds one row per model for every channel. Only channel
// types whose upstream publishes prices (OpenRouter, Vercel AI Gateway) have rows.
//
// Token prices are USD per million tokens; Image and Request are USD per image and per request;
// AudioSecond is USD per second of audio;
// VideoSecond is the lowest USD per second of video the upstream lists (prices vary by
// resolution). A model whose prices are all zero is free on that upstream.
type ChannelPrice struct {
	Id          int     `json:"-" gorm:"primaryKey"`
	ChannelId   int     `json:"-" gorm:"not null;uniqueIndex:idx_channel_price_model"`
	Model       string  `json:"model" gorm:"type:varchar(191);not null;uniqueIndex:idx_channel_price_model"`
	Input       float64 `json:"input"`
	Output      float64 `json:"output"`
	CacheRead   float64 `json:"cache_read,omitempty"`
	CacheWrite  float64 `json:"cache_write,omitempty"`
	Image       float64 `json:"image,omitempty"`
	Request     float64 `json:"request,omitempty"`
	VideoSecond float64 `json:"video_second,omitempty"`
	// AudioSecond is USD per second of audio: transcribed (speech-to-text) or generated.
	AudioSecond float64 `json:"audio_second,omitempty"`
	// Other marks a model the upstream prices only in a way not kept here (per realtime session,
	// per video token): it has a price, just not one of the above.
	Other bool `json:"other,omitempty"`
	// Kind is the model kind the upstream's own list states (ModelKind*; "" for a language model
	// or when the list does not say). The channel panel tags non-chat models with it.
	Kind     string `json:"kind,omitempty" gorm:"type:varchar(16);default:''"`
	SyncedAt int64  `json:"synced_at,omitempty"`

	// Where the price comes from and where to check it, filled in by DescribeChannelPrices for
	// the console; not stored.
	Source    string `json:"source,omitempty" gorm:"-"`
	Via       string `json:"via,omitempty" gorm:"-"`
	PageURL   string `json:"page_url,omitempty" gorm:"-"`
	PageExact bool   `json:"page_exact,omitempty" gorm:"-"`
	// Official marks the vendor's list price shown for a vendor channel that has no price list of
	// its own (OfficialChannelPrice); not stored.
	Official bool `json:"official,omitempty" gorm:"-"`
}

// USDPerMillionTokens converts a per-token USD price, the unit upstream price lists use, to USD
// per million tokens.
func USDPerMillionTokens(perToken float64) float64 {
	return perToken * 1e6
}

// ChannelPriceList is a channel's stored upstream prices.
type ChannelPriceList struct {
	// SyncedAt is when the prices were read from the upstream; 0 when they never were.
	SyncedAt int64           `json:"synced_at"`
	Prices   []*ChannelPrice `json:"prices"`
}

// ReplaceChannelPrices stores prices as the channel's whole upstream price list: models the
// upstream no longer lists are removed. An empty list is ignored, so a failed or empty read
// never wipes what was stored.
func ReplaceChannelPrices(channelID int, prices []*ChannelPrice) error {
	if channelID == 0 || len(prices) == 0 {
		return nil
	}
	now := time.Now().Unix()
	seen := make(map[string]bool, len(prices))
	rows := make([]*ChannelPrice, 0, len(prices))
	for _, p := range prices {
		if p == nil || p.Model == "" || seen[p.Model] {
			continue
		}
		seen[p.Model] = true
		row := *p
		row.Id, row.ChannelId, row.SyncedAt = 0, channelID, now
		rows = append(rows, &row)
	}
	defer InvalidateChannelPriceCache()
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("channel_id = ?", channelID).Delete(&ChannelPrice{}).Error; err != nil {
			return err
		}
		return tx.CreateInBatches(rows, 200).Error
	})
}

// GetChannelPrices returns the channel's stored upstream prices, sorted by model name.
func GetChannelPrices(channelID int) (*ChannelPriceList, error) {
	out := &ChannelPriceList{Prices: []*ChannelPrice{}}
	if err := DB.Where("channel_id = ?", channelID).Find(&out.Prices).Error; err != nil {
		return nil, err
	}
	sort.Slice(out.Prices, func(i, j int) bool { return out.Prices[i].Model < out.Prices[j].Model })
	for _, p := range out.Prices {
		if p.SyncedAt > out.SyncedAt {
			out.SyncedAt = p.SyncedAt
		}
	}
	return out, nil
}

func deleteChannelPrices(channelIDs []int) {
	if len(channelIDs) > 0 {
		DB.Where("channel_id IN ?", channelIDs).Delete(&ChannelPrice{})
		InvalidateChannelPriceCache()
	}
}
