package drives

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// Pins what the Aliyun OSS driver sends and returns, so an SDK upgrade or swap (for example to
// V4 signing) can be checked without real credentials. An IP endpoint makes the SDK use
// path-style addressing, which lets a local server stand in for OSS.
func TestAliOSSUploadPutsObjectAndReturnsSignedURL(t *testing.T) {
	var (
		mu   sync.Mutex
		puts []*http.Request
		body string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodPut {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		puts = append(puts, r)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	up := NewAliOSSUpload(srv.URL, "test-ak", "test-sk", "bucket")
	signed, err := up.Upload([]byte("hello"), "dir/a.png")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}

	if len(puts) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(puts))
	}
	put := puts[0]
	if put.URL.Path != "/bucket/dir/a.png" {
		t.Fatalf("path = %q", put.URL.Path)
	}
	if body != "hello" {
		t.Fatalf("body = %q", body)
	}
	if auth := put.Header.Get("Authorization"); !strings.HasPrefix(auth, "OSS test-ak:") {
		t.Fatalf("authorization = %q", auth)
	}

	u, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("signed URL %q: %v", signed, err)
	}
	if got := u.Scheme + "://" + u.Host + u.Path; got != srv.URL+"/bucket/dir/a.png" {
		t.Fatalf("signed URL base = %q", got)
	}
	q := u.Query()
	if q.Get("OSSAccessKeyId") != "test-ak" || q.Get("Expires") == "" || q.Get("Signature") == "" {
		t.Fatalf("signed URL query = %v", q)
	}
	if strings.Contains(signed, "test-sk") {
		t.Fatal("the signed URL must not contain the secret")
	}
}
