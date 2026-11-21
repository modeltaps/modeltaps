package requester

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// maxRetryAfter is the longest wait a Retry-After header is trusted for. A daily quota resets
// within a day; anything further away is a broken clock or a header that means something else.
const maxRetryAfter = 24 * time.Hour

// retryAfterStatus reports whether a status code is one whose Retry-After header says when the
// upstream takes requests again: a rate limit, or a service that is over capacity for now.
func retryAfterStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, 529:
		return true
	}
	return false
}

// retryAfterResetAt reads from a failed response's headers when the upstream takes requests
// again, as unix seconds: retry-after-ms (milliseconds), or Retry-After as seconds or an HTTP
// date. It returns 0 when the headers say nothing usable. Part of a second counts as a whole one,
// so the time returned is never before the upstream's; a wait over maxRetryAfter is cut to it.
func retryAfterResetAt(header http.Header, now time.Time) int64 {
	wait := time.Duration(0)
	if ms := strings.TrimSpace(header.Get("retry-after-ms")); ms != "" {
		if v, err := strconv.ParseFloat(ms, 64); err == nil && v > 0 && !math.IsInf(v, 0) {
			wait = time.Duration(v * float64(time.Millisecond))
		}
	}
	if wait <= 0 {
		value := strings.TrimSpace(header.Get("Retry-After"))
		if value == "" {
			return 0
		}
		if v, err := strconv.ParseFloat(value, 64); err == nil {
			if v <= 0 || math.IsInf(v, 0) || math.IsNaN(v) {
				return 0
			}
			wait = time.Duration(math.Min(v, maxRetryAfter.Seconds()) * float64(time.Second))
		} else if at, err := http.ParseTime(value); err == nil {
			wait = at.Sub(now)
		}
	}
	if wait <= 0 {
		return 0
	}
	if wait > maxRetryAfter {
		wait = maxRetryAfter
	}
	seconds := int64(math.Ceil(wait.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return now.Unix() + seconds
}
