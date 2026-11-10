package model

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/datatypes"
)

func pendingChannel(t *testing.T, name string, priority int64, weight uint, newModels ...string) *Channel {
	t.Helper()
	ch := &Channel{Name: name, Status: config.ChannelStatusEnabled, Group: "default", Priority: &priority, Weight: &weight}
	if len(newModels) > 0 {
		drift := datatypes.NewJSONType(ModelDriftResult{NewModels: newModels, UpstreamSnapshot: newModels, OK: true})
		ch.ModelDrift = &drift
	}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	return ch
}

func newModelsOf(t *testing.T, id int) []string {
	t.Helper()
	ch, err := GetChannelById(id)
	if err != nil {
		t.Fatal(err)
	}
	if drift := ch.GetModelDrift(); drift != nil {
		return drift.NewModels
	}
	return nil
}

func pendingSummary(t *testing.T) string {
	t.Helper()
	pending, err := GetPendingUpstreamModels()
	if err != nil {
		t.Fatal(err)
	}
	out := ""
	for _, p := range pending {
		ids := []int{}
		for _, c := range p.Channels {
			ids = append(ids, c.ChannelId)
		}
		out += fmt.Sprintf("[%s %v -> %s/%s/%v]", p.UpstreamModel, ids, p.Suggestion.Model, p.Suggestion.Source, p.Suggestion.Exists)
	}
	return out
}

