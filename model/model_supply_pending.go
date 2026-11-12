package model

import (
	"errors"
	"slices"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// New upstream models do not become public on their own: the drift check collects them per
// channel (ModelDriftResult.NewModels), and while no supply on that channel sends the name
// upstream it waits here for an admin to attach it to a public model or to ignore it.

// PendingUpstreamModel is one upstream model name waiting on one or more channels.
type PendingUpstreamModel struct {
	UpstreamModel string            `json:"upstream_model"`
	Suggestion    PendingSuggestion `json:"suggestion"`
	Channels      []PendingChannel  `json:"channels"`
}

// PendingSuggestion is the public model the name most likely belongs to.
type PendingSuggestion struct {
	// Model is the name the canonical resolver finds, or the upstream name itself.
	Model string `json:"model"`
	// Source is how Model was found (CanonicalSourceAlias, Seed, Bare); empty for the name itself.
	Source string `json:"source"`
	// Exists says Model is already a public model; attaching to it otherwise creates it.
	Exists bool   `json:"exists"`
	Status string `json:"status,omitempty"`
}

type PendingChannel struct {
	ChannelId     int    `json:"channel_id"`
	ChannelName   string `json:"channel_name"`
	ChannelStatus int    `json:"channel_status"`
	ChannelType   int    `json:"channel_type"`
	ChannelGroup  string `json:"channel_group"`
}

var (
	ErrPendingNotFound = errors.New("this upstream model is not waiting on any of the channels")
	ErrModelStatus     = errors.New("invalid status")
)

// suppliedUpstreamNames maps each channel to the upstream names its supplies send.
func suppliedUpstreamNames(channelId int) (map[int]map[string]bool, error) {
	var supplies []*ModelSupply
	query := DB.Select("channel_id", "upstream_model")
	if channelId > 0 {
		query = query.Where("channel_id = ?", channelId)
	}
	if err := query.Find(&supplies).Error; err != nil {
		return nil, err
	}
	out := map[int]map[string]bool{}
	for _, s := range supplies {
		if out[s.ChannelId] == nil {
			out[s.ChannelId] = map[string]bool{}
		}
		out[s.ChannelId][s.UpstreamModel] = true
	}
	return out, nil
}

// WithoutSuppliedModels drops the names a supply on the channel already sends upstream, so the
// drift check does not keep reporting a model once it is attached.
func WithoutSuppliedModels(channelId int, names []string) []string {
	if len(names) == 0 {
		return names
	}
	supplied, err := suppliedUpstreamNames(channelId)
	if err != nil || len(supplied[channelId]) == 0 {
		return names
	}
	var kept []string
	for _, name := range names {
		if !supplied[channelId][name] {
			kept = append(kept, name)
		}
	}
	return kept
}

// GetPendingUpstreamModels lists the new upstream models waiting on channels, by name.
func GetPendingUpstreamModels() ([]*PendingUpstreamModel, error) {
	var channels []*Channel
	if err := DB.Select("id", "name", "status", "type", "group", "model_drift").Order("id").Find(&channels).Error; err != nil {
		return nil, err
	}
	supplied, err := suppliedUpstreamNames(0)
	if err != nil {
		return nil, err
	}
	byName := map[string]*PendingUpstreamModel{}
	for _, channel := range channels {
		drift := channel.GetModelDrift()
		if drift == nil {
			continue
		}
		for _, name := range drift.NewModels {
			name = strings.TrimSpace(name)
			if name == "" || supplied[channel.Id][name] {
				continue
			}
			entry := byName[name]
			if entry == nil {
				entry = &PendingUpstreamModel{UpstreamModel: name}
				byName[name] = entry
			}
			if slices.ContainsFunc(entry.Channels, func(c PendingChannel) bool { return c.ChannelId == channel.Id }) {
				continue
			}
			entry.Channels = append(entry.Channels, PendingChannel{
				ChannelId: channel.Id, ChannelName: channel.Name, ChannelStatus: channel.Status,
				ChannelType: channel.Type, ChannelGroup: channel.Group,
			})
		}
	}

	var infos []*ModelInfo
	if err := DB.Find(&infos).Error; err != nil {
		return nil, err
	}
	var seed *CatalogSeed
	if loaded, err := LoadCatalogSeed(); err == nil {
		seed = loaded
	}
	resolver := newCanonicalResolver(infos, seed)

	pending := make([]*PendingUpstreamModel, 0, len(byName))
	for _, entry := range byName {
		entry.Suggestion = suggestPublicModel(resolver, entry.UpstreamModel)
		pending = append(pending, entry)
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].UpstreamModel < pending[j].UpstreamModel })
	return pending, nil
}

func suggestPublicModel(r *CanonicalResolver, upstream string) PendingSuggestion {
	suggestion := PendingSuggestion{Model: upstream}
	if match := r.Resolve(upstream); match.Source != CanonicalSourceNone {
		if info := r.byModel[match.Canonical]; info == nil || info.Kind == "" {
			suggestion.Model, suggestion.Source = match.Canonical, match.Source
		}
	}
	if info := r.byModel[suggestion.Model]; info != nil && info.AliasOf == "" && info.Kind == "" {
		suggestion.Exists, suggestion.Status = true, info.Status
	}
	return suggestion
}

