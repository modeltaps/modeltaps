package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func setResponsesRequest(t *testing.T, body string) error {
	t.Helper()
	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return NewRelayResponses(c).setRequest()
}

// TestResponsesRejectsBackground: a background response comes back queued without usage and
// runs on the channel's account afterwards, which the gateway cannot bill (SEC-67).
func TestResponsesRejectsBackground(t *testing.T) {
	for _, background := range []string{`true`, `"true"`, `1`, `{}`} {
		err := setResponsesRequest(t, `{"model":"o3","input":"hi","background":`+background+`}`)
		if err != errBackgroundResponse {
			t.Errorf("background=%s: got %v, want errBackgroundResponse", background, err)
		}
	}
}

func TestResponsesAcceptsForegroundRequests(t *testing.T) {
	for _, body := range []string{
		`{"model":"o3","input":"hi"}`,
		`{"model":"o3","input":"hi","background":false}`,
		`{"model":"o3","input":"hi","background":null}`,
		// The image generation tool has its own background option (transparent / opaque).
		`{"model":"o3","input":"hi","tools":[{"type":"image_generation","background":"transparent"}]}`,
	} {
		if err := setResponsesRequest(t, body); err != nil {
			t.Errorf("%s: %v", body, err)
		}
	}
}

func TestResponsesCompactRejectsBackground(t *testing.T) {
	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses/compact", strings.NewReader(`{"model":"o3","input":"hi","background":true}`))
	c.Request.Header.Set("Content-Type", "application/json")
	if err := NewRelayResponsesCompact(c).setRequest(); err != errBackgroundResponse {
		t.Fatalf("got %v, want errBackgroundResponse", err)
	}
}
