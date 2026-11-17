package relay

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// TestNewDecisionContextUsesParentClientIP checks that the decision sub-call logs the client IP
// the main engine resolved with its trusted-proxy settings, not a forged forwarding header.
func TestNewDecisionContextUsesParentClientIP(t *testing.T) {
	policy := &model.RoutingPolicy{Name: "auto/t", DecisionModel: "router-llm"}

	cases := []struct {
		name    string
		trusted []string
		remote  string
		xff     string
		want    string
	}{
		{name: "untrusted peer forges XFF", trusted: nil, remote: "203.0.113.7:4321", xff: "10.9.9.9", want: "203.0.113.7"},
		{name: "trusted proxy forwards XFF", trusted: []string{"127.0.0.1"}, remote: "127.0.0.1:4321", xff: "198.51.100.2", want: "198.51.100.2"},
		{name: "ipv6 peer", trusted: nil, remote: "[2001:db8::1]:4321", xff: "10.9.9.9", want: "2001:db8::1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, engine := gin.CreateTestContext(httptest.NewRecorder())
			if err := engine.SetTrustedProxies(tc.trusted); err != nil {
				t.Fatal(err)
			}
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			c.Request.RemoteAddr = tc.remote
			c.Request.Header.Set("X-Forwarded-For", tc.xff)
			c.Request.Header.Set("X-Real-Ip", tc.xff)
			if got := c.ClientIP(); got != tc.want {
				t.Fatalf("parent ClientIP = %q, want %q", got, tc.want)
			}

			decisionCtx, err := newDecisionContext(c, context.Background(), policy)
			if err != nil {
				t.Fatal(err)
			}
			if got := decisionCtx.ClientIP(); got != tc.want {
				t.Fatalf("decision ClientIP = %q, want %q", got, tc.want)
			}
			if v := decisionCtx.Request.Header.Get("X-Forwarded-For"); v != "" {
				t.Fatalf("decision request should not carry X-Forwarded-For, got %q", v)
			}
		})
	}
}
