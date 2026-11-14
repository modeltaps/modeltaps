package model

import (
	"sort"
	"strings"
)

// States of an upstream model on a channel, checked in this order.
const (
	UpstreamModelInUse        = "in_use"         // an enabled supply on the channel sends the name
	UpstreamModelOff          = "off"            // a supply on the channel sends the name but is switched off
	UpstreamModelAvailable    = "available"      // it belongs to a public model that has no supply on this channel
	UpstreamModelNotInCatalog = "not_in_catalog" // a known model that is not in the catalog
	UpstreamModelUnknown      = "unknown"        // nothing recognises the name
)

// ChannelUpstreamModel is one model the channel's upstream listed, with the public model it
// belongs to and whether the channel supplies it.
type ChannelUpstreamModel struct {
	Name string `json:"name"`
	// PublicModel is the catalog model the name belongs to, or the name the resolver suggests
	// when it is not in the catalog; empty when the name is unknown.
	PublicModel string `json:"public_model,omitempty"`
	// Source is how PublicModel was found: "supply", a CanonicalSource* value, or empty when the
	// upstream name is the public name itself.
	Source string `json:"source,omitempty"`
	// PublicStatus is the public model's status (draft, published, retired) when it is in the catalog.
	PublicStatus string `json:"public_status,omitempty"`
	State        string `json:"state"`
	SupplyId     int    `json:"supply_id,omitempty"`
}

// ChannelUpstreamModels is everything the upstream listed at the last model check.
type ChannelUpstreamModels struct {
	// CheckedAt is when the list was read from the upstream; 0 when it never was.
	CheckedAt int64                   `json:"checked_at"`
	Models    []*ChannelUpstreamModel `json:"models"`
	Counts    map[string]int          `json:"counts"`
}

// upstreamModelSource marks a public model found through a supply rather than by name.
const upstreamModelSource = "supply"

// GetChannelUpstreamModels lists the models the channel's upstream returned at the last model
// check (ModelDriftResult.UpstreamSnapshot) and says, for each, which public model it belongs
// to and whether this channel supplies it. It reads stored data only and never calls the upstream.
func GetChannelUpstreamModels(channel *Channel) (*ChannelUpstreamModels, error) {
	out := &ChannelUpstreamModels{Models: []*ChannelUpstreamModel{}, Counts: map[string]int{}}
	drift := channel.GetModelDrift()
	if drift == nil {
		return out, nil
	}
	out.CheckedAt = drift.CheckedAt
	if len(drift.UpstreamSnapshot) == 0 {
		return out, nil
	}

	var supplies []*ModelSupply
	if err := DB.Where("channel_id = ?", channel.Id).Find(&supplies).Error; err != nil {
		return nil, err
	}
	byUpstream := make(map[string]*ModelSupply, len(supplies))
	for _, s := range supplies {
		// An enabled supply wins when two public models send the same upstream name.
		if cur, ok := byUpstream[s.UpstreamModel]; !ok || (!cur.Enabled && s.Enabled) {
			byUpstream[s.UpstreamModel] = s
		}
	}
	resolver, err := NewCanonicalResolver()
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool, len(drift.UpstreamSnapshot))
	for _, name := range drift.UpstreamSnapshot {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		row := resolver.upstreamModelRow(name, byUpstream[name])
		out.Models = append(out.Models, row)
		out.Counts[row.State]++
	}
	sort.SliceStable(out.Models, func(i, j int) bool { return out.Models[i].Name < out.Models[j].Name })
	return out, nil
}

// publicModel returns the catalog row of a public model: not an alias and not a routing policy.
func (r *CanonicalResolver) publicModel(name string) *ModelInfo {
	if info := r.byModel[name]; info != nil && info.AliasOf == "" && info.Kind == "" {
		return info
	}
	return nil
}

func (r *CanonicalResolver) upstreamModelRow(name string, supply *ModelSupply) *ChannelUpstreamModel {
	row := &ChannelUpstreamModel{Name: name}
	if supply != nil {
		row.PublicModel, row.Source, row.SupplyId = supply.Model, upstreamModelSource, supply.Id
		if info := r.publicModel(supply.Model); info != nil {
			row.PublicStatus = info.Status
		}
		row.State = UpstreamModelOff
		if supply.Enabled {
			row.State = UpstreamModelInUse
		}
		return row
	}
	match := r.Resolve(name)
	if info := r.publicModel(match.Canonical); info != nil {
		row.PublicModel, row.Source, row.PublicStatus = info.Model, match.Source, info.Status
		row.State = UpstreamModelAvailable
		return row
	}
	if match.Source != CanonicalSourceNone {
		row.PublicModel, row.Source, row.State = match.Canonical, match.Source, UpstreamModelNotInCatalog
		return row
	}
	if _, ok := r.seedCanonicals[strings.ToLower(name)]; ok || LibraryKnowsModel(name) {
		row.PublicModel, row.State = name, UpstreamModelNotInCatalog
		return row
	}
	row.State = UpstreamModelUnknown
	return row
}
