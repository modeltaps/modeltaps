package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
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
