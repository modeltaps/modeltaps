package model

import "testing"

func TestDecideIntakeDefaults(t *testing.T) {
	rules := DefaultIntakeRules()
	cases := []struct {
		name     string
		subject  intakeSubject
		action   string
		joinLast bool
	}{
		{"chat model nobody supplies", intakeSubject{model: "gpt-9", endpoints: []string{"chat"}}, IntakeActionEnable, false},
		{"chat model others supply", intakeSubject{model: "gpt-9", endpoints: []string{"chat"}, supplied: true}, IntakeActionEnable, true},
		{"preview chat model", intakeSubject{model: "gpt-9-preview", endpoints: []string{"chat"}}, IntakeActionPending, false},
		{"image model", intakeSubject{model: "h3", endpoints: []string{"images"}}, IntakeActionPending, false},
		{"model with no endpoints", intakeSubject{model: "x"}, IntakeActionPending, false},
	}
	for _, c := range cases {
		rule := decideIntake(rules, c.subject)
		if rule.Action != c.action {
			t.Errorf("%s: action %q, want %q", c.name, rule.Action, c.action)
		}
		if got := rule.joinsLast(c.subject.supplied); got != c.joinLast {
			t.Errorf("%s: joins last %v, want %v", c.name, got, c.joinLast)
		}
	}
}

func TestIntakeRuleConditions(t *testing.T) {
	rules := []SupplyIntakeRule{
		{Name: "claude", Families: []string{"claude-*"}, Action: IntakeActionEnable, Placement: IntakePlacePolicy},
		{Name: "vendor 7 when unsupplied", Vendors: []int{7}, Supplied: intakeBool(false), Action: IntakeActionEnable},
		{Name: "images skipped", Endpoints: []string{"images"}, Action: IntakeActionSkip},
	}
	claude := newIntakeSubject(&ModelInfo{Model: "claude-opus-6-0"}, true)
	if r := decideIntake(rules, claude); r.Name != "claude" || r.joinsLast(true) {
		t.Fatalf("claude-opus-6-0 (family %q) took %+v", claude.family, r)
	}
	if r := decideIntake(rules, intakeSubject{model: "m", vendor: 7}); r.Name != "vendor 7 when unsupplied" {
		t.Fatalf("vendor rule: %+v", r)
	}
	if r := decideIntake(rules, intakeSubject{model: "m", vendor: 7, supplied: true}); r.Action != IntakeActionPending || r.Name != "" {
		t.Fatalf("supplied vendor model should fall through to the fallback: %+v", r)
	}
	if r := decideIntake(rules, intakeSubject{model: "h3", endpoints: []string{"images"}}); r.Action != IntakeActionSkip {
		t.Fatalf("image rule: %+v", r)
	}
	if _, err := ParseIntakeRules(`[{"action":"maybe"}]`); err == nil {
		t.Fatal("an unknown action should not parse")
	}
	if _, err := ParseIntakeRules(`[{"action":"enable","placement":"first"}]`); err == nil {
		t.Fatal("an unknown placement should not parse")
	}
	if _, err := ParseIntakeRules(`[{"action":"enable","endpoints":["video"]}]`); err == nil {
		t.Fatal("an unknown endpoint should not parse")
	}
}

func setModelEndpoints(t *testing.T, name, endpoints string) {
	t.Helper()
	if err := DB.Model(&ModelInfo{}).Where("model = ?", name).Update("endpoints", endpoints).Error; err != nil {
		t.Fatal(err)
	}
}

