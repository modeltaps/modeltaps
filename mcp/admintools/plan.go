package admintools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/settings"
	"github.com/modeltaps/modeltaps/model"
)

// A change of a platform setting the settings assistant proposes. The admin reviews the checked
// plan and applies it through the settings page's own save (PUT /api/option/); nothing here
// writes. Only platform settings can be proposed, and never the ones a wrong value locks people
// out with or that hold money or secrets.

// ProposedChange is one change as the model proposes it.
type ProposedChange struct {
	Key    string `json:"key"`
	Value  string `json:"value"`
	Reason string `json:"reason,omitempty"`
}

// PlannedChange is a proposed change after the check.
type PlannedChange struct {
	Key        string `json:"key"`
	Label      string `json:"label,omitempty"`
	Where      string `json:"where,omitempty"`
	ConsoleURL string `json:"console_url,omitempty"`
	Current    string `json:"current"`
	Default    string `json:"default,omitempty"`
	// Value is the proposed value, normalized ("true" for a switch, a plain number).
	Value  string `json:"value"`
	Reason string `json:"reason,omitempty"`
	// Allowed says the change may be applied; Blocked says why not.
	Allowed bool   `json:"allowed"`
	Blocked string `json:"blocked,omitempty"`
	// Unchanged: the setting already has this value.
	Unchanged bool `json:"unchanged,omitempty"`
}

// blockedGroups are settings page groups the assistant never changes: sign-in (a wrong value
// locks users or admins out) and payment (money).
var blockedGroups = map[string]string{
	"auth":    "sign-in settings are changed by hand only",
	"payment": "payment settings are changed by hand only",
}

// blockedKeys are single settings it never changes.
var blockedKeys = map[string]string{
	"AccountSystem": "the account system is changed by hand only",
	"ServerAddress": "the server address is changed by hand only: a wrong one breaks sign-in callbacks and links",
}

// PlanChanges checks proposed changes.
func PlanChanges(changes []ProposedChange) []PlannedChange {
	defaults := config.GlobalOption.Defaults()
	out := make([]PlannedChange, 0, len(changes))
	for _, ch := range changes[:min(len(changes), maxLimit)] {
		p := PlannedChange{Key: ch.Key, Value: strings.TrimSpace(ch.Value), Reason: ch.Reason}
		s, ok := settings.Get(ch.Key)
		switch {
		case !ok:
			p.Blocked = "unknown setting key"
		case s.Scope != settings.ScopeSite:
			p.Blocked = "an object field: change it on the object's page"
		case s.Type == settings.TypeSecret || config.GlobalOption.IsSecret(s.Key):
			p.Blocked = "secrets are changed by hand only"
		case blockedKeys[s.Key] != "":
			p.Blocked = blockedKeys[s.Key]
		case blockedGroups[s.Group] != "":
			p.Blocked = blockedGroups[s.Group]
		case s.Group == "mail" && strings.HasPrefix(s.Key, "SMTP"):
			p.Blocked = "mail sending is changed through the mail services"
		}
		if ok {
			p.Label, p.Where, p.ConsoleURL = s.Label, settings.Location(s), consoleURL(s)
			if p.Blocked == "" {
				p.Current = model.SiteSettingValue(s.Key)
				p.Default = defaults[s.Key]
				value, err := normalizeValue(s, p.Value)
				if err != nil {
					p.Blocked = err.Error()
				} else {
					p.Value = value
					p.Allowed = true
					p.Unchanged = value == p.Current
				}
			}
		}
		out = append(out, p)
	}
	return out
}

// normalizeValue checks a value against the setting's type, range and allowed values.
func normalizeValue(s settings.Setting, value string) (string, error) {
	inRange := func(n float64) error {
		if s.Min != nil && n < *s.Min {
			return fmt.Errorf("below the minimum %v", *s.Min)
		}
		if s.Max != nil && n > *s.Max {
			return fmt.Errorf("above the maximum %v", *s.Max)
		}
		return nil
	}
	switch s.Type {
	case settings.TypeBool:
		b, err := strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			return "", fmt.Errorf("must be true or false")
		}
		return strconv.FormatBool(b), nil
	case settings.TypeInt:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return "", fmt.Errorf("must be a whole number")
		}
		return strconv.FormatInt(n, 10), inRange(float64(n))
	case settings.TypeFloat, settings.TypePercent, settings.TypeRatio:
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return "", fmt.Errorf("must be a number")
		}
		return strconv.FormatFloat(n, 'f', -1, 64), inRange(n)
	case settings.TypeEnum:
		for _, e := range s.Enum {
			if e.Value == value {
				return value, nil
			}
		}
		allowed := make([]string, 0, len(s.Enum))
		for _, e := range s.Enum {
			allowed = append(allowed, e.Value)
		}
		return "", fmt.Errorf("must be one of %s", strings.Join(allowed, ", "))
	case settings.TypeJSON:
		if value != "" && !json.Valid([]byte(value)) {
			return "", fmt.Errorf("must be valid JSON")
		}
	}
	return value, nil
}
