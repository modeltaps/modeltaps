package model

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/gorm"
)

// SupplyMigrationReport describes what migrating channel configuration to supplies does (dry
// run) or did. Every list is something an admin should look at before switching
// ModelSupplyEnabled on.
type SupplyMigrationReport struct {
	DryRun           bool           `json:"dry_run"`
	SuppliesPlanned  int            `json:"supplies_planned"`
	SuppliesExisting int            `json:"supplies_existing"`
	Supplies         []*ModelSupply `json:"supplies"`
	// NewModels are names channels list without a catalog row; they get a published row.
	NewModels []string `json:"new_models"`
	// NewAliases are names channels list without a catalog row that the seed or the name without
	// its "vendor/" prefix identifies as a public model: that model is supplied instead, and the
	// name becomes its alias so requests for it keep working.
	NewAliases    []SupplyNewAlias     `json:"new_aliases"`
	StatusChanges []SupplyStatusChange `json:"status_changes"`
	// PriceCopies give a public model the price of the upstream name it was billed by.
	PriceCopies    []SupplyPriceCopy     `json:"price_copies"`
	PriceConflicts []SupplyPriceConflict `json:"price_conflicts"`
	// BillingChanges are supplies whose requests will be priced differently: they were billed by
	// the upstream name and will be billed by the public model.
	BillingChanges []SupplyBillingChange `json:"billing_changes"`
	// PlusMappings are model_mapping values starting with "+" (bill by the requested name),
	// which public-model billing makes the rule for every supply.
	PlusMappings        []SupplyEntryNote   `json:"plus_mappings"`
	ExpandedWildcards   []SupplyWildcard    `json:"expanded_wildcards"`
	UnresolvedWildcards []SupplyWildcard    `json:"unresolved_wildcards"`
	DuplicateEntries    []SupplyEntryNote   `json:"duplicate_entries"`
	SkippedEntries      []SupplyEntryNote   `json:"skipped_entries"`
	PolicyGaps          []SupplyPolicyGap   `json:"policy_gaps"`
	RoutingDifferences  []SupplyRoutingDiff `json:"routing_differences"`
}

type SupplyNewAlias struct {
	Alias string `json:"alias"`
	Model string `json:"model"`
	// Source is how the model was found: "seed" or "bare" (see CanonicalSourceSeed).
	Source string `json:"source"`
}

type SupplyStatusChange struct {
	Model string `json:"model"`
	From  string `json:"from"`
	To    string `json:"to"`
}

