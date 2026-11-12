package model

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// useAllocationTestDB is the all-groups test database with supplies routing requests and the
// allocation tables, with the model info cache reset around the test.
func useAllocationTestDB(t *testing.T) {
	t.Helper()
	useAllGroupsTestDB(t, "default")
	if err := DB.AutoMigrate(&AllocationPolicy{}); err != nil {
		t.Fatal(err)
	}
	config.ModelSupplyEnabled = true
	modelInfoCache.Lock()
	oldLoaded := modelInfoCache.loaded
	modelInfoCache.loaded = false
	modelInfoCache.Unlock()
	t.Cleanup(func() {
		modelInfoCache.Lock()
		modelInfoCache.loaded = oldLoaded
		modelInfoCache.Unlock()
	})
}

type testChannel struct {
	name     string
	typ      int
	base     string
	priority int64
	weight   uint
}

func makeAllocationChannel(t *testing.T, c testChannel) *Channel {
	t.Helper()
	priority := c.priority
	weight := c.weight
	ch := &Channel{Name: c.name, Type: c.typ, Status: config.ChannelStatusEnabled, Group: "default", Priority: &priority, Weight: &weight}
	if c.base != "" {
		base := c.base
		ch.BaseURL = &base
	}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	return ch
}

func makePublicModel(t *testing.T, name string) *ModelInfo {
	t.Helper()
	info := &ModelInfo{Model: name, Status: ModelStatusPublished}
	if err := DB.Create(info).Error; err != nil {
		t.Fatal(err)
	}
	return info
}

func makeSupply(t *testing.T, model string, ch *Channel, priority int64, weight uint) *ModelSupply {
	t.Helper()
	s := &ModelSupply{Model: model, ChannelId: ch.Id, UpstreamModel: model, Priority: priority, Weight: weight, Enabled: true}
	if err := DB.Create(s).Error; err != nil {
		t.Fatal(err)
	}
	return s
}

// routingShares is, for one model in the default group, each tier's channel ids with their
// share of the tier's weight, first tier first.
func routingShares(t *testing.T, modelName string) []map[int]float64 {
	t.Helper()
	ChannelGroup.RLock()
	defer ChannelGroup.RUnlock()
	var out []map[int]float64
	for _, tier := range ChannelGroup.Rule["default"][modelName] {
		total := 0
		for _, id := range tier {
			total += ChannelGroup.supplyWeight(modelName, ChannelGroup.Channels[id].Channel)
		}
		shares := map[int]float64{}
		for _, id := range tier {
			shares[id] = float64(ChannelGroup.supplyWeight(modelName, ChannelGroup.Channels[id].Channel)) / float64(total)
		}
		out = append(out, shares)
	}
	return out
}

func sameShares(a, b []map[int]float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for id, share := range a[i] {
			if other, ok := b[i][id]; !ok || other-share > 1e-9 || share-other > 1e-9 {
				return false
			}
		}
	}
	return true
}

func TestProviderKeyOf(t *testing.T) {
	cases := []struct {
		typ  int
		base string
		want string
	}{
		{1, "", "1:"},
		{1, "https://Router.Requesty.ai/v1", "1:router.requesty.ai"},
		{1, "api.venice.ai/api/v1", "1:api.venice.ai"},
		{62, "", "62:"},
	}
	for _, c := range cases {
		ch := &Channel{Type: c.typ}
		if c.base != "" {
			base := c.base
			ch.BaseURL = &base
		}
		if got := ProviderKeyOf(ch); got != c.want {
			t.Errorf("ProviderKeyOf(%d, %q) = %q, want %q", c.typ, c.base, got, c.want)
		}
	}
}

