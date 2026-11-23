package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

var (
	ErrSupplyBatchAction = errors.New("action must be add or remove")
)

// SupplyBatch adds or removes one channel as a supply of several public models.
type SupplyBatch struct {
	Action    string   `json:"action"`
	ChannelId int      `json:"channel_id"`
	Models    []string `json:"models"`
}

type SupplyBatchResult struct {
	// Changed is how many supplies were added or removed.
	Changed int `json:"changed"`
	// Skipped are the models left as they were: already supplied by the channel (add), not
	// supplied by it (remove), or not a public model.
	Skipped []string `json:"skipped"`
}

// ApplySupplyBatch adds the channel as a supply of every listed public model that it does not
// supply yet, or removes its supplies of them. A new supply is sent the upstream name the add
// form would suggest and takes the channel's priority and weight, so it follows the channel.
func ApplySupplyBatch(req SupplyBatch) (*SupplyBatchResult, error) {
	if req.Action != "add" && req.Action != "remove" {
		return nil, ErrSupplyBatchAction
	}
	channel, err := GetChannelById(req.ChannelId)
	if err != nil || channel == nil {
		return nil, ErrSupplyChannelNotFound
	}
	resolver, err := NewCanonicalResolver()
	if err != nil {
		return nil, err
	}
	result := &SupplyBatchResult{Skipped: []string{}}
	seen := map[string]bool{}
	err = DB.Transaction(func(tx *gorm.DB) error {
		for _, name := range req.Models {
			name = strings.TrimSpace(name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			if resolver.publicModel(name) == nil {
				result.Skipped = append(result.Skipped, name)
				continue
			}
			if req.Action == "remove" {
				deleted := tx.Where("model = ? AND channel_id = ?", name, channel.Id).Delete(&ModelSupply{})
				if deleted.Error != nil {
					return deleted.Error
				}
				if deleted.RowsAffected == 0 {
					result.Skipped = append(result.Skipped, name)
					continue
				}
				result.Changed += int(deleted.RowsAffected)
				continue
			}
			var count int64
			if err := tx.Model(&ModelSupply{}).Where("model = ? AND channel_id = ?", name, channel.Id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped = append(result.Skipped, name)
				continue
			}
			supply := &ModelSupply{
				Model: name, ChannelId: channel.Id, UpstreamModel: SuggestSupplyUpstream(name, channel).UpstreamModel,
				Priority: channel.GetPriority(), Weight: uint(channel.GetWeight()),
				Enabled: true, Source: ModelSupplySourceManual,
			}
			if err := tx.Create(supply).Error; err != nil {
				return err
			}
			result.Changed++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if result.Changed > 0 {
		reloadSupplies()
	}
	return result, nil
}
