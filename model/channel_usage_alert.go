package model

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/datatypes"
)

// DefaultUsageAlertTiers are the subscription usage alert tiers when the site sets none.
var DefaultUsageAlertTiers = []float64{50, 75, 100}

// ErrInvalidUsageAlertTiers is returned for tiers that are not percentages between 1 and 100.
var ErrInvalidUsageAlertTiers = errors.New("usage alert tiers must be percentages between 1 and 100 separated by commas, or 0 to turn the alerts off")

// ParseUsageAlertTiers reads tiers written as "50, 75, 100". It returns nil for an empty value
// (not set: use the next level up), an empty slice for "0" or "off" (alerts off), and otherwise
// the tiers sorted, without duplicates.
func ParseUsageAlertTiers(value string) ([]float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if value == "0" || strings.EqualFold(value, "off") {
		return []float64{}, nil
	}
	fields := strings.FieldsFunc(value, func(r rune) bool { return r == ',' || r == '，' || r == ' ' || r == '%' })
	tiers := make([]float64, 0, len(fields))
	for _, field := range fields {
		tier, err := strconv.ParseFloat(field, 64)
		if err != nil || tier < 1 || tier > 100 {
			return nil, ErrInvalidUsageAlertTiers
		}
		tiers = append(tiers, tier)
	}
	if len(tiers) == 0 {
		return nil, ErrInvalidUsageAlertTiers
	}
	slices.Sort(tiers)
	return slices.Compact(tiers), nil
}

// FormatUsageAlertTiers writes tiers the way they are stored: "50,75,100", or "0" for off.
func FormatUsageAlertTiers(tiers []float64) string {
	if len(tiers) == 0 {
		return "0"
	}
	parts := make([]string, len(tiers))
	for i, tier := range tiers {
		parts[i] = strconv.FormatFloat(tier, 'f', -1, 64)
	}
	return strings.Join(parts, ",")
}

// NormalizeUsageAlertTiers validates tiers and returns them in their stored form; an empty value
// stays empty.
func NormalizeUsageAlertTiers(value string) (string, error) {
	tiers, err := ParseUsageAlertTiers(value)
	if err != nil || tiers == nil {
		return "", err
	}
	return FormatUsageAlertTiers(tiers), nil
}

// SiteUsageAlertTiers are the tiers that apply to channels without tiers of their own; empty
// when the site turned the alerts off.
func SiteUsageAlertTiers() []float64 {
	tiers, err := ParseUsageAlertTiers(config.UsageAlertTiers)
	if err != nil || tiers == nil {
		return slices.Clone(DefaultUsageAlertTiers)
	}
	return tiers
}

// UsageAlertTierList is the tiers that apply to this channel: its own, or else the site's.
// Empty means no usage alerts for the channel.
func (channel *Channel) UsageAlertTierList() []float64 {
	if channel.UsageAlertTiers != nil {
		if tiers, err := ParseUsageAlertTiers(*channel.UsageAlertTiers); err == nil && tiers != nil {
			return tiers
		}
	}
	return SiteUsageAlertTiers()
}

// UsageAlertState is what was last alerted for one usage window of a subscription channel.
type UsageAlertState struct {
	// Tier is the highest tier alerted in the current window period.
	Tier float64 `json:"tier"`
	// ResetAt is when that period ends (unix seconds, 0 when unknown); after it the tiers fire again.
	ResetAt   int64 `json:"reset_at"`
	AlertedAt int64 `json:"alerted_at"`
}

// ChannelUsageAlerts maps a usage window's name ("5h", "7d") to its alert state.
type ChannelUsageAlerts map[string]UsageAlertState

// UsageAlertStates returns a copy of the stored alert states, never nil.
func (channel *Channel) UsageAlertStates() ChannelUsageAlerts {
	out := ChannelUsageAlerts{}
	if channel.UsageAlerts != nil {
		for k, v := range channel.UsageAlerts.Data() {
			out[k] = v
		}
	}
	return out
}

// UpdateUsageAlerts stores the usage alert states without touching other columns.
func (channel *Channel) UpdateUsageAlerts(states ChannelUsageAlerts) error {
	jt := datatypes.NewJSONType(states)
	if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("usage_alerts", jt).Error; err != nil {
		return err
	}
	channel.UsageAlerts = &jt
	return nil
}
