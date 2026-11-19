package model

import (
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// States of a supply in a preview, checked in this order.
const (
	SupplyStateChannelDisabled = "channel_disabled" // the channel is disabled or deleted
	SupplyStatePending         = "pending"          // an intake rule added it for an admin to confirm
	SupplyStateDisabled        = "disabled"         // the supply is switched off
	SupplyStateUpstreamGone    = "upstream_gone"    // the upstream no longer lists its model
	SupplyStatePaused          = "paused"           // a stored pause runs until Until
	SupplyStateAwaitingTest    = "awaiting_test"    // the pause is over; a recovery test decides
	SupplyStateCoolingDown     = "cooling_down"     // a short in-memory cooldown runs until Until
	SupplyStateAvailable       = "available"
)

// SupplyPreviewEntry is one supply as the next request for the model would see it.
type SupplyPreviewEntry struct {
	*ModelSupplyView
	Groups []string `json:"groups"`
	State  string   `json:"state"`
	// Until ends a pause or cooldown (Unix seconds); 0 when neither runs.
	Until       int64  `json:"until,omitempty"`
	PauseId     int    `json:"pause_id,omitempty"`
	PauseRule   string `json:"pause_rule,omitempty"`
	PauseReason string `json:"pause_reason,omitempty"`
	// Share is the chance this supply takes a request that reaches its tier: by weight among
	// the tier's available supplies for balance, the heaviest available ones for failover.
	Share float64 `json:"share"`
	// Reason says where the supply's position comes from (a SupplyReason* value); PolicyTier is
	// the allocation policy tier it sits in, counted from 1, when the reason names one.
	Reason     string `json:"reason"`
	PolicyTier int    `json:"policy_tier,omitempty"`
}

// Where a supply's position comes from.
const (
	SupplyReasonPolicyChannel  = "policy_channel"  // the policy places this channel itself
	SupplyReasonPolicyProvider = "policy_provider" // the policy places the channel's provider
	SupplyReasonUnplaced       = "unplaced"        // the policy places neither: after every tier
	SupplyReasonJoinLast       = "join_last"       // joined after the model's other supplies
	SupplyReasonChannel        = "channel"         // the channel's own priority (no policy, or manual model)
	SupplyReasonOwn            = "own"             // numbers set on the supply itself
)

// SupplyPreviewTier holds supplies tried together; tiers are tried from the first. Supplies are
// grouped by priority and cost as SupplyCostOrder says (supply_cost_tier.go): with "priority" a
// priority with supplies at different costs is split into one tier per cost, CostTier counting
// them from 0 within the priority; with "cost" tiers run cheapest first across priorities and
// CostTier counts cost levels over the whole model.
type SupplyPreviewTier struct {
	Priority int64                 `json:"priority"`
	CostTier int                   `json:"cost_tier"`
	Supplies []*SupplyPreviewEntry `json:"supplies"`
}

// SupplyPreview shows in which order a request for a public model would try its supplies.
type SupplyPreview struct {
	Model string `json:"model"`
	// RequestedModel is the name asked for when it was an alias of Model.
	RequestedModel string `json:"requested_model,omitempty"`
	Group          string `json:"group,omitempty"`
	Status         string `json:"status"`
	// Mode is the mode requests are routed with; OwnMode is what the model stores (balance,
	// failover, or default to follow DefaultMode, the site default).
	Mode        string `json:"mode"`
	OwnMode     string `json:"own_mode"`
	DefaultMode string `json:"default_mode"`
	// Active says supplies route requests now (ModelSupplyEnabled); when false the preview
	// shows what the supply configuration will do once switched on.
	Active bool `json:"active"`
	// Allocation is "policy" when the model's supplies take their numbers from the allocation
	// policy, "manual" when they keep their own, empty when no policy applies.
	Allocation string               `json:"allocation"`
	// CostOrder is SupplyCostOrder: how far cost reorders the tiers.
	CostOrder string               `json:"cost_order"`
	Tiers     []*SupplyPreviewTier `json:"tiers"`
	// FirstChoice is the supply that takes the next request when that is certain: the only
	// heaviest available supply of the first reachable tier (failover), or its only available
	// supply. 0 when the choice is random or nothing is available.
	FirstChoice int `json:"first_choice,omitempty"`
	// OutsideGroup counts supplies whose channel is not in Group, left out of the tiers.
	OutsideGroup int `json:"outside_group"`
}

func supplyStateOf(view *ModelSupplyView, modelName string, now int64) (*SupplyPreviewEntry, bool) {
	entry := &SupplyPreviewEntry{ModelSupplyView: view, State: SupplyStateAvailable}
	switch {
	case view.ChannelStatus != config.ChannelStatusEnabled:
		entry.State = SupplyStateChannelDisabled
	case view.Pending:
		entry.State = SupplyStatePending
	case !view.Enabled:
		entry.State = SupplyStateDisabled
	case view.UpstreamGoneAt > 0:
		entry.State = SupplyStateUpstreamGone
	default:
		if pause := ActivePauseFor(view.ChannelId, modelName); pause != nil {
			entry.State, entry.Until = SupplyStatePaused, pause.Until
			if pause.Until <= now {
				entry.State = SupplyStateAwaitingTest
			}
			entry.PauseId, entry.PauseRule, entry.PauseReason = pause.Id, pause.Rule, pause.Reason
		} else if until, cooling := ChannelGroup.CooldownUntil(view.ChannelId, modelName); cooling {
			entry.State, entry.Until = SupplyStateCoolingDown, until
		}
	}
	return entry, entry.State == SupplyStateAvailable
}

// PreviewModelSupply builds the preview for a public model (aliases resolve to it), limited to
// supplies whose channel serves group when group is set.
func PreviewModelSupply(name, group string) (*SupplyPreview, error) {
	canonical, isAlias := ResolveCanonicalModel(strings.TrimSpace(name))
	info := lookupModelInfo(canonical)
	if info == nil {
		return nil, ErrSupplyModelNotFound
	}
	if info.Kind != "" {
		return nil, ErrSupplyModelNotPublic
	}
	views, err := GetModelSupplyViews(canonical, 0)
	if err != nil {
		return nil, err
	}

	preview := &SupplyPreview{
		Model: canonical, Group: group, Status: info.Status, Mode: routingSupplyMode(info, allocationInForce()),
		OwnMode: info.SupplyMode, DefaultMode: EffectiveSupplyMode(""),
		Active: config.ModelSupplyEnabled, Tiers: []*SupplyPreviewTier{},
	}
	if allocationInForce() {
		preview.Allocation = "policy"
		if !followsPolicy(info) {
			preview.Allocation = AllocationManual
		}
	}
	if !IsValidSupplyMode(preview.OwnMode) {
		preview.OwnMode = SupplyModeDefault
	}
	if isAlias {
		preview.RequestedModel = name
	}

	reasonOf := supplyReasons(views, preview.Allocation == "policy")
	now := time.Now().Unix()
	byPriority := map[int64]*SupplyPreviewTier{}
	for _, view := range views {
		// A channel open to all groups ("*") serves every group.
		groups := ExpandChannelGroup(view.ChannelGroup)
		if group != "" && !slices.Contains(groups, group) {
			preview.OutsideGroup++
			continue
		}
		entry, _ := supplyStateOf(view, canonical, now)
		entry.Groups = groups
		entry.Reason, entry.PolicyTier = reasonOf(view)
		tier := byPriority[view.Priority]
		if tier == nil {
			tier = &SupplyPreviewTier{Priority: view.Priority}
			byPriority[view.Priority] = tier
			preview.Tiers = append(preview.Tiers, tier)
		}
		tier.Supplies = append(tier.Supplies, entry)
	}
	sort.Slice(preview.Tiers, func(i, j int) bool { return preview.Tiers[i].Priority > preview.Tiers[j].Priority })
	preview.Tiers = orderPreviewTiers(preview.Tiers)
	preview.CostOrder = config.SupplyCostOrder

	for _, tier := range preview.Tiers {
		sort.SliceStable(tier.Supplies, func(i, j int) bool {
			a, b := tier.Supplies[i], tier.Supplies[j]
			if a.EffectiveWeight() != b.EffectiveWeight() {
				return a.EffectiveWeight() > b.EffectiveWeight()
			}
			return a.Id < b.Id
		})
		var available []*SupplyPreviewEntry
		for _, entry := range tier.Supplies {
			if entry.State == SupplyStateAvailable {
				available = append(available, entry)
			}
		}
		if len(available) == 0 {
			continue
		}
		if preview.Mode == SupplyModeFailover {
			heaviest := available[0].EffectiveWeight()
			var tied []*SupplyPreviewEntry
			for _, entry := range available {
				if entry.EffectiveWeight() == heaviest {
					tied = append(tied, entry)
				}
			}
			for _, entry := range tied {
				entry.Share = 1 / float64(len(tied))
			}
			if preview.FirstChoice == 0 && len(tied) == 1 {
				preview.FirstChoice = tied[0].Id
			}
		} else {
			total := 0
			for _, entry := range available {
				total += entry.EffectiveWeight()
			}
			for _, entry := range available {
				entry.Share = float64(entry.EffectiveWeight()) / float64(total)
			}
			if preview.FirstChoice == 0 && len(available) == 1 {
				preview.FirstChoice = available[0].Id
			}
		}
		// Only the first tier with an available supply takes requests while it stays available.
		break
	}
	return preview, nil
}

// supplyReasons returns a function telling, for each supply of a model, where its position
// comes from. onPolicy is true when the model takes its numbers from the allocation policy.
func supplyReasons(views []*ModelSupplyView, onPolicy bool) func(*ModelSupplyView) (string, int) {
	var plan *allocationPlan
	channels := map[int]*Channel{}
	if onPolicy {
		if policy, err := GetDefaultAllocationPolicy(); err == nil && policy != nil {
			plan = newAllocationPlan(policy.TierList())
		}
		ids := make([]int, 0, len(views))
		for _, v := range views {
			ids = append(ids, v.ChannelId)
		}
		if plan != nil && len(ids) > 0 {
			var list []*Channel
			if err := DB.Select("id", "type", "base_url").Where("id IN ?", ids).Find(&list).Error; err == nil {
				for _, ch := range list {
					channels[ch.Id] = ch
				}
			}
		}
	}
	return func(v *ModelSupplyView) (string, int) {
		if plan != nil {
			if v.JoinLast {
				return SupplyReasonJoinLast, 0
			}
			ch := channels[v.ChannelId]
			if ch == nil {
				return SupplyReasonUnplaced, 0
			}
			slot := plan.slotOf(ch)
			switch {
			case strings.HasPrefix(slot.key, "c"):
				return SupplyReasonPolicyChannel, slot.tier + 1
			case strings.HasPrefix(slot.key, "p"):
				return SupplyReasonPolicyProvider, slot.tier + 1
			default:
				return SupplyReasonUnplaced, 0
			}
		}
		if v.JoinLast {
			return SupplyReasonJoinLast, 0
		}
		if v.Priority == v.ChannelPriority {
			return SupplyReasonChannel, 0
		}
		return SupplyReasonOwn, 0
	}
}

// orderPreviewTiers regroups the priority tiers the way the router tries them
// (ChannelsChooser.routeTiers).
func orderPreviewTiers(tiers []*SupplyPreviewTier) []*SupplyPreviewTier {
	var entries []*SupplyPreviewEntry
	var prio []int64
	var costs []float64
	for _, tier := range tiers {
		for _, e := range tier.Supplies {
			entries = append(entries, e)
			prio = append(prio, tier.Priority)
			costs = append(costs, supplyViewRankCost(e.ModelSupplyView))
		}
	}
	byCost := costFirst()
	groups := orderSupplyTiers(prio, costs, byCost)
	levels := costTierIndex(costs)
	out := make([]*SupplyPreviewTier, 0, len(groups))
	level := map[int64]int{} // priority -> tiers seen (priority order)
	for _, idx := range groups {
		t := &SupplyPreviewTier{Priority: prio[idx[0]]}
		if byCost {
			t.CostTier = levels[idx[0]]
		} else {
			t.CostTier = level[t.Priority]
			level[t.Priority]++
		}
		for _, i := range idx {
			t.Supplies = append(t.Supplies, entries[i])
		}
		out = append(out, t)
	}
	return out
}
