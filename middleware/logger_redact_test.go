package middleware

import "testing"

func TestRedactLogQuery(t *testing.T) {
	cases := map[string]string{
		"":                            "",
		"a=1&b=2":                     "a=1&b=2",
		"key=sk-secret&alt=sse":       "key=***&alt=sse",
		"key=AIzaSyExample":           "key=***",
		"alt=sse&API_KEY=abc":         "alt=sse&API_KEY=***",
		"access_token=t0k&p=1":        "access_token=***&p=1",
		"token=":                      "token=",
		"monkey=banana":               "monkey=banana",
		"x%2Dapi%2Dkey=secret&page=2": "x%2Dapi%2Dkey=***&page=2",
		"key=sk-a&key=sk-b":           "key=***&key=***",
	}
	for in, want := range cases {
		if got := redactLogQuery(in); got != want {
			t.Errorf("redactLogQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactLogPath(t *testing.T) {
	cases := map[string]string{
		"/v1/chat/completions":   "/v1/chat/completions",
		"/mcp":                   "/mcp",
		"/mcp/":                  "/mcp/",
		"/mcp/sse":               "/mcp/sse",
		"/mcp/message":           "/mcp/message",
		"/mcp/abcdef123":         "/mcp/***",
		"/mcp/sse/abcdef123":     "/mcp/sse/***",
		"/mcp/message/abcdef123": "/mcp/message/***",
		"/api/mcp/abc":           "/api/mcp/abc",
	}
	for in, want := range cases {
		if got := redactLogPath(in); got != want {
			t.Errorf("redactLogPath(%q) = %q, want %q", in, got, want)
		}
	}
}
