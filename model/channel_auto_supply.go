package model

import (
	"encoding/json"
	"sort"
	"strings"

	"gorm.io/gorm"
)

// ModelSupplySourceChannel marks supplies created on their own when a channel was added or its
// upstream listed a new model.
const ModelSupplySourceChannel = "channel"

// channelMappingOf reads the channel's model mapping (public name → upstream name) without the
// leading "+" that only changes billing.
func channelMappingOf(channel *Channel) map[string]string {
	out := map[string]string{}
	raw := strings.TrimSpace(channel.GetModelMapping())
	if raw == "" {
		return out
	}
	mapping := map[string]string{}
	if json.Unmarshal([]byte(raw), &mapping) != nil {
		return out
	}
	for name, upstream := range mapping {
		if upstream = strings.TrimPrefix(strings.TrimSpace(upstream), "+"); upstream != "" {
			out[strings.TrimSpace(name)] = upstream
		}
	}
	return out
}

// AutoSupplyChannel adds a supply on the channel for every name in names that belongs to a
// public model the channel does not supply yet, and returns the supplies it created. names are
// what the channel lists: public names, aliases or platform spellings; a name with an entry in
// the channel's model mapping is sent upstream as the mapped name. Names that match no public
// model are left alone (they show as "not in catalog" or wait under new upstream models), and
// wildcards are skipped. Supplies take the channel's priority and weight, like the migration,
// so the "usage" chosen for the channel decides where it sits next to the other channels.
func AutoSupplyChannel(channel *Channel, names []string) ([]*ModelSupply, error) {
	if channel == nil || channel.Id == 0 || len(names) == 0 {
		return nil, nil
	}
	resolver, err := NewCanonicalResolver()
	if err != nil {
		return nil, err
	}
	// Names are resolved before the transaction: the resolver may load the model library through
	// the global connection, which a SQLite transaction would block.
	candidates := autoSupplyCandidates(resolver, channel, names)
	var created []*ModelSupply
	err = DB.Transaction(func(tx *gorm.DB) error {
		var err error
		created, err = createAutoSupplies(tx, channel, candidates)
		return err
	})
	if err != nil {
		return nil, err
	}
	if len(created) > 0 {
		reloadSupplies()
	}
	return created, nil
}

type autoSupplyCandidate struct{ model, upstream string }

// autoSupplyCandidates gives the public model and upstream name for each listed name that belongs
// to a public model, ordered so the preferred spelling of each model comes first.
func autoSupplyCandidates(resolver *CanonicalResolver, channel *Channel, names []string) []autoSupplyCandidate {
	mapping := channelMappingOf(channel)
	var candidates []autoSupplyCandidate
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" || strings.HasSuffix(name, "*") {
			continue
		}
		info := resolver.publicModel(resolver.Resolve(name).Canonical)
		if info == nil {
			continue
		}
		upstream := name
		if mapped, ok := mapping[name]; ok {
			upstream = mapped
		}
		candidates = append(candidates, autoSupplyCandidate{model: info.Model, upstream: upstream})
	}
	// A model has one supply per channel. When several listed names belong to the same model, the
	// one spelled like the model wins, then the shortest, so "gpt-4o" beats "gpt-4o-2024-08-06".
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.model != b.model {
			return a.model < b.model
		}
		if (a.upstream == a.model) != (b.upstream == b.model) {
			return a.upstream == a.model
		}
		if len(a.upstream) != len(b.upstream) {
			return len(a.upstream) < len(b.upstream)
		}
		return a.upstream < b.upstream
	})
	return candidates
}

// createAutoSupplies creates the supplies of candidates the channel does not supply yet.
func createAutoSupplies(tx *gorm.DB, channel *Channel, candidates []autoSupplyCandidate) ([]*ModelSupply, error) {
	var existing []*ModelSupply
	if err := tx.Where("channel_id = ?", channel.Id).Find(&existing).Error; err != nil {
		return nil, err
	}
	supplied := make(map[string]bool, len(existing))
	for _, s := range existing {
		supplied[s.Model] = true
	}
	var created []*ModelSupply
	for _, c := range candidates {
		if supplied[c.model] {
			continue
		}
		supplied[c.model] = true
		supply := &ModelSupply{
			Model: c.model, ChannelId: channel.Id, UpstreamModel: c.upstream,
			Priority: channel.GetPriority(), Weight: uint(channel.GetWeight()),
			Enabled: true, Source: ModelSupplySourceChannel,
		}
		if err := tx.Create(supply).Error; err != nil {
			return nil, err
		}
		created = append(created, supply)
	}
	return created, nil
}

