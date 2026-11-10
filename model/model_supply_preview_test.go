package model

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// usePreviewTestDB is useSupplyTestDB plus the pause table, an empty pause index and the catalog
// read straight from the database.
func usePreviewTestDB(t *testing.T) {
	t.Helper()
	useSupplyTestDB(t)
	if err := DB.AutoMigrate(&SupplyPause{}); err != nil {
		t.Fatal(err)
	}
	pauseIndex.Lock()
	oldIndex := pauseIndex.byChannel
	pauseIndex.byChannel = map[int]map[string]*SupplyPause{}
	pauseIndex.Unlock()
	modelInfoCache.Lock()
	oldLoaded, oldByModel, oldAliases := modelInfoCache.loaded, modelInfoCache.byModel, modelInfoCache.aliasesByModel
	modelInfoCache.loaded, modelInfoCache.byModel, modelInfoCache.aliasesByModel = false, nil, nil
	modelInfoCache.Unlock()
	t.Cleanup(func() {
		pauseIndex.Lock()
		pauseIndex.byChannel = oldIndex
		pauseIndex.Unlock()
		modelInfoCache.Lock()
		modelInfoCache.loaded, modelInfoCache.byModel, modelInfoCache.aliasesByModel = oldLoaded, oldByModel, oldAliases
		modelInfoCache.Unlock()
	})
}

func previewChannel(t *testing.T, name, group string, status int) *Channel {
	t.Helper()
	weight := uint(1)
	ch := &Channel{Name: name, Group: group, Status: status, Weight: &weight}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	return ch
}

func previewSupply(t *testing.T, modelName string, ch *Channel, priority int64, weight uint, enabled bool) *ModelSupply {
	t.Helper()
	s := &ModelSupply{Model: modelName, ChannelId: ch.Id, UpstreamModel: "up/" + ch.Name, Priority: priority, Weight: weight, Enabled: enabled}
	if err := DB.Create(s).Error; err != nil {
		t.Fatal(err)
	}
	return s
}

func previewStates(p *SupplyPreview) string {
	out := ""
	for _, tier := range p.Tiers {
		out += fmt.Sprintf("[p%d", tier.Priority)
		for _, e := range tier.Supplies {
			out += fmt.Sprintf(" %s:%s:%.2f", e.ChannelName, e.State, e.Share)
		}
		out += "]"
	}
	return out
}