// The case that started it: H3 only came from C, at the back of the policy. When A, first in the
// policy, starts listing H3, its new supply joins after C instead of taking H3's traffic.
func TestIntakeJoinsLastUnderPolicy(t *testing.T) {
	useAllocationTestDB(t)
	a := makeAllocationChannel(t, testChannel{"a", 1, "https://a.example", 10, 1})
	c := makeAllocationChannel(t, testChannel{"c", 1, "https://c.example", 0, 1})
	makePublicModel(t, "h3")
	setModelEndpoints(t, "h3", `["images"]`)
	makeSupply(t, "h3", c, 0, 1)
	if err := EnsureDefaultAllocationPolicy(); err != nil {
		t.Fatal(err)
	}
	ChannelGroup.Load()
	if err := SetCustomIntakeRules(`[{"name":"images","endpoints":["images"],"action":"enable"}]`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = SetCustomIntakeRules("") })

	outcomes, err := IntakeChannelModels(a, []string{"h3"})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 || outcomes[0].Action != IntakeActionEnable || !outcomes[0].JoinLast {
		t.Fatalf("outcomes = %+v", outcomes)
	}
	want := []map[int]float64{{c.Id: 1}, {a.Id: 1}}
	if got := routingShares(t, "h3"); !sameShares(got, want) {
		t.Fatalf("h3 routes %v, want C then A", got)
	}

	// Saving the supply with the numbers it is shown keeps the model on the policy; clearing
	// the mark puts A where the policy places it.
	views, err := GetModelSupplyViews("h3", a.Id)
	if err != nil || len(views) != 1 {
		t.Fatalf("views %v, err %v", views, err)
	}
	shown := *views[0].ModelSupply
	if err := UpdateModelSupply(&shown); err != nil {
		t.Fatal(err)
	}
	if !followsPolicy(lookupModelInfoFresh(t, "h3")) {
		t.Fatal("saving the shown numbers switched the model to manual")
	}
	if got := routingShares(t, "h3"); !sameShares(got, want) {
		t.Fatalf("after a plain save h3 routes %v", got)
	}
	shown.JoinLast = false
	if err := UpdateModelSupply(&shown); err != nil {
		t.Fatal(err)
	}
	if got := routingShares(t, "h3"); !sameShares(got, []map[int]float64{{a.Id: 1}, {c.Id: 1}}) {
		t.Fatalf("after clearing join_last h3 routes %v, want A then C", got)
	}
}

func lookupModelInfoFresh(t *testing.T, name string) *ModelInfo {
	t.Helper()
	var info ModelInfo
	if err := DB.Where("model = ?", name).First(&info).Error; err != nil {
		t.Fatal(err)
	}
	return &info
}

// A model with its own numbers gets the new supply below its lowest priority; a pending one
// does not route until an admin switches it on.
func TestIntakeManualModelAndPending(t *testing.T) {
	useAllocationTestDB(t)
	a := makeAllocationChannel(t, testChannel{"a", 1, "https://a.example", 10, 1})
	c := makeAllocationChannel(t, testChannel{"c", 1, "https://c.example", 0, 1})
	info := makePublicModel(t, "pinned")
	setModelEndpoints(t, "pinned", `["chat"]`)
	if err := DB.Model(info).Update("allocation", AllocationManual).Error; err != nil {
		t.Fatal(err)
	}
	makeSupply(t, "pinned", c, 3, 1)
	makePublicModel(t, "img")
	setModelEndpoints(t, "img", `["images"]`)
	makeSupply(t, "img", c, 0, 1)
	ChannelGroup.Load()

	outcomes, err := IntakeChannelModels(a, []string{"pinned", "img"})
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 2 {
		t.Fatalf("outcomes = %+v", outcomes)
	}
	var pinned, img ModelSupply
	DB.Where("model = ? AND channel_id = ?", "pinned", a.Id).First(&pinned)
	DB.Where("model = ? AND channel_id = ?", "img", a.Id).First(&img)
	if !pinned.Enabled || !pinned.JoinLast || pinned.Priority != 2 {
		t.Fatalf("pinned supply = %+v, want enabled, join_last, priority 2", pinned)
	}
	if img.Enabled || !img.Pending {
		t.Fatalf("img supply = %+v, want pending", img)
	}
	if got := routingShares(t, "img"); !sameShares(got, []map[int]float64{{c.Id: 1}}) {
		t.Fatalf("a pending supply routes: %v", got)
	}
	preview, err := PreviewModelSupply("img", "")
	if err != nil {
		t.Fatal(err)
	}
	states := map[int]string{}
	for _, tier := range preview.Tiers {
		for _, e := range tier.Supplies {
			states[e.ChannelId] = e.State
		}
	}
	if states[a.Id] != SupplyStatePending {
		t.Fatalf("preview states %v", states)
	}
	img.Enabled = true
	if err := UpdateModelSupply(&img); err != nil {
		t.Fatal(err)
	}
	if stored, _ := GetModelSupply(img.Id); stored.Pending || !stored.Enabled {
		t.Fatalf("confirmed supply = %+v", stored)
	}
	// Running the intake again adds nothing: the channel already supplies both models.
	if again, _ := IntakeChannelModels(a, []string{"pinned", "img"}); len(again) != 0 {
		t.Fatalf("second intake = %+v", again)
	}
}