type SupplyPrice struct {
	Type   string  `json:"type"`
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

type SupplyPriceCopy struct {
	Model string      `json:"model"`
	From  string      `json:"from"`
	Price SupplyPrice `json:"price"`
}

type SupplyPriceConflict struct {
	Model      string                 `json:"model"`
	Candidates map[string]SupplyPrice `json:"candidates"`
	Chosen     string                 `json:"chosen"`
}

type SupplyBillingChange struct {
	Model          string       `json:"model"`
	ChannelId      int          `json:"channel_id"`
	LegacyBilledAs string       `json:"legacy_billed_as"`
	LegacyPrice    *SupplyPrice `json:"legacy_price"`
	NewPrice       *SupplyPrice `json:"new_price"`
}

type SupplyEntryNote struct {
	ChannelId int    `json:"channel_id"`
	Entry     string `json:"entry"`
	Detail    string `json:"detail"`
}

type SupplyWildcard struct {
	ChannelId int      `json:"channel_id"`
	Pattern   string   `json:"pattern"`
	Models    []string `json:"models,omitempty"`
}

type SupplyPolicyGap struct {
	Policy string `json:"policy"`
	Role   string `json:"role"`
	Model  string `json:"model"`
}

// SupplyRoutingDiff compares, for one group and public model, the priority tiers of channels a
// request reaches before and after the migration. Kind is "changed", "lost", "wildcard"
// (reached through a pattern before), or "newly_reachable" (an alias or upstream name was listed
// where the public name should have been, so the public name was not routable before).
type SupplyRoutingDiff struct {
	Group           string   `json:"group"`
	Model           string   `json:"model"`
	Kind            string   `json:"kind"`
	Legacy          [][]int  `json:"legacy"`
	New             [][]int  `json:"new"`
	UpstreamChanges []string `json:"upstream_changes,omitempty"`
}

type supplyMigrationInput struct {
	channels []*Channel
	infos    []*ModelInfo
	prices   []*Price
	existing []*ModelSupply
	policies []*RoutingPolicy
	// seed is the curated catalog seed; nil when it does not load.
	seed *CatalogSeed
}

type supplyMigrationPlan struct {
	newInfos      []*ModelInfo
	newAliases    []*ModelInfo
	statusUpdates map[int]string
	priceCopies   []*Price
	supplies      []*ModelSupply
}

// entry ranks: a supply produced by listing the public name itself wins over one produced by an
// alias, which wins over one produced by expanding a wildcard.
const (
	rankCanonical = iota
	rankAlias
	rankWildcard
)

type plannedSupply struct {
	supply *ModelSupply
	rank   int
	entry  string
	plus   bool
}

func supplyPriceOf(p *Price) *SupplyPrice {
	if p == nil {
		return nil
	}
	return &SupplyPrice{Type: p.Type, Input: p.Input, Output: p.Output}
}

// priceIndex finds a price the way billing does: an exact row, then a wildcard row.
type priceIndex struct {
	exact    map[string]*Price
	patterns []string
}

func newPriceIndex(prices []*Price) *priceIndex {
	idx := &priceIndex{exact: make(map[string]*Price, len(prices))}
	for _, p := range prices {
		idx.exact[p.Model] = p
		if strings.HasSuffix(p.Model, "*") {
			idx.patterns = append(idx.patterns, p.Model)
		}
	}
	sort.Strings(idx.patterns)
	return idx
}

func (idx *priceIndex) find(name string) *Price {
	if p, ok := idx.exact[name]; ok {
		return p
	}
	for _, pattern := range idx.patterns {
		if strings.HasPrefix(name, strings.TrimSuffix(pattern, "*")) {
			return idx.exact[pattern]
		}
	}
	return nil
}

func channelGroups(channel *Channel) []string {
	return ExpandChannelGroup(channel.Group)
}

func channelEntries(channel *Channel) []string {
	seen := map[string]bool{}
	var entries []string
	for _, m := range strings.Split(channel.Models, ",") {
		m = strings.TrimSpace(m)
		if m != "" && !seen[m] {
			seen[m] = true
			entries = append(entries, m)
		}
	}
	return entries
}

// stripPlus removes the "+" billing prefix from a model_mapping value.
func stripPlus(value string) (string, bool) {
	if strings.HasPrefix(value, "+") {
		return value[1:], true
	}
	return value, false
}

type supplyKey struct {
	model   string
	channel int
}

// planSupplyMigration turns channel configuration into supplies and reports everything that
// changes or needs a decision. It reads only its input.
func planSupplyMigration(in supplyMigrationInput) (*supplyMigrationPlan, *SupplyMigrationReport) {
	report := &SupplyMigrationReport{}
	plan := &supplyMigrationPlan{statusUpdates: map[int]string{}}

	byModel := make(map[string]*ModelInfo, len(in.infos))
	for _, info := range in.infos {
		byModel[info.Model] = info
	}
	prices := newPriceIndex(in.prices)

	// A name with a catalog row resolves through the catalog only: a non-alias row is a public
	// model of its own (catalog sync creates rows such as "xai/grok-4"), and turning it into an
	// alias is the seed's job. A name without a row may still be a public model under another
	// name, which the seed or the name without its vendor prefix tells; it becomes an alias.
	resolver := newCanonicalResolver(in.infos, in.seed)
	newAliases := map[string]SupplyNewAlias{}
	publicName := func(name string, record bool) string {
		if byModel[name] != nil {
			return resolveCanonicalIn(byModel, name)
		}
		if alias, ok := newAliases[name]; ok {
			return alias.Model
		}
		match := resolver.Resolve(name)
		if match.Source == CanonicalSourceNone || match.Canonical == name {
			return name
		}
		canonical := resolveCanonicalIn(byModel, match.Canonical)
		if info := byModel[canonical]; canonical == name || (info != nil && info.Kind != "") {
			return name
		}
		if record {
			newAliases[name] = SupplyNewAlias{Alias: name, Model: canonical, Source: match.Source}
		}
		return canonical
	}

	channels := slices.Clone(in.channels)
	sort.Slice(channels, func(i, j int) bool { return channels[i].Id < channels[j].Id })

	existing := map[supplyKey]*ModelSupply{}
	for _, s := range in.existing {
		existing[supplyKey{s.Model, s.ChannelId}] = s
	}

	planned := map[supplyKey]*plannedSupply{}
	consider := func(channel *Channel, entry, name, upstream string, plus bool, rank int) {
		canonical := publicName(name, true)
		if info := byModel[canonical]; info != nil && info.Kind != "" {
			report.SkippedEntries = append(report.SkippedEntries, SupplyEntryNote{channel.Id, entry, "names a routing policy, not a model"})
			return
		}
		if canonical != name && rank == rankCanonical {
			rank = rankAlias
		}
		key := supplyKey{canonical, channel.Id}
		if _, ok := existing[key]; ok {
			return
		}
		candidate := &plannedSupply{
			supply: &ModelSupply{
				Model:         canonical,
				ChannelId:     channel.Id,
				UpstreamModel: upstream,
				Priority:      channel.GetPriority(),
				Weight:        uint(channel.GetWeight()),
				Enabled:       true,
				Source:        ModelSupplySourceMigrated,
			},
			rank: rank, entry: entry, plus: plus,
		}
		if current, ok := planned[key]; ok {
			kept, dropped := current, candidate
			if candidate.rank < current.rank {
				kept, dropped = candidate, current
			}
			planned[key] = kept
			report.DuplicateEntries = append(report.DuplicateEntries, SupplyEntryNote{channel.Id, dropped.entry,
				fmt.Sprintf("%s is already supplied through %q", canonical, kept.entry)})
			return
		}
		planned[key] = candidate
	}

	for _, channel := range channels {
		mapping := map[string]string{}
		if raw := strings.TrimSpace(channel.GetModelMapping()); raw != "" && raw != "{}" {
			if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
				report.SkippedEntries = append(report.SkippedEntries, SupplyEntryNote{channel.Id, "", "model_mapping is not valid JSON; entries use their own names"})
				mapping = map[string]string{}
			}
		}
		for _, entry := range channelEntries(channel) {
			if strings.HasSuffix(entry, "*") {
				var snapshot []string
				if channel.ModelDrift != nil {
					snapshot = channel.ModelDrift.Data().UpstreamSnapshot
				}
				prefix := strings.TrimSuffix(entry, "*")
				var expanded []string
				for _, name := range snapshot {
					if strings.HasPrefix(name, prefix) {
						expanded = append(expanded, name)
					}
				}
				sort.Strings(expanded)
				if len(expanded) == 0 {
					report.UnresolvedWildcards = append(report.UnresolvedWildcards, SupplyWildcard{ChannelId: channel.Id, Pattern: entry})
					continue
				}
				report.ExpandedWildcards = append(report.ExpandedWildcards, SupplyWildcard{ChannelId: channel.Id, Pattern: entry, Models: expanded})
				for _, name := range expanded {
					value := mapping[name]
					if value == "" {
						value = mapping[entry]
					}
					upstream, plus := stripPlus(value)
					if upstream == "" {
						upstream = name
					}
					consider(channel, entry, name, upstream, plus, rankWildcard)
				}
				continue
			}
			upstream, plus := stripPlus(mapping[entry])
			if upstream == "" {
				upstream = entry
			}
			if plus {
				report.PlusMappings = append(report.PlusMappings, SupplyEntryNote{channel.Id, entry, "+" + upstream})
			}
			consider(channel, entry, entry, upstream, plus, rankCanonical)
		}
	}

	keys := make([]supplyKey, 0, len(planned))
	for key := range planned {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].model != keys[j].model {
			return keys[i].model < keys[j].model
		}
		return keys[i].channel < keys[j].channel
	})
	for _, key := range keys {
		plan.supplies = append(plan.supplies, planned[key].supply)
	}
	report.Supplies = plan.supplies
	report.SuppliesPlanned = len(plan.supplies)
	report.SuppliesExisting = len(in.existing)

	// The catalog after the migration: new aliases resolve to their public model.
	after := maps.Clone(byModel)
	aliasNames := slices.Sorted(maps.Keys(newAliases))
	for _, name := range aliasNames {
		alias := newAliases[name]
		row := &ModelInfo{Model: name, Name: name, AliasOf: alias.Model, Source: ModelInfoSourceMigrated}
		plan.newAliases = append(plan.newAliases, row)
		report.NewAliases = append(report.NewAliases, alias)
		after[name] = row
	}

	// Every supply, existing or planned, grouped by public model in a stable order.
	all := slices.Clone(in.existing)
	all = append(all, plan.supplies...)
	byPublic := map[string][]*ModelSupply{}
	for _, s := range all {
		byPublic[s.Model] = append(byPublic[s.Model], s)
	}
	publicNames := make([]string, 0, len(byPublic))
	for name := range byPublic {
		publicNames = append(publicNames, name)
		sort.Slice(byPublic[name], func(i, j int) bool { return byPublic[name][i].ChannelId < byPublic[name][j].ChannelId })
	}
	sort.Strings(publicNames)

	// Catalog rows: create missing ones, publish what was visible, keep hidden ones as drafts.
	for _, name := range publicNames {
		info := byModel[name]
		if info == nil {
			plan.newInfos = append(plan.newInfos, &ModelInfo{
				Model: name, Name: name, Status: ModelStatusPublished, SupplyMode: SupplyModeBalance,
				Source: ModelInfoSourceMigrated,
			})
			report.NewModels = append(report.NewModels, name)
			continue
		}
		target := ModelStatusPublished
		if info.Hidden {
			target = ModelStatusDraft
		}
		if info.Status != target {
			plan.statusUpdates[info.Id] = target
			report.StatusChanges = append(report.StatusChanges, SupplyStatusChange{name, info.Status, target})
		}
	}

	// Prices: a public model that has no price of its own takes the one it was billed by.
	plusByKey := map[supplyKey]bool{}
	for key, p := range planned {
		plusByKey[key] = p.plus
	}
	copied := map[string]*Price{}
	for _, name := range publicNames {
		if prices.find(name) != nil {
			continue
		}
		candidates := map[string]SupplyPrice{}
		var first string
		for _, s := range byPublic[name] {
			if p := prices.find(s.UpstreamModel); p != nil {
				if _, seen := candidates[s.UpstreamModel]; !seen {
					candidates[s.UpstreamModel] = *supplyPriceOf(p)
					if first == "" {
						first = s.UpstreamModel
					}
				}
			}
		}
		if first == "" {
			continue
		}
		source := prices.find(first)
		price := *source
		price.Model = name
		price.Locked = false
		price.ModelInfo = nil
		plan.priceCopies = append(plan.priceCopies, &price)
		copied[name] = &price
		report.PriceCopies = append(report.PriceCopies, SupplyPriceCopy{name, first, *supplyPriceOf(source)})
		distinct := map[SupplyPrice]bool{}
		for _, p := range candidates {
			distinct[p] = true
		}
		if len(distinct) > 1 {
			report.PriceConflicts = append(report.PriceConflicts, SupplyPriceConflict{name, candidates, first})
		}
	}
	priceAfter := func(name string) *Price {
		if p := prices.find(name); p != nil {
			return p
		}
		return copied[name]
	}

	// Billing: before, a request was priced by the upstream name unless the mapping had "+".
	for _, s := range all {
		legacyBilled := s.UpstreamModel
		if plusByKey[supplyKey{s.Model, s.ChannelId}] {
			legacyBilled = s.Model
		}
		if legacyBilled == s.Model {
			continue
		}
		legacyPrice, newPrice := supplyPriceOf(prices.find(legacyBilled)), supplyPriceOf(priceAfter(s.Model))
		if legacyPrice != nil && newPrice != nil && *legacyPrice == *newPrice {
			continue
		}
		report.BillingChanges = append(report.BillingChanges, SupplyBillingChange{s.Model, s.ChannelId, legacyBilled, legacyPrice, newPrice})
	}

	// Routing policies must target supplied models once supplies drive routing.
	for _, policy := range in.policies {
		if !policy.Enabled {
			continue
		}
		check := func(role, name string) {
			if name == "" {
				return
			}
			canonical := publicName(name, false)
			if len(byPublic[canonical]) == 0 {
				report.PolicyGaps = append(report.PolicyGaps, SupplyPolicyGap{policy.Name, role, canonical})
			}
		}
		check("decision", policy.DecisionModel)
		check("fallback", policy.FallbackModel)
		for _, route := range policy.Routes {
			check("route:"+route.Label, route.TargetModel)
		}
	}

	report.RoutingDifferences = compareSupplyRouting(channels, after, all)
	return plan, report
}