func pendingFixture(t *testing.T) (a, b *Channel) {
	t.Helper()
	usePreviewTestDB(t)
	for _, info := range []*ModelInfo{
		{Model: "claude-opus-5", Status: ModelStatusPublished},
		{Model: "opus", AliasOf: "claude-opus-5"},
		{Model: "auto/coder", Kind: ModelInfoKindRoutingPolicy},
	} {
		if err := DB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	a = pendingChannel(t, "a", 10, 3, "anthropic/claude-opus-5", "brand-new-1", "already-supplied")
	b = pendingChannel(t, "b", 0, 1, "anthropic/claude-opus-5")
	pendingChannel(t, "no-drift", 0, 1)
	if err := DB.Create(&ModelSupply{Model: "claude-opus-5", ChannelId: a.Id, UpstreamModel: "already-supplied", Enabled: true}).Error; err != nil {
		t.Fatal(err)
	}
	return a, b
}

// The list collects every channel's new models by name, leaves out names a supply on the
// channel already sends, and suggests the public model through the name resolver.
func TestGetPendingUpstreamModels(t *testing.T) {
	a, b := pendingFixture(t)
	want := fmt.Sprintf("[anthropic/claude-opus-5 [%d %d] -> claude-opus-5/seed/true][brand-new-1 [%d] -> brand-new-1//false]", a.Id, b.Id, a.Id)
	if got := pendingSummary(t); got != want {
		t.Fatalf("pending\n got %s\nwant %s", got, want)
	}
	if got := WithoutSuppliedModels(a.Id, []string{"already-supplied", "x"}); fmt.Sprint(got) != "[x]" {
		t.Fatalf("WithoutSuppliedModels = %v", got)
	}
}

func TestAttachPendingUpstreamModel(t *testing.T) {
	a, b := pendingFixture(t)

	// Attaching through an alias of the public model, on every channel the name waits on.
	result, err := AttachPendingUpstreamModel(PendingAttach{UpstreamModel: "anthropic/claude-opus-5", Model: "opus"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Model != "claude-opus-5" || result.CreatedModel || len(result.Supplies) != 1 || fmt.Sprint(result.Skipped) != fmt.Sprintf("[%d]", a.Id) {
		t.Fatalf("result = %+v; want one new supply on b and a skipped (it already supplies the model)", result)
	}
	if s := result.Supplies[0]; s.ChannelId != b.Id || s.UpstreamModel != "anthropic/claude-opus-5" || s.Priority != 0 || s.Weight != 1 || !s.Enabled {
		t.Fatalf("supply = %+v", s)
	}
	if slices.Contains(newModelsOf(t, b.Id), "anthropic/claude-opus-5") {
		t.Fatal("the name still waits on b after being attached there")
	}
	if !slices.Contains(newModelsOf(t, a.Id), "anthropic/claude-opus-5") {
		t.Fatal("the name left a, which could not take a second supply of the model")
	}

	// A public model that does not exist yet is created only when asked, as a draft.
	if _, err := AttachPendingUpstreamModel(PendingAttach{UpstreamModel: "brand-new-1", Model: "brand-new"}); !errors.Is(err, ErrSupplyModelNotFound) {
		t.Fatalf("attach to a missing model without create: err = %v", err)
	}
	result, err = AttachPendingUpstreamModel(PendingAttach{UpstreamModel: "brand-new-1", Model: "brand-new", Create: true})
	if err != nil {
		t.Fatal(err)
	}
	info, _ := GetModelInfoByModel("brand-new")
	if !result.CreatedModel || info == nil || info.Status != ModelStatusDraft || len(result.Supplies) != 1 {
		t.Fatalf("result = %+v, row = %+v", result, info)
	}
	if s := result.Supplies[0]; s.ChannelId != a.Id || s.Priority != 10 || s.Weight != 3 {
		t.Fatalf("supply = %+v, want channel a's priority and weight", s)
	}
	if got := newModelsOf(t, a.Id); fmt.Sprint(got) != "[anthropic/claude-opus-5 already-supplied]" {
		t.Fatalf("a's new models = %v", got)
	}
	if got := pendingSummary(t); got != fmt.Sprintf("[anthropic/claude-opus-5 [%d] -> claude-opus-5/seed/true]", a.Id) {
		t.Fatalf("pending after attaching = %s", got)
	}

	for name, req := range map[string]PendingAttach{
		"routing policy": {UpstreamModel: "anthropic/claude-opus-5", Model: "auto/coder", ChannelIds: []int{a.Id}},
		"bad status":     {UpstreamModel: "anthropic/claude-opus-5", Model: "x", Create: true, Status: "live"},
		"not waiting":    {UpstreamModel: "nowhere", Model: "claude-opus-5"},
		"no channel":     {UpstreamModel: "anthropic/claude-opus-5", Model: "claude-opus-5", ChannelIds: []int{999}},
		"no upstream":    {Model: "claude-opus-5"},
	} {
		_, err := AttachPendingUpstreamModel(req)
		want := map[string]error{
			"routing policy": ErrSupplyModelNotPublic, "bad status": ErrModelStatus, "not waiting": ErrPendingNotFound,
			"no channel": ErrSupplyChannelNotFound, "no upstream": ErrSupplyUpstreamMissing,
		}[name]
		if !errors.Is(err, want) {
			t.Errorf("%s: err = %v, want %v", name, err, want)
		}
	}
}

func TestIgnorePendingUpstreamModel(t *testing.T) {
	a, b := pendingFixture(t)
	if n, err := IgnorePendingUpstreamModel("anthropic/claude-opus-5", []int{b.Id}); err != nil || n != 1 {
		t.Fatalf("ignore on b: %d channels, err %v", n, err)
	}
	if got := pendingSummary(t); got != fmt.Sprintf("[anthropic/claude-opus-5 [%d] -> claude-opus-5/seed/true][brand-new-1 [%d] -> brand-new-1//false]", a.Id, a.Id) {
		t.Fatalf("pending after ignoring on b = %s", got)
	}
	if n, err := IgnorePendingUpstreamModel("anthropic/claude-opus-5", nil); err != nil || n != 1 {
		t.Fatalf("ignore everywhere: %d channels, err %v", n, err)
	}
	if _, err := IgnorePendingUpstreamModel("anthropic/claude-opus-5", nil); !errors.Is(err, ErrPendingNotFound) {
		t.Fatalf("ignoring a name that waits nowhere: err = %v", err)
	}
	// The rest of the drift result stays.
	ch, _ := GetChannelById(a.Id)
	if drift := ch.GetModelDrift(); drift == nil || len(drift.UpstreamSnapshot) != 3 || !drift.OK {
		t.Fatalf("drift after ignoring = %+v", drift)
	}
}
