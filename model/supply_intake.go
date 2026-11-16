package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"gorm.io/gorm"
)

// Intake rules decide what happens to a model a channel's upstream starts listing, when the
// channel adds new upstream models on its own (Channel.DriftAutoAdd): whether a supply is made,
// whether it routes at once or waits for an admin, and where it joins the model's order. Rules
// are tried in order and the first one whose conditions all hold decides; when none does, the
// supply waits for an admin and joins with the automatic placement. A channel can replace the
// site-wide rules with its own (Channel.SupplyIntakeRules).

const (
	IntakeActionEnable  = "enable"  // add the supply, routing at once
	IntakeActionPending = "pending" // add it switched off, waiting for an admin to confirm
	IntakeActionDisable = "disable" // add it switched off
	IntakeActionSkip    = "skip"    // add nothing; the name stays among the new upstream models

	// IntakePlaceAuto joins last when other channels already supply the model, else normally.
	IntakePlaceAuto   = "auto"
	IntakePlacePolicy = "policy" // join where the allocation policy or the channel puts it
	IntakePlaceLast   = "last"   // always join after the model's other supplies
)

var ErrIntakeRuleInvalid = errors.New("invalid intake rule")

// SupplyIntakeRule is one intake rule. Every condition that is set must hold.
type SupplyIntakeRule struct {
	Name string `json:"name"`
	// Endpoints are the model's interface abilities (chat, images, ...); any of them matches.
	Endpoints []string `json:"endpoints,omitempty"`
	// Vendors are vendor ids (ModelInfo.VendorID); any of them matches.
	Vendors []int `json:"vendors,omitempty"`
	// Families are globs on the model family parsed from the public name (ParseModelFamily):
	// "claude-*" matches every Claude model, including versions released later.
	Families []string `json:"families,omitempty"`
	// Patterns are globs on the public model name ("*-preview*").
	Patterns []string `json:"patterns,omitempty"`
	// Supplied: another channel already supplies the model (enabled, upstream not gone).
	Supplied  *bool  `json:"supplied,omitempty"`
	Action    string `json:"action"`
	Placement string `json:"placement,omitempty"`
}

func intakeBool(v bool) *bool { return &v }

// DefaultIntakeRules are the rules used until an admin saves their own.
func DefaultIntakeRules() []SupplyIntakeRule {
	return []SupplyIntakeRule{
		{Name: "Previews wait for review", Patterns: []string{"*-preview*", "*-exp*", "*-experimental*"}, Action: IntakeActionPending},
		{Name: "Chat models join at once", Endpoints: []string{ModelEndpointChat, ModelEndpointResponses}, Action: IntakeActionEnable, Placement: IntakePlaceAuto},
		{Name: "Everything else waits for review", Action: IntakeActionPending},
	}
}

// fallbackIntake is what happens when no rule matches.
var fallbackIntake = SupplyIntakeRule{Name: "", Action: IntakeActionPending, Placement: IntakePlaceAuto}

// ParseIntakeRules reads and checks rules saved as JSON.
func ParseIntakeRules(raw string) ([]SupplyIntakeRule, error) {
	var rules []SupplyIntakeRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIntakeRuleInvalid, err)
	}
	for i, r := range rules {
		switch r.Action {
		case IntakeActionEnable, IntakeActionPending, IntakeActionDisable, IntakeActionSkip:
		default:
			return nil, fmt.Errorf("%w: rule %d has action %q", ErrIntakeRuleInvalid, i+1, r.Action)
		}
		switch r.Placement {
		case "", IntakePlaceAuto, IntakePlacePolicy, IntakePlaceLast:
		default:
			return nil, fmt.Errorf("%w: rule %d has placement %q", ErrIntakeRuleInvalid, i+1, r.Placement)
		}
		for _, p := range append(append([]string{}, r.Patterns...), r.Families...) {
			if _, err := path.Match(strings.ToLower(p), ""); err != nil {
				return nil, fmt.Errorf("%w: rule %d pattern %q", ErrIntakeRuleInvalid, i+1, p)
			}
		}
		if bad := UnknownModelEndpoints(r.Endpoints); len(bad) > 0 {
			return nil, fmt.Errorf("%w: rule %d endpoint %q", ErrIntakeRuleInvalid, i+1, bad[0])
		}
	}
	if rules == nil {
		rules = []SupplyIntakeRule{}
	}
	return rules, nil
}

var (
	customIntakeRulesMu sync.RWMutex
	customIntakeRules   []SupplyIntakeRule
)

