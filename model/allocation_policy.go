package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"sync/atomic"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"gorm.io/gorm"
)

// An allocation policy decides, while supplies route requests, which supply of a public model
// gets a request and which ones are tried after it. The default policy is a list of tiers tried
// in order; a tier holds one or more entries that share its traffic by weight. An entry is a
// provider (every channel of one type and address, sharing the entry's weight equally) or one
// channel. Models that follow the policy (ModelInfo.Allocation empty) get their supplies'
// priority and weight from it when the routing table loads; a model set to manual keeps the
// numbers stored on its supplies.

const (
	AllocationScopeDefault = "default"
	// AllocationManual marks a model whose supplies keep their own priority and weight.
	AllocationManual = "manual"
)

var (
	ErrAllocationEntry     = errors.New("each entry names either a provider or a channel")
	ErrAllocationDuplicate = errors.New("a provider or channel appears more than once in the policy")
	ErrAllocationValue     = errors.New("allocation must be \"policy\" or \"manual\"")
)

// AllocationEntry is one member of a tier: Provider (see ProviderKeyOf) or ChannelId, never both.
type AllocationEntry struct {
	Provider  string `json:"provider,omitempty"`
	ChannelId int    `json:"channel_id,omitempty"`
	Weight    uint   `json:"weight"`
}

func (e AllocationEntry) effectiveWeight() int {
	if e.Weight == 0 {
		return 1
	}
	return int(e.Weight)
}

