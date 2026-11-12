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
	mapping := channelMappingOf(channel)

	type candidate struct{ model, upstream string }
	var candidates []candidate
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
		candidates = append(candidates, candidate{model: info.Model, upstream: upstream})
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

	var created []*ModelSupply
	err = DB.Transaction(func(tx *gorm.DB) error {
		var existing []*ModelSupply
		if err := tx.Where("channel_id = ?", channel.Id).Find(&existing).Error; err != nil {
			return err
		}
		supplied := make(map[string]bool, len(existing))
		for _, s := range existing {
			supplied[s.Model] = true
		}
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
				return err
			}
			created = append(created, supply)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(created) > 0 {
		reloadSupplies()
	}
	return created, nil
}
