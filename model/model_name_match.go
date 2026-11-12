package model

import (
	"regexp"
	"sort"
	"strings"
)

// How an upstream model name on a channel relates to a public model name. See
// design/model-library.md §7 in the notes repository.
const (
	// MatchExact: the same name, a confirmed channel alias, or a catalog alias.
	MatchExact = "exact"
	// MatchPlatform: the same model under the platform's spelling (prefix, suffix, separators,
	// a dated snapshot of an undated public name). Known spellings come from the model library.
	MatchPlatform = "platform"
	// MatchVariant: the name adds a part that makes it a different model ("-mini", "-thinking").
	// Not a match; listed so the admin can see it was considered.
	MatchVariant = "variant"
	// MatchPending: the public name is all there, but something unknown is added. Offered for
	// confirmation, not selected by default.
	MatchPending = "pending"
)

// NameMatch is one upstream name considered for a public name.
type NameMatch struct {
	Tier string `json:"match"`
	// Default is true when the channel should be selected without the admin doing anything.
	Default bool `json:"default"`
	// Extra lists what the upstream name has beyond the public name ("thinking", "free"), shown
	// as the reason for variant and pending matches.
	Extra []string `json:"extra,omitempty"`
}

// parsedName is a model name with platform decoration taken off and the rest split into words.
type parsedName struct {
	words    []string
	core     string // words joined with "-"
	modifier string // the part after ":", without the colon
}

var nameSeparators = regexp.MustCompile(`[._\s]+`)

func parseModelName(name string, rules NamingRules) parsedName {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.TrimPrefix(s, "~")
	if i := strings.LastIndex(s, "/"); i >= 0 {
		s = s[i+1:]
	}
	for changed := true; changed; {
		changed = false
		for _, p := range rules.Prefixes {
			if strings.HasPrefix(s, p) && len(s) > len(p) {
				s = s[len(p):]
				changed = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, re := range rules.Suffixes {
			if loc := re.FindStringIndex(s); loc != nil && loc[0] > 0 {
				s = s[:loc[0]]
				changed = true
			}
		}
	}
	var out parsedName
	if i := strings.Index(s, ":"); i >= 0 {
		out.modifier = s[i+1:]
		s = s[:i]
	}
	s = nameSeparators.ReplaceAllString(s, "-")
	for _, w := range strings.Split(s, "-") {
		if w != "" {
			out.words = append(out.words, w)
		}
	}
	out.core = strings.Join(out.words, "-")
	return out
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isSnapshotWord is a date stamp ("20261101", "0613", "2508"), not a version number.
func isSnapshotWord(w string) bool { return isDigits(w) && len(w) >= 4 }

// subsequenceExtra returns the words of up that are not part of an in-order occurrence of pub,
// and false when pub's words do not all appear in order. Words must match whole: "5" does not
// match "52".
func subsequenceExtra(up, pub []string) ([]string, bool) {
	var extra []string
	j := 0
	for _, w := range up {
		if j < len(pub) && w == pub[j] {
			j++
			continue
		}
		extra = append(extra, w)
	}
	return extra, j == len(pub)
}

// matchModelName compares an upstream name on a channel of the given type with a public name.
// ok is false when they are unrelated. The library's known platform spellings and channel aliases
// are checked by the caller; this is the rule-based part.
func matchModelName(upstream, public string, channelType int) (NameMatch, bool) {
	if strings.EqualFold(strings.TrimSpace(upstream), strings.TrimSpace(public)) {
		return NameMatch{Tier: MatchExact, Default: true}, true
	}
	rules := namingRulesFor(channelType)
	up := parseModelName(upstream, rules)
	pub := parseModelName(public, genericNamingRules)
	if len(pub.words) == 0 || len(up.words) == 0 {
		return NameMatch{}, false
	}
	extra, ok := subsequenceExtra(up.words, pub.words)
	if !ok {
		return NameMatch{}, false
	}
	m := NameMatch{Tier: MatchPlatform, Default: true}
	variant := false
	var unknown []string
	for _, w := range extra {
		switch {
		case isSnapshotWord(w):
			// A dated snapshot of an undated public name is the same model; a public name that
			// carries its own date never matches another date (pub's words would be missing).
			m.Extra = append(m.Extra, w)
		case isDigits(w):
			// Another version number: "gpt-5-1" is not "gpt-5".
			return NameMatch{}, false
		case isVariantWord(w):
			variant = true
		default:
			unknown = append(unknown, w)
		}
	}
	if variant {
		return NameMatch{Tier: MatchVariant, Extra: extra}, true
	}
	if up.modifier != "" && up.modifier != pub.modifier {
		switch rules.Modifiers[up.modifier] {
		case ModifierSame:
		case ModifierSameOptional:
			m.Default = false
			m.Extra = append(m.Extra, ":"+up.modifier)
		case ModifierVariant:
			return NameMatch{Tier: MatchVariant, Extra: append(extra, ":"+up.modifier)}, true
		default:
			unknown = append(unknown, ":"+up.modifier)
		}
	}
	if len(unknown) > 0 {
		return NameMatch{Tier: MatchPending, Extra: append(m.Extra, unknown...)}, true
	}
	return m, true
}

// tierRank orders tiers from strongest to weakest.
func tierRank(tier string) int {
	switch tier {
	case MatchExact:
		return 0
	case MatchPlatform:
		return 1
	case MatchPending:
		return 2
	case MatchVariant:
		return 3
	}
	return 4
}

// publicNameIndex finds the public names worth comparing with an upstream name: those whose first
// word appears in it.
type publicNameIndex struct {
	byFirstWord map[string][]string
	parsed      map[string]parsedName
	lower       map[string]bool
}

// has reports whether name (any case) is one of the public names.
func (idx *publicNameIndex) has(name string) bool { return idx.lower[strings.ToLower(name)] }

func newPublicNameIndex(names []string) *publicNameIndex {
	idx := &publicNameIndex{byFirstWord: map[string][]string{}, parsed: map[string]parsedName{}, lower: map[string]bool{}}
	for _, n := range names {
		idx.lower[strings.ToLower(n)] = true
		p := parseModelName(n, genericNamingRules)
		if len(p.words) == 0 {
			continue
		}
		if _, seen := idx.parsed[n]; seen {
			continue
		}
		idx.parsed[n] = p
		idx.byFirstWord[p.words[0]] = append(idx.byFirstWord[p.words[0]], n)
	}
	for _, list := range idx.byFirstWord {
		sort.Strings(list)
	}
	return idx
}

// candidates returns the public names to try for an upstream name on a channel of this type.
func (idx *publicNameIndex) candidates(upstream string, channelType int) []string {
	up := parseModelName(upstream, namingRulesFor(channelType))
	seen := map[string]bool{}
	var out []string
	for _, w := range up.words {
		for _, n := range idx.byFirstWord[w] {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}
