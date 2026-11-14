package model

import (
	"strings"
	"time"

	"gorm.io/gorm/clause"
)

// ChannelModelAlias records an admin's decision about one upstream name on one channel: Excluded
// false confirms that the name is the public model (matched as exact from then on), Excluded true
// rejects a match the rules found. Decisions are per channel on purpose: one relay's private
// spelling must not change how other channels are matched.
type ChannelModelAlias struct {
	Id            int    `json:"id" gorm:"primaryKey"`
	ChannelId     int    `json:"channel_id" gorm:"uniqueIndex:idx_channel_model_alias,priority:1;not null"`
	UpstreamModel string `json:"upstream_model" gorm:"type:varchar(255);uniqueIndex:idx_channel_model_alias,priority:2;not null"`
	Model         string `json:"model" gorm:"type:varchar(255);uniqueIndex:idx_channel_model_alias,priority:3;not null"`
	Excluded      bool   `json:"excluded" gorm:"default:false"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
}

// channelAliasKey identifies an alias row by its lower-cased names.
type channelAliasKey struct {
	channel  int
	upstream string
	model    string
}

func newChannelAliasKey(channelID int, upstream, model string) channelAliasKey {
	return channelAliasKey{channelID, strings.ToLower(strings.TrimSpace(upstream)), strings.ToLower(strings.TrimSpace(model))}
}

// loadChannelAliases returns every alias decision, keyed by channel, upstream and model.
func loadChannelAliases() (map[channelAliasKey]bool, error) {
	var rows []*ChannelModelAlias
	if err := DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[channelAliasKey]bool, len(rows))
	for _, r := range rows {
		out[newChannelAliasKey(r.ChannelId, r.UpstreamModel, r.Model)] = r.Excluded
	}
	return out, nil
}

// SaveChannelAlias records (or changes) a decision for one upstream name on one channel.
func SaveChannelAlias(channelID int, upstream, model string, excluded bool) error {
	row := &ChannelModelAlias{
		ChannelId:     channelID,
		UpstreamModel: strings.TrimSpace(upstream),
		Model:         strings.TrimSpace(model),
		Excluded:      excluded,
		CreatedAt:     time.Now().Unix(),
	}
	return DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "channel_id"}, {Name: "upstream_model"}, {Name: "model"}},
		DoUpdates: clause.AssignmentColumns([]string{"excluded"}),
	}).Create(row).Error
}

// deleteChannelAliases drops the decisions of deleted channels.
func deleteChannelAliases(channelIDs []int) {
	if len(channelIDs) > 0 {
		DB.Where("channel_id IN ?", channelIDs).Delete(&ChannelModelAlias{})
	}
}
