package controller

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"gorm.io/datatypes"
)

func TestDriftIgnoreMatcher(t *testing.T) {
	m := newDriftIgnoreMatcher("gpt-4o-audio\n*-preview\nvendor/*-beta\nre:^o\\d-mini$\nre:([bad\n# comment")
	cases := map[string]bool{
		"gpt-4o-audio":      true,
		"gpt-5-preview":     true,
		"vendor/model-beta": true,
		"other/model-beta":  false,
		"o4-mini":           true,
		"o4-mini-high":      false,
		"gpt-4o":            false,
		"# comment":         false,
	}
	for name, want := range cases {
		if got := m.ignored(name); got != want {
			t.Errorf("%s: ignored=%v, want %v", name, got, want)
		}
	}
	if bad := validateDriftIgnoreRules("ok\nre:([bad"); bad != "re:([bad" {
		t.Errorf("validate = %q", bad)
	}
}

// TestDriftAutoAddAndIgnore: with a baseline snapshot, models new upstream are added to an
// opted-in channel unless an ignore rule matches; ignored models are not reported either.
func TestDriftAutoAddAndIgnore(t *testing.T) {
	setupBalanceTestDB(t)
	upstream := []string{"m-old", "m-new", "m-new-preview"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.RawQuery != "" {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		var b strings.Builder
		b.WriteString(`{"data":[`)
		for i, m := range upstream {
			if i > 0 {
				b.WriteString(",")
			}
			b.WriteString(`{"id":"` + m + `","pricing":{"prompt":"0.000001","completion":"0.000001"}}`)
		}
		b.WriteString(`]}`)
		_, _ = w.Write([]byte(b.String()))
	}))
	defer srv.Close()

	base := srv.URL + "/api"
	prev := model.ModelDriftResult{UpstreamSnapshot: []string{"m-old"}, OK: true}
	ch := model.Channel{Name: "or", Type: config.ChannelTypeOpenRouter, Key: "sk", BaseURL: &base, Models: "m-old",
		Group: "default", Status: config.ChannelStatusEnabled, DriftAutoAdd: true, DriftIgnoreRules: "*-preview"}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}
	if err := ch.UpdateModelDrift(prev); err != nil {
		t.Fatal(err)
	}

	summary, err := CheckAllChannelsModelDrift()
	if err != nil {
		t.Fatal(err)
	}
	stored, _ := model.GetChannelById(ch.Id)
	if stored.Models != "m-old,m-new" {
		t.Fatalf("models = %q, want m-old,m-new (preview ignored)", stored.Models)
	}
	if drift := stored.GetModelDrift(); drift == nil || len(drift.NewModels) != 0 {
		t.Fatalf("new models left in the drift result: %+v", drift)
	}
	if summary.Added != 1 || len(summary.Channels) != 1 || len(summary.Channels[0].AddedModels) != 1 {
		t.Fatalf("summary = %+v", summary)
	}
}

// TestDriftSyncFollowsUpstream: the check adds every upstream model the channel does not list,
// switched off (auto add is off), except removed, ignored and wildcard-covered names; a new
// model stays flagged after joining; removing it in a save keeps the next check from adding it
// back, and switching a model on clears its flag.
func TestDriftSyncFollowsUpstream(t *testing.T) {
	setupBalanceTestDB(t)
	upstream := []string{"m-old", "m-back", "m-new", "m-removed", "m-new-preview", "w-1"}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.RawQuery != "" {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		ids := make([]string, 0, len(upstream))
		for _, m := range upstream {
			ids = append(ids, `{"id":"`+m+`","pricing":{"prompt":"0.000001","completion":"0.000001"}}`)
		}
		_, _ = w.Write([]byte(`{"data":[` + strings.Join(ids, ",") + `]}`))
	}))
	defer srv.Close()

	base := srv.URL + "/api"
	ch := model.Channel{Name: "or", Type: config.ChannelTypeOpenRouter, Key: "sk", BaseURL: &base, Models: "m-old,w-*,m-stale",
		Group: "default", Status: config.ChannelStatusEnabled, DriftIgnoreRules: "*-preview"}
	if err := model.DB.Create(&ch).Error; err != nil {
		t.Fatal(err)
	}
	prev := model.ModelDriftResult{UpstreamSnapshot: []string{"m-old", "m-back", "m-removed", "w-1", "m-stale"}, Removed: []string{"m-removed"}}
	if err := ch.UpdateModelDrift(prev); err != nil {
		t.Fatal(err)
	}

	check := func() *model.Channel {
		t.Helper()
		if _, err := CheckAllChannelsModelDrift(); err != nil {
			t.Fatal(err)
		}
		stored, err := model.GetChannelById(ch.Id)
		if err != nil {
			t.Fatal(err)
		}
		return stored
	}
	stored := check()
	if stored.Models != "m-old,w-*,m-stale,m-back,m-new" {
		t.Fatalf("models = %q", stored.Models)
	}
	if got := strings.Join(*stored.DisabledModels, ","); got != "m-back,m-new" {
		t.Fatalf("disabled = %q, want the joined models switched off", got)
	}
	drift := stored.GetModelDrift()
	if strings.Join(drift.NewModels, ",") != "m-new" || !slices.Contains(drift.MissingModels, "m-stale") || strings.Join(drift.Removed, ",") != "m-removed" {
		t.Fatalf("drift = %+v", drift)
	}
	if drift = check().GetModelDrift(); strings.Join(drift.NewModels, ",") != "m-new" {
		t.Fatalf("new models after the second check = %v, want m-new still flagged", drift.NewModels)
	}

	// Remove m-new and switch m-back on.
	stored.Models = "m-old,w-*,m-stale,m-back"
	off := datatypes.JSONSlice[string]{}
	stored.DisabledModels = &off
	if err := stored.Update(true); err != nil {
		t.Fatal(err)
	}
	saved, _ := model.GetChannelById(ch.Id)
	if drift = saved.GetModelDrift(); len(drift.NewModels) != 0 || strings.Join(drift.Removed, ",") != "m-new,m-removed" {
		t.Fatalf("drift after save = %+v", drift)
	}
	if stored = check(); stored.Models != "m-old,w-*,m-stale,m-back" {
		t.Fatalf("models after removal = %q, want m-new kept out", stored.Models)
	}

	// Listing it again forgets the removal.
	stored.Models += ",m-new"
	if err := stored.Update(true); err != nil {
		t.Fatal(err)
	}
	if drift = check().GetModelDrift(); strings.Join(drift.Removed, ",") != "m-removed" {
		t.Fatalf("removed after adding back = %v", drift.Removed)
	}
}
