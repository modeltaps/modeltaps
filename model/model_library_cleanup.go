package model

import (
	"encoding/json"
	"fmt"

	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// Before the model library existed, the models.dev sync put every model of eleven provider lists
// into the catalog as drafts, one row per platform spelling. The library now holds those, and
// admins add what they publish. This migration removes the synced drafts nothing uses: a row is
// kept when it is not a models.dev draft, is locked, is an alias or routing-policy row, is the
// target of an alias, has a supply, or is named (as a model or a mapping key) by any channel.

type catalogCleanupRow struct {
	Id      int
	Model   string
	Source  string
	Status  string
	Locked  bool
	AliasOf string
	Kind    string
}

// staleCatalogRows returns the catalog rows the cleanup removes.
func staleCatalogRows(tx *gorm.DB) ([]catalogCleanupRow, error) {
	var rows []catalogCleanupRow
	if err := tx.Table("model_info").Select("id", "model", "source", "status", "locked", "alias_of", "kind").Find(&rows).Error; err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, r := range rows {
		if r.AliasOf != "" {
			used[r.AliasOf] = true
		}
	}
	var supplied []string
	if err := tx.Table("model_supplies").Distinct("model").Pluck("model", &supplied).Error; err != nil {
		return nil, err
	}
	for _, m := range supplied {
		used[m] = true
	}
	type channelNames struct {
		Models       string
		ModelMapping *string
	}
	var channels []channelNames
	// Deleted channels included: a restored channel should find its models again.
	if err := tx.Table("channels").Select("models", "model_mapping").Find(&channels).Error; err != nil {
		return nil, err
	}
	for _, ch := range channels {
		for _, name := range SplitChannelModels(ch.Models) {
			used[name] = true
		}
		if ch.ModelMapping != nil && *ch.ModelMapping != "" {
			mapping := map[string]any{}
			if json.Unmarshal([]byte(*ch.ModelMapping), &mapping) == nil {
				for k := range mapping {
					used[k] = true
				}
			}
		}
	}
	var stale []catalogCleanupRow
	for _, r := range rows {
		if r.Source != ModelInfoSourceModelsDev || r.Status != ModelStatusDraft || r.Locked ||
			r.AliasOf != "" || r.Kind != "" || used[r.Model] {
			continue
		}
		stale = append(stale, r)
	}
	return stale, nil
}

func cleanupSyncedCatalogDrafts() *gormigrate.Migration {
	return &gormigrate.Migration{
		ID: "202611120001",
		Migrate: func(tx *gorm.DB) error {
			stale, err := staleCatalogRows(tx)
			if err != nil {
				return err
			}
			if len(stale) == 0 {
				return nil
			}
			ids := make([]int, len(stale))
			for i, r := range stale {
				ids[i] = r.Id
			}
			for start := 0; start < len(ids); start += 500 {
				end := min(start+500, len(ids))
				if err := tx.Where("id IN ?", ids[start:end]).Delete(&ModelInfo{}).Error; err != nil {
					return err
				}
			}
			logger.SysLog(fmt.Sprintf("removed %d unused synced draft models from the catalog; they stay in the model library", len(stale)))
			return nil
		},
	}
}