// PendingAttach attaches an upstream model to a public model on some channels.
type PendingAttach struct {
	UpstreamModel string `json:"upstream_model"`
	Model         string `json:"model"`
	// ChannelIds are the channels to add a supply on; empty means every channel it waits on.
	ChannelIds []int `json:"channel_ids"`
	// Create allows creating Model when it is not a public model yet, with Status (draft when
	// empty, so nothing is listed before it has a price and is published).
	Create bool   `json:"create"`
	Status string `json:"status"`
}

type PendingAttachResult struct {
	Model        string         `json:"model"`
	CreatedModel bool           `json:"created_model"`
	Supplies     []*ModelSupply `json:"supplies"`
	// Skipped are channels that already supply Model (under another upstream name: a model has
	// one supply per channel); the name keeps waiting there.
	Skipped []int `json:"skipped"`
}

// AttachPendingUpstreamModel adds a supply of the public model on each channel, sending the
// upstream name, and takes the name off those channels' new models. Supplies take the
// channel's priority and weight, like the migration.
func AttachPendingUpstreamModel(req PendingAttach) (*PendingAttachResult, error) {
	upstream := strings.TrimSpace(req.UpstreamModel)
	if upstream == "" {
		return nil, ErrSupplyUpstreamMissing
	}
	name := strings.TrimSpace(req.Model)
	if name == "" {
		return nil, ErrSupplyModelNotFound
	}
	name, _ = ResolveCanonicalModel(name)
	status := req.Status
	if status == "" {
		status = ModelStatusDraft
	}
	if !IsValidModelStatus(status) {
		return nil, ErrModelStatus
	}
	info, _ := GetModelInfoByModel(name)
	switch {
	case info == nil && !req.Create:
		return nil, ErrSupplyModelNotFound
	case info != nil && (info.AliasOf != "" || info.Kind != ""):
		return nil, ErrSupplyModelNotPublic
	}

	channelIds := req.ChannelIds
	if len(channelIds) == 0 {
		pending, err := GetPendingUpstreamModels()
		if err != nil {
			return nil, err
		}
		for _, entry := range pending {
			if entry.UpstreamModel == upstream {
				for _, c := range entry.Channels {
					channelIds = append(channelIds, c.ChannelId)
				}
			}
		}
		if len(channelIds) == 0 {
			return nil, ErrPendingNotFound
		}
	}
	channels := make([]*Channel, 0, len(channelIds))
	for _, id := range channelIds {
		if slices.ContainsFunc(channels, func(c *Channel) bool { return c.Id == id }) {
			continue
		}
		channel, err := GetChannelById(id)
		if err != nil || channel == nil {
			return nil, ErrSupplyChannelNotFound
		}
		channels = append(channels, channel)
	}

	result := &PendingAttachResult{Model: name, Supplies: []*ModelSupply{}, Skipped: []int{}}
	err := DB.Transaction(func(tx *gorm.DB) error {
		if info == nil {
			created := &ModelInfo{Model: name, Name: name, Status: status, SupplyMode: SupplyModeDefault, Source: ModelInfoSourceManual}
			if err := tx.Create(created).Error; err != nil {
				return err
			}
			result.CreatedModel = true
		}
		for _, channel := range channels {
			var count int64
			if err := tx.Model(&ModelSupply{}).Where("model = ? AND channel_id = ?", name, channel.Id).Count(&count).Error; err != nil {
				return err
			}
			if count > 0 {
				result.Skipped = append(result.Skipped, channel.Id)
				continue
			}
			supply := &ModelSupply{
				Model: name, ChannelId: channel.Id, UpstreamModel: upstream,
				Priority: channel.GetPriority(), Weight: uint(channel.GetWeight()),
				Enabled: true, Source: ModelSupplySourceManual,
			}
			if err := tx.Create(supply).Error; err != nil {
				return err
			}
			result.Supplies = append(result.Supplies, supply)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, channel := range channels {
		if !slices.Contains(result.Skipped, channel.Id) {
			_ = channel.RemoveNewModels(upstream)
		}
	}
	if result.CreatedModel {
		refreshCatalogCaches()
	}
	reloadSupplies()
	return result, nil
}

// IgnorePendingUpstreamModel takes the upstream name off the channels' new models (every channel
// it waits on when channelIds is empty) and returns how many channels changed. The drift check
// does not report it again unless it leaves the upstream list and comes back.
func IgnorePendingUpstreamModel(upstream string, channelIds []int) (int, error) {
	upstream = strings.TrimSpace(upstream)
	if upstream == "" {
		return 0, ErrSupplyUpstreamMissing
	}
	var channels []*Channel
	query := DB.Select("id", "model_drift")
	if len(channelIds) > 0 {
		query = query.Where("id IN ?", channelIds)
	}
	if err := query.Find(&channels).Error; err != nil {
		return 0, err
	}
	changed := 0
	for _, channel := range channels {
		drift := channel.GetModelDrift()
		if drift == nil || !slices.Contains(drift.NewModels, upstream) {
			continue
		}
		if err := channel.RemoveNewModels(upstream); err != nil {
			return changed, err
		}
		changed++
	}
	if changed == 0 {
		return 0, ErrPendingNotFound
	}
	return changed, nil
}