// Tiers go first to last, a provider's weight is split between the channels that can take
// requests, a channel entry wins over its provider's, and unlisted providers come last.
func TestAllocationPlanAllocate(t *testing.T) {
	vercelA := &Channel{Id: 1, Type: 62, Status: config.ChannelStatusEnabled}
	vercelB := &Channel{Id: 2, Type: 62, Status: config.ChannelStatusEnabled}
	vercelOff := &Channel{Id: 3, Type: 62, Status: config.ChannelStatusManuallyDisabled}
	venice := &Channel{Id: 4, Type: 1, Status: config.ChannelStatusEnabled}
	other := &Channel{Id: 5, Type: 14, Status: config.ChannelStatusEnabled}
	channels := map[int]*Channel{1: vercelA, 2: vercelB, 3: vercelOff, 4: venice, 5: other}
	plan := newAllocationPlan([][]AllocationEntry{
		{{Provider: "62:", Weight: 3}, {ChannelId: 4, Weight: 1}},
		{{ChannelId: 2, Weight: 1}},
	})
	supplies := []*ModelSupply{
		{Id: 11, ChannelId: 1, Enabled: true},
		{Id: 12, ChannelId: 2, Enabled: true},
		{Id: 13, ChannelId: 3, Enabled: true},
		{Id: 14, ChannelId: 4, Enabled: true},
		{Id: 15, ChannelId: 5, Enabled: true},
	}
	got := plan.allocate(supplies, channels)
	want := map[int]supplyNumbers{
		11: {Priority: 2, Weight: 3}, // Vercel's 3 goes to the one enabled channel left
		13: {Priority: 2, Weight: 3},
		14: {Priority: 2, Weight: 1},
		12: {Priority: 1, Weight: 1}, // its own entry, second tier
		15: {Priority: 0, Weight: 1}, // not listed: below every tier
	}
	for id, n := range want {
		if got[id] != n {
			t.Errorf("supply %d = %+v, want %+v", id, got[id], n)
		}
	}

	// Two enabled Vercel keys split its weight: everything is doubled to keep it whole.
	vercelOff.Status = config.ChannelStatusEnabled
	got = plan.allocate(supplies, channels)
	want = map[int]supplyNumbers{
		11: {Priority: 2, Weight: 3}, // 6 / 2
		13: {Priority: 2, Weight: 3},
		14: {Priority: 2, Weight: 2},
		12: {Priority: 1, Weight: 2},
		15: {Priority: 0, Weight: 2},
	}
	for id, n := range want {
		if got[id] != n {
			t.Errorf("supply %d = %+v, want %+v", id, got[id], n)
		}
	}
}

// The first policy reproduces how every model routed before; models with their own numbers,
// or that pick the heaviest supply of a shared tier, become manual.
func TestEnsureDefaultAllocationPolicyKeepsRouting(t *testing.T) {
	useAllocationTestDB(t)
	keyA := makeAllocationChannel(t, testChannel{"vercel-a", 62, "", 10, 1})
	keyB := makeAllocationChannel(t, testChannel{"vercel-b", 62, "", 10, 1})
	requesty := makeAllocationChannel(t, testChannel{"requesty", 1, "https://router.requesty.ai/v1", 5, 1})
	mainCh := makeAllocationChannel(t, testChannel{"openai-main", 1, "", 10, 3})
	backup := makeAllocationChannel(t, testChannel{"openai-backup", 1, "", 10, 1})

	makePublicModel(t, "both-keys")
	makeSupply(t, "both-keys", keyA, 10, 1)
	makeSupply(t, "both-keys", keyB, 10, 1)
	makeSupply(t, "both-keys", requesty, 5, 1)
	// Only one Vercel key lists this one: a single Vercel entry would double its share.
	makePublicModel(t, "one-key")
	makeSupply(t, "one-key", keyA, 10, 1)
	makeSupply(t, "one-key", mainCh, 10, 3)
	makePublicModel(t, "openai")
	makeSupply(t, "openai", mainCh, 10, 3)
	makeSupply(t, "openai", backup, 10, 1)
	custom := makePublicModel(t, "custom")
	makeSupply(t, "custom", mainCh, 99, 3)
	makeSupply(t, "custom", backup, 10, 1)
	failover := makePublicModel(t, "failover")
	if err := DB.Model(failover).Update("supply_mode", SupplyModeFailover).Error; err != nil {
		t.Fatal(err)
	}
	makeSupply(t, "failover", mainCh, 10, 3)
	makeSupply(t, "failover", backup, 10, 1)

	// Routing as it was, from the stored supply numbers.
	names := []string{"both-keys", "one-key", "openai", "custom", "failover"}
	before := map[string][]map[int]float64{}
	for _, name := range names {
		before[name] = storedShares(t, name)
	}

	ChannelGroup.Load()
	policy, err := GetDefaultAllocationPolicy()
	if err != nil || policy == nil {
		t.Fatalf("policy = %v, err %v", policy, err)
	}
	for _, name := range names {
		if after := routingShares(t, name); !sameShares(before[name], after) {
			t.Errorf("%s routes %v, was %v", name, after, before[name])
		}
	}
	var manual []string
	DB.Model(&ModelInfo{}).Where("allocation = ?", AllocationManual).Order("model").Pluck("model", &manual)
	if !slices.Equal(manual, []string{"custom", "failover"}) {
		t.Fatalf("manual models = %v", manual)
	}
	if custom.Id == 0 {
		t.Fatal("custom model not created")
	}
	// Vercel's keys are split into channel entries; Requesty stays one provider entry.
	raw, _ := json.Marshal(policy.TierList())
	var tiers [][]AllocationEntry
	_ = json.Unmarshal(raw, &tiers)
	hasProvider := func(key string) bool {
		for _, tier := range tiers {
			for _, e := range tier {
				if e.Provider == key {
					return true
				}
			}
		}
		return false
	}
	if hasProvider("62:") || !hasProvider("1:router.requesty.ai") {
		t.Fatalf("tiers = %s", raw)
	}

	// A second load keeps the policy as it is.
	ChannelGroup.Load()
	var count int64
	DB.Model(&AllocationPolicy{}).Count(&count)
	if count != 1 {
		t.Fatalf("%d policies after reloading", count)
	}
}