// SetCustomIntakeRules installs the site-wide rules; an empty value goes back to the defaults.
func SetCustomIntakeRules(raw string) error {
	var rules []SupplyIntakeRule
	if strings.TrimSpace(raw) != "" {
		parsed, err := ParseIntakeRules(raw)
		if err != nil {
			return err
		}
		rules = parsed
	}
	customIntakeRulesMu.Lock()
	customIntakeRules = rules
	customIntakeRulesMu.Unlock()
	return nil
}

// EffectiveIntakeRules returns the saved site-wide rules, or the defaults when none are saved.
func EffectiveIntakeRules() []SupplyIntakeRule {
	customIntakeRulesMu.RLock()
	custom := customIntakeRules
	customIntakeRulesMu.RUnlock()
	if custom != nil {
		return custom
	}
	return DefaultIntakeRules()
}

// ChannelIntakeRules returns the channel's own rules, nil when it uses the site-wide ones (or
// its saved value no longer parses).
func ChannelIntakeRules(channel *Channel) []SupplyIntakeRule {
	if channel == nil || channel.SupplyIntakeRules == nil || strings.TrimSpace(*channel.SupplyIntakeRules) == "" {
		return nil
	}
	rules, err := ParseIntakeRules(*channel.SupplyIntakeRules)
	if err != nil {
		return nil
	}
	return rules
}

// IntakeRulesForChannel returns the rules that apply to the channel.
func IntakeRulesForChannel(channel *Channel) []SupplyIntakeRule {
	if rules := ChannelIntakeRules(channel); rules != nil {
		return rules
	}
	return EffectiveIntakeRules()
}

// intakeSubject is what the rules look at for one public model.
type intakeSubject struct {
	model     string
	family    string
	endpoints []string
	vendor    int
	supplied  bool
}

func newIntakeSubject(info *ModelInfo, supplied bool) intakeSubject {
	subject := intakeSubject{model: info.Model, family: ParseModelFamily(info.Model).Family, vendor: info.VendorID, supplied: supplied}
	_ = jsonUnmarshalStrings(info.Endpoints, &subject.endpoints)
	return subject
}

func globAny(patterns []string, value string) bool {
	value = strings.ToLower(value)
	for _, p := range patterns {
		if ok, _ := path.Match(strings.ToLower(strings.TrimSpace(p)), value); ok {
			return true
		}
	}
	return false
}

