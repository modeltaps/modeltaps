package model

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/config"
)

const (
	PauseActionPause = "pause"
	PauseActionRetry = "retry"

	PauseScopeChannel = "channel"
	PauseScopeModel   = "model"

	// PauseCategoryQuota marks pauses caused by an exhausted balance or quota; a balance refresh
	// that finds money again ends them early.
	PauseCategoryQuota = "quota"
)

// PauseRule decides what happens to a channel after a failed request. Rules are tried in order
// and the first match wins.
//
// A rule matches when at least one of its plain conditions holds (status code, keyword, error
// type or error code; a rule with none of them matches any error) and every special condition
// it sets holds too. Times and WindowSeconds turn it into a counter: it fires on the Times-th
// match for the same channel and model within WindowSeconds.
type PauseRule struct {
	Name string `json:"name"`
	// Disabled switches a rule off without removing it.
	Disabled bool `json:"disabled,omitempty"`

	StatusCodes []int `json:"status_codes,omitempty"`
	// Keywords match the error message case-insensitively as substrings; a keyword starting
	// with "re:" is a regular expression.
	Keywords   []string `json:"keywords,omitempty"`
	ErrorTypes []string `json:"error_types,omitempty"`
	ErrorCodes []string `json:"error_codes,omitempty"`

	// Unusable requires an error that says the channel can no longer serve (a rejected
	// credential, a disabled account, a disable keyword).
	Unusable bool `json:"unusable,omitempty"`
	// ModelNotFound requires an error saying the upstream does not have the model.
	ModelNotFound bool `json:"model_not_found,omitempty"`
	// RateLimitReset requires the upstream to have said when the limit resets.
	RateLimitReset bool `json:"rate_limit_reset,omitempty"`
	// PerStatusConfig requires an entry for the status code in RetryCooldownPerStatus, whose
	// seconds then apply.
	PerStatusConfig bool `json:"per_status_config,omitempty"`

	Times         int `json:"times,omitempty"`
	WindowSeconds int `json:"window_seconds,omitempty"`

	Action string `json:"action"`
	// Seconds is how long to pause. With RateLimitReset or PerStatusConfig it may be 0, and
	// the upstream's reset time or the configured seconds apply.
	Seconds int    `json:"seconds,omitempty"`
	Scope   string `json:"scope,omitempty"`
	// Probe sends a test request before resuming; a failed probe doubles the pause.
	Probe    bool   `json:"probe,omitempty"`
	Notify   bool   `json:"notify,omitempty"`
	Category string `json:"category,omitempty"`

	keywordRegexps []*regexp.Regexp
}

// DefaultPauseRules are the built-in rules, using the current cooldown settings.
func DefaultPauseRules() []PauseRule {
	return []PauseRule{
		{
			Name:        "quota",
			StatusCodes: []int{http.StatusPaymentRequired},
			ErrorTypes:  []string{"insufficient_quota"},
			ErrorCodes:  []string{"insufficient_quota", "billing_not_active"},
			Keywords: []string{
				"credit balance is too low", "exceeded your current quota", "balance is insufficient",
				"insufficient balance", "insufficient_balance", "余额不足",
			},
			Action: PauseActionPause, Seconds: 1800, Scope: PauseScopeChannel, Probe: true, Notify: true,
			Category: PauseCategoryQuota,
		},
		{
			Name: "credential", Unusable: true,
			Action: PauseActionPause, Seconds: 3600, Scope: PauseScopeChannel, Probe: true, Notify: true,
		},
		{
			Name: "rate_limit_reset", StatusCodes: []int{http.StatusTooManyRequests}, RateLimitReset: true,
			Action: PauseActionPause, Scope: PauseScopeModel,
		},
		{
			Name: "rate_limit", StatusCodes: []int{http.StatusTooManyRequests},
			Action: PauseActionPause, Seconds: config.RetryCooldownSeconds, Scope: PauseScopeModel,
		},
		{
			Name: "model_not_found", ModelNotFound: true,
			Action: PauseActionPause, Seconds: config.ModelNotFoundCooldownSeconds, Scope: PauseScopeModel, Notify: true,
		},
		{
			Name: "per_status", PerStatusConfig: true,
			Action: PauseActionPause, Scope: PauseScopeModel,
		},
		{
			Name:        "upstream_failure",
			StatusCodes: []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout},
			Times:       3, WindowSeconds: 60,
			Action: PauseActionPause, Seconds: 120, Scope: PauseScopeModel, Probe: true, Notify: true,
		},
	}
}

var (
	customPauseRulesMu sync.RWMutex
	customPauseRules   []PauseRule
)

// ParsePauseRules validates custom rules and compiles their regular expressions.
func ParsePauseRules(raw string) ([]PauseRule, error) {
	var rules []PauseRule
	if err := json.Unmarshal([]byte(raw), &rules); err != nil {
		return nil, fmt.Errorf("pause rules are not valid JSON: %w", err)
	}
	for i := range rules {
		if err := rules[i].compile(); err != nil {
			return nil, fmt.Errorf("pause rule %d (%s): %w", i+1, rules[i].Name, err)
		}
	}
	return rules, nil
}

