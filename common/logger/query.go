package logger

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/utils"
)

// Log levels as the system log viewer groups them. Entries carry the raw zap or helper level
// ("INFO", "ERR", "FATAL", ...); NormalizeLevel folds them into these four.
const (
	LevelError = "error"
	LevelWarn  = "warn"
	LevelInfo  = "info"
	LevelDebug = "debug"
)

// NormalizeLevel maps a raw level name to one of the viewer's four levels.
func NormalizeLevel(level string) string {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "err", "error", "fatal", "panic", "dpanic":
		return LevelError
	case "warn", "warning":
		return LevelWarn
	case "debug":
		return LevelDebug
	default:
		return LevelInfo
	}
}

// LogQueryParams represents parameters for advanced log queries
type LogQueryParams struct {
	Count      int    `json:"count"`
	SearchTerm string `json:"search_term,omitempty"`
	UseRegex   bool   `json:"use_regex,omitempty"`
	// Level keeps only entries of one normalized level; empty keeps every level.
	Level string `json:"level,omitempty"`
}

// LogQueryResult is a page of matching entries, newest first.
type LogQueryResult struct {
	Logs []LogEntry `json:"logs"`
	// TotalCount is the number of entries matching the search and level, before Count cuts them.
	TotalCount int  `json:"total_count"`
	HasMore    bool `json:"has_more"`
	// LevelCounts counts the entries matching the search, per normalized level, ignoring Level.
	LevelCounts map[string]int `json:"level_counts"`
}

// LogContextParams locates one entry and asks for the lines around it.
type LogContextParams struct {
	Timestamp    time.Time `json:"timestamp"`
	Message      string    `json:"message"`
	ContextLines int       `json:"context_lines"`
}

// LogContextResult holds the entries around the target in log order (oldest first).
// TargetPosition is the target's index in Logs, or -1 when it was not found.
type LogContextResult struct {
	Logs           []LogEntry `json:"logs"`
	TargetPosition int        `json:"target_position"`
}

// GetLatestLogs gets the latest n logs from memory
func GetLatestLogs(n int) ([]LogEntry, error) {
	return logHistory.GetLatestEntries(n), nil
}

// QueryLogs searches the in-memory history and returns the newest matches.
func QueryLogs(params LogQueryParams) (*LogQueryResult, error) {
	match, err := searchMatcher(params.SearchTerm, params.UseRegex)
	if err != nil {
		return nil, err
	}
	entries := logHistory.allEntries()

	level := ""
	if params.Level != "" {
		level = NormalizeLevel(params.Level)
	}
	result := &LogQueryResult{
		Logs:        []LogEntry{},
		LevelCounts: map[string]int{LevelError: 0, LevelWarn: 0, LevelInfo: 0, LevelDebug: 0},
	}
	// Walk newest to oldest so the page holds the latest matches.
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !match(entry.Message) {
			continue
		}
		entryLevel := NormalizeLevel(entry.Level)
		result.LevelCounts[entryLevel]++
		if level != "" && entryLevel != level {
			continue
		}
		result.TotalCount++
		if len(result.Logs) < params.Count {
			result.Logs = append(result.Logs, entry)
		}
	}
	result.HasMore = result.TotalCount > len(result.Logs)
	return result, nil
}

// LogContext returns the in-memory entries around the one matching the timestamp (to the second) and
// message, searching from the newest entry back.
func LogContext(params LogContextParams) (*LogContextResult, error) {
	entries := logHistory.allEntries()
	target := -1
	want := strings.TrimSpace(params.Message)
	ts := params.Timestamp.Truncate(time.Second)
	for i := len(entries) - 1; i >= 0; i-- {
		if strings.TrimSpace(entries[i].Message) == want && entries[i].Timestamp.Truncate(time.Second).Equal(ts) {
			target = i
			break
		}
	}
	if target == -1 {
		return &LogContextResult{Logs: []LogEntry{}, TargetPosition: -1}, nil
	}
	start := max(0, target-params.ContextLines)
	end := min(len(entries), target+params.ContextLines+1)
	logs := make([]LogEntry, end-start)
	copy(logs, entries[start:end])
	return &LogContextResult{Logs: logs, TargetPosition: target - start}, nil
}

// searchMatcher builds the message filter. Plain terms and regular expressions both ignore case.
func searchMatcher(term string, useRegex bool) (func(string) bool, error) {
	term = strings.TrimSpace(term)
	if term == "" {
		return func(string) bool { return true }, nil
	}
	if useRegex {
		re, err := regexp.Compile("(?i)" + term)
		if err != nil {
			return nil, fmt.Errorf("invalid regular expression: %v", err)
		}
		return re.MatchString, nil
	}
	lower := strings.ToLower(term)
	return func(msg string) bool { return strings.Contains(strings.ToLower(msg), lower) }, nil
}

// LogFilePath is the active log file. Rotated files sit next to it.
func LogFilePath() (string, error) {
	logDir := getLogDir()
	if logDir == "" {
		return "", fmt.Errorf("log directory not configured")
	}
	return filepath.Join(logDir, utils.GetOrDefault("logs.filename", "modeltaps.log")), nil
}
