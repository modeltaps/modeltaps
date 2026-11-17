package base

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/modeltaps/modeltaps/model"
)

// SEC-80: a "*" or regex pass-through does not forward the address and identity headers a proxy
// in front of the gateway adds about the end user; an explicit {client_header:...} still does.
func TestHeaderOverridePassthroughSkipsProxyIdentityHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	c.Request.Header.Set("X-Custom", "yes")
	c.Request.Header.Set("X-Forwarded-For", "203.0.113.7")
	c.Request.Header.Set("X-Real-IP", "203.0.113.7")
	c.Request.Header.Set("CF-Connecting-IP", "203.0.113.7")
	c.Request.Header.Set("X-Forwarded-User", "alice")

	for _, override := range []string{`{"*":""}`, `{"re:^x-":""}`} {
		override := override
		p := &BaseProvider{Context: c, Channel: &model.Channel{HeaderOverride: &override}}
		headers := map[string]string{}
		p.applyHeaderOverride(headers)
		if headers["X-Custom"] != "yes" {
			t.Errorf("%s: X-Custom not passed through: %v", override, headers)
		}
		for _, name := range []string{"X-Forwarded-For", "X-Real-Ip", "Cf-Connecting-Ip", "X-Forwarded-User"} {
			if _, ok := headers[name]; ok {
				t.Errorf("%s: %s passed through", override, name)
			}
		}
	}

	explicit := `{"X-Client-Addr":"{client_header:X-Forwarded-For}"}`
	p := &BaseProvider{Context: c, Channel: &model.Channel{HeaderOverride: &explicit}}
	headers := map[string]string{}
	p.applyHeaderOverride(headers)
	if headers["X-Client-Addr"] != "203.0.113.7" {
		t.Errorf("explicit client_header rule: %v", headers)
	}
}
