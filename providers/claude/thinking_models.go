package claude

import (
	"regexp"
	"strconv"
	"strings"
)

// Which thinking configuration a Claude model takes, from its name. Two generations matter:
//
//   - Opus 4.6 and Sonnet 4.6 take adaptive thinking ({type: "adaptive"}); budget_tokens still
//     works there but is deprecated, and their thinking is shown in full by default.
//   - Opus 4.7 and later, Sonnet 5 and later, Haiku 5 and later, and the Fable and Mythos models
//     reject budget_tokens with a 400 and hide the thinking text by default (display "omitted":
//     thinking blocks arrive with empty text), so a client that asked to see the reasoning gets
//     nothing unless the request sets display "summarized". Display changes visibility only;
//     thinking is billed the same either way.
//
// Older models (Sonnet 4.5, Haiku 4.5, the 3.x names) only take {type: "enabled",
// budget_tokens: N}. A name that is not recognised is treated as an older model.

// claudeModelRe matches "claude-<family>-<major>[-.<minor>]" anywhere in a model name, so vendor
// prefixes ("anthropic/claude-opus-5.5") and date suffixes ("claude-sonnet-4-5-20250929") work.
// The minor version is one or two digits followed by a non-digit or the end, so a date right
// after the major ("claude-opus-4-20250514") is not read as a minor version.
var claudeModelRe = regexp.MustCompile(`(?i)claude-(opus|sonnet|haiku|fable|mythos)-(\d+)(?:[.-](\d{1,2})(?:\D|$))?`)

func claudeModelVersion(name string) (family string, major, minor int, ok bool) {
	m := claudeModelRe.FindStringSubmatch(name)
	if m == nil {
		return "", 0, 0, false
	}
	major, _ = strconv.Atoi(m[2])
	if m[3] != "" {
		minor, _ = strconv.Atoi(m[3])
	}
	return strings.ToLower(m[1]), major, minor, true
}

// AdaptiveThinkingSupported reports whether the model takes {type: "adaptive"} thinking.
func AdaptiveThinkingSupported(name string) bool {
	family, major, minor, ok := claudeModelVersion(name)
	if !ok {
		return false
	}
	switch family {
	case "fable", "mythos":
		return true
	case "opus", "sonnet":
		return major > 4 || (major == 4 && minor >= 6)
	case "haiku":
		return major >= 5
	}
	return false
}

// ThinkingHiddenByDefault reports whether the model rejects budget_tokens and returns empty
// thinking text unless the request sets display "summarized".
func ThinkingHiddenByDefault(name string) bool {
	family, major, minor, ok := claudeModelVersion(name)
	if !ok {
		return false
	}
	switch family {
	case "fable", "mythos":
		return true
	case "opus":
		return major > 4 || (major == 4 && minor >= 7)
	case "sonnet", "haiku":
		return major >= 5
	}
	return false
}

// ClaudeEffort is claudeEffort for other providers that serve Claude models.
func ClaudeEffort(effort, model string) string { return claudeEffort(effort, model) }

// AlwaysThinks reports whether the model thinks on every request and takes no thinking
// configuration but the effort (Fable, Mythos): omitting thinking is the same as adaptive.
func AlwaysThinks(name string) bool {
	family, _, _, ok := claudeModelVersion(name)
	return ok && (family == "fable" || family == "mythos")
}

// claudeEffort maps an OpenAI-style reasoning effort onto Claude's output_config.effort, "" when
// it has no counterpart. "minimal" becomes "low"; "xhigh" becomes "high" on models that do not
// have it (Opus 4.6, Sonnet 4.6).
func claudeEffort(effort, model string) string {
	switch e := strings.ToLower(strings.TrimSpace(effort)); e {
	case "minimal", "low":
		return "low"
	case "medium", "high", "max":
		return e
	case "xhigh":
		if ThinkingHiddenByDefault(model) {
			return e
		}
		return "high"
	}
	return ""
}
