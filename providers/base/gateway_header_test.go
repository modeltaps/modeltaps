package base

import "testing"

// The gateway's own control headers stay with the gateway.
func TestGatewayHeadersAreNotForwarded(t *testing.T) {
	for _, name := range []string{"X-Modeltaps-Prompt-Cache", "x-modeltaps-route"} {
		if !IsProxyIdentityHeader(name) {
			t.Errorf("%s would be forwarded upstream", name)
		}
	}
}