// tierList turns priority → channel ids into tiers, highest priority first, ids ascending.
func tierList(byPriority map[int64][]int) [][]int {
	priorities := make([]int64, 0, len(byPriority))
	for p := range byPriority {
		priorities = append(priorities, p)
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	tiers := make([][]int, 0, len(priorities))
	for _, p := range priorities {
		ids := slices.Clone(byPriority[p])
		sort.Ints(ids)
		tiers = append(tiers, ids)
	}
	return tiers
}

func tiersEqual(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !slices.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

// compareSupplyRouting rebuilds, for enabled channels, what a request for each public model
// reached before (channel model lists, exact name first, then wildcard) and after (supplies).
func compareSupplyRouting(channels []*Channel, byModel map[string]*ModelInfo, supplies []*ModelSupply) []SupplyRoutingDiff {
	type gm struct{ group, model string }
	enabled := map[int]*Channel{}
	legacyExact := map[gm]map[int64][]int{}
	legacyPattern := map[gm]map[int64][]int{}
	legacyUpstream := map[supplyKey]string{}
	for _, channel := range channels {
		if channel.Status != config.ChannelStatusEnabled {
			continue
		}
		enabled[channel.Id] = channel
		mapping := map[string]string{}
		_ = json.Unmarshal([]byte(channel.GetModelMapping()), &mapping)
		for _, group := range channelGroups(channel) {
			for _, entry := range channelEntries(channel) {
				target := legacyExact
				if strings.HasSuffix(entry, "*") {
					target = legacyPattern
				}
				key := gm{group, entry}
				if target[key] == nil {
					target[key] = map[int64][]int{}
				}
				target[key][channel.GetPriority()] = append(target[key][channel.GetPriority()], channel.Id)
			}
		}
		for _, entry := range channelEntries(channel) {
			upstream, _ := stripPlus(mapping[entry])
			if upstream == "" {
				upstream = entry
			}
			legacyUpstream[supplyKey{entry, channel.Id}] = upstream
		}
	}

	newRule := map[gm]map[int64][]int{}
	newUpstream := map[supplyKey]string{}
	for _, s := range supplies {
		channel := enabled[s.ChannelId]
		if channel == nil || !s.Enabled {
			continue
		}
		newUpstream[supplyKey{s.Model, s.ChannelId}] = s.UpstreamModel
		for _, group := range channelGroups(channel) {
			key := gm{group, s.Model}
			if newRule[key] == nil {
				newRule[key] = map[int64][]int{}
			}
			newRule[key][s.Priority] = append(newRule[key][s.Priority], s.ChannelId)
		}
	}

	keys := map[gm]bool{}
	for key := range newRule {
		keys[key] = true
	}
	for key := range legacyExact {
		// Only public names count: an alias was never routable under its own name, and listing a
		// routing policy's name on a channel never did anything.
		if resolveCanonicalIn(byModel, key.model) != key.model {
			continue
		}
		if info := byModel[key.model]; info != nil && info.Kind != "" {
			continue
		}
		keys[key] = true
	}
	sorted := make([]gm, 0, len(keys))
	for key := range keys {
		sorted = append(sorted, key)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].group != sorted[j].group {
			return sorted[i].group < sorted[j].group
		}
		return sorted[i].model < sorted[j].model
	})

	var diffs []SupplyRoutingDiff
	for _, key := range sorted {
		newTiers := tierList(newRule[key])
		var legacyTiers [][]int
		kind := ""
		if exact, ok := legacyExact[key]; ok {
			legacyTiers = tierList(exact)
		} else {
			var patterns []string
			for p := range legacyPattern {
				if p.group == key.group && strings.HasPrefix(key.model, strings.TrimSuffix(p.model, "*")) {
					patterns = append(patterns, p.model)
				}
			}
			sort.Strings(patterns)
			if len(patterns) > 0 {
				legacyTiers = tierList(legacyPattern[gm{key.group, patterns[0]}])
				kind = "wildcard"
			} else {
				kind = "newly_reachable"
			}
		}
		var upstreamChanges []string
		if kind == "" {
			for _, tier := range legacyTiers {
				for _, id := range tier {
					before, after := legacyUpstream[supplyKey{key.model, id}], newUpstream[supplyKey{key.model, id}]
					if after != "" && before != after {
						upstreamChanges = append(upstreamChanges, fmt.Sprintf("channel %d: %s -> %s", id, before, after))
					}
				}
			}
		}
		switch {
		case kind != "":
		case len(newTiers) == 0:
			kind = "lost"
		case !tiersEqual(legacyTiers, newTiers) || len(upstreamChanges) > 0:
			kind = "changed"
		default:
			continue
		}
		diffs = append(diffs, SupplyRoutingDiff{key.group, key.model, kind, legacyTiers, newTiers, upstreamChanges})
	}
	return diffs
}

