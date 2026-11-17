package common

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// SEC-83: transport errors quote the request URL; a key in its query (Vertex AI Express ?key=,
// Baidu ?access_token=) must not reach the system log or the client.
func TestErrorWrapperRedactsURLSecrets(t *testing.T) {
	for _, method := range []string{"Post", "Get"} {
		err := &url.Error{Op: method, URL: "https://us-central1-aiplatform.googleapis.com/v1/m:generateContent?alt=sse&key=AIzaSecret123", Err: errors.New("i/o timeout")}
		got := ErrorWrapper(err, "http_request_failed", http.StatusInternalServerError)
		if strings.Contains(got.Message, "AIzaSecret123") {
			t.Fatalf("%s: message %q carries the key", method, got.Message)
		}
	}
}

func TestRedactURLSecrets(t *testing.T) {
	cases := map[string]string{
		`Post "https://h/x?key=AIza123": EOF`:                       `Post "https://h/x?key=***": EOF`,
		`Get "https://h/x?alt=sse&KEY=abc&page=2": EOF`:             `Get "https://h/x?alt=sse&KEY=***&page=2": EOF`,
		`Post "https://h/chat/m?access_token=24.abc": EOF`:          `Post "https://h/chat/m?access_token=***": EOF`,
		`Get "https://acct.blob/x?sv=1&sig=ZZ%2F": EOF`:             `Get "https://acct.blob/x?sv=1&sig=***": EOF`,
		`parse "http://user:p4ss@proxy:8080": invalid`:              `parse "http://user:***@proxy:8080": invalid`,
		`Post "https://api.example.com:8443/v1?api-version=1": EOF`: `Post "https://api.example.com:8443/v1?api-version=1": EOF`,
		`no url here, monkey=1`:                                     `no url here, monkey=1`,
	}
	for in, want := range cases {
		if got := RedactURLSecrets(in); got != want {
			t.Errorf("RedactURLSecrets(%q) = %q, want %q", in, got, want)
		}
	}
}
