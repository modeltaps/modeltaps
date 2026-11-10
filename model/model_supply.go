package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

const (
	ModelSupplySourceManual   = "manual"
	ModelSupplySourceMigrated = "migrated"

	// SupplyIdContextKey holds the id of the supply a request was sent through.
	SupplyIdContextKey = "supply_id"
)

// ModelSupply says that a channel supplies a public model, and under which upstream name. While
// ModelSupplyEnabled is on, supplies replace the channels' models and model_mapping fields as
// the source of the routing table: priority forms the tiers (higher first), weight splits
// traffic within a tier. A model has at most one supply per channel.
type ModelSupply struct {
	Id            int    `json:"id"`
	Model         string `json:"model" gorm:"type:varchar(100);not null;uniqueIndex:idx_model_supplies_model_channel,priority:1"`
	ChannelId     int    `json:"channel_id" gorm:"not null;index;uniqueIndex:idx_model_supplies_model_channel,priority:2"`
	UpstreamModel string `json:"upstream_model" gorm:"type:varchar(255);not null"`
	Priority      int64  `json:"priority" gorm:"bigint;default:0"`
	Weight        uint   `json:"weight" gorm:"default:1"`
	// Enabled has no column default: gorm skips false on insert, and a default of true would
	// silently turn a disabled supply back on.
	Enabled   bool   `json:"enabled" gorm:"not null"`
	Source    string `json:"source" gorm:"type:varchar(16);default:'manual'"`
	Note      string `json:"note" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (s *ModelSupply) TableName() string {
	return "model_supplies"
}

// EffectiveWeight treats an unset weight as 1, like channels do.
func (s *ModelSupply) EffectiveWeight() int {
	if s.Weight == 0 {
		return 1
	}
	return int(s.Weight)
}

var (
	ErrSupplyModelNotFound   = errors.New("the public model does not exist")
	ErrSupplyModelNotPublic  = errors.New("supplies attach to public models, not to aliases or routing policies")
	ErrSupplyChannelNotFound = errors.New("the channel does not exist")
	ErrSupplyUpstreamMissing = errors.New("the upstream model name is required")
	ErrSupplyExists          = errors.New("this channel already supplies the model")
)

// validateSupplyTarget checks that a supply points at a public model row and an existing channel.
func validateSupplyTarget(modelName string, channelId int, upstream string) error {
	info, err := GetModelInfoByModel(modelName)
	if err != nil || info == nil {
		return ErrSupplyModelNotFound
	}
	if info.AliasOf != "" || info.Kind != "" {
		return ErrSupplyModelNotPublic
	}
	var count int64
	if err := DB.Model(&Channel{}).Where("id = ?", channelId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrSupplyChannelNotFound
	}
	if strings.TrimSpace(upstream) == "" {
		return ErrSupplyUpstreamMissing
	}
	return nil
}

// GetModelSupplies lists supplies, filtered by model and channel when those are set, ordered
// by model, then priority (highest first), then id.
func GetModelSupplies(modelName string, channelId int) ([]*ModelSupply, error) {
	var supplies []*ModelSupply
	query := DB.Model(&ModelSupply{})
	if modelName != "" {
		query = query.Where("model = ?", modelName)
	}
	if channelId > 0 {
		query = query.Where("channel_id = ?", channelId)
	}
	err := query.Order("model asc, priority desc, id asc").Find(&supplies).Error
	return supplies, err
}

// ModelSupplyView is a supply with the channel facts the console shows on every row. A
// deleted channel leaves the name empty and the status 0.
type ModelSupplyView struct {
	*ModelSupply
	ChannelName   string `json:"channel_name"`
	ChannelStatus int    `json:"channel_status"`
	ChannelType   int    `json:"channel_type"`
	ChannelTag    string `json:"channel_tag"`
}

// GetModelSupplyViews lists supplies like GetModelSupplies, each with its channel's name,
// status, type and tag, reading the channels in one query.
func GetModelSupplyViews(modelName string, channelId int) ([]*ModelSupplyView, error) {
	supplies, err := GetModelSupplies(modelName, channelId)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(supplies))
	seen := map[int]bool{}
	for _, s := range supplies {
		if !seen[s.ChannelId] {
			seen[s.ChannelId] = true
			ids = append(ids, s.ChannelId)
		}
	}
	byId := map[int]*Channel{}
	if len(ids) > 0 {
		var channels []*Channel
		if err := DB.Select("id", "name", "status", "type", "tag").Where("id IN ?", ids).Find(&channels).Error; err != nil {
			return nil, err
		}
		for _, ch := range channels {
			byId[ch.Id] = ch
		}
	}
	views := make([]*ModelSupplyView, 0, len(supplies))
	for _, s := range supplies {
		view := &ModelSupplyView{ModelSupply: s}
		if ch := byId[s.ChannelId]; ch != nil {
			view.ChannelName, view.ChannelStatus, view.ChannelType, view.ChannelTag = ch.Name, ch.Status, ch.Type, ch.Tag
		}
		views = append(views, view)
	}
	return views, nil
}

func GetModelSupply(id int) (*ModelSupply, error) {
	var supply ModelSupply
	if err := DB.First(&supply, id).Error; err != nil {
		return nil, err
	}
	return &supply, nil
}

// CreateModelSupply adds a supply after validating its model and channel.
func CreateModelSupply(supply *ModelSupply) error {
	supply.Model = strings.TrimSpace(supply.Model)
	supply.UpstreamModel = strings.TrimSpace(supply.UpstreamModel)
	if err := validateSupplyTarget(supply.Model, supply.ChannelId, supply.UpstreamModel); err != nil {
		return err
	}
	var count int64
	if err := DB.Model(&ModelSupply{}).Where("model = ? AND channel_id = ?", supply.Model, supply.ChannelId).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrSupplyExists
	}
	if supply.Source == "" {
		supply.Source = ModelSupplySourceManual
	}
	if err := DB.Create(supply).Error; err != nil {
		return err
	}
	reloadSupplies()
	return nil
}

// UpdateModelSupply changes what can change on a supply: upstream name, priority, weight,
// enabled and note. Model and channel identify the supply and stay as they are.
func UpdateModelSupply(supply *ModelSupply) error {
	current, err := GetModelSupply(supply.Id)
	if err != nil {
		return err
	}
	upstream := strings.TrimSpace(supply.UpstreamModel)
	if upstream == "" {
		return ErrSupplyUpstreamMissing
	}
	err = DB.Model(current).Select("upstream_model", "priority", "weight", "enabled", "note").Updates(&ModelSupply{
		UpstreamModel: upstream,
		Priority:      supply.Priority,
		Weight:        supply.Weight,
		Enabled:       supply.Enabled,
		Note:          supply.Note,
	}).Error
	if err != nil {
		return err
	}
	reloadSupplies()
	return nil
}

func DeleteModelSupply(id int) error {
	result := DB.Delete(&ModelSupply{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("supply %d not found", id)
	}
	reloadSupplies()
	return nil
}

// reloadSupplies rebuilds the routing table after a supply change while supplies drive it.
func reloadSupplies() {
	if config.ModelSupplyEnabled && ChannelGroup.Loaded() {
		ChannelGroup.Load()
	}
}
