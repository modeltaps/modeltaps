package logger

import (
	"testing"
	"time"
)

func withHistory(t *testing.T, entries []LogEntry) {
	t.Helper()
	saved, savedImportant := logHistory.entries, logHistory.important
	logHistory.entries, logHistory.important = entries, nil
	t.Cleanup(func() { logHistory.entries, logHistory.important = saved, savedImportant })
}

func TestQueryLogsNewestFirstWithLevelCounts(t *testing.T) {
	base := time.Date(2026, 11, 16, 10, 0, 0, 0, time.Local)
	withHistory(t, []LogEntry{
		{Timestamp: base, Level: "INFO", Message: "[SYS] | started"},
		{Timestamp: base.Add(time.Second), Level: "ERR", Message: "req-1 | upstream failed"},
		{Timestamp: base.Add(2 * time.Second), Level: "WARN", Message: "req-2 | slow upstream"},
		{Timestamp: base.Add(3 * time.Second), Level: "ERR", Message: "req-3 | Upstream timeout"},
	})

	res, err := QueryLogs(LogQueryParams{Count: 1, SearchTerm: "upstream", Level: "error"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Logs) != 1 || res.Logs[0].Message != "req-3 | Upstream timeout" {
		t.Fatalf("want the newest error first, got %+v", res.Logs)
	}
	if res.TotalCount != 2 || !res.HasMore {
		t.Fatalf("want 2 matches with more, got total=%d more=%v", res.TotalCount, res.HasMore)
	}
	if res.LevelCounts[LevelError] != 2 || res.LevelCounts[LevelWarn] != 1 || res.LevelCounts[LevelInfo] != 0 {
		t.Fatalf("unexpected level counts %v", res.LevelCounts)
	}

	if _, err := QueryLogs(LogQueryParams{Count: 10, SearchTerm: "(", UseRegex: true}); err == nil {
		t.Fatal("want an error for an invalid regular expression")
	}
}

func TestLogContext(t *testing.T) {
	base := time.Date(2026, 11, 16, 10, 0, 0, 0, time.Local)
	var entries []LogEntry
	for i := 0; i < 10; i++ {
		entries = append(entries, LogEntry{Timestamp: base.Add(time.Duration(i) * time.Second), Level: "INFO", Message: string(rune('a' + i))})
	}
	withHistory(t, entries)

	res, err := LogContext(LogContextParams{Timestamp: base.Add(time.Second + 300*time.Millisecond), Message: "b", ContextLines: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Logs) != 5 || res.TargetPosition != 1 || res.Logs[0].Message != "a" {
		t.Fatalf("unexpected context %+v", res)
	}
}
