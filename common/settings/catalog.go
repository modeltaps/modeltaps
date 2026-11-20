// Package settings is the catalog of everything an operator configures: every platform setting
// (the options of the settings page) and the object fields that need explaining (a channel's fee
// rate, a model's supply mode, a user group's ratio...). One entry says what a setting does, where
// it lives in the console, its type and range, and gives examples.
//
// The catalog is the single source of the English text: the console reads it from the generated
// web/src/generated/settings-catalog.json (translations live in
// web/src/i18n/locales/catalog/<language>/<group>.json), the operator docs and the admin MCP tools
// read it from here. Run `go generate ./common/settings` after changing an entry.
package settings

//go:generate go run ./gen

import "sort"

// Scope is the object a setting belongs to.
type Scope string

const (
	ScopeSite          Scope = "site"           // a platform setting (an option of the settings page)
	ScopeChannel       Scope = "channel"        // a field of a channel
	ScopeModel         Scope = "model"          // a field of a public model in the catalog
	ScopeSupply        Scope = "supply"         // a field of a model supply (model x channel)
	ScopeAllocation    Scope = "allocation"     // the allocation policy
	ScopeUserGroup     Scope = "user_group"     // a field of a user group
	ScopeRoutingPolicy Scope = "routing_policy" // a field of an auto-routing policy
	ScopeOrganization  Scope = "organization"   // a field of an organization
	ScopeToken         Scope = "token"          // a field of an API key
)

// Type is how a setting's value is written.
type Type string

const (
	TypeBool    Type = "bool"
	TypeInt     Type = "int"
	TypeFloat   Type = "float"
	TypePercent Type = "percent" // a number of percent: 4 means 4%
	TypeRatio   Type = "ratio"   // a multiplier: 1.12 means 12% more
	TypeString  Type = "string"
	TypeText    Type = "text" // multi-line text or HTML / Markdown
	TypeURL     Type = "url"
	TypeEnum    Type = "enum"
	TypeList    Type = "list" // a comma- or line-separated list
	TypeJSON    Type = "json"
	TypeSecret  Type = "secret" // never shown back; only whether it is set
)

// EnumValue is one value an enum setting takes.
type EnumValue struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// Example is a value and what it does, with numbers where the setting is a number.
type Example struct {
	Value  string `json:"value"`
	Effect string `json:"effect"`
}

// Setting is one catalog entry.
type Setting struct {
	// Key is the option key for site settings ("RetryTimes"), "<scope>.<field>" for object
	// fields ("channel.fee_rate").
	Key   string `json:"key"`
	Scope Scope  `json:"scope"`
	// Group and Card place a site setting on the settings page (the group and card ids of
	// web/src/views/panel/Setting/registry.js). For an object field, Group is the object's page
	// ("channels", "models", "user-groups"...) and Card its tab or section.
	Group string `json:"group"`
	Card  string `json:"card"`
	// Field is the JSON name of an object field; empty for site settings.
	Field string      `json:"field,omitempty"`
	Type  Type        `json:"type"`
	Unit  string      `json:"unit,omitempty"`
	Min   *float64    `json:"min,omitempty"`
	Max   *float64    `json:"max,omitempty"`
	Enum  []EnumValue `json:"enum,omitempty"`
	// Default is for object fields: the value a new object starts with. Site settings take
	// theirs from the running instance (config.GlobalOption.Defaults).
	Default string `json:"default,omitempty"`
	// Label names the setting in a few words. Description says what it does, what happens when it
	// is empty, off or 0, and how it combines with other settings; it may hold several sentences.
	Label       string    `json:"label"`
	Description string    `json:"description"`
	Examples    []Example `json:"examples,omitempty"`
	// Related are keys of settings worth reading together with this one.
	Related []string `json:"related,omitempty"`
	// Keywords are other words people search with (English; translations add their own).
	Keywords []string `json:"keywords,omitempty"`
	// Advanced settings are rarely changed: search shows them only when asked (@advanced).
	Advanced bool `json:"advanced,omitempty"`
	// Docs is a page of the operator docs (docs/ops/<lang>/...) that explains the setting in
	// context, as a path without the language: "use/pricing_and_routing.md#channel-fee-rates".
	Docs string `json:"docs,omitempty"`
}

var (
	entries []Setting
	byKey   = map[string]int{}
)

// add appends entries; a key may appear once.
func add(list ...Setting) {
	for _, s := range list {
		if _, dup := byKey[s.Key]; dup {
			panic("settings: duplicate catalog key " + s.Key)
		}
		byKey[s.Key] = len(entries)
		entries = append(entries, s)
	}
}

// All returns every entry, site settings first in settings page order, then object fields.
func All() []Setting {
	out := append([]Setting(nil), entries...)
	sort.SliceStable(out, func(i, j int) bool {
		return groupRank(out[i]) < groupRank(out[j])
	})
	return out
}

// Get returns the entry of key.
func Get(key string) (Setting, bool) {
	i, ok := byKey[key]
	if !ok {
		return Setting{}, false
	}
	return entries[i], true
}

// groupOrder is the order of the settings page groups, then the object pages.
var groupOrder = []string{"site", "auth", "billing", "payment", "supply", "gateway", "privacy", "media", "entries", "mail", "notify", "orgpolicy",
	"channels", "models", "allocation", "user-groups", "routing-policies", "organizations", "tokens"}

func groupRank(s Setting) int {
	for i, g := range groupOrder {
		if g == s.Group {
			return i
		}
	}
	return len(groupOrder)
}

func num(v float64) *float64 { return &v }

// internalKeys are registered options nobody configures: legacy keys kept for migrations and
// internal state. They stay out of the catalog.
var internalKeys = map[string]string{
	"OIDCAuthEnabled":       "legacy single OIDC provider, migrated to the identity providers list",
	"OIDCClientId":          "legacy single OIDC provider",
	"OIDCClientSecret":      "legacy single OIDC provider",
	"OIDCDisplayName":       "legacy single OIDC provider",
	"OIDCIssuer":            "legacy single OIDC provider",
	"OIDCScopes":            "legacy single OIDC provider",
	"OIDCUsernameClaims":    "legacy single OIDC provider",
	"OidcAutoRedirect":      "legacy single OIDC provider",
	"OldTokenMaxId":         "internal: the last API key id from before key hashing",
	"UpdateOptionTestValue": "internal: used by tests of the option API",
}

// IsInternal reports whether a registered option key is left out of the catalog on purpose.
func IsInternal(key string) bool {
	_, ok := internalKeys[key]
	return ok
}