// Saving a policy moves every model that follows it, shares a tier by weight whatever the site
// default mode, and leaves manual models alone.
func TestSaveDefaultAllocationPolicyRoutes(t *testing.T) {
	useAllocationTestDB(t)
	vercel := makeAllocationChannel(t, testChannel{"vercel", 62, "", 0, 1})
	requesty := makeAllocationChannel(t, testChannel{"requesty", 1, "https://router.requesty.ai", 0, 1})
	venice := makeAllocationChannel(t, testChannel{"venice", 1, "https://api.venice.ai", 0, 1})
	makePublicModel(t, "shared")
	makeSupply(t, "shared", vercel, 0, 1)
	makeSupply(t, "shared", requesty, 0, 1)
	makeSupply(t, "shared", venice, 0, 1)
	manual := makePublicModel(t, "pinned")
	makeSupply(t, "pinned", venice, 7, 1)
	makeSupply(t, "pinned", vercel, 1, 1)
	if err := DB.Model(manual).Update("allocation", AllocationManual).Error; err != nil {
		t.Fatal(err)
	}
	if err := EnsureDefaultAllocationPolicy(); err != nil {
		t.Fatal(err)
	}
	// A failover site default does not apply to models on the policy.
	oldDefault := config.SupplyModeDefault
	config.SupplyModeDefault = SupplyModeFailover
	t.Cleanup(func() { config.SupplyModeDefault = oldDefault })

	if _, err := SaveDefaultAllocationPolicy([][]AllocationEntry{
		{{Provider: "62:", Weight: 70}, {Provider: "1:api.venice.ai", Weight: 30}},
		{{Provider: "1:router.requesty.ai", Weight: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	ChannelGroup.Load()
	want := []map[int]float64{{vercel.Id: 0.7, venice.Id: 0.3}, {requesty.Id: 1}}
	if got := routingShares(t, "shared"); !sameShares(got, want) {
		t.Fatalf("shared routes %v, want %v", got, want)
	}
	if supplyModeOf("shared") != SupplyModeBalance {
		t.Fatal("a model on the policy should share its tier by weight")
	}
	if got := routingShares(t, "pinned"); !sameShares(got, []map[int]float64{{venice.Id: 1}, {vercel.Id: 1}}) {
		t.Fatalf("manual model moved: %v", got)
	}

	views, err := GetModelSupplyViews("shared", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range views {
		if !v.FollowsPolicy {
			t.Fatalf("view %d does not report the policy", v.Id)
		}
	}
	if pinned, _ := GetModelSupplyViews("pinned", 0); pinned[0].FollowsPolicy {
		t.Fatal("a manual model's supplies should show their own numbers")
	}

	if _, err := SaveDefaultAllocationPolicy([][]AllocationEntry{{{Provider: "62:"}, {Provider: "62:"}}}); err != ErrAllocationDuplicate {
		t.Fatalf("duplicate provider: err %v", err)
	}
	if _, err := SaveDefaultAllocationPolicy([][]AllocationEntry{{{Provider: "62:", ChannelId: 1}}}); err != ErrAllocationEntry {
		t.Fatalf("entry with both targets: err %v", err)
	}
}

// Going manual keeps the numbers the model routed with; renumbering one supply of a model on
// the policy makes it manual, while saving the numbers it was shown does not.
func TestModelAllocationSwitch(t *testing.T) {
	useAllocationTestDB(t)
	vercel := makeAllocationChannel(t, testChannel{"vercel", 62, "", 0, 1})
	venice := makeAllocationChannel(t, testChannel{"venice", 1, "https://api.venice.ai", 0, 1})
	info := makePublicModel(t, "m")
	sVercel := makeSupply(t, "m", vercel, 0, 1)
	sVenice := makeSupply(t, "m", venice, 0, 1)
	if _, err := SaveDefaultAllocationPolicy([][]AllocationEntry{{{Provider: "62:"}}, {{Provider: "1:api.venice.ai"}}}); err != nil {
		t.Fatal(err)
	}

	// Toggling a field other than the numbers, sending the numbers shown.
	shown, _ := GetModelSupplyViews("m", 0)
	for _, v := range shown {
		if v.Id == sVenice.Id {
			update := &ModelSupply{Id: v.Id, UpstreamModel: "venice/m", Priority: v.Priority, Weight: v.Weight, Enabled: true}
			if err := UpdateModelSupply(update); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got, _ := GetModelInfoByModel("m"); got.Allocation != "" {
		t.Fatalf("allocation = %q after saving the shown numbers", got.Allocation)
	}

	// Renumbering puts Venice first and makes the model manual; Vercel keeps the policy's place.
	if err := UpdateModelSupply(&ModelSupply{Id: sVenice.Id, UpstreamModel: "venice/m", Priority: 5, Weight: 1, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetModelInfoByModel("m"); got.Allocation != AllocationManual {
		t.Fatalf("allocation = %q after renumbering", got.Allocation)
	}
	if s, _ := GetModelSupply(sVercel.Id); s.Priority != 2 || s.Weight != 1 {
		t.Fatalf("vercel supply = %d / %d, want the policy's 2 / 1", s.Priority, s.Weight)
	}
	ChannelGroup.Load()
	if got := routingShares(t, "m"); !sameShares(got, []map[int]float64{{venice.Id: 1}, {vercel.Id: 1}}) {
		t.Fatalf("routes %v", got)
	}

	// Back to the policy, then manual again: the frozen numbers are the policy's.
	if _, err := SetModelAllocation([]int{info.Id}, "policy"); err != nil {
		t.Fatal(err)
	}
	if got := routingShares(t, "m"); !sameShares(got, []map[int]float64{{vercel.Id: 1}, {venice.Id: 1}}) {
		t.Fatalf("back on the policy routes %v", got)
	}
	if _, err := SetModelAllocation([]int{info.Id}, AllocationManual); err != nil {
		t.Fatal(err)
	}
	if s, _ := GetModelSupply(sVenice.Id); s.Priority != 1 || s.Weight != 1 {
		t.Fatalf("frozen venice supply = %d / %d", s.Priority, s.Weight)
	}
	if _, err := SetModelAllocation([]int{info.Id}, "bogus"); err != ErrAllocationValue {
		t.Fatalf("bogus value: err %v", err)
	}
}

// The request preview counts a channel open to all groups as serving the asked group.
func TestPreviewTreatsAllGroupsChannelAsInGroup(t *testing.T) {
	useAllocationTestDB(t)
	ch := makeAllocationChannel(t, testChannel{"all", 62, "", 0, 1})
	if err := DB.Model(ch).Update("group", ChannelGroupAll).Error; err != nil {
		t.Fatal(err)
	}
	GlobalUserGroupRatio.Load()
	makePublicModel(t, "m")
	makeSupply(t, "m", ch, 0, 1)
	ChannelGroup.Load()
	preview, err := PreviewModelSupply("m", "default")
	if err != nil {
		t.Fatal(err)
	}
	if preview.OutsideGroup != 0 || len(preview.Tiers) != 1 {
		t.Fatalf("outside %d, tiers %d", preview.OutsideGroup, len(preview.Tiers))
	}
}

// storedShares is routingShares computed from the supply rows, as routing works without a
// policy: tiers by priority, highest first, shares by weight.
func storedShares(t *testing.T, modelName string) []map[int]float64 {
	t.Helper()
	var supplies []*ModelSupply
	if err := DB.Where("model = ? AND enabled = ?", modelName, true).Order("priority desc").Find(&supplies).Error; err != nil {
		t.Fatal(err)
	}
	var out []map[int]float64
	var priorities []int64
	byPriority := map[int64][]*ModelSupply{}
	for _, s := range supplies {
		if _, ok := byPriority[s.Priority]; !ok {
			priorities = append(priorities, s.Priority)
		}
		byPriority[s.Priority] = append(byPriority[s.Priority], s)
	}
	for _, p := range priorities {
		total := 0
		for _, s := range byPriority[p] {
			total += s.EffectiveWeight()
		}
		shares := map[int]float64{}
		for _, s := range byPriority[p] {
			shares[s.ChannelId] = float64(s.EffectiveWeight()) / float64(total)
		}
		out = append(out, shares)
	}
	return out
}

// Saving a catalog row whole (the model form) keeps the model's allocation.
func TestUpdateModelInfoKeepsAllocation(t *testing.T) {
	useAllocationTestDB(t)
	info := makePublicModel(t, "m")
	if err := DB.Model(info).Update("allocation", AllocationManual).Error; err != nil {
		t.Fatal(err)
	}
	form := &ModelInfo{Id: info.Id, Model: "m", Status: ModelStatusPublished, Description: "edited"}
	if err := UpdateModelInfo(form); err != nil {
		t.Fatal(err)
	}
	if got, _ := GetModelInfoByModel("m"); got.Allocation != AllocationManual || got.Description != "edited" {
		t.Fatalf("after saving: allocation %q, description %q", got.Allocation, got.Description)
	}
}
