package model

import (
	"fmt"
	"math"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/datatypes"
)

// UsageWindow is one rolling usage limit of a subscription account (for example the 5-hour
// and weekly windows of a ChatGPT or Claude plan).
type UsageWindow struct {
	Name          string  `json:"name"`           // "5h", "7d", or the window length in seconds
	UsedPercent   float64 `json:"used_percent"`   // 0–100
	ResetAt       int64   `json:"reset_at"`       // unix seconds, 0 when unknown
	WindowSeconds int64   `json:"window_seconds"` // 0 when unknown
}

// ChannelUsageWindows is the last known usage of a subscription channel (Codex, Claude Code):
// these accounts have no money balance, the usage windows are what runs out.
type ChannelUsageWindows struct {
	PlanType  string        `json:"plan_type,omitempty"`
	Status    string        `json:"status,omitempty"` // upstream limit status, e.g. allowed / rejected
	Windows   []UsageWindow `json:"windows"`
	Source    string        `json:"source"` // "api" (queried) or "headers" (from the last response)
	FetchedAt int64         `json:"fetched_at"`
}

// WindowName names a window by its length: 18000 s is "5h", 604800 s is "7d".
func WindowName(seconds int64) string {
	switch {
	case seconds <= 0:
		return ""
	case seconds%86400 == 0:
		return fmt.Sprintf("%dd", seconds/86400)
	case seconds%3600 == 0:
		return fmt.Sprintf("%dh", seconds/3600)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// UpdateUsageWindows stores the usage windows without touching other columns.
func (channel *Channel) UpdateUsageWindows(w ChannelUsageWindows) error {
	jt := datatypes.NewJSONType(w)
	if err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("usage_windows", jt).Error; err != nil {
		return err
	}
	channel.UsageWindows = &jt
	return nil
}

// claudeUnifiedHeader matches anthropic-ratelimit-unified-<window>-<field>, e.g.
// anthropic-ratelimit-unified-5h-utilization or -7d-reset.
var claudeUnifiedHeader = regexp.MustCompile(`^anthropic-ratelimit-unified-(\d+[hd])-(utilization|reset)$`)

// ParseClaudeUnifiedHeaders reads the subscription usage that Anthropic reports on Claude Code
// responses. It returns nil when the response carries no unified window headers.
func ParseClaudeUnifiedHeaders(h http.Header) *ChannelUsageWindows {
	byName := map[string]*UsageWindow{}
	for key, values := range h {
		if len(values) == 0 {
			continue
		}
		m := claudeUnifiedHeader.FindStringSubmatch(strings.ToLower(key))
		if m == nil {
			continue
		}
		w := byName[m[1]]
		if w == nil {
			w = &UsageWindow{Name: m[1], WindowSeconds: windowSeconds(m[1])}
			byName[m[1]] = w
		}
		v := strings.TrimSpace(values[0])
		switch m[2] {
		case "utilization":
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				if f <= 1 { // reported as a fraction
					f *= 100
				}
				w.UsedPercent = math.Round(f*10) / 10
			}
		case "reset":
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				w.ResetAt = n
			}
		}
	}
	if len(byName) == 0 {
		return nil
	}
	out := &ChannelUsageWindows{Source: "headers", FetchedAt: utils.GetTimestamp(),
		Status: h.Get("anthropic-ratelimit-unified-status")}
	for _, w := range byName {
		out.Windows = append(out.Windows, *w)
	}
	sort.Slice(out.Windows, func(i, j int) bool { return out.Windows[i].WindowSeconds < out.Windows[j].WindowSeconds })
	return out
}

func windowSeconds(name string) int64 {
	n, err := strconv.ParseInt(name[:len(name)-1], 10, 64)
	if err != nil {
		return 0
	}
	if strings.HasSuffix(name, "d") {
		return n * 86400
	}
	return n * 3600
}

var (
	usageHeaderMu    sync.Mutex
	usageHeaderSaved = map[int]time.Time{}
)

// ObserveClaudeUsageHeaders stores the usage windows from a Claude Code response, at most once
// a minute per channel so busy channels do not write on every request.
func ObserveClaudeUsageHeaders(channelId int, h http.Header) {
	w := ParseClaudeUnifiedHeaders(h)
	if w == nil || channelId <= 0 {
		return
	}
	usageHeaderMu.Lock()
	if time.Since(usageHeaderSaved[channelId]) < time.Minute {
		usageHeaderMu.Unlock()
		return
	}
	usageHeaderSaved[channelId] = time.Now()
	usageHeaderMu.Unlock()
	go func() {
		ch := &Channel{Id: channelId}
		if err := ch.UpdateUsageWindows(*w); err != nil {
			logger.SysError(fmt.Sprintf("save usage windows for channel %d failed: %s", channelId, err.Error()))
		}
	}()
}
