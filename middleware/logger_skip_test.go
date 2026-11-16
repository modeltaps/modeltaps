package middleware

import "testing"

func TestSkipAccessLog(t *testing.T) {
	cases := []struct {
		method, path string
		status       int
		want         bool
	}{
		{"GET", "/assets/index-abc.js", 200, true},
		{"GET", "/favicon.ico", 304, true},
		{"GET", "/assets/missing.js", 404, false},
		{"POST", "/api/system_info/log/query", 200, true},
		{"POST", "/api/system_info/log/query", 500, false},
		{"POST", "/v1/chat/completions", 200, false},
		{"GET", "/panel/admin/system-logs", 200, false},
	}
	for _, c := range cases {
		if got := skipAccessLog(c.method, c.path, c.status); got != c.want {
			t.Errorf("skipAccessLog(%s %s %d) = %v, want %v", c.method, c.path, c.status, got, c.want)
		}
	}
}