func (r *PauseRule) compile() error {
	switch r.Action {
	case PauseActionPause:
		if r.Seconds <= 0 && !r.RateLimitReset && !r.PerStatusConfig {
			return fmt.Errorf("a pause needs seconds > 0")
		}
	case PauseActionRetry:
	default:
		return fmt.Errorf("unknown action %q", r.Action)
	}
	switch r.Scope {
	case "", PauseScopeChannel, PauseScopeModel:
	default:
		return fmt.Errorf("unknown scope %q", r.Scope)
	}
	if r.Times < 0 || r.WindowSeconds < 0 || (r.Times > 1 && r.WindowSeconds == 0) {
		return fmt.Errorf("times needs a window_seconds")
	}
	r.keywordRegexps = nil
	for _, keyword := range r.Keywords {
		if pattern, ok := strings.CutPrefix(keyword, "re:"); ok {
			re, err := regexp.Compile("(?i)" + pattern)
			if err != nil {
				return fmt.Errorf("keyword %q: %w", keyword, err)
			}
			r.keywordRegexps = append(r.keywordRegexps, re)
		}
	}
	return nil
}

// SetCustomPauseRules stores the rules saved in the PauseRules setting; empty means defaults.
func SetCustomPauseRules(raw string) error {
	var rules []PauseRule
	if strings.TrimSpace(raw) != "" {
		parsed, err := ParsePauseRules(raw)
		if err != nil {
			return err
		}
		rules = parsed
	}
	customPauseRulesMu.Lock()
	customPauseRules = rules
	customPauseRulesMu.Unlock()
	return nil
}

// EffectivePauseRules returns the custom rules if any are saved, otherwise the defaults.
func EffectivePauseRules() []PauseRule {
	customPauseRulesMu.RLock()
	custom := customPauseRules
	customPauseRulesMu.RUnlock()
	if custom != nil {
		return custom
	}
	rules := DefaultPauseRules()
	for i := range rules {
		_ = rules[i].compile()
	}
	return rules
}

// PauseFailure is what a rule is matched against.
type PauseFailure struct {
	StatusCode     int
	Message        string
	ErrorType      string
	ErrorCode      string
	Unusable       bool
	ModelNotFound  bool
	RateLimitReset bool
}

// Matches reports whether the rule applies to the failure, ignoring Times.
func (r *PauseRule) Matches(f PauseFailure) bool {
	if r.Disabled {
		return false
	}
	if r.Unusable && !f.Unusable || r.ModelNotFound && !f.ModelNotFound || r.RateLimitReset && !f.RateLimitReset {
		return false
	}
	if r.PerStatusConfig {
		if _, ok := config.GetRetryCooldownForStatus(f.StatusCode); !ok {
			return false
		}
	}
	hasPlain := len(r.StatusCodes)+len(r.Keywords)+len(r.ErrorTypes)+len(r.ErrorCodes) > 0
	if !hasPlain {
		return true
	}
	for _, code := range r.StatusCodes {
		if code == f.StatusCode {
			return true
		}
	}
	for _, t := range r.ErrorTypes {
		if t != "" && strings.EqualFold(t, f.ErrorType) {
			return true
		}
	}
	for _, c := range r.ErrorCodes {
		if c != "" && strings.EqualFold(c, f.ErrorCode) {
			return true
		}
	}
	message := strings.ToLower(f.Message)
	for _, keyword := range r.Keywords {
		if !strings.HasPrefix(keyword, "re:") && keyword != "" && strings.Contains(message, strings.ToLower(keyword)) {
			return true
		}
	}
	for _, re := range r.keywordRegexps {
		if re.MatchString(f.Message) {
			return true
		}
	}
	return false
}

// PauseSecondsFor is how long the rule pauses for this failure; resetAt is the upstream's reset
// time (unix seconds) when it gave one.
func (r *PauseRule) PauseSecondsFor(f PauseFailure, now, resetAt int64) int64 {
	if r.RateLimitReset && resetAt > now {
		return resetAt - now
	}
	if r.PerStatusConfig {
		if secs, ok := config.GetRetryCooldownForStatus(f.StatusCode); ok {
			return int64(secs)
		}
	}
	return int64(r.Seconds)
}

// channelPauseRulesCache keeps parsed channel rules by their JSON text, so a failure does not
// parse them again.
var channelPauseRulesCache = struct {
	sync.Mutex
	byText map[string][]PauseRule
}{byText: map[string][]PauseRule{}}

// ChannelPauseRules parses a channel's own rules; invalid or empty text gives none (channel
// saves reject invalid rules, so that only happens for rows written around the API).
func ChannelPauseRules(channel *Channel) []PauseRule {
	if channel == nil || channel.PauseRules == nil || strings.TrimSpace(*channel.PauseRules) == "" {
		return nil
	}
	raw := *channel.PauseRules
	channelPauseRulesCache.Lock()
	defer channelPauseRulesCache.Unlock()
	if rules, ok := channelPauseRulesCache.byText[raw]; ok {
		return rules
	}
	rules, err := ParsePauseRules(raw)
	if err != nil {
		rules = nil
	}
	if len(channelPauseRulesCache.byText) > 256 {
		channelPauseRulesCache.byText = map[string][]PauseRule{}
	}
	channelPauseRulesCache.byText[raw] = rules
	return rules
}

// PauseRulesForChannel is the rule list a failure on channel goes through: the channel's own
// rules first, then the site-wide rules. A channel that should never fall through to the
// site-wide rules ends its list with a rule without conditions.
func PauseRulesForChannel(channel *Channel) []PauseRule {
	own := ChannelPauseRules(channel)
	if len(own) == 0 {
		return EffectivePauseRules()
	}
	return append(slices.Clone(own), EffectivePauseRules()...)
}
