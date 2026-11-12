package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
)

// Inclusion rules decide, after each library sync, which library models join the catalog and
// which catalog models are retired, so new releases need no one to add them. Rules are tried in
// order and the first one whose conditions all hold decides. A decision is made once per model
// and action: a model an admin removed after a rule added it is not added again.

const (
	LibraryActionPublish = "publish" // add to the catalog, published
	LibraryActionDraft   = "draft"   // add to the catalog as a draft
	LibraryActionIgnore  = "ignore"  // leave it in the library; later rules are not tried
	LibraryActionRetire  = "retire"  // a catalog model: no longer listed, still callable
)

// LibraryRule is one inclusion rule. Every condition that is set must hold.
type LibraryRule struct {
	Name string `json:"name"`
	// Labs are canonical-id prefixes as models.dev writes them ("anthropic", "zhipuai").
	Labs []string `json:"labs,omitempty"`
	// Patterns are globs on the model id ("*-preview*").
	Patterns []string `json:"patterns,omitempty"`
	// ReleasedWithinDays: released at most this many days before the run (0 = any time; a
	// model without a release date never matches a non-zero value).
	ReleasedWithinDays int `json:"released_within_days,omitempty"`
	// Supplied: an enabled channel lists the model (or a platform spelling of it).
	Supplied    *bool    `json:"supplied,omitempty"`
	Deprecated  *bool    `json:"deprecated,omitempty"`
	OpenWeights *bool    `json:"open_weights,omitempty"`
	Endpoints   []string `json:"endpoints,omitempty"` // any of
	Action      string   `json:"action"`
}

func libraryBool(v bool) *bool { return &v }

// DefaultLibraryRules are the rules used until an admin saves their own.
func DefaultLibraryRules() []LibraryRule {
	return []LibraryRule{
		{Name: "Retire deprecated models", Deprecated: libraryBool(true), Action: LibraryActionRetire},
		{Name: "Previews as drafts", Patterns: []string{"*-preview*", "*-exp*", "*-experimental*", "*-latest"}, Supplied: libraryBool(true), Action: LibraryActionDraft},
		{
			Name:               "Publish new models from the main labs",
			Labs:               []string{"anthropic", "openai", "google", "xai", "deepseek", "alibaba", "moonshotai", "zhipuai", "minimax"},
			ReleasedWithinDays: 60,
			Supplied:           libraryBool(true),
			Action:             LibraryActionPublish,
		},
		{Name: "Other supplied models as drafts", Supplied: libraryBool(true), Action: LibraryActionDraft},
	}
}

var ErrLibraryRuleInvalid = errors.New("invalid inclusion rule")

// ParseLibraryRules reads and checks rules saved as JSON.
func ParseLibraryRules(raw string) ([]LibraryRule, error) {
	var rules []LibraryRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrLibraryRuleInvalid, err)
	}
	for i, r := range rules {
		switch r.Action {
		case LibraryActionPublish, LibraryActionDraft, LibraryActionIgnore, LibraryActionRetire:
		default:
			return nil, fmt.Errorf("%w: rule %d has action %q", ErrLibraryRuleInvalid, i+1, r.Action)
		}
		for _, p := range r.Patterns {
			if _, err := path.Match(strings.ToLower(p), ""); err != nil {
				return nil, fmt.Errorf("%w: rule %d pattern %q", ErrLibraryRuleInvalid, i+1, p)
			}
		}
		if r.ReleasedWithinDays < 0 {
			return nil, fmt.Errorf("%w: rule %d released_within_days", ErrLibraryRuleInvalid, i+1)
		}
		if bad := UnknownModelEndpoints(r.Endpoints); len(bad) > 0 {
			return nil, fmt.Errorf("%w: rule %d endpoint %q", ErrLibraryRuleInvalid, i+1, bad[0])
		}
	}
	return rules, nil
}

var (
	customLibraryRulesMu sync.RWMutex
	customLibraryRules   []LibraryRule
)

// SetCustomLibraryRules installs saved rules; an empty value goes back to the defaults.
func SetCustomLibraryRules(raw string) error {
	var rules []LibraryRule
	if strings.TrimSpace(raw) != "" {
		parsed, err := ParseLibraryRules(raw)
		if err != nil {
			return err
		}
		rules = parsed
		if rules == nil {
			rules = []LibraryRule{}
		}
	}
	customLibraryRulesMu.Lock()
	customLibraryRules = rules
	customLibraryRulesMu.Unlock()
	return nil
}

// EffectiveLibraryRules returns the saved rules, or the defaults when none are saved.
func EffectiveLibraryRules() []LibraryRule {
	customLibraryRulesMu.RLock()
	custom := customLibraryRules
	customLibraryRulesMu.RUnlock()
	if custom != nil {
		return custom
	}
	return DefaultLibraryRules()
}

