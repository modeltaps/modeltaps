package requester

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/types"
)

func TestRetryAfterResetAt(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cases := []struct {
		name   string
		header map[string]string
		want   int64
	}{
		{"none", nil, 0},
		{"seconds", map[string]string{"Retry-After": "20"}, now.Unix() + 20},
		{"fraction rounds up", map[string]string{"Retry-After": "0.4"}, now.Unix() + 1},
		{"zero", map[string]string{"Retry-After": "0"}, 0},
		{"negative", map[string]string{"Retry-After": "-5"}, 0},
		{"garbage", map[string]string{"Retry-After": "soon"}, 0},
		{"http date", map[string]string{"Retry-After": now.Add(90 * time.Second).UTC().Format(http.TimeFormat)}, now.Unix() + 90},
		{"date in the past", map[string]string{"Retry-After": now.Add(-time.Minute).UTC().Format(http.TimeFormat)}, 0},
		{"milliseconds win", map[string]string{"retry-after-ms": "2500", "Retry-After": "60"}, now.Unix() + 3},
		{"bad milliseconds fall back", map[string]string{"retry-after-ms": "x", "Retry-After": "7"}, now.Unix() + 7},
		{"capped at a day", map[string]string{"Retry-After": "9999999"}, now.Unix() + int64(maxRetryAfter.Seconds())},
	}
	for _, tc := range cases {
		header := http.Header{}
		for k, v := range tc.header {
			header.Set(k, v)
		}
		if got := retryAfterResetAt(header, now); got != tc.want {
			t.Errorf("%s: retryAfterResetAt = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func errorResponse(status int, header map[string]string) *http.Response {
	resp := &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"slow down"}}`)),
	}
	for k, v := range header {
		resp.Header.Set(k, v)
	}
	return resp
}

// A rate limit or an overloaded upstream that sends Retry-After is paused until that time,
// whichever provider it is; other failures ignore the header.
func TestHandleErrorRespReadsRetryAfter(t *testing.T) {
	plain := func(*http.Response) *types.OpenAIError { return &types.OpenAIError{Message: "slow down"} }
	before := time.Now().Unix()

	for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable, 529} {
		got := HandleErrorResp(errorResponse(status, map[string]string{"Retry-After": "30"}), plain, false)
		if wait := got.OpenAIError.RateLimitResetAt - before; wait < 30 || wait > 32 {
			t.Errorf("status %d: reset in %ds, want about 30", status, wait)
		}
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusInternalServerError} {
		got := HandleErrorResp(errorResponse(status, map[string]string{"Retry-After": "30"}), plain, false)
		if got.OpenAIError.RateLimitResetAt != 0 {
			t.Errorf("status %d: reset time %d taken from Retry-After", status, got.OpenAIError.RateLimitResetAt)
		}
	}
	if got := HandleErrorResp(errorResponse(http.StatusTooManyRequests, nil), plain, false); got.OpenAIError.RateLimitResetAt != 0 {
		t.Errorf("no header: reset time %d", got.OpenAIError.RateLimitResetAt)
	}

	// A reset time the provider's own handler read from the body is more specific and is kept.
	own := func(*http.Response) *types.OpenAIError {
		return &types.OpenAIError{Message: "quota", RateLimitResetAt: before + 600}
	}
	got := HandleErrorResp(errorResponse(http.StatusTooManyRequests, map[string]string{"Retry-After": "30"}), own, false)
	if got.OpenAIError.RateLimitResetAt != before+600 {
		t.Errorf("provider reset time replaced: %d, want %d", got.OpenAIError.RateLimitResetAt, before+600)
	}
}
