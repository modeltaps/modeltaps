package requester

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "i/o timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

// Only a wait for the answer that ran out is an upstream timeout: failing to connect, in time or
// not, and every other failure keep the general code.
func TestTransportErrorCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"deadline", &url.Error{Op: "Post", URL: "https://up", Err: context.DeadlineExceeded}, CodeUpstreamTimeout},
		{"awaiting headers", &url.Error{Op: "Post", URL: "https://up", Err: timeoutErr{}}, CodeUpstreamTimeout},
		{"dial timeout", &url.Error{Op: "Post", URL: "https://up", Err: &net.OpError{Op: "dial", Net: "tcp", Err: timeoutErr{}}}, "http_request_failed"},
		{"tls handshake timeout", &url.Error{Op: "Post", URL: "https://up", Err: errors.New("net/http: TLS handshake timeout")}, "http_request_failed"},
		{"refused", &url.Error{Op: "Post", URL: "https://up", Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}}, "http_request_failed"},
		{"reset", &url.Error{Op: "Post", URL: "https://up", Err: errors.New("connection reset by peer")}, "http_request_failed"},
		{"caller left", &url.Error{Op: "Post", URL: "https://up", Err: context.Canceled}, "http_request_failed"},
	}
	for _, tc := range cases {
		if got := transportErrorCode(tc.err); got != tc.want {
			t.Errorf("%s: transportErrorCode = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A real upstream that accepts the request and stays silent past the response header timeout is
// reported as an upstream timeout; one that is not there at all is not.
func TestUnansweredRequestIsUpstreamTimeout(t *testing.T) {
	if HTTPClient == nil {
		InitHttpClient()
	}
	transport := HTTPClient.Transport.(*http.Transport)
	old := transport.ResponseHeaderTimeout
	transport.ResponseHeaderTimeout = 100 * time.Millisecond
	t.Cleanup(func() { transport.ResponseHeaderTimeout = old })

	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(func() { close(release); srv.Close() })

	r := NewHTTPRequester("", nil)
	req, err := r.NewRequest(http.MethodPost, srv.URL+"/v1/chat/completions")
	if err != nil {
		t.Fatal(err)
	}
	_, failure := r.SendRequestRaw(req)
	if !IsUpstreamTimeout(failure) {
		t.Fatalf("silent upstream: %+v, want an upstream timeout", failure)
	}

	gone := httptest.NewServer(http.NotFoundHandler())
	goneURL := gone.URL
	gone.Close()
	req, _ = r.NewRequest(http.MethodPost, goneURL+"/v1/chat/completions")
	if _, failure := r.SendRequestRaw(req); failure == nil || IsUpstreamTimeout(failure) {
		t.Fatalf("upstream that is not there: %+v, want a plain request failure", failure)
	}
}