// matches reports whether every condition of the rule holds for the entry.
func (r LibraryRule) matches(e *ModelLibraryEntry, supplied bool, now int64) bool {
	if len(r.Labs) > 0 && !containsFold(r.Labs, e.Lab) {
		return false
	}
	if len(r.Patterns) > 0 {
		name := strings.ToLower(e.Model)
		hit := false
		for _, p := range r.Patterns {
			if ok, _ := path.Match(strings.ToLower(p), name); ok {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if r.ReleasedWithinDays > 0 && (e.ReleasedAt == 0 || e.ReleasedAt < now-int64(r.ReleasedWithinDays)*86400) {
		return false
	}
	if r.Supplied != nil && *r.Supplied != supplied {
		return false
	}
	if r.Deprecated != nil && *r.Deprecated != e.Deprecated {
		return false
	}
	if r.OpenWeights != nil && (e.OpenWeights == nil || *e.OpenWeights != *r.OpenWeights) {
		return false
	}
	if len(r.Endpoints) > 0 {
		var have []string
		_ = jsonUnmarshalStrings(e.Endpoints, &have)
		hit := false
		for _, want := range r.Endpoints {
			if containsFold(have, want) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	return true
}

func containsFold(list []string, v string) bool {
	for _, x := range list {
		if strings.EqualFold(strings.TrimSpace(x), v) {
			return true
		}
	}
	return false
}

// ModelLibraryDecision records a rule's decision for one model, so it is acted on (or reported)
// once. Applied is false for a decision only reported in a dry run.
type ModelLibraryDecision struct {
	Id        int    `json:"id"`
	Canonical string `json:"canonical" gorm:"type:varchar(255);uniqueIndex:idx_model_library_decision"`
	Action    string `json:"action" gorm:"type:varchar(16);uniqueIndex:idx_model_library_decision"`
	Model     string `json:"model" gorm:"type:varchar(255)"`
	Rule      string `json:"rule" gorm:"type:varchar(255)"`
	Applied   bool   `json:"applied"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func (ModelLibraryDecision) TableName() string { return "model_library_decisions" }

// LibraryRuleOutcome is what the rules decide for one model.
type LibraryRuleOutcome struct {
	Canonical string `json:"canonical"`
	Model     string `json:"model"`
	Lab       string `json:"lab"`
	Action    string `json:"action"`
	Rule      string `json:"rule"`
	// Done says the decision was already applied in an earlier run, so nothing happens now.
	Done bool `json:"done"`
}

// catalogRuleState is what the rules need to know about a catalog row.
type catalogRuleState struct {
	status string
	exempt bool // edited by an admin or locked: rules leave it alone
}

// EvaluateLibraryRules works out, without changing anything, what the rules would do now: add
// library models missing from the catalog, or retire catalog models. Catalog rows an admin
// edited (source manual) or locked are never retired.
func EvaluateLibraryRules(rules []LibraryRule, now int64) ([]LibraryRuleOutcome, error) {
	var entries []*ModelLibraryEntry
	if err := DB.Order("model").Find(&entries).Error; err != nil {
		return nil, err
	}
	var rows []*ModelInfo
	if err := DB.Select("model", "status", "source", "locked").Find(&rows).Error; err != nil {
		return nil, err
	}
	catalog := make(map[string]catalogRuleState, len(rows))
	for _, r := range rows {
		catalog[strings.ToLower(r.Model)] = catalogRuleState{status: r.Status, exempt: r.Locked || r.Source == ModelInfoSourceManual}
	}
	offers, err := channelOffers()
	if err != nil {
		return nil, err
	}
	var decided []*ModelLibraryDecision
	if err := DB.Where("applied = ?", true).Find(&decided).Error; err != nil {
		return nil, err
	}
	done := make(map[string]bool, len(decided))
	for _, d := range decided {
		done[d.Canonical+"\x00"+d.Action] = true
	}
	var out []LibraryRuleOutcome
	for _, e := range entries {
		key := strings.ToLower(e.Model)
		supplied := len(offers[key]) > 0
		for _, rule := range rules {
			if !rule.matches(e, supplied, now) {
				continue
			}
			state, inCatalog := catalog[key]
			act := false
			switch rule.Action {
			case LibraryActionPublish, LibraryActionDraft:
				act = !inCatalog
			case LibraryActionRetire:
				act = inCatalog && !state.exempt && state.status != ModelStatusRetired
			}
			if act {
				out = append(out, LibraryRuleOutcome{
					Canonical: e.Canonical, Model: e.Model, Lab: e.Lab, Action: rule.Action, Rule: rule.Name,
					Done: done[e.Canonical+"\x00"+rule.Action],
				})
			}
			break
		}
	}
	return out, nil
}

// LibraryRulesRun is the result of one run.
type LibraryRulesRun struct {
	Apply    bool                 `json:"apply"`
	Outcomes []LibraryRuleOutcome `json:"outcomes"`
	Added    int                  `json:"added"`
	Retired  int                  `json:"retired"`
	Reported int                  `json:"reported"`
}

// RunLibraryRules evaluates the rules and, with apply, carries the new decisions out; without
// it, it records them as reported. Either way an admin notice lists what is new since the last
// run, so a dry run does not repeat itself every day.
func RunLibraryRules(apply bool) (*LibraryRulesRun, error) {
	now := time.Now().Unix()
	outcomes, err := EvaluateLibraryRules(EffectiveLibraryRules(), now)
	if err != nil {
		return nil, err
	}
	run := &LibraryRulesRun{Apply: apply, Outcomes: outcomes}
	var reported []*ModelLibraryDecision
	if err := DB.Where("applied = ?", false).Find(&reported).Error; err != nil {
		return nil, err
	}
	seenReport := make(map[string]bool, len(reported))
	for _, d := range reported {
		seenReport[d.Canonical+"\x00"+d.Action] = true
	}

	var fresh []LibraryRuleOutcome
	byAction := map[string][]LibraryRuleOutcome{}
	for _, o := range outcomes {
		if o.Done {
			continue
		}
		key := o.Canonical + "\x00" + o.Action
		if apply || !seenReport[key] {
			fresh = append(fresh, o)
		}
		byAction[o.Action] = append(byAction[o.Action], o)
	}

	if apply {
		for _, action := range []string{LibraryActionPublish, LibraryActionDraft} {
			list := byAction[action]
			if len(list) == 0 {
				continue
			}
			canonicals := make([]string, len(list))
			for i, o := range list {
				canonicals[i] = o.Canonical
			}
			status := ModelStatusPublished
			if action == LibraryActionDraft {
				status = ModelStatusDraft
			}
			res, err := AddLibraryModels(LibraryAddRequest{Canonicals: canonicals, Status: status, Supplies: true})
			if err != nil {
				return nil, err
			}
			run.Added += len(res.Added)
		}
		if list := byAction[LibraryActionRetire]; len(list) > 0 {
			names := make([]string, len(list))
			for i, o := range list {
				names[i] = o.Model
			}
			res := DB.Model(&ModelInfo{}).Where("model IN ?", names).Updates(map[string]any{"status": ModelStatusRetired, "hidden": true})
			if res.Error != nil {
				return nil, res.Error
			}
			run.Retired = int(res.RowsAffected)
			refreshCatalogCaches()
		}
	}
	for _, o := range outcomes {
		if o.Done {
			continue
		}
		if err := recordLibraryDecision(o, apply, now); err != nil {
			return nil, err
		}
	}
	if !apply {
		run.Reported = len(fresh)
	}
	if len(fresh) > 0 {
		notifyLibraryRules(fresh, apply)
	}
	return run, nil
}

func recordLibraryDecision(o LibraryRuleOutcome, applied bool, now int64) error {
	var d ModelLibraryDecision
	err := DB.Where("canonical = ? AND action = ?", o.Canonical, o.Action).First(&d).Error
	if err == nil {
		if d.Applied || !applied {
			return nil
		}
		return DB.Model(&d).Updates(map[string]any{"applied": true, "rule": o.Rule, "updated_at": now}).Error
	}
	return DB.Create(&ModelLibraryDecision{
		Canonical: o.Canonical, Action: o.Action, Model: o.Model, Rule: o.Rule, Applied: applied, CreatedAt: now, UpdatedAt: now,
	}).Error
}

var libraryActionTitles = map[string]string{
	LibraryActionPublish: "Published",
	LibraryActionDraft:   "Added as drafts",
	LibraryActionRetire:  "Retired",
}

var libraryActionDryTitles = map[string]string{
	LibraryActionPublish: "Would publish",
	LibraryActionDraft:   "Would add as drafts",
	LibraryActionRetire:  "Would retire",
}

func notifyLibraryRules(list []LibraryRuleOutcome, applied bool) {
	sort.Slice(list, func(i, j int) bool { return list[i].Model < list[j].Model })
	titles := libraryActionDryTitles
	title := fmt.Sprintf("Inclusion rules would change %d models (dry run)", len(list))
	if applied {
		titles = libraryActionTitles
		title = fmt.Sprintf("Inclusion rules changed %d models", len(list))
	}
	var b strings.Builder
	for _, action := range []string{LibraryActionPublish, LibraryActionDraft, LibraryActionRetire} {
		var names []string
		for _, o := range list {
			if o.Action == action {
				names = append(names, "`"+o.Model+"`")
			}
		}
		if len(names) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s (%d): %s\n\n", titles[action], len(names), strings.Join(names, ", "))
	}
	if !applied {
		b.WriteString("Nothing was changed. Turn on \"Apply the rules\" in the inclusion rules to let them change the catalog.")
	}
	notify.SendKind(notify.KindModelLibrary, title, strings.TrimSpace(b.String()))
}

// runLibraryRulesAfterSync is called by the catalog sync once the library is rebuilt.
func runLibraryRulesAfterSync() {
	if !config.ModelLibraryRulesEnabled {
		return
	}
	run, err := RunLibraryRules(config.ModelLibraryRulesApply)
	if err != nil {
		logger.SysError("inclusion rules failed: " + err.Error())
		return
	}
	logger.SysLog(fmt.Sprintf("inclusion rules: %d decisions, %d added, %d retired, %d reported (apply=%v)",
		len(run.Outcomes), run.Added, run.Retired, run.Reported, run.Apply))
}
