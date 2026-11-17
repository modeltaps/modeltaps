package model

import (
	"maps"
	"slices"
	"strings"

	"gorm.io/datatypes"
)

// A channel's model list follows its upstream: the model check adds every name the upstream
// lists that the channel neither lists nor had removed (see JoinUpstreamModels), and a listed
// model the upstream stops listing stays, marked gone. What an admin decides is whether each
// model is switched on, and which models to remove. A removed name is remembered in the check's
// result (ModelDriftResult.Removed) so the next check does not add it back; listing it again
// forgets the removal.

// listedNames is the set of names a model list holds, by their own name and as mapping targets.
func listedNames(modelsCSV string, mapping map[string]string) map[string]bool {
	out := map[string]bool{}
	for _, name := range SplitChannelModels(modelsCSV) {
		out[name] = true
		if up, ok := mapping[name]; ok {
			out[up] = true
		}
	}
	return out
}

// ListedUpstreamNames is the set of names the channel lists, by their own name and as mapping
// targets.
func (channel *Channel) ListedUpstreamNames() map[string]bool {
	return listedNames(channel.Models, channelMappingOf(channel))
}

// CoveredByWildcard reports whether an enabled "prefix*" entry of the channel's list serves name.
func (channel *Channel) CoveredByWildcard(name string) bool {
	disabled := channel.DisabledModelSet()
	for _, entry := range SplitChannelModels(channel.Models) {
		if strings.HasSuffix(entry, "*") && !disabled[entry] && strings.HasPrefix(name, strings.TrimSuffix(entry, "*")) {
			return true
		}
	}
	return false
}

// modelListState is what a save can change about a channel's model list.
type modelListState struct {
	models   string
	mapping  map[string]string
	disabled map[string]bool
}

func modelListStateOf(channel *Channel) modelListState {
	return modelListState{models: channel.Models, mapping: channelMappingOf(channel), disabled: channel.DisabledModelSet()}
}

// loadModelListState reads the stored model list of a channel, with its drift result.
func loadModelListState(id int) (*Channel, modelListState, bool) {
	var row Channel
	if err := DB.Select("id", "models", "model_mapping", "disabled_models", "model_drift").First(&row, id).Error; err != nil {
		return nil, modelListState{}, false
	}
	return &row, modelListStateOf(&row), true
}

// noteModelListChange records what a save did to a channel's model list in its drift result:
// names it removed are remembered so the model check does not add them back, names listed again
// are forgotten, and a new upstream model the admin removed or switched on or off is no longer
// flagged as new.
func noteModelListChange(id int, before modelListState) error {
	row, after, ok := loadModelListState(id)
	if !ok || before.models == after.models && maps.Equal(before.disabled, after.disabled) && maps.Equal(before.mapping, after.mapping) {
		return nil
	}
	oldNames := listedNames(before.models, before.mapping)
	newNames := listedNames(after.models, after.mapping)

	result := row.GetModelDrift()
	if result == nil {
		result = &ModelDriftResult{}
	}
	changed := false

	removed := make([]string, 0, len(result.Removed))
	for _, name := range result.Removed {
		if !newNames[name] {
			removed = append(removed, name)
		}
	}
	for name := range oldNames {
		if !newNames[name] && !slices.Contains(removed, name) {
			removed = append(removed, name)
		}
	}
	slices.Sort(removed)
	if !slices.Equal(removed, sortedCopy(result.Removed)) {
		changed = true
	}
	if len(removed) == 0 {
		removed = nil
	}
	result.Removed = removed

	// Names whose switch the save turned, by listed name and mapping target.
	toggled := map[string]bool{}
	for _, name := range SplitChannelModels(after.models) {
		if oldNames[name] && before.disabled[name] != after.disabled[name] {
			toggled[name] = true
			if up, ok := after.mapping[name]; ok {
				toggled[up] = true
			}
		}
	}
	if len(result.NewModels) > 0 {
		kept := make([]string, 0, len(result.NewModels))
		for _, name := range result.NewModels {
			if newNames[name] && !toggled[name] {
				kept = append(kept, name)
			}
		}
		if len(kept) != len(result.NewModels) {
			changed = true
			if len(kept) == 0 {
				kept = nil
			}
			result.NewModels = kept
		}
	}
	if !changed {
		return nil
	}
	return row.UpdateModelDrift(*result)
}

// JoinUpstreamModels appends upstream names to the channel's model list (deduplicated), the
// names in off switched off, and reloads the routing cache. It touches no other column.
func (channel *Channel) JoinUpstreamModels(names, off []string) error {
	if len(names) == 0 {
		return nil
	}
	existing := SplitChannelModels(channel.Models)
	seen := make(map[string]bool, len(existing))
	for _, m := range existing {
		seen[m] = true
	}
	joined := map[string]bool{}
	for _, m := range names {
		if m = strings.TrimSpace(m); m != "" && !seen[m] {
			seen[m] = true
			joined[m] = true
			existing = append(existing, m)
		}
	}
	if len(joined) == 0 {
		return nil
	}
	disabled := datatypes.JSONSlice[string]{}
	if channel.DisabledModels != nil {
		disabled = append(disabled, *channel.DisabledModels...)
	}
	for _, m := range off {
		if joined[m] && !slices.Contains(disabled, m) {
			disabled = append(disabled, m)
		}
	}
	merged := strings.Join(existing, ",")
	if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).
		Updates(map[string]any{"models": merged, "disabled_models": disabled}).Error; err != nil {
		return err
	}
	channel.Models = merged
	channel.DisabledModels = &disabled
	ChannelGroup.Load()
	return nil
}

func sortedCopy(names []string) []string {
	out := append([]string(nil), names...)
	slices.Sort(out)
	return out
}
