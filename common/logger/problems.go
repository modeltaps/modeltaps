package logger

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Problems: warnings and errors folded by message template, so a message that repeats every
// minute is one row with a count instead of hundreds of lines. Counts start at process start;
// a group the operator marks as handled is removed and comes back if it happens again.

const (
	maxProblemGroups  = 200
	problemSampleSize = 5
)

// ProblemGroup is one recurring warning or error.
type ProblemGroup struct {
	Key       string     `json:"key"`
	Level     string     `json:"level"`
	Source    string     `json:"source"`
	Template  string     `json:"template"`
	Count     int        `json:"count"`
	FirstSeen time.Time  `json:"first_seen"`
	LastSeen  time.Time  `json:"last_seen"`
	Samples   []LogEntry `json:"samples"` // newest last
}

type problemStore struct {
	mu     sync.Mutex
	groups map[string]*ProblemGroup
}

var problems = &problemStore{groups: map[string]*ProblemGroup{}}

var (
	// Request ids are a 14-digit timestamp plus random characters; long hex or base62 tokens
	// are ids, keys or hashes. Quoted values and numbers change between occurrences too.
	requestIDRe = regexp.MustCompile(`^\d{14}\w+$`)
	tokenRe     = regexp.MustCompile(`\b[0-9A-Za-z_-]{20,}\b`)
	quotedRe    = regexp.MustCompile(`"[^"]*"|'[^']*'`)
	numberRe    = regexp.MustCompile(`\d+(\.\d+)?`)
	spaceRe     = regexp.MustCompile(`\s+`)
)

// splitSource splits "<source> | <text>". Request ids fold into one "request" source so the
// same failure on different requests is one group.
func splitSource(message string) (source, text string) {
	head, rest, ok := strings.Cut(message, " | ")
	if !ok || strings.ContainsAny(head, " \t") {
		return "", message
	}
	head = strings.Trim(head, "[]")
	if requestIDRe.MatchString(head) || head == "unknown" {
		head = "request"
	}
	return head, rest
}

// messageTemplate replaces the parts that change between occurrences with placeholders.
func messageTemplate(text string) string {
	t := tokenRe.ReplaceAllString(text, "*")
	t = quotedRe.ReplaceAllString(t, `"…"`)
	t = numberRe.ReplaceAllString(t, "N")
	t = spaceRe.ReplaceAllString(strings.TrimSpace(t), " ")
	if len(t) > 300 {
		t = t[:300] + "…"
	}
	return t
}

func (ps *problemStore) add(entry LogEntry) {
	level := NormalizeLevel(entry.Level)
	if level != LevelError && level != LevelWarn {
		return
	}
	source, text := splitSource(entry.Message)
	template := messageTemplate(text)
	sum := sha1.Sum([]byte(level + "\x00" + source + "\x00" + template))
	key := hex.EncodeToString(sum[:8])

	ps.mu.Lock()
	defer ps.mu.Unlock()
	g, ok := ps.groups[key]
	if !ok {
		if len(ps.groups) >= maxProblemGroups {
			ps.evictOldestLocked()
		}
		g = &ProblemGroup{Key: key, Level: level, Source: source, Template: template, FirstSeen: entry.Timestamp}
		ps.groups[key] = g
	}
	g.Count++
	g.LastSeen = entry.Timestamp
	g.Samples = append(g.Samples, entry)
	if len(g.Samples) > problemSampleSize {
		g.Samples = g.Samples[len(g.Samples)-problemSampleSize:]
	}
}

func (ps *problemStore) evictOldestLocked() {
	var oldest *ProblemGroup
	for _, g := range ps.groups {
		if oldest == nil || g.LastSeen.Before(oldest.LastSeen) {
			oldest = g
		}
	}
	if oldest != nil {
		delete(ps.groups, oldest.Key)
	}
}

// ListProblems returns every group, most recent first, as copies.
func ListProblems() []ProblemGroup {
	problems.mu.Lock()
	defer problems.mu.Unlock()
	list := make([]ProblemGroup, 0, len(problems.groups))
	for _, g := range problems.groups {
		c := *g
		c.Samples = append([]LogEntry(nil), g.Samples...)
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].LastSeen.After(list[j].LastSeen) })
	return list
}

// ResolveProblems removes the given groups, or every group when keys is empty.
func ResolveProblems(keys []string) int {
	problems.mu.Lock()
	defer problems.mu.Unlock()
	if len(keys) == 0 {
		n := len(problems.groups)
		problems.groups = map[string]*ProblemGroup{}
		return n
	}
	n := 0
	for _, k := range keys {
		if _, ok := problems.groups[k]; ok {
			delete(problems.groups, k)
			n++
		}
	}
	return n
}
