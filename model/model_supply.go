package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/gorm"
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
	Enabled bool `json:"enabled" gorm:"not null"`
	// Pending marks a supply an intake rule added for an admin to confirm. It is off (Enabled
	// false) until then; any admin update of the supply clears it.
	Pending bool `json:"pending" gorm:"not null;default:false"`
	// JoinLast routes the supply after every other supply of its model. Intake rules set it on
	// supplies added to a model other channels already supply, so a new arrival does not take the
	// model's traffic. Models that follow the allocation policy apply it when the policy's numbers
	// are computed; for the others the supply was given the lowest priority when it was added.
	JoinLast bool `json:"join_last" gorm:"not null;default:false"`
	// UpstreamMissing counts the model checks in a row whose upstream list lacked UpstreamModel;
	// UpstreamGoneAt is when that count reached upstreamGoneChecks, 0 while the upstream lists
	// it. A supply whose upstream is gone is not routed; it comes back when the name returns.
	UpstreamMissing int    `json:"upstream_missing" gorm:"default:0"`
	UpstreamGoneAt  int64  `json:"upstream_gone_at" gorm:"bigint;default:0"`
	Source          string `json:"source" gorm:"type:varchar(16);default:'manual'"`
	Note            string `json:"note" gorm:"type:text"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
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
// deleted channel leaves the name and groups empty and the status 0.
type ModelSupplyView struct {
	*ModelSupply
	ChannelName   string `json:"channel_name"`
	ChannelStatus int    `json:"channel_status"`
	ChannelType   int    `json:"channel_type"`
	ChannelTag    string `json:"channel_tag"`
	// ChannelGroup is the channel's comma-separated user groups.
	ChannelGroup string `json:"channel_group"`
	// ChannelPriority and ChannelWeight are the channel's own numbers. A supply whose priority
	// and weight equal them follows the channel (see SyncSuppliesWithChannel).
	ChannelPriority int64 `json:"channel_priority"`
	ChannelWeight   int64 `json:"channel_weight"`
	// FollowsPolicy is true when the priority and weight above come from the allocation policy
	// rather than from the supply row.
	FollowsPolicy bool `json:"follows_policy"`
	// UpstreamPrice is what the channel's upstream charges for UpstreamModel, from the channel's
	// stored price list; nil when the list has no entry for it. UpstreamPriced is true when the
	// channel has a price list at all, so a missing entry can be told from an upstream that
	// publishes no prices.
	UpstreamPrice  *ChannelPrice `json:"upstream_price"`
	UpstreamPriced bool          `json:"upstream_priced"`
	// SellInputPrice and SellOutputPrice are the public model's base sell price per million
	// tokens, to compare with the upstream price; nil when it is not sold per token.
	SellInputPrice  *float64 `json:"sell_input_price"`
	SellOutputPrice *float64 `json:"sell_output_price"`
	// Cost is what the channel charges for the model with its fee, against the quoted price.
	Cost *SupplyCost `json:"cost"`
	// Traffic says whether the supply takes requests now (SupplyTraffic*).
	Traffic string `json:"traffic"`
}

// GetModelSupplyViews lists supplies like GetModelSupplies, each with its channel's name,
// status, type and tag, reading the channels in one query.
func GetModelSupplyViews(modelName string, channelId int) ([]*ModelSupplyView, error) {
	supplies, err := GetModelSupplies(modelName, channelId)
	if err != nil {
		return nil, err
	}
	// Show the numbers supplies route with: the policy's for models that follow it.
	policyNumbers := map[int]supplyNumbers{}
	if config.ModelSupplyEnabled && len(supplies) > 0 {
		names := map[string]bool{}
		for _, s := range supplies {
			names[s.Model] = true
		}
		list := make([]string, 0, len(names))
		for name := range names {
			list = append(list, name)
		}
		if policyNumbers, err = allocatedNumbers(list); err != nil {
			return nil, err
		}
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
		columns := append([]string{"id", "name", "status", "type", "tag", "group", "priority", "weight"}, supplyCostChannelColumns...)
		if err := DB.Select(columns).Where("id IN ?", ids).Find(&channels).Error; err != nil {
			return nil, err
		}
		for _, ch := range channels {
			byId[ch.Id] = ch
		}
	}
	channelList := make([]*Channel, 0, len(byId))
	for _, ch := range byId {
		channelList = append(channelList, ch)
	}
	prices, priced, err := supplyUpstreamPrices(supplies, ids, channelList)
	if err != nil {
		return nil, err
	}
	type sellPrice struct{ in, out *float64 }
	sells := map[string]sellPrice{}
	views := make([]*ModelSupplyView, 0, len(supplies))
	for _, s := range supplies {
		view := &ModelSupplyView{ModelSupply: s}
		view.UpstreamPrice = prices[channelPriceKey{s.ChannelId, supplyUpstreamName(s)}]
		view.UpstreamPriced = priced[s.ChannelId]
		sell, ok := sells[s.Model]
		if !ok {
			sell.in, sell.out = tokenSellPrice(s.Model)
			sells[s.Model] = sell
		}
		view.SellInputPrice, view.SellOutputPrice = sell.in, sell.out
		if n, ok := policyNumbers[s.Id]; ok {
			copied := *s
			copied.Priority, copied.Weight = n.Priority, n.Weight
			view.ModelSupply, view.FollowsPolicy = &copied, true
		}
		if ch := byId[s.ChannelId]; ch != nil {
			view.ChannelName, view.ChannelStatus, view.ChannelType, view.ChannelTag = ch.Name, ch.Status, ch.Type, ch.Tag
			view.ChannelGroup = ch.Group
			view.ChannelPriority, view.ChannelWeight = ch.GetPriority(), ch.GetWeight()
		}
		view.Cost = supplyCostOf(s.Model, byId[s.ChannelId], view.UpstreamPrice)
		views = append(views, view)
	}
	markSupplyTraffic(views)
	return views, nil
}

// supplyUpstreamName is the name the supply sends upstream: its upstream model, else the model.
func supplyUpstreamName(s *ModelSupply) string {
	if s.UpstreamModel != "" {
		return s.UpstreamModel
	}
	return s.Model
}

type channelPriceKey struct {
	channelID int
	model     string
}

// supplyUpstreamPrices reads the stored upstream prices of the supplies' upstream models on
// their channels, and which of the channels have a price list at all.
func supplyUpstreamPrices(supplies []*ModelSupply, channelIDs []int, channelTypes []*Channel) (map[channelPriceKey]*ChannelPrice, map[int]bool, error) {
	prices := map[channelPriceKey]*ChannelPrice{}
	priced := map[int]bool{}
	if len(supplies) == 0 {
		return prices, priced, nil
	}
	var pricedIDs []int
	if err := DB.Model(&ChannelPrice{}).Where("channel_id IN ?", channelIDs).Distinct().Pluck("channel_id", &pricedIDs).Error; err != nil {
		return nil, nil, err
	}
	for _, id := range pricedIDs {
		priced[id] = true
	}
	if len(pricedIDs) == 0 {
		return prices, priced, nil
	}
	names := make([]string, 0, len(supplies))
	for _, s := range supplies {
		if priced[s.ChannelId] {
			names = append(names, supplyUpstreamName(s))
		}
	}
	var rows []*ChannelPrice
	if err := DB.Where("channel_id IN ? AND model IN ?", pricedIDs, names).Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	types := map[int]int{}
	for _, ch := range channelTypes {
		types[ch.Id] = ch.Type
	}
	for _, row := range rows {
		DescribeChannelPrices(types[row.ChannelId], row)
		prices[channelPriceKey{row.ChannelId, row.Model}] = row
	}
	return prices, priced, nil
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
// enabled, note and whether it joins last; it also clears Pending. Model and channel identify the supply and stay as they are.
func UpdateModelSupply(supply *ModelSupply) error {
	current, err := GetModelSupply(supply.Id)
	if err != nil {
		return err
	}
	upstream := strings.TrimSpace(supply.UpstreamModel)
	if upstream == "" {
		return ErrSupplyUpstreamMissing
	}
	if err := takeManualOnRenumber(current.Model, supply); err != nil {
		return err
	}
	// An admin's update decides a pending supply: it stays as the update leaves it.
	err = DB.Model(current).Select("upstream_model", "priority", "weight", "enabled", "note", "pending", "join_last").Updates(&ModelSupply{
		UpstreamModel: upstream,
		Priority:      supply.Priority,
		Weight:        supply.Weight,
		Enabled:       supply.Enabled,
		Note:          supply.Note,
		JoinLast:      supply.JoinLast,
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

// SyncSuppliesWithChannel carries a change of a channel's priority or weight to its supplies.
// A supply follows the channel while it has the channel's number: a supply still at the old
// priority moves to the new one, and likewise for the weight. Supplies that were given their own
// priority or weight on the model page are left alone. It returns how many supplies changed and
// does not rebuild the routing table; callers that changed the channel do.
func SyncSuppliesWithChannel(channelId int, oldPriority, newPriority, oldWeight, newWeight int64) (int64, error) {
	var changed int64
	if oldPriority != newPriority {
		result := DB.Model(&ModelSupply{}).Where("channel_id = ? AND priority = ?", channelId, oldPriority).
			Update("priority", newPriority)
		if result.Error != nil {
			return changed, result.Error
		}
		changed += result.RowsAffected
	}
	if oldWeight != newWeight {
		// An unset weight counts as 1, like EffectiveWeight.
		weights := []int64{oldWeight}
		if oldWeight == 1 {
			weights = append(weights, 0)
		}
		result := DB.Model(&ModelSupply{}).Where("channel_id = ? AND weight IN ?", channelId, weights).
			Update("weight", newWeight)
		if result.Error != nil {
			return changed, result.Error
		}
		changed += result.RowsAffected
	}
	return changed, nil
}

// channelNumbers reads the priority and weight of channels before a change, by id.
func channelNumbers(query *gorm.DB) (map[int][2]int64, error) {
	var channels []*Channel
	if err := query.Select("id", "priority", "weight").Find(&channels).Error; err != nil {
		return nil, err
	}
	numbers := make(map[int][2]int64, len(channels))
	for _, ch := range channels {
		numbers[ch.Id] = [2]int64{ch.GetPriority(), ch.GetWeight()}
	}
	return numbers, nil
}

// reloadSupplies rebuilds the routing table after a supply change while supplies drive it.
func reloadSupplies() {
	if config.ModelSupplyEnabled && ChannelGroup.Loaded() {
		ChannelGroup.Load()
	}
}
