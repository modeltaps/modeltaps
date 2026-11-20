package model

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// PriceSourceState is how the last reads of one price source went: a channel's upstream price
// list ("channel:4") or the models.dev catalog ("source:models.dev"). A failed read keeps the
// prices read before, so without this record a source that stopped working goes unnoticed while
// its prices get older. The subjects are those of the source's alert (PriceSourceAlertKind).
type PriceSourceState struct {
	Subject       string `json:"subject" gorm:"type:varchar(191);primaryKey"`
	LastAttemptAt int64  `json:"last_attempt_at" gorm:"bigint;default:0"`
	LastSuccessAt int64  `json:"last_success_at" gorm:"bigint;default:0"`
	// FailingSince is the first failed read after the last good one; 0 while reads succeed.
	FailingSince int64 `json:"failing_since" gorm:"bigint;default:0"`
	// Failures counts the failed reads since FailingSince.
	Failures int `json:"failures" gorm:"default:0"`
	// LastError is the reason of the last failed read, masked and shortened.
	LastError string `json:"last_error" gorm:"type:varchar(512);default:''"`
	// LastReason is the kind of the last failure (PriceSourceReason*).
	LastReason string `json:"last_reason" gorm:"type:varchar(16);default:''"`
	// LastCount is how many models the last good read listed.
	LastCount int `json:"last_count" gorm:"default:0"`
}

func (s *PriceSourceState) TableName() string {
	return "price_source_states"
}

// Kinds of a failed read.
const (
	// PriceSourceReasonRead: the source could not be read or parsed.
	PriceSourceReasonRead = "read"
	// PriceSourceReasonShrank: the list lost most of its models (see CheckChannelPriceList).
	PriceSourceReasonShrank = "shrank"
	// PriceSourceReasonZeroed: many models lost their price (see CheckChannelPriceList).
	PriceSourceReasonZeroed = "zeroed"
)

// ModelsDevPriceSource is the subject of the models.dev catalog.
const ModelsDevPriceSource = "source:models.dev"

// ChannelPriceSubject is the subject of a channel's upstream price list.
func ChannelPriceSubject(channelID int) string {
	return fmt.Sprintf("channel:%d", channelID)
}

// priceSourceNow is replaced in tests.
var priceSourceNow = utils.GetTimestamp

// RecordPriceSourceSuccess records a good read of the source that listed count models.
func RecordPriceSourceSuccess(subject string, count int) {
	if DB == nil {
		return
	}
	now := priceSourceNow()
	state := PriceSourceState{Subject: subject, LastAttemptAt: now, LastSuccessAt: now, LastCount: count}
	err := DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "subject"}},
		DoUpdates: clause.Assignments(map[string]any{
			"last_attempt_at": now, "last_success_at": now, "failing_since": 0, "failures": 0,
			"last_error": "", "last_reason": "", "last_count": count,
		}),
	}).Create(&state).Error
	if err != nil {
		return
	}
	// A good read ends the condition at once; the next review would close it too.
	_, _ = ReportAlert(PriceSourceAlertKind, subject, nil)
}

// RecordPriceSourceFailure records a failed read of the source.
func RecordPriceSourceFailure(subject, reason string, readErr error) {
	if DB == nil {
		return
	}
	if reason == "" {
		reason = PriceSourceReasonRead
	}
	msg := ""
	if readErr != nil {
		msg = utils.MaskSensitiveInfo(readErr.Error())
	}
	if len(msg) > 500 {
		msg = msg[:500]
	}
	now := priceSourceNow()
	_ = DB.Transaction(func(tx *gorm.DB) error {
		var state PriceSourceState
		err := tx.Where("subject = ?", subject).First(&state).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		state.Subject = subject
		state.LastAttemptAt = now
		if state.FailingSince == 0 {
			state.FailingSince = now
			state.Failures = 0
		}
		state.Failures++
		state.LastError, state.LastReason = msg, reason
		return tx.Save(&state).Error
	})
}

// GetPriceSourceState returns the state of the source, nil when it was never read.
func GetPriceSourceState(subject string) *PriceSourceState {
	var state PriceSourceState
	if err := DB.Where("subject = ?", subject).First(&state).Error; err != nil {
		return nil
	}
	return &state
}

func deletePriceSourceStates(channelIDs []int) {
	if len(channelIDs) == 0 {
		return
	}
	subjects := make([]string, len(channelIDs))
	for i, id := range channelIDs {
		subjects[i] = ChannelPriceSubject(id)
	}
	DB.Where("subject IN ?", subjects).Delete(&PriceSourceState{})
	DB.Where("kind = ? AND subject IN ?", PriceSourceAlertKind, subjects).Delete(&Alert{})
}

