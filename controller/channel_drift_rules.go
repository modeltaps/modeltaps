package controller

import (
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/modeltaps/modeltaps/common"

	"github.com/gin-gonic/gin"
)

// driftIgnoreMatcher matches upstream model names against a channel's DriftIgnoreRules:
// one rule per line, an exact name, a glob with * or ?, or a regular expression after "re:".
// Invalid regular expressions are skipped.
type driftIgnoreMatcher struct {
	exact map[string]bool
	globs []string
	res   []*regexp.Regexp
}

func newDriftIgnoreMatcher(rules string) *driftIgnoreMatcher {
	m := &driftIgnoreMatcher{exact: map[string]bool{}}
	for _, line := range strings.Split(rules, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case strings.HasPrefix(line, "re:"):
			if re, err := regexp.Compile(strings.TrimSpace(line[3:])); err == nil {
				m.res = append(m.res, re)
			}
		case strings.ContainsAny(line, "*?"):
			m.globs = append(m.globs, line)
		default:
			m.exact[line] = true
		}
	}
	return m
}

func (m *driftIgnoreMatcher) ignored(name string) bool {
	if m == nil {
		return false
	}
	if m.exact[name] {
		return true
	}
	for _, g := range m.globs {
		// path.Match treats "/" as a separator; model ids like vendor/model use it, so match
		// the glob against the whole id with "/" escaped away.
		if ok, _ := path.Match(strings.ReplaceAll(g, "/", "\x00"), strings.ReplaceAll(name, "/", "\x00")); ok {
			return true
		}
	}
	for _, re := range m.res {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// validateDriftIgnoreRules returns the first rule that is not a valid regular expression.
func validateDriftIgnoreRules(rules string) (bad string) {
	for _, line := range strings.Split(rules, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "re:") {
			if _, err := regexp.Compile(strings.TrimSpace(line[3:])); err != nil {
				return line
			}
		}
	}
	return ""
}

// rejectInvalidDriftRules answers with an error and returns true when a "re:" rule does not
// compile, so a typo is reported on save instead of silently ignoring nothing.
func rejectInvalidDriftRules(c *gin.Context, rules string) bool {
	bad := validateDriftIgnoreRules(rules)
	if bad == "" {
		return false
	}
	common.APIRespondWithCode(c, http.StatusOK, common.APICodeChannelDriftRuleInvalid,
		"invalid regular expression in drift ignore rules: "+bad, map[string]any{"rule": bad})
	return true
}
