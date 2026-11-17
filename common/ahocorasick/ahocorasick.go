// Package ahocorasick is a small rune-level Aho-Corasick multi-pattern matcher.
//
// It replaces an unmaintained third-party matcher and keeps its observable behaviour: patterns
// and text are compared rune by rune with no case folding or normalization, positions are rune
// offsets, every occurrence is reported (overlapping ones included), and the occurrences that
// end at the same rune are reported shortest pattern first. Duplicate patterns are reported
// once. The one deliberate difference is that empty patterns are ignored; the previous matcher
// panicked on them.
//
// A Matcher is immutable after New and safe for concurrent use.
package ahocorasick

// Match is one occurrence of a pattern in the searched text.
type Match struct {
	// Pos is the rune offset of the first rune of Word in the searched text.
	Pos int
	// Word is the matched pattern. It is shared with the Matcher and must not be modified.
	Word []rune
}

type node struct {
	next map[rune]int32
	fail int32
	// out lists the patterns that end at this node: those reached through the failure chain
	// (shorter ones) first, the node's own pattern last.
	out []int32
}

// Matcher finds every occurrence of a fixed set of patterns in a text.
type Matcher struct {
	nodes    []node
	patterns [][]rune
}

// New builds a Matcher for patterns. Empty and duplicate patterns are skipped; a Matcher with
// no patterns matches nothing.
func New(patterns []string) *Matcher {
	m := &Matcher{nodes: []node{{}}}
	seen := make(map[string]struct{}, len(patterns))
	for _, p := range patterns {
		if p == "" {
			continue
		}
		word := []rune(p)
		// Deduplicate on the rune form: invalid UTF-8 bytes all decode to U+FFFD, so two
		// different byte strings can be the same pattern.
		key := string(word)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		m.insert(word)
	}
	m.link()
	return m
}

func (m *Matcher) insert(word []rune) {
	cur := int32(0)
	for _, r := range word {
		nxt, ok := m.nodes[cur].next[r]
		if !ok {
			nxt = int32(len(m.nodes))
			m.nodes = append(m.nodes, node{})
			if m.nodes[cur].next == nil {
				m.nodes[cur].next = make(map[rune]int32)
			}
			m.nodes[cur].next[r] = nxt
		}
		cur = nxt
	}
	m.nodes[cur].out = []int32{int32(len(m.patterns))}
	m.patterns = append(m.patterns, word)
}

// link computes the failure links breadth first and merges each node's output with the output
// of its failure target, so a search never has to walk the failure chain to collect matches.
func (m *Matcher) link() {
	queue := make([]int32, 0, len(m.nodes))
	for _, child := range m.nodes[0].next {
		queue = append(queue, child)
	}
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		for r, v := range m.nodes[u].next {
			f := m.nodes[u].fail
			for {
				if t, ok := m.nodes[f].next[r]; ok {
					m.nodes[v].fail = t
					break
				}
				if f == 0 {
					m.nodes[v].fail = 0
					break
				}
				f = m.nodes[f].fail
			}
			if inherited := m.nodes[m.nodes[v].fail].out; len(inherited) > 0 {
				out := make([]int32, 0, len(inherited)+len(m.nodes[v].out))
				out = append(out, inherited...)
				m.nodes[v].out = append(out, m.nodes[v].out...)
			}
			queue = append(queue, v)
		}
	}
}

func (m *Matcher) step(state int32, r rune) int32 {
	for {
		if nxt, ok := m.nodes[state].next[r]; ok {
			return nxt
		}
		if state == 0 {
			return 0
		}
		state = m.nodes[state].fail
	}
}

// FindAll returns every occurrence of every pattern in text, ordered by the rune offset where
// the occurrence ends and, for the same end, shortest pattern first.
func (m *Matcher) FindAll(text []rune) []Match {
	var matches []Match
	state := int32(0)
	for i, r := range text {
		state = m.step(state, r)
		for _, idx := range m.nodes[state].out {
			word := m.patterns[idx]
			matches = append(matches, Match{Pos: i - len(word) + 1, Word: word})
		}
	}
	return matches
}

// ContainsString reports whether s contains any pattern. s is read rune by rune exactly as
// []rune(s) would decode it, without allocating.
func (m *Matcher) ContainsString(s string) bool {
	if len(m.patterns) == 0 {
		return false
	}
	state := int32(0)
	for _, r := range s {
		state = m.step(state, r)
		if len(m.nodes[state].out) > 0 {
			return true
		}
	}
	return false
}