// A source is told about once it failed at least twice over at least this long: the daily
// syncs failed two days running, not one network blip or two quick retries.
const (
	priceSourceAlertFailures = 2
	priceSourceAlertAfter    = int64(20 * time.Hour / time.Second)
)

// PriceSourceAlertKind is the notice and alert kind of price sources that stopped working
// (notify.KindPriceSource).
const PriceSourceAlertKind = "price_source"

// FailingPriceSource is a source that stopped working, with the alert item that names it:
// "<reason>|<failing since>|<last success>", the same item while the condition lasts.
type FailingPriceSource struct {
	*PriceSourceState
	Item string
}

// ReviewPriceSources reports the sources that have been failing long enough as alerts,
// resolves the others, and returns the ones that are new (worth a notice). Only subjects
// watched accepts are reported: a source nothing reads on a schedule is not told about.
func ReviewPriceSources(watched func(subject string) bool) ([]FailingPriceSource, error) {
	var states []*PriceSourceState
	if err := DB.Where("failing_since > ?", 0).Find(&states).Error; err != nil {
		return nil, err
	}
	now := priceSourceNow()
	var failing []string
	var fresh []FailingPriceSource
	for _, s := range states {
		if (watched != nil && !watched(s.Subject)) || s.Failures < priceSourceAlertFailures || now-s.FailingSince < priceSourceAlertAfter {
			continue
		}
		item := strings.Join([]string{s.LastReason, fmt.Sprint(s.FailingSince), fmt.Sprint(s.LastSuccessAt)}, "|")
		failing = append(failing, s.Subject)
		added, err := ReportAlert(PriceSourceAlertKind, s.Subject, []string{item})
		if err != nil {
			return nil, err
		}
		if len(added) > 0 {
			fresh = append(fresh, FailingPriceSource{PriceSourceState: s, Item: item})
		}
	}
	if err := ResolveAlertsExcept(PriceSourceAlertKind, failing); err != nil {
		return nil, err
	}
	return fresh, nil
}

// A new price list is held when, against a stored list of at least priceListCheckMin models,
// it lists fewer than priceListShrinkShare of them, or the share of models with no price at all
// grows by priceListZeroedGrowth or more. Both are what a changed list format looks like; a
// platform that really dropped that many models is taken with a forced refresh.
const (
	priceListCheckMin     = 20
	priceListShrinkShare  = 0.5
	priceListZeroedGrowth = 0.2
)

// SuspiciousPriceListError is a new price list CheckChannelPriceList holds back.
type SuspiciousPriceListError struct {
	Reason string // PriceSourceReasonShrank or PriceSourceReasonZeroed
	// Before and After are the model counts (shrank) or the counts of unpriced models (zeroed).
	Before, After int
}

func (e *SuspiciousPriceListError) Error() string {
	if e.Reason == PriceSourceReasonShrank {
		return fmt.Sprintf("the price list went from %d to %d models; kept the stored list", e.Before, e.After)
	}
	return fmt.Sprintf("models without a price went from %d to %d; kept the stored list", e.Before, e.After)
}

// CheckChannelPriceList compares a new price list with the stored one and returns a
// *SuspiciousPriceListError when the new one should not replace it.
func CheckChannelPriceList(stored, next []*ChannelPrice) error {
	if len(stored) < priceListCheckMin || len(next) == 0 {
		return nil
	}
	if float64(len(next)) < float64(len(stored))*priceListShrinkShare {
		return &SuspiciousPriceListError{Reason: PriceSourceReasonShrank, Before: len(stored), After: len(next)}
	}
	before, after := unpricedCount(stored), unpricedCount(next)
	if float64(after)/float64(len(next))-float64(before)/float64(len(stored)) >= priceListZeroedGrowth {
		return &SuspiciousPriceListError{Reason: PriceSourceReasonZeroed, Before: before, After: after}
	}
	return nil
}

// unpricedCount counts the models with no price of any kind (listed as free, or a price field
// that was not read).
func unpricedCount(prices []*ChannelPrice) int {
	n := 0
	for _, p := range prices {
		if p != nil && !p.Other && p.Input == 0 && p.Output == 0 && p.CacheRead == 0 && p.CacheWrite == 0 &&
			p.Image == 0 && p.Request == 0 && p.VideoSecond == 0 && p.AudioSecond == 0 {
			n++
		}
	}
	return n
}