func TestMarkSupplyUpstreams(t *testing.T) {
	useAllocationTestDB(t)
	a := makeAllocationChannel(t, testChannel{"a", 1, "https://a.example", 10, 1})
	c := makeAllocationChannel(t, testChannel{"c", 1, "https://c.example", 0, 1})
	makePublicModel(t, "m1")
	makePublicModel(t, "m2")
	makePublicModel(t, "solo")
	s1 := makeSupply(t, "m1", a, 10, 1)
	makeSupply(t, "m1", c, 0, 1)
	makeSupply(t, "m2", a, 10, 1)
	makeSupply(t, "solo", a, 10, 1)
	ChannelGroup.Load()

	// First miss: counted, not gone.
	change, err := MarkSupplyUpstreams(a, []string{"m2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(change.Gone) != 0 || len(change.Missing) != 2 {
		t.Fatalf("first check = %+v", change)
	}
	// Second miss: gone, and solo has no other supply.
	change, _ = MarkSupplyUpstreams(a, []string{"m2"})
	if len(change.Gone) != 2 || len(change.Unsupplied) != 1 || change.Unsupplied[0] != "solo" {
		t.Fatalf("second check = %+v", change)
	}
	if got := routingShares(t, "m1"); !sameShares(got, []map[int]float64{{c.Id: 1}}) {
		t.Fatalf("m1 still routes to the gone supply: %v", got)
	}
	// An admin switches m1 off while it is gone; when the upstream lists it again it comes back
	// as the admin left it.
	off := *s1
	off.Enabled = false
	if err := UpdateModelSupply(&off); err != nil {
		t.Fatal(err)
	}
	change, _ = MarkSupplyUpstreams(a, []string{"m1", "m2", "solo"})
	if len(change.Back) != 2 {
		t.Fatalf("back = %+v", change)
	}
	if stored, _ := GetModelSupply(s1.Id); stored.Enabled || stored.UpstreamGoneAt != 0 || stored.UpstreamMissing != 0 {
		t.Fatalf("m1 after coming back = %+v", stored)
	}

	// A list that drops every supply of the channel is held.
	MarkSupplyUpstreams(a, []string{"other"})
	change, _ = MarkSupplyUpstreams(a, []string{"other"})
	if !change.Held || len(change.Gone) != 0 {
		t.Fatalf("held check = %+v", change)
	}
	var gone int64
	DB.Model(&ModelSupply{}).Where("channel_id = ? AND upstream_gone_at > 0", a.Id).Count(&gone)
	if gone != 0 {
		t.Fatalf("%d supplies marked gone by a list that dropped all of them", gone)
	}

	// A channel that keeps missing models only counts.
	keep := *c
	keep.DriftKeepMissing = true
	MarkSupplyUpstreams(&keep, []string{"x"})
	change, _ = MarkSupplyUpstreams(&keep, []string{"x"})
	if len(change.Gone) != 0 || len(change.Missing) != 1 || change.Held {
		t.Fatalf("keep-missing check = %+v", change)
	}
}