func (r SupplyIntakeRule) matches(s intakeSubject) bool {
	if len(r.Endpoints) > 0 {
		hit := false
		for _, want := range r.Endpoints {
			if containsFold(s.endpoints, want) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if len(r.Vendors) > 0 {
		hit := false
		for _, v := range r.Vendors {
			if v == s.vendor {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	if len(r.Families) > 0 && (s.family == "" || !globAny(r.Families, s.family)) {
		return false
	}
	if len(r.Patterns) > 0 && !globAny(r.Patterns, s.model) {
		return false
	}
	if r.Supplied != nil && *r.Supplied != s.supplied {
		return false
	}
	return true
}

// decideIntake returns the first rule that matches, or the fallback.
func decideIntake(rules []SupplyIntakeRule, s intakeSubject) SupplyIntakeRule {
	for _, r := range rules {
		if r.matches(s) {
			return r
		}
	}
	return fallbackIntake
}

// joinsLast reports whether a supply placed by the rule joins after the model's other supplies.
func (r SupplyIntakeRule) joinsLast(supplied bool) bool {
	switch r.Placement {
	case IntakePlaceLast:
		return true
	case IntakePlacePolicy:
		return false
	default:
		return supplied
	}
}

// IntakeOutcome reports what the rules did with one upstream name.
type IntakeOutcome struct {
	Name     string `json:"name"`
	Model    string `json:"model"`
	Action   string `json:"action"`
	JoinLast bool   `json:"join_last,omitempty"`
	Rule     string `json:"rule,omitempty"`
	SupplyId int    `json:"supply_id,omitempty"`
}

// IntakeChannelModels adds supplies for the new upstream names of a channel as its intake
// rules say, and reports what happened to each name that belongs to a public model the channel
// does not supply yet. Names that match no public model, and wildcards, are left alone.
func IntakeChannelModels(channel *Channel, names []string) ([]IntakeOutcome, error) {
	if channel == nil || channel.Id == 0 || len(names) == 0 {
		return nil, nil
	}
	resolver, err := NewCanonicalResolver()
	if err != nil {
		return nil, err
	}
	candidates := autoSupplyCandidates(resolver, channel, names)
	if len(candidates) == 0 {
		return nil, nil
	}
	models := make([]string, 0, len(candidates))
	for _, c := range candidates {
		models = append(models, c.model)
	}
	var others []*ModelSupply
	if err := DB.Where("model IN ? AND channel_id <> ?", models, channel.Id).Find(&others).Error; err != nil {
		return nil, err
	}
	supplied := map[string]bool{}
	lowest := map[string]int64{}
	for _, s := range others {
		if p, ok := lowest[s.Model]; !ok || s.Priority < p {
			lowest[s.Model] = s.Priority
		}
		if s.Enabled && s.UpstreamGoneAt == 0 {
			supplied[s.Model] = true
		}
	}
	policyInForce := allocationInForce()
	rules := IntakeRulesForChannel(channel)

	var outcomes []IntakeOutcome
	var created []*ModelSupply
	err = DB.Transaction(func(tx *gorm.DB) error {
		var existing []*ModelSupply
		if err := tx.Where("channel_id = ?", channel.Id).Find(&existing).Error; err != nil {
			return err
		}
		have := make(map[string]bool, len(existing))
		for _, s := range existing {
			have[s.Model] = true
		}
		for _, c := range candidates {
			if have[c.model] {
				continue
			}
			have[c.model] = true
			info := resolver.publicModel(c.model)
			if info == nil {
				continue
			}
			rule := decideIntake(rules, newIntakeSubject(info, supplied[c.model]))
			outcome := IntakeOutcome{Name: c.upstream, Model: c.model, Action: rule.Action, Rule: rule.Name}
			if rule.Action == IntakeActionSkip {
				outcomes = append(outcomes, outcome)
				continue
			}
			supply := &ModelSupply{
				Model: c.model, ChannelId: channel.Id, UpstreamModel: c.upstream,
				Priority: channel.GetPriority(), Weight: uint(channel.GetWeight()),
				Enabled: rule.Action == IntakeActionEnable, Pending: rule.Action == IntakeActionPending,
				Source: ModelSupplySourceChannel,
			}
			if rule.joinsLast(supplied[c.model]) {
				if _, any := lowest[c.model]; any {
					supply.JoinLast = true
					outcome.JoinLast = true
					// Models off the policy keep stored numbers: write the lowest one.
					if !policyInForce || !followsPolicy(info) {
						supply.Priority = lowest[c.model] - 1
					}
				}
			}
			if err := tx.Create(supply).Error; err != nil {
				return err
			}
			outcome.SupplyId = supply.Id
			outcomes = append(outcomes, outcome)
			created = append(created, supply)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(created) > 0 {
		reloadSupplies()
	}
	return outcomes, nil
}

// placeJoinLast moves the numbers of a model's supplies that join last below every other supply
// of the model, keeping their order among themselves. numbers holds the policy's numbers by
// supply id and is changed in place.
func placeJoinLast(supplies []*ModelSupply, numbers map[int]supplyNumbers) {
	var lowest, highestLast int64
	haveOthers, haveLast := false, false
	for _, s := range supplies {
		n, ok := numbers[s.Id]
		if !ok {
			continue
		}
		if s.JoinLast {
			if !haveLast || n.Priority > highestLast {
				highestLast = n.Priority
			}
			haveLast = true
			continue
		}
		if !haveOthers || n.Priority < lowest {
			lowest = n.Priority
		}
		haveOthers = true
	}
	if !haveOthers || !haveLast || highestLast < lowest {
		return
	}
	shift := highestLast - lowest + 1
	for _, s := range supplies {
		if n, ok := numbers[s.Id]; ok && s.JoinLast {
			n.Priority -= shift
			numbers[s.Id] = n
		}
	}
}

// upstreamGoneChecks is how many model checks in a row must miss a supply's upstream name
// before the supply is marked gone: one short list from the upstream is not enough.
const upstreamGoneChecks = 2

// SupplyUpstreamChange is what one model check changed on a channel's supplies.
type SupplyUpstreamChange struct {
	// Gone and Back are the public models whose supply on the channel was marked gone, or came
	// back because the upstream lists the name again.
	Gone []string `json:"gone,omitempty"`
	Back []string `json:"back,omitempty"`
	// Missing are supplies the upstream did not list that are not marked gone (yet): seen
	// missing fewer times than needed, or kept because the channel keeps missing models.
	Missing []string `json:"missing,omitempty"`
	// Held is true when the check would have marked every supply of the channel gone; nothing
	// was marked, since that usually means the upstream's model list is broken.
	Held bool `json:"held,omitempty"`
	// Unsupplied are models of Gone that no other supply can serve now.
	Unsupplied []string `json:"unsupplied,omitempty"`
}

// Empty reports whether the check changed nothing worth reporting.
func (c *SupplyUpstreamChange) Empty() bool {
	return c == nil || (len(c.Gone) == 0 && len(c.Back) == 0 && !c.Held)
}

// MarkSupplyUpstreams compares the channel's supplies with the names its upstream lists now and
// marks the ones whose name is gone, or back. An empty list changes nothing: an upstream that
// lists no model at all is more likely broken than empty.
func MarkSupplyUpstreams(channel *Channel, upstream []string) (*SupplyUpstreamChange, error) {
	change := &SupplyUpstreamChange{}
	if channel == nil || channel.Id == 0 || len(upstream) == 0 {
		return change, nil
	}
	listed := make(map[string]bool, len(upstream))
	for _, name := range upstream {
		if name = strings.TrimSpace(name); name != "" {
			listed[name] = true
		}
	}
	var supplies []*ModelSupply
	if err := DB.Where("channel_id = ?", channel.Id).Order("model").Find(&supplies).Error; err != nil {
		return nil, err
	}
	if len(supplies) == 0 {
		return change, nil
	}

	type update struct {
		id      int
		missing int
		goneAt  int64
	}
	var updates []update
	var goneIds []int
	stillGone := 0
	now := utils.GetTimestamp()
	for _, s := range supplies {
		if listed[strings.TrimSpace(s.UpstreamModel)] {
			if s.UpstreamMissing != 0 || s.UpstreamGoneAt != 0 {
				updates = append(updates, update{id: s.Id})
			}
			if s.UpstreamGoneAt != 0 {
				change.Back = append(change.Back, s.Model)
			}
			continue
		}
		count := s.UpstreamMissing + 1
		u := update{id: s.Id, missing: count, goneAt: s.UpstreamGoneAt}
		switch {
		case s.UpstreamGoneAt != 0:
			stillGone++
		case count >= upstreamGoneChecks && !channel.DriftKeepMissing:
			u.goneAt = now
			goneIds = append(goneIds, len(updates))
			change.Gone = append(change.Gone, s.Model)
		default:
			change.Missing = append(change.Missing, s.Model)
		}
		updates = append(updates, u)
	}
	// Every supply of the channel would be gone at once: hold off and only report it.
	if len(goneIds) > 0 && len(supplies) > 1 && len(goneIds)+stillGone == len(supplies) {
		for _, i := range goneIds {
			updates[i].goneAt = 0
		}
		change.Missing = append(change.Missing, change.Gone...)
		change.Gone = nil
		change.Held = true
	}
	if len(updates) == 0 {
		return change, nil
	}
	err := DB.Transaction(func(tx *gorm.DB) error {
		for _, u := range updates {
			if err := tx.Model(&ModelSupply{}).Where("id = ?", u.id).
				Updates(map[string]any{"upstream_missing": u.missing, "upstream_gone_at": u.goneAt}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(change.Gone) > 0 || len(change.Back) > 0 {
		reloadSupplies()
	}
	if len(change.Gone) > 0 {
		change.Unsupplied, err = modelsWithoutRoutableSupply(change.Gone)
		if err != nil {
			return nil, err
		}
	}
	return change, nil
}

// modelsWithoutRoutableSupply returns the models of names that no enabled supply on an enabled
// channel, with its upstream still listed, can serve.
func modelsWithoutRoutableSupply(names []string) ([]string, error) {
	var supplies []*ModelSupply
	if err := DB.Select("model", "channel_id").Where("model IN ? AND enabled = ? AND upstream_gone_at = ?", names, true, 0).Find(&supplies).Error; err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(supplies))
	for _, s := range supplies {
		ids = append(ids, s.ChannelId)
	}
	enabled := map[int]bool{}
	if len(ids) > 0 {
		var channels []*Channel
		if err := DB.Select("id").Where("id IN ? AND status = ?", ids, config.ChannelStatusEnabled).Find(&channels).Error; err != nil {
			return nil, err
		}
		for _, ch := range channels {
			enabled[ch.Id] = true
		}
	}
	served := map[string]bool{}
	for _, s := range supplies {
		if enabled[s.ChannelId] {
			served[s.Model] = true
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, name := range names {
		if !served[name] && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out, nil
}
