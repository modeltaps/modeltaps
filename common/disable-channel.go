package common

import (
	"strings"
	"sync"

	"github.com/modeltaps/modeltaps/common/ahocorasick"
)

// DisableChannelKeyword holds the operator's keyword list: an upstream error message that
// contains any of them (exact, case-sensitive substring) disables the channel.
type DisableChannelKeyword struct {
	keywords string
	matcher  *ahocorasick.Matcher // nil when no keyword is set
	mutex    sync.RWMutex
}

var DisableChannelKeywordsInstance = &DisableChannelKeyword{}

func GetDefaultDisableChannelKeywords() string {
	return `Your credit balance is too low
This organization has been disabled.
You exceeded your current quota
Permission denied
Quota exceeded for quota metric
API key not valid
API Key not found
API key expired
The security token included in the request is invalid
Operation not allowed
Your account is not authorized
your account balance is insufficient
Your account is currently blocked
too many invalid requests`
}

func (d *DisableChannelKeyword) Load(keywords string) {
	var patterns []string
	for _, keyword := range strings.Split(keywords, "\n") {
		// Trim before the emptiness check: a whitespace-only line must not become an empty
		// keyword (it would match nothing, and crashed the previous matcher).
		if keyword = strings.TrimSpace(keyword); keyword != "" {
			patterns = append(patterns, keyword)
		}
	}
	var matcher *ahocorasick.Matcher
	if len(patterns) > 0 {
		matcher = ahocorasick.New(patterns)
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.matcher = matcher
	d.keywords = keywords
}

func (d *DisableChannelKeyword) IsContains(message string) bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.matcher != nil && d.matcher.ContainsString(message)
}

func (d *DisableChannelKeyword) GetKeywords() string {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.keywords
}