// MigrateChannelsToSupplies plans the migration from the current database and, unless dryRun,
// applies it in one transaction. Running it again only adds what is still missing.
func MigrateChannelsToSupplies(dryRun bool) (*SupplyMigrationReport, error) {
	var in supplyMigrationInput
	var err error
	if in.channels, err = GetAllChannels(); err != nil {
		return nil, err
	}
	if in.infos, err = GetAllModelInfo(); err != nil {
		return nil, err
	}
	if in.prices, err = GetAllPrices(); err != nil {
		return nil, err
	}
	if in.existing, err = GetModelSupplies("", 0); err != nil {
		return nil, err
	}
	if in.policies, err = GetRoutingPolicies(); err != nil {
		return nil, err
	}
	if seed, err := LoadCatalogSeed(); err == nil {
		in.seed = seed
	}

	plan, report := planSupplyMigration(in)
	report.DryRun = dryRun
	if dryRun {
		return report, nil
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		for _, info := range plan.newInfos {
			if err := tx.Create(info).Error; err != nil {
				return err
			}
		}
		for _, info := range plan.newAliases {
			if err := tx.Create(info).Error; err != nil {
				return err
			}
		}
		for id, status := range plan.statusUpdates {
			if err := tx.Model(&ModelInfo{}).Where("id = ?", id).Update("status", status).Error; err != nil {
				return err
			}
		}
		if len(plan.priceCopies) > 0 {
			if err := InsertPrices(tx, plan.priceCopies); err != nil {
				return err
			}
		}
		if len(plan.supplies) > 0 {
			if err := tx.CreateInBatches(plan.supplies, 100).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	refreshCatalogCaches()
	reloadSupplies()
	return report, nil
}