// ChannelSupplySync counts what SyncChannelSupplies changed.
type ChannelSupplySync struct {
	Attached int `json:"attached"`
	Updated  int `json:"updated"`
	Removed  int `json:"removed"`
}

// followsChannelList reports whether a supply was made from a channel's model list (when the
// channel was added, by the drift check, or by the migration) and so follows later edits of it.
// Supplies added by hand or from the model library are the admin's own and are left alone.
func followsChannelList(s *ModelSupply) bool {
	return s.Source == ModelSupplySourceChannel || s.Source == ModelSupplySourceMigrated
}

// SyncChannelSupplies carries an edit of a channel's model list or model mapping to its
// supplies while supplies route requests. Names the edit added are attached as AutoSupplyChannel
// does. A supply made from the list follows it while it still sends what the old list and mapping
// gave it: when the mapping now names another upstream model, the supply sends that one; when
// the name left the list and no other listed name belongs to the same public model, the supply is
// removed, so adding the name back attaches it again. A supply whose upstream name was changed on
// the model page, or that was added by hand, is left alone.
func SyncChannelSupplies(before, after *Channel) (ChannelSupplySync, error) {
	var out ChannelSupplySync
	if before == nil || after == nil || after.Id == 0 {
		return out, nil
	}
	oldNames, newNames := SplitChannelModels(before.Models), SplitChannelModels(after.Models)
	oldMapping, newMapping := channelMappingOf(before), channelMappingOf(after)
	upstreamOf := func(mapping map[string]string, name string) string {
		if mapped, ok := mapping[name]; ok {
			return mapped
		}
		return name
	}

	resolver, err := NewCanonicalResolver()
	if err != nil {
		return out, err
	}
	publicOf := func(name string) string {
		if strings.HasSuffix(name, "*") {
			return ""
		}
		if info := resolver.publicModel(resolver.Resolve(name).Canonical); info != nil {
			return info.Model
		}
		return ""
	}
	kept := map[string]bool{}
	stillServed := map[string]bool{} // public models some name in the new list belongs to
	for _, name := range newNames {
		kept[name] = true
		if model := publicOf(name); model != "" {
			stillServed[model] = true
		}
	}

	type change struct {
		supply   *ModelSupply
		upstream string // new upstream name; empty removes the supply
	}
	var changes []change
	var existing []*ModelSupply
	if err := DB.Where("channel_id = ?", after.Id).Find(&existing).Error; err != nil {
		return out, err
	}
	byModel := make(map[string]*ModelSupply, len(existing))
	for _, s := range existing {
		byModel[s.Model] = s
	}
	seen := map[int]bool{}
	for _, name := range oldNames {
		model := publicOf(name)
		supply := byModel[model]
		if model == "" || supply == nil || seen[supply.Id] || !followsChannelList(supply) {
			continue
		}
		if supply.UpstreamModel != upstreamOf(oldMapping, name) {
			continue // changed on the model page, or made from another listed name
		}
		switch {
		case kept[name]:
			if up := upstreamOf(newMapping, name); up != supply.UpstreamModel {
				changes = append(changes, change{supply, up})
				seen[supply.Id] = true
			}
		case !stillServed[model]:
			changes = append(changes, change{supply, ""})
			seen[supply.Id] = true
		}
	}

	if len(changes) > 0 {
		err = DB.Transaction(func(tx *gorm.DB) error {
			for _, c := range changes {
				if c.upstream == "" {
					if err := tx.Delete(&ModelSupply{}, c.supply.Id).Error; err != nil {
						return err
					}
					out.Removed++
					continue
				}
				if err := tx.Model(&ModelSupply{}).Where("id = ?", c.supply.Id).Update("upstream_model", c.upstream).Error; err != nil {
					return err
				}
				out.Updated++
			}
			return nil
		})
		if err != nil {
			return ChannelSupplySync{}, err
		}
		reloadSupplies()
	}

	created, err := AutoSupplyChannel(after, AddedChannelModels(before.Models, after.Models))
	if err != nil {
		return out, err
	}
	out.Attached = len(created)
	return out, nil
}
