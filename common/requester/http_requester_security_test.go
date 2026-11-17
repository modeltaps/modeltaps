package requester

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/safefetch"
	"github.com/modeltaps/modeltaps/types"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// SEC-79: a PublicOnly requester refuses addresses that are not public, directly and through a
// proxy, and says so; without the flag the same request goes through.
func TestPublicOnlyRefusesPrivateAddress(t *testing.T) {
	if HTTPClient == nil {
		InitHttpClient()
	}
	srv := newTestServer(t)

	send := func(r *HTTPRequester) *types.OpenAIErrorWithStatusCode {
		req, err := r.NewRequest(http.MethodGet, srv.URL+"/v1/models")
		if err != nil {
			t.Fatal(err)
		}
		var out map[string]any
		_, errWithCode := r.SendRequest(req, &out, false)
		return errWithCode
	}

	open := NewHTTPRequester("", nil)
	if errWithCode := send(open); errWithCode != nil {
		t.Fatalf("plain requester: %v", errWithCode.Message)
	}

	guarded := NewHTTPRequester("", nil)
	guarded.PublicOnly = true
	errWithCode := send(guarded)
	if errWithCode == nil || errWithCode.Code != "upstream_address_not_public" || errWithCode.StatusCode != http.StatusForbidden {
		t.Fatalf("public-only requester to loopback: %+v", errWithCode)
	}

	// The proxy address is the administrator's and is not checked; the target is, before the
	// proxy is contacted (nothing listens on the proxy port).
	proxied := NewHTTPRequester("http://127.0.0.1:1", nil)
	proxied.PublicOnly = true
	if errWithCode := send(proxied); errWithCode == nil || errWithCode.Code != "upstream_address_not_public" {
		t.Fatalf("public-only requester through a proxy: %+v", errWithCode)
	}

	// The refusal comes from the address check: allowing loopback lets the request through.
	restore := safefetch.SetAllowAddrForTest(func(netip.AddrPort) bool { return true })
	defer restore()
	if errWithCode := send(guarded); errWithCode != nil {
		t.Fatalf("public-only requester with loopback allowed: %v", errWithCode.Message)
	}
}

// SEC-79: an error body in no known format is passed on as the message, but only its start, and
// only the first maxErrorBodyBytes are read.
func TestHandleErrorRespBoundsRawBody(t *testing.T) {
	body := `{"detail":"` + strings.Repeat("é", 3000) + `"}`
	resp := &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
	got := HandleErrorResp(resp, func(*http.Response) *types.OpenAIError { return nil }, false)
	msg := got.OpenAIError.Message
	if !strings.HasPrefix(msg, `{"detail":"éé`) || !strings.HasSuffix(msg, "...(truncated)") {
		t.Fatalf("message %q", msg)
	}
	if n := len(strings.TrimSuffix(msg, "...(truncated)")); n > maxRawErrorEchoBytes {
		t.Fatalf("echoed %d bytes, want at most %d", n, maxRawErrorEchoBytes)
	}
	if !strings.HasSuffix(strings.TrimSuffix(msg, "...(truncated)"), "é") {
		t.Fatalf("cut inside a character: %q", msg[len(msg)-20:])
	}

	short := `{"detail":"nope"}`
	resp = &http.Response{
		StatusCode: http.StatusNotFound,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(short)),
	}
	if got := HandleErrorResp(resp, func(*http.Response) *types.OpenAIError { return nil }, false); got.OpenAIError.Message != short {
		t.Fatalf("short body: %q", got.OpenAIError.Message)
	}

	var seen int
	huge := bytes.Repeat([]byte("a"), maxErrorBodyBytes+4096)
	resp = &http.Response{
		StatusCode: http.StatusBadGateway,
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader(huge)),
	}
	HandleErrorResp(resp, func(r *http.Response) *types.OpenAIError {
		b, _ := io.ReadAll(r.Body)
		seen = len(b)
		return nil
	}, false)
	if seen != maxErrorBodyBytes {
		t.Fatalf("error handler saw %d bytes, want %d", seen, maxErrorBodyBytes)
	}
}
