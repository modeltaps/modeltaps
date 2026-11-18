package model

import (
	"slices"
	"sort"

	"github.com/modeltaps/modeltaps/common/utils"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Alert is a condition an admin is told about, kept while it lasts so it is told once: one row
// per kind (a notice kind) and subject (what it is about, as "channel:4"). Items are the facts
// that make it up, one string each ("gpt-5.5 → openai/gpt-5.5"). A check reports the items it
// finds now; only items that were not open before are worth a notice, so a condition that goes
// on, shrinks or comes back unchanged after a restart sends nothing. A check that finds no items
// resolves the alert; items that come back later open it again.
type Alert struct {
	Id         int                         `json:"id"`
	Kind       string                      `json:"kind" gorm:"type:varchar(32);not null;uniqueIndex:idx_alerts_kind_subject,priority:1"`
	Subject    string                      `json:"subject" gorm:"type:varchar(191);not null;uniqueIndex:idx_alerts_kind_subject,priority:2"`
	Items      datatypes.JSONSlice[string] `json:"items" gorm:"type:json"`
	OpenedAt   int64                       `json:"opened_at" gorm:"bigint;default:0"`
	NotifiedAt int64                       `json:"notified_at" gorm:"bigint;default:0"`
	ResolvedAt int64                       `json:"resolved_at" gorm:"bigint;default:0"`
	UpdatedAt  int64                       `json:"updated_at" gorm:"autoUpdateTime"`
}

func (a *Alert) TableName() string {
	return "alerts"
}

// Open reports whether the alert is open: it has items and was not resolved.
func (a *Alert) Open() bool {
	return a != nil && a.ResolvedAt == 0 && len(a.Items) > 0
}

// ReportAlert records the items a check found for (kind, subject) and returns the ones that
// are new: not among the open alert's items. No items resolves the alert. The caller notifies
// about the returned items (with whatever context it has) and nothing when there are none.
func ReportAlert(kind, subject string, items []string) ([]string, error) {
	items = normalizeAlertItems(items)
	var fresh []string
	err := DB.Transaction(func(tx *gorm.DB) error {
		var current Alert
		err := tx.Where("kind = ? AND subject = ?", kind, subject).First(&current).Error
		found := err == nil
		if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}
		now := utils.GetTimestamp()
		if len(items) == 0 {
			if !current.Open() {
				return nil
			}
			return tx.Model(&current).Updates(map[string]any{"items": datatypes.JSONSlice[string]{}, "resolved_at": now}).Error
		}
		var known []string
		if current.Open() {
			known = current.Items
		}
		for _, item := range items {
			if !slices.Contains(known, item) {
				fresh = append(fresh, item)
			}
		}
		updates := map[string]any{"items": datatypes.JSONSlice[string](items), "resolved_at": 0}
		if !current.Open() {
			updates["opened_at"] = now
		}
		if len(fresh) > 0 {
			updates["notified_at"] = now
		}
		if !found {
			return tx.Create(&Alert{Kind: kind, Subject: subject, Items: items, OpenedAt: now, NotifiedAt: now}).Error
		}
		return tx.Model(&current).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

// ResolveAlertsExcept resolves the open alerts of kind whose subject is not in keep: the
// subjects a full check no longer finds anything for.
func ResolveAlertsExcept(kind string, keep []string) error {
	q := DB.Model(&Alert{}).Where("kind = ? AND resolved_at = ?", kind, 0)
	if len(keep) > 0 {
		q = q.Where("subject NOT IN ?", keep)
	}
	return q.Updates(map[string]any{"items": datatypes.JSONSlice[string]{}, "resolved_at": utils.GetTimestamp()}).Error
}

// OpenAlerts lists the open alerts of kind (all kinds when empty), newest first.
func OpenAlerts(kind string) ([]*Alert, error) {
	var out []*Alert
	q := DB.Where("resolved_at = ?", 0)
	if kind != "" {
		q = q.Where("kind = ?", kind)
	}
	if err := q.Order("opened_at DESC").Find(&out).Error; err != nil {
		return nil, err
	}
	return slices.DeleteFunc(out, func(a *Alert) bool { return !a.Open() }), nil
}

func normalizeAlertItems(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item != "" && !slices.Contains(out, item) {
			out = append(out, item)
		}
	}
	sort.Strings(out)
	return out
}

// AlertItems is the items of the open alert of (kind, subject); nil when there is none.
func AlertItems(kind, subject string) ([]string, error) {
	var current Alert
	err := DB.Where("kind = ? AND subject = ?", kind, subject).First(&current).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !current.Open() {
		return nil, nil
	}
	return current.Items, nil
}