func TestPreviewModelSupply(t *testing.T) {
	usePreviewTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "m", Status: ModelStatusPublished, SupplyMode: SupplyModeFailover}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelInfo{Model: "m-alias", AliasOf: "m"}).Error; err != nil {
		t.Fatal(err)
	}

	primary := previewChannel(t, "primary", "default", config.ChannelStatusEnabled)
	drained := previewChannel(t, "drained", "default", config.ChannelStatusEnabled)
	testing_ := previewChannel(t, "testing", "default", config.ChannelStatusEnabled)
	cooling := previewChannel(t, "cooling", "default", config.ChannelStatusEnabled)
	backupA := previewChannel(t, "backup-a", "default,vip", config.ChannelStatusEnabled)
	backupB := previewChannel(t, "backup-b", "default", config.ChannelStatusEnabled)
	off := previewChannel(t, "off", "default", config.ChannelStatusManuallyDisabled)
	switchedOff := previewChannel(t, "switched-off", "default", config.ChannelStatusEnabled)
	vipOnly := previewChannel(t, "vip-only", "vip", config.ChannelStatusEnabled)

	previewSupply(t, "m", primary, 10, 1, true)
	previewSupply(t, "m", drained, 10, 5, true)
	previewSupply(t, "m", testing_, 10, 1, true)
	previewSupply(t, "m", cooling, 10, 1, true)
	firstBackup := previewSupply(t, "m", backupA, 0, 3, true)
	previewSupply(t, "m", backupB, 0, 1, true)
	previewSupply(t, "m", off, 0, 1, true)
	previewSupply(t, "m", switchedOff, 0, 1, false)
	previewSupply(t, "m", vipOnly, 0, 1, true)

	now := time.Now().Unix()
	if _, err := SavePause(&SupplyPause{ChannelId: drained.Id, Until: now + 600, Seconds: 600, Rule: "quota", Reason: "no money"}); err != nil {
		t.Fatal(err)
	}
	if _, err := SavePause(&SupplyPause{ChannelId: testing_.Id, Model: "m", Until: now - 5, Seconds: 120, Rule: "upstream_failure", Probe: true}); err != nil {
		t.Fatal(err)
	}
	ChannelGroup.SetCooldownsWithDuration(cooling.Id, "m", 30)
	t.Cleanup(func() { ChannelGroup.Cooldowns.Delete(fmt.Sprintf("%d:m", cooling.Id)) })

	p, err := PreviewModelSupply("m-alias", "default")
	if err != nil {
		t.Fatal(err)
	}
	if p.Model != "m" || p.RequestedModel != "m-alias" || p.Mode != SupplyModeFailover || p.Status != ModelStatusPublished {
		t.Fatalf("preview header = %+v", p)
	}
	// The top tier: the drained channel is the heaviest but paused, so the three weight-1
	// supplies are left, of which only the primary is available: it takes every request.
	want := "[p10 drained:paused:0.00 primary:available:1.00 testing:awaiting_test:0.00 cooling:cooling_down:0.00]" +
		"[p0 backup-a:available:0.00 backup-b:available:0.00 off:channel_disabled:0.00 switched-off:disabled:0.00]"
	if got := previewStates(p); got != want {
		t.Fatalf("tiers\n got %s\nwant %s", got, want)
	}
	if p.FirstChoice == 0 || p.OutsideGroup != 1 {
		t.Fatalf("first choice %d, outside group %d; want the primary and one vip-only supply left out", p.FirstChoice, p.OutsideGroup)
	}
	paused := p.Tiers[0].Supplies[0]
	if paused.PauseRule != "quota" || paused.PauseReason != "no money" || paused.Until != now+600 || paused.PauseId == 0 {
		t.Fatalf("paused entry = %+v", paused)
	}

	// The primary goes away too: the backup tier takes over, failover picks its heaviest.
	if _, err := SavePause(&SupplyPause{ChannelId: primary.Id, Until: now + 60, Seconds: 60, Rule: "manual"}); err != nil {
		t.Fatal(err)
	}
	p, _ = PreviewModelSupply("m", "default")
	if p.FirstChoice != firstBackup.Id || p.Tiers[1].Supplies[0].Share != 1 {
		t.Fatalf("after pausing the primary: first choice %d, tiers %s", p.FirstChoice, previewStates(p))
	}

	// Balance spreads the backup tier by weight.
	DB.Model(&ModelInfo{}).Where("model = ?", "m").Update("supply_mode", SupplyModeBalance)
	p, _ = PreviewModelSupply("m", "default")
	if got := previewStates(p); got != "[p10 drained:paused:0.00 primary:paused:0.00 testing:awaiting_test:0.00 cooling:cooling_down:0.00]"+
		"[p0 backup-a:available:0.75 backup-b:available:0.25 off:channel_disabled:0.00 switched-off:disabled:0.00]" || p.FirstChoice != 0 {
		t.Fatalf("balance tiers %s, first choice %d", got, p.FirstChoice)
	}

	// Without a group every supply is shown.
	if p, _ := PreviewModelSupply("m", ""); p.OutsideGroup != 0 {
		t.Fatalf("outside group without a group filter = %d", p.OutsideGroup)
	}

	if _, err := PreviewModelSupply("nope", ""); !errors.Is(err, ErrSupplyModelNotFound) {
		t.Fatalf("unknown model: err = %v", err)
	}
	if err := DB.Create(&ModelInfo{Model: "auto/x", Kind: ModelInfoKindRoutingPolicy}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := PreviewModelSupply("auto/x", ""); !errors.Is(err, ErrSupplyModelNotPublic) {
		t.Fatalf("routing policy: err = %v", err)
	}
}

// A channel's own rules are tried before the site-wide ones; invalid ones are ignored.
func TestPauseRulesForChannel(t *testing.T) {
	if err := SetCustomPauseRules(""); err != nil {
		t.Fatal(err)
	}
	site := len(EffectivePauseRules())
	if got := len(PauseRulesForChannel(&Channel{})); got != site {
		t.Fatalf("no channel rules: %d rules, want the %d site-wide", got, site)
	}
	own := `[{"name":"teapot","status_codes":[503],"action":"pause","seconds":600,"scope":"channel"}]`
	rules := PauseRulesForChannel(&Channel{PauseRules: &own})
	if len(rules) != site+1 || rules[0].Name != "teapot" {
		t.Fatalf("rules = %d, first %q; want the channel rule first", len(rules), rules[0].Name)
	}
	bad := `[{"name":"x","action":"explode"}]`
	if got := len(PauseRulesForChannel(&Channel{PauseRules: &bad})); got != site {
		t.Fatalf("invalid channel rules gave %d rules, want only the site-wide ones", got)
	}
}

// A channel saved without pause_rules (the console saves whole rows) keeps them.
func TestChannelUpdateKeepsPauseRules(t *testing.T) {
	useSupplyTestDB(t)
	rules := `[{"name":"teapot","status_codes":[503],"action":"pause","seconds":600}]`
	weight := uint(1)
	ch := &Channel{Name: "c", Weight: &weight, PauseRules: &rules}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatal(err)
	}
	edit := &Channel{Id: ch.Id, Name: "renamed", Weight: &weight}
	if err := edit.Update(true); err != nil {
		t.Fatal(err)
	}
	var stored Channel
	if err := DB.First(&stored, ch.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Name != "renamed" || stored.PauseRules == nil || *stored.PauseRules != rules {
		t.Fatalf("stored = name %q rules %v, want the rules kept", stored.Name, stored.PauseRules)
	}
	empty := ""
	clear := &Channel{Id: ch.Id, Name: "renamed", Weight: &weight, PauseRules: &empty}
	if err := clear.Update(true); err != nil {
		t.Fatal(err)
	}
	DB.First(&stored, ch.Id)
	if stored.PauseRules == nil || *stored.PauseRules != "" {
		t.Fatalf("explicitly cleared rules = %v, want empty", stored.PauseRules)
	}
}
