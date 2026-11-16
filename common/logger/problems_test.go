package logger

import (
	"testing"
	"time"
)

func TestProblemsFoldRepeatsByTemplate(t *testing.T) {
	saved := problems.groups
	problems.groups = map[string]*ProblemGroup{}
	t.Cleanup(func() { problems.groups = saved })

	base := time.Date(2026, 11, 16, 10, 0, 0, 0, time.Local)
	add := func(sec int, level, msg string) {
		problems.add(LogEntry{Timestamp: base.Add(time.Duration(sec) * time.Second), Level: level, Message: msg})
	}
	add(0, "ERR", "[SYS] | update balance failed for channel requesty (ID 12): timeout")
	add(1, "ERR", "[SYS] | update balance failed for channel requesty (ID 13): timeout")
	add(2, "WARN", "20261116073038937019000GBC6JFc0 | upstream returned 429")
	add(3, "WARN", "20261116073038937019000ABCDEF12 | upstream returned 429")
	add(4, "INFO", "[SYS] | started")

	list := ListProblems()
	if len(list) != 2 {
		t.Fatalf("want 2 groups, got %d: %+v", len(list), list)
	}
	if list[0].Source != "request" || list[0].Count != 2 || list[0].Template != "upstream returned N" {
		t.Fatalf("unexpected newest group %+v", list[0])
	}
	if list[1].Source != "SYS" || list[1].Count != 2 || !list[1].FirstSeen.Equal(base) || len(list[1].Samples) != 2 {
		t.Fatalf("unexpected balance group %+v", list[1])
	}

	if n := ResolveProblems([]string{list[0].Key}); n != 1 || len(ListProblems()) != 1 {
		t.Fatalf("resolve one: n=%d left=%d", n, len(ListProblems()))
	}
	if n := ResolveProblems(nil); n != 1 || len(ListProblems()) != 0 {
		t.Fatalf("resolve all: n=%d", n)
	}
}

func TestAllEntriesKeepsOlderWarnings(t *testing.T) {
	base := time.Date(2026, 11, 16, 10, 0, 0, 0, time.Local)
	saved, savedImportant := logHistory.entries, logHistory.important
	t.Cleanup(func() { logHistory.entries, logHistory.important = saved, savedImportant })
	old := LogEntry{Timestamp: base, Level: "ERR", Message: "old error"}
	recent := LogEntry{Timestamp: base.Add(time.Minute), Level: "ERR", Message: "recent error"}
	logHistory.important = []LogEntry{old, recent}
	logHistory.entries = []LogEntry{recent, {Timestamp: base.Add(2 * time.Minute), Level: "INFO", Message: "info"}}

	all := logHistory.allEntries()
	if len(all) != 3 || all[0].Message != "old error" || all[1].Message != "recent error" {
		t.Fatalf("unexpected merge %+v", all)
	}
}
