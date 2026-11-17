package model

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/config"
	"gorm.io/gorm"
)

// channelFeeDefault is what a provider charges on top of its listed prices, as its own
// documentation states it.
type channelFeeDefault struct {
	rate float64
	note string
}

// channelFeeDefaults are the fee rates new channels of a type start with. The providers change
// these: check the source and update the date when they do.
var channelFeeDefaults = map[int]channelFeeDefault{
	// Checked 2026-11-16: https://docs.requesty.ai/features/billing.md
	config.ChannelTypeRequesty: {0.05, "Top-up fee: $105 buys $100 of credit; usage is billed at list price."},
	// Checked 2026-11-16: https://openrouter.ai/docs/faq.md#pricing-and-fees
	config.ChannelTypeOpenRouter: {0.055, "Credit purchase fee 5.5% (minimum $0.80); inference is resold at list price. Crypto payments: 5%."},
	// Checked 2026-11-16: https://vercel.com/docs/ai-gateway/pricing.md (no markup on inference).
}

// ChannelFeeDefault returns the default fee rate and note of a channel type; 0 and "" when the
// provider charges nothing on top of its prices.
func ChannelFeeDefault(channelType int) (float64, string) {
	d := channelFeeDefaults[channelType]
	return d.rate, d.note
}

// applyFeeDefault fills in the type's default fee for a new channel that does not set one.
func (channel *Channel) applyFeeDefault() {
	if channel.FeeRate != nil {
		return
	}
	rate, note := ChannelFeeDefault(channel.Type)
	channel.FeeRate = &rate
	if channel.FeeNote == "" {
		channel.FeeNote = note
	}
}

// backfillChannelFeeRates gives existing channels of the types that charge a fee their default.
// Only channels that never had a fee set are touched (rate 0 and no note), so it can run again.
func backfillChannelFeeRates() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611170501",
		Migrate: func(tx *gorm.DB) error {
			if !tx.Migrator().HasColumn(&Channel{}, "fee_rate") {
				return nil
			}
			for channelType, d := range channelFeeDefaults {
				err := tx.Model(&Channel{}).
					Where("type = ? AND (fee_rate IS NULL OR fee_rate = 0) AND (fee_note IS NULL OR fee_note = '')", channelType).
					Updates(map[string]any{"fee_rate": d.rate, "fee_note": d.note}).Error
				if err != nil {
					return err
				}
			}
			return nil
		},
		Rollback: func(tx *gorm.DB) error {
			return nil
		},
	}
}