// AllocationPolicy is stored once per organization and scope; organization 0 is the platform.
type AllocationPolicy struct {
	Id             int    `json:"id"`
	OrganizationId int    `json:"organization_id" gorm:"not null;default:0;uniqueIndex:idx_allocation_policies_scope,priority:1"`
	Scope          string `json:"scope" gorm:"type:varchar(32);not null;uniqueIndex:idx_allocation_policies_scope,priority:2"`
	// Tiers is the JSON of [][]AllocationEntry, first tier first.
	Tiers     string `json:"-" gorm:"type:text"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (p *AllocationPolicy) TableName() string {
	return "allocation_policies"
}

// TierList decodes Tiers; a broken value reads as no tiers.
func (p *AllocationPolicy) TierList() [][]AllocationEntry {
	var tiers [][]AllocationEntry
	if p == nil || strings.TrimSpace(p.Tiers) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(p.Tiers), &tiers); err != nil {
		return nil
	}
	return tiers
}

// ProviderKeyOf identifies the provider behind a channel: its type and the host of its address,
// empty for the type's own address. Two relays of the same type are two providers.
func ProviderKeyOf(channel *Channel) string {
	return fmt.Sprintf("%d:%s", channel.Type, channelHost(channel.GetBaseURL()))
}

func channelHost(base string) string {
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return strings.ToLower(base)
	}
	return strings.ToLower(parsed.Host)
}

// ---------- Compiling the policy into supply numbers ----------

// supplyNumbers is the priority and weight a supply routes with.
type supplyNumbers struct {
	Priority int64
	Weight   uint
}

// allocationPlan is a policy indexed for compiling.
type allocationPlan struct {
	tiers     int
	byChannel map[int]entrySlot
	byProvid  map[string]entrySlot
}

type entrySlot struct {
	tier   int
	key    string
	weight int
}

func newAllocationPlan(tiers [][]AllocationEntry) *allocationPlan {
	plan := &allocationPlan{tiers: len(tiers), byChannel: map[int]entrySlot{}, byProvid: map[string]entrySlot{}}
	for t, tier := range tiers {
		for _, entry := range tier {
			if entry.ChannelId > 0 {
				plan.byChannel[entry.ChannelId] = entrySlot{tier: t, key: fmt.Sprintf("c%d", entry.ChannelId), weight: entry.effectiveWeight()}
			} else if entry.Provider != "" {
				plan.byProvid[entry.Provider] = entrySlot{tier: t, key: "p" + entry.Provider, weight: entry.effectiveWeight()}
			}
		}
	}
	return plan
}

// slotOf finds where a channel sits: its own entry first, then its provider's. A channel the
// policy does not place goes below every tier, each in a position of its own in the order the
// channels were created, so a new channel is tried last until it is moved.
func (p *allocationPlan) slotOf(channel *Channel) entrySlot {
	if slot, ok := p.byChannel[channel.Id]; ok {
		return slot
	}
	key := ProviderKeyOf(channel)
	if slot, ok := p.byProvid[key]; ok {
		return slot
	}
	return entrySlot{tier: p.tiers + channel.Id, key: fmt.Sprintf("u%d", channel.Id), weight: 1}
}

// allocate computes the numbers of one model's supplies. A provider entry's weight is split
// equally between the supplies it covers that can take requests (supply and channel enabled);
// when none can, between all of them, so they still have numbers once one comes back. Weights
// are multiplied by the least common multiple of those pool sizes so the split stays whole:
// without a shared pool they are the entries' own weights.
func (p *allocationPlan) allocate(supplies []*ModelSupply, channels map[int]*Channel) map[int]supplyNumbers {
	slots := make(map[int]entrySlot, len(supplies))
	active := map[string]int{}
	all := map[string]int{}
	for _, s := range supplies {
		channel := channels[s.ChannelId]
		if channel == nil {
			continue
		}
		slot := p.slotOf(channel)
		slots[s.Id] = slot
		all[slot.key]++
		if s.Enabled && channel.Status == config.ChannelStatusEnabled {
			active[slot.key]++
		}
	}
	poolOf := func(key string) int {
		if active[key] > 0 {
			return active[key]
		}
		return all[key]
	}
	scale := 1
	for key := range all {
		scale = lcm(scale, poolOf(key))
		if scale > 720720 { // keep weights in range for absurd pools; rounding is fine there
			scale = 720720
			break
		}
	}
	out := make(map[int]supplyNumbers, len(slots))
	for _, s := range supplies {
		slot, ok := slots[s.Id]
		if !ok {
			continue
		}
		weight := uint(math.Round(float64(slot.weight*scale) / float64(poolOf(slot.key))))
		if weight == 0 {
			weight = 1
		}
		out[s.Id] = supplyNumbers{Priority: int64(p.tiers - slot.tier), Weight: weight}
	}
	return out
}

// followsPolicy reports whether a model takes its supply numbers from the policy.
func followsPolicy(info *ModelInfo) bool {
	return info == nil || info.Allocation != AllocationManual
}

// GetDefaultAllocationPolicy reads the platform's default policy, nil when there is none yet.
func GetDefaultAllocationPolicy() (*AllocationPolicy, error) {
	var policy AllocationPolicy
	err := DB.Where("organization_id = ? AND scope = ?", 0, AllocationScopeDefault).First(&policy).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &policy, nil
}

// applyAllocation replaces, for models that follow the policy, the priority and weight of the
// given supplies with the policy's. Supplies are copied, never changed in place. Every supply of
// a model must be in the list, since a provider's weight is split between them.
func applyAllocation(policy *AllocationPolicy, supplies []*ModelSupply, channels map[int]*Channel, infos map[string]*ModelInfo) []*ModelSupply {
	if policy == nil {
		return supplies
	}
	plan := newAllocationPlan(policy.TierList())
	byModel := map[string][]*ModelSupply{}
	for _, s := range supplies {
		byModel[s.Model] = append(byModel[s.Model], s)
	}
	numbers := map[int]supplyNumbers{}
	for name, list := range byModel {
		if !followsPolicy(infos[name]) {
			continue
		}
		for id, n := range plan.allocate(list, channels) {
			numbers[id] = n
		}
	}
	out := make([]*ModelSupply, len(supplies))
	for i, s := range supplies {
		n, ok := numbers[s.Id]
		if !ok {
			out[i] = s
			continue
		}
		copied := *s
		copied.Priority, copied.Weight = n.Priority, n.Weight
		out[i] = &copied
	}
	return out
}

// allocatedNumbers returns, by supply id, the numbers the policy gives the supplies of the given
// models that follow it. Supplies of manual models, and every supply when there is no policy,
// are left out.
func allocatedNumbers(models []string) (map[int]supplyNumbers, error) {
	out := map[int]supplyNumbers{}
	if len(models) == 0 {
		return out, nil
	}
	policy, err := GetDefaultAllocationPolicy()
	if err != nil || policy == nil {
		return out, err
	}
	var supplies []*ModelSupply
	if err := DB.Where("model IN ?", models).Find(&supplies).Error; err != nil {
		return nil, err
	}
	channels, infos, err := allocationContext(supplies)
	if err != nil {
		return nil, err
	}
	plan := newAllocationPlan(policy.TierList())
	byModel := map[string][]*ModelSupply{}
	for _, s := range supplies {
		byModel[s.Model] = append(byModel[s.Model], s)
	}
	for name, list := range byModel {
		if !followsPolicy(infos[name]) {
			continue
		}
		for id, n := range plan.allocate(list, channels) {
			out[id] = n
		}
	}
	return out, nil
}

// allocationInForce reports whether supplies route requests under an allocation policy.
func allocationInForce() bool {
	if !config.ModelSupplyEnabled {
		return false
	}
	policy, err := GetDefaultAllocationPolicy()
	return err == nil && policy != nil
}

func allocationContext(supplies []*ModelSupply) (map[int]*Channel, map[string]*ModelInfo, error) {
	channelIds := map[int]bool{}
	names := map[string]bool{}
	for _, s := range supplies {
		channelIds[s.ChannelId] = true
		names[s.Model] = true
	}
	ids := make([]int, 0, len(channelIds))
	for id := range channelIds {
		ids = append(ids, id)
	}
	channels := map[int]*Channel{}
	if len(ids) > 0 {
		var list []*Channel
		if err := DB.Select("id", "type", "base_url", "status").Where("id IN ?", ids).Find(&list).Error; err != nil {
			return nil, nil, err
		}
		for _, ch := range list {
			channels[ch.Id] = ch
		}
	}
	infos := map[string]*ModelInfo{}
	modelNames := make([]string, 0, len(names))
	for name := range names {
		modelNames = append(modelNames, name)
	}
	if len(modelNames) > 0 {
		var list []*ModelInfo
		if err := DB.Select("model", "allocation", "supply_mode").Where("model IN ?", modelNames).Find(&list).Error; err != nil {
			return nil, nil, err
		}
		for _, info := range list {
			infos[info.Model] = info
		}
	}
	return channels, infos, nil
}

// ---------- Saving ----------

// validateTiers drops empty tiers and checks entries: one target each, no target twice.
func validateTiers(tiers [][]AllocationEntry) ([][]AllocationEntry, error) {
	seen := map[string]bool{}
	var out [][]AllocationEntry
	for _, tier := range tiers {
		var entries []AllocationEntry
		for _, entry := range tier {
			entry.Provider = strings.TrimSpace(entry.Provider)
			if (entry.Provider == "") == (entry.ChannelId <= 0) {
				return nil, ErrAllocationEntry
			}
			key := entry.Provider
			if entry.ChannelId > 0 {
				key = fmt.Sprintf("#%d", entry.ChannelId)
			}
			if seen[key] {
				return nil, ErrAllocationDuplicate
			}
			seen[key] = true
			if entry.Weight == 0 {
				entry.Weight = 1
			}
			entries = append(entries, entry)
		}
		if len(entries) > 0 {
			out = append(out, entries)
		}
	}
	return out, nil
}

// SaveDefaultAllocationPolicy stores the platform's default tiers and rebuilds the routing table.
func SaveDefaultAllocationPolicy(tiers [][]AllocationEntry) (*AllocationPolicy, error) {
	clean, err := validateTiers(tiers)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(clean)
	if err != nil {
		return nil, err
	}
	policy, err := GetDefaultAllocationPolicy()
	if err != nil {
		return nil, err
	}
	if policy == nil {
		policy = &AllocationPolicy{OrganizationId: 0, Scope: AllocationScopeDefault, Tiers: string(raw)}
		err = DB.Create(policy).Error
	} else {
		err = DB.Model(policy).Update("tiers", string(raw)).Error
		policy.Tiers = string(raw)
	}
	if err != nil {
		return nil, err
	}
	reloadSupplies()
	return policy, nil
}

// SetModelAllocation switches public models (by catalog id) between following the policy and
// manual numbers. Going manual stores the numbers the models route with now on their supplies,
// so nothing moves.
func SetModelAllocation(ids []int, allocation string) (int64, error) {
	var value string
	switch allocation {
	case "policy", "":
		value = ""
	case AllocationManual:
		value = AllocationManual
	default:
		return 0, ErrAllocationValue
	}
	if len(ids) == 0 {
		return 0, nil
	}
	var models []string
	if err := DB.Model(&ModelInfo{}).Where("id IN ? AND alias_of = ? AND kind = ?", ids, "", "").Pluck("model", &models).Error; err != nil {
		return 0, err
	}
	if len(models) == 0 {
		return 0, nil
	}
	var frozen map[int]supplyNumbers
	if value == AllocationManual {
		var err error
		if frozen, err = allocatedNumbers(models); err != nil {
			return 0, err
		}
	}
	var affected int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := writeSupplyNumbers(tx, frozen); err != nil {
			return err
		}
		result := tx.Model(&ModelInfo{}).Where("model IN ?", models).Update("allocation", value)
		affected = result.RowsAffected
		return result.Error
	})
	if err != nil {
		return 0, err
	}
	refreshCatalogCaches()
	reloadSupplies()
	return affected, nil
}

// writeSupplyNumbers stores priority and weight on supplies by id.
func writeSupplyNumbers(tx *gorm.DB, numbers map[int]supplyNumbers) error {
	for id, n := range numbers {
		if err := tx.Model(&ModelSupply{}).Where("id = ?", id).
			Updates(map[string]any{"priority": n.Priority, "weight": n.Weight}).Error; err != nil {
			return err
		}
	}
	return nil
}

// ---------- First policy ----------

// EnsureDefaultAllocationPolicy creates the default policy from the current channel numbers the
// first time supplies route requests, and marks manual every model whose routing the policy
// could not reproduce, so the switch moves no traffic. It does nothing once a policy exists.
func EnsureDefaultAllocationPolicy() error {
	existing, err := GetDefaultAllocationPolicy()
	if err != nil || existing != nil {
		return err
	}
	var channels []*Channel
	if err := DB.Select("id", "type", "base_url", "status", "priority", "weight").Find(&channels).Error; err != nil {
		return err
	}
	var supplies []*ModelSupply
	if err := DB.Find(&supplies).Error; err != nil {
		return err
	}
	var infos []*ModelInfo
	if err := DB.Select("model", "allocation", "supply_mode").Find(&infos).Error; err != nil {
		return err
	}
	tiers, manual := buildDefaultAllocation(channels, supplies, infos)
	raw, err := json.Marshal(tiers)
	if err != nil {
		return err
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&AllocationPolicy{OrganizationId: 0, Scope: AllocationScopeDefault, Tiers: string(raw)}).Error; err != nil {
			return err
		}
		if len(manual) > 0 {
			return tx.Model(&ModelInfo{}).Where("model IN ?", manual).Update("allocation", AllocationManual).Error
		}
		return nil
	})
	if err != nil {
		// Another node may have created it first.
		if again, getErr := GetDefaultAllocationPolicy(); getErr == nil && again != nil {
			return nil
		}
		return err
	}
	refreshCatalogCaches()
	logger.SysLog(fmt.Sprintf("allocation policy created from channel numbers: %d tiers, %d models set to manual", len(tiers), len(manual)))
	return nil
}

// buildDefaultAllocation turns channel priorities and weights into tiers (one per distinct
// priority, highest first) and lists the models that must stay manual. A provider becomes one
// entry when all its channels share a priority and weight and that reproduces every model's
// routing; otherwise each of its channels gets its own entry.
func buildDefaultAllocation(channels []*Channel, supplies []*ModelSupply, infos []*ModelInfo) ([][]AllocationEntry, []string) {
	byId := make(map[int]*Channel, len(channels))
	for _, ch := range channels {
		byId[ch.Id] = ch
	}
	infoByModel := make(map[string]*ModelInfo, len(infos))
	for _, info := range infos {
		infoByModel[info.Model] = info
	}
	byModel := map[string][]*ModelSupply{}
	for _, s := range supplies {
		byModel[s.Model] = append(byModel[s.Model], s)
	}

	// Models whose supplies do not all follow their channels, or that pick the heaviest supply
	// of a shared tier, keep their own numbers.
	manualSet := map[string]bool{}
	for name, list := range byModel {
		info := infoByModel[name]
		if info == nil {
			continue
		}
		if info.Allocation == AllocationManual || !suppliesFollowChannels(list, byId) ||
			(EffectiveSupplyMode(info.SupplyMode) == SupplyModeFailover && sharesTier(list, byId)) {
			manualSet[name] = true
		}
	}

	split := map[string]bool{} // providers written as one entry per channel
	var tiers [][]AllocationEntry
	for round := 0; round < 4; round++ {
		tiers = tiersFromChannels(channels, split)
		plan := newAllocationPlan(tiers)
		changed := false
		for name, list := range byModel {
			if manualSet[name] || infoByModel[name] == nil {
				continue
			}
			for _, provider := range routingDiff(list, byId, plan.allocate(list, byId)) {
				if !split[provider] {
					split[provider] = true
					changed = true
				}
			}
		}
		if !changed {
			break
		}
	}
	// Whatever still differs after splitting every provider stays manual.
	plan := newAllocationPlan(tiers)
	for name, list := range byModel {
		if manualSet[name] || infoByModel[name] == nil {
			continue
		}
		if len(routingDiff(list, byId, plan.allocate(list, byId))) > 0 {
			manualSet[name] = true
		}
	}
	manual := make([]string, 0, len(manualSet))
	for name := range manualSet {
		manual = append(manual, name)
	}
	sort.Strings(manual)
	return tiers, manual
}

func suppliesFollowChannels(list []*ModelSupply, channels map[int]*Channel) bool {
	for _, s := range list {
		ch := channels[s.ChannelId]
		if ch == nil {
			continue
		}
		if s.Priority != ch.GetPriority() || int64(s.EffectiveWeight()) != ch.GetWeight() {
			return false
		}
	}
	return true
}

func sharesTier(list []*ModelSupply, channels map[int]*Channel) bool {
	count := map[int64]int{}
	for _, s := range list {
		if ch := channels[s.ChannelId]; ch != nil && s.Enabled && ch.Status == config.ChannelStatusEnabled {
			count[s.Priority]++
			if count[s.Priority] > 1 {
				return true
			}
		}
	}
	return false
}

func tiersFromChannels(channels []*Channel, split map[string]bool) [][]AllocationEntry {
	type group struct {
		priority int64
		channels []*Channel
	}
	byProvider := map[string][]*Channel{}
	for _, ch := range channels {
		key := ProviderKeyOf(ch)
		byProvider[key] = append(byProvider[key], ch)
	}
	byPriority := map[int64][]AllocationEntry{}
	providers := make([]string, 0, len(byProvider))
	for key := range byProvider {
		providers = append(providers, key)
	}
	sort.Strings(providers)
	for _, key := range providers {
		list := byProvider[key]
		sort.Slice(list, func(i, j int) bool { return list[i].Id < list[j].Id })
		uniform := !split[key]
		for _, ch := range list[1:] {
			if ch.GetPriority() != list[0].GetPriority() || ch.GetWeight() != list[0].GetWeight() {
				uniform = false
			}
		}
		if uniform {
			p := list[0].GetPriority()
			byPriority[p] = append(byPriority[p], AllocationEntry{Provider: key, Weight: uint(list[0].GetWeight()) * uint(len(list))})
			continue
		}
		for _, ch := range list {
			p := ch.GetPriority()
			byPriority[p] = append(byPriority[p], AllocationEntry{ChannelId: ch.Id, Weight: uint(ch.GetWeight())})
		}
	}
	priorities := make([]int64, 0, len(byPriority))
	for p := range byPriority {
		priorities = append(priorities, p)
	}
	sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
	tiers := make([][]AllocationEntry, 0, len(priorities))
	for _, p := range priorities {
		tiers = append(tiers, byPriority[p])
	}
	return tiers
}

// routingDiff compares how a model routes with its stored numbers and with the compiled ones:
// the same supplies must form the same tiers in the same order, with the same shares inside
// each tier. Only supplies that can take requests count. It returns the providers of supplies
// that differ.
func routingDiff(list []*ModelSupply, channels map[int]*Channel, compiled map[int]supplyNumbers) []string {
	type numbered struct {
		id       int
		provider string
		priority int64
		weight   float64
	}
	var oldList, newList []numbered
	for _, s := range list {
		ch := channels[s.ChannelId]
		if ch == nil || !s.Enabled || ch.Status != config.ChannelStatusEnabled {
			continue
		}
		n, ok := compiled[s.Id]
		if !ok {
			continue
		}
		key := ProviderKeyOf(ch)
		oldList = append(oldList, numbered{s.Id, key, s.Priority, float64(s.EffectiveWeight())})
		newList = append(newList, numbered{s.Id, key, n.Priority, float64(n.Weight)})
	}
	shape := func(items []numbered) (map[int]int, map[int]float64) {
		var priorities []int64
		seen := map[int64]bool{}
		sums := map[int64]float64{}
		for _, it := range items {
			if !seen[it.priority] {
				seen[it.priority] = true
				priorities = append(priorities, it.priority)
			}
			sums[it.priority] += it.weight
		}
		sort.Slice(priorities, func(i, j int) bool { return priorities[i] > priorities[j] })
		rank := map[int64]int{}
		for i, p := range priorities {
			rank[p] = i
		}
		tier := map[int]int{}
		share := map[int]float64{}
		for _, it := range items {
			tier[it.id] = rank[it.priority]
			share[it.id] = it.weight / sums[it.priority]
		}
		return tier, share
	}
	oldTier, oldShare := shape(oldList)
	newTier, newShare := shape(newList)
	differs := map[string]bool{}
	for _, it := range oldList {
		if oldTier[it.id] != newTier[it.id] || math.Abs(oldShare[it.id]-newShare[it.id]) > 1e-6 {
			differs[it.provider] = true
		}
	}
	out := make([]string, 0, len(differs))
	for p := range differs {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// ---------- Routing table ----------

var allocationActive atomic.Bool

// currentAllocationActive reports whether the last routing table load applied a policy.
func currentAllocationActive() bool {
	return allocationActive.Load()
}

// allocateLoaded applies the default policy to the supplies the routing table loads, creating
// the policy first when supplies route requests for the first time.
func (cc *ChannelsChooser) allocateLoaded(supplies []*ModelSupply, channels []*Channel) []*ModelSupply {
	if err := EnsureDefaultAllocationPolicy(); err != nil {
		logger.SysError("create allocation policy failed: " + err.Error())
	}
	policy, err := GetDefaultAllocationPolicy()
	if err != nil {
		logger.SysError("load allocation policy failed: " + err.Error())
	}
	allocationActive.Store(policy != nil)
	if policy == nil {
		return supplies
	}
	byId := make(map[int]*Channel, len(channels))
	for _, ch := range channels {
		byId[ch.Id] = ch
	}
	infos := map[string]*ModelInfo{}
	for _, s := range supplies {
		if _, ok := infos[s.Model]; !ok {
			infos[s.Model] = lookupModelInfo(s.Model)
		}
	}
	return applyAllocation(policy, supplies, byId, infos)
}

// takeManualOnRenumber switches a model that follows the policy to manual when an update gives
// one of its supplies numbers other than the policy's: the other supplies keep the policy's
// numbers, written onto them, and this one takes the new ones. Updates that only change other
// fields (they send the numbers they were shown) leave the model on the policy.
func takeManualOnRenumber(modelName string, supply *ModelSupply) error {
	if !allocationInForce() || !followsPolicy(lookupModelInfo(modelName)) {
		return nil
	}
	numbers, err := allocatedNumbers([]string{modelName})
	if err != nil {
		return err
	}
	shown, ok := numbers[supply.Id]
	if !ok || (shown.Priority == supply.Priority && shown.Weight == effectiveUint(supply.Weight)) {
		// Same numbers: keep the policy, and do not store the shown numbers on the row.
		if ok {
			stored, getErr := GetModelSupply(supply.Id)
			if getErr == nil {
				supply.Priority, supply.Weight = stored.Priority, stored.Weight
			}
		}
		return nil
	}
	err = DB.Transaction(func(tx *gorm.DB) error {
		delete(numbers, supply.Id)
		if err := writeSupplyNumbers(tx, numbers); err != nil {
			return err
		}
		return tx.Model(&ModelInfo{}).Where("model = ?", modelName).Update("allocation", AllocationManual).Error
	})
	if err != nil {
		return err
	}
	refreshCatalogCaches()
	return nil
}

func effectiveUint(weight uint) uint {
	if weight == 0 {
		return 1
	}
	return weight
}

// ---------- Admin view ----------

// AllocationProvider is one provider the policy can place, with its channels.
type AllocationProvider struct {
	Key      string                      `json:"key"`
	Type     int                         `json:"type"`
	Host     string                      `json:"host"`
	Channels []AllocationProviderChannel `json:"channels"`
}

type AllocationProviderChannel struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
	Tag    string `json:"tag"`
	// Models are the public models the channel has an enabled supply for, sorted. Entries of a
	// tier only share traffic for the models they have in common.
	Models []string `json:"models"`
}

// AllocationOverview is what the allocation page shows.
type AllocationOverview struct {
	// SupplyRouting is false while channels' model lists still route requests: the policy then
	// has no effect.
	SupplyRouting bool                  `json:"supply_routing"`
	Tiers         [][]AllocationEntry   `json:"tiers"`
	Providers     []*AllocationProvider `json:"providers"`
	PolicyModels  int64                 `json:"policy_models"`
	ManualModels  int64                 `json:"manual_models"`
	UpdatedAt     int64                 `json:"updated_at"`
}

// GetAllocationOverview reads the default policy (creating it first when supplies route
// requests), the providers behind every channel and how many public models follow the policy.
func GetAllocationOverview() (*AllocationOverview, error) {
	overview := &AllocationOverview{SupplyRouting: config.ModelSupplyEnabled, Tiers: [][]AllocationEntry{}, Providers: []*AllocationProvider{}}
	if config.ModelSupplyEnabled {
		if err := EnsureDefaultAllocationPolicy(); err != nil {
			return nil, err
		}
	}
	policy, err := GetDefaultAllocationPolicy()
	if err != nil {
		return nil, err
	}
	if policy != nil {
		if tiers := policy.TierList(); tiers != nil {
			overview.Tiers = tiers
		}
		overview.UpdatedAt = policy.UpdatedAt
	}
	var channels []*Channel
	if err := DB.Select("id", "name", "type", "base_url", "status", "tag").Order("id").Find(&channels).Error; err != nil {
		return nil, err
	}
	var supplies []*ModelSupply
	if err := DB.Select("model", "channel_id").Where("enabled = ?", true).Order("model").Find(&supplies).Error; err != nil {
		return nil, err
	}
	modelsByChannel := map[int][]string{}
	for _, s := range supplies {
		modelsByChannel[s.ChannelId] = append(modelsByChannel[s.ChannelId], s.Model)
	}
	byKey := map[string]*AllocationProvider{}
	for _, ch := range channels {
		key := ProviderKeyOf(ch)
		provider := byKey[key]
		if provider == nil {
			provider = &AllocationProvider{Key: key, Type: ch.Type, Host: channelHost(ch.GetBaseURL())}
			byKey[key] = provider
			overview.Providers = append(overview.Providers, provider)
		}
		models := modelsByChannel[ch.Id]
		if models == nil {
			models = []string{}
		}
		provider.Channels = append(provider.Channels, AllocationProviderChannel{Id: ch.Id, Name: ch.Name, Status: ch.Status, Tag: ch.Tag, Models: models})
	}
	public := DB.Model(&ModelInfo{}).Where("alias_of = ? AND kind = ?", "", "")
	if err := public.Session(&gorm.Session{}).Where("allocation = ?", AllocationManual).Count(&overview.ManualModels).Error; err != nil {
		return nil, err
	}
	var total int64
	if err := public.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, err
	}
	overview.PolicyModels = total - overview.ManualModels
	return overview, nil
}

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func lcm(a, b int) int {
	if a == 0 || b == 0 {
		return max(a, b)
	}
	return a / gcd(a, b) * b
}
