package settings

import (
	"net/url"
	"sort"
	"strings"
)

// objectPages are the console pages object fields are set on (kept in step with OBJECT_PAGES in
// web/src/components/settings/catalog.js).
var objectPages = map[string]string{
	"channels":         "/panel/admin/providers",
	"models":           "/panel/admin/models",
	"allocation":       "/panel/admin/providers",
	"user-groups":      "/panel/admin/user-groups",
	"routing-policies": "/panel/admin/routing",
	"organizations":    "/panel/admin/orgs",
	"tokens":           "/panel/keys",
}

// ConsolePath is where a setting is changed in the console: the field of a platform setting, or
// the page of the object a field belongs to.
func ConsolePath(s Setting) string {
	if s.Scope == ScopeSite && s.Page != "" {
		return objectPages[s.Page] + "?highlight=" + url.QueryEscape(s.Key)
	}
	if s.Scope == ScopeSite {
		return "/panel/admin/settings/" + s.Group + "?highlight=" + url.QueryEscape(s.Key)
	}
	return objectPages[s.Group]
}

// GroupTitles are the English names of the settings page groups and object pages, for places
// without the console's translations (docs, MCP).
var GroupTitles = map[string]string{
	"site": "Site", "auth": "Sign-in and sign-up", "billing": "Billing", "payment": "Payment",
	"supply": "Supply allocation", "gateway": "Gateway", "privacy": "Logs and privacy", "media": "Media and storage", "entries": "Entries", "mail": "Mail",
	"notify": "Notifications", "orgpolicy": "Organization policy",
	"channels": "Channels", "models": "Models", "allocation": "Allocation policy",
	"user-groups": "User groups", "routing-policies": "Routing policies", "organizations": "Organizations",
	"tokens": "API keys",
}

// Location says in words where a setting is set: "Platform settings > Gateway > retry".
func Location(s Setting) string {
	if s.Scope == ScopeSite && s.Page != "" {
		return GroupTitles[s.Page] + " > " + s.Card
	}
	if s.Scope == ScopeSite {
		return "Platform settings > " + GroupTitles[s.Group] + " > " + s.Card
	}
	return GroupTitles[s.Group] + " > " + s.Card
}

// SearchText is everything a search matches against, lower-cased.
func SearchText(s Setting) string {
	parts := []string{s.Key, s.Label, s.Description, string(s.Scope), s.Group, s.Card}
	parts = append(parts, s.Keywords...)
	for _, e := range s.Enum {
		parts = append(parts, e.Value, e.Label, e.Description)
	}
	for _, ex := range s.Examples {
		parts = append(parts, ex.Value, ex.Effect)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

// Filter narrows a search.
type Filter struct {
	Scope    Scope
	Group    string
	Advanced bool // include advanced settings
}

// Search returns the entries every word of query appears in, best matches first: the key, then
// the label, then the rest. An empty query matches every entry the filter lets through.
//
// Words no entry contains are dropped first, so a word in another language or a typo does not
// empty the result (the catalog is English). When no entry has every remaining word, the entries
// with the most of them are returned, so a question-like query still finds something.
func Search(query string, f Filter) []Setting {
	words := knownWords(strings.Fields(strings.ToLower(query)))
	type scored struct {
		s       Setting
		matched int
		score   int
	}
	var hits []scored
	best := 0
	for i, s := range All() {
		if f.Scope != "" && s.Scope != f.Scope {
			continue
		}
		if f.Group != "" && s.Group != f.Group {
			continue
		}
		if s.Advanced && !f.Advanced && len(words) == 0 {
			continue
		}
		text := SearchText(s)
		matched, score := 0, 0
		key, label := strings.ToLower(s.Key), strings.ToLower(s.Label)
		for _, w := range words {
			if !strings.Contains(text, w) {
				continue
			}
			matched++
			switch {
			case key == w:
				score += 100
			case strings.Contains(key, w):
				score += 30
			case strings.Contains(label, w):
				score += 20
			default:
				score += 1
			}
		}
		if len(words) > 0 && matched == 0 {
			continue
		}
		if s.Advanced && !f.Advanced {
			score -= 10
		}
		best = max(best, matched)
		hits = append(hits, scored{s, matched, score*10000 - i})
	}
	kept := hits[:0]
	for _, h := range hits {
		if h.matched == best {
			kept = append(kept, h)
		}
	}
	hits = kept
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].score > hits[j].score })
	out := make([]Setting, len(hits))
	for i, h := range hits {
		out[i] = h.s
	}
	return out
}

// knownWords drops the words no entry contains. When none is left the words are kept, so the
// search finds nothing rather than everything.
func knownWords(words []string) []string {
	if len(words) == 0 {
		return words
	}
	texts := make([]string, 0, len(All()))
	for _, s := range All() {
		texts = append(texts, SearchText(s))
	}
	known := words[:0:0]
	for _, w := range words {
		for _, t := range texts {
			if strings.Contains(t, w) {
				known = append(known, w)
				break
			}
		}
	}
	if len(known) == 0 {
		return words
	}
	return known
}
