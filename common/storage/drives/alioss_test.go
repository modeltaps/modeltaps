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

// Pins what the Aliyun OSS driver sends and returns, so an SDK upgrade or swap can be checked
// without real credentials. Both the upload and the signed URL must use V4 signatures, which new
// accounts and buckets require. An IP endpoint makes the SDK use path-style addressing, which lets
// a local server stand in for OSS; it carries no region, so the test sets one.
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

	up := NewAliOSSUpload(srv.URL, "cn-beijing", "test-ak", "test-sk", "bucket")
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
	auth := put.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "OSS4-HMAC-SHA256 Credential=test-ak/") ||
		!strings.Contains(auth, "/cn-beijing/oss/aliyun_v4_request") || !strings.Contains(auth, "Signature=") {
		t.Fatalf("authorization = %q", auth)
	}
	if put.Header.Get("X-Oss-Content-Sha256") != "UNSIGNED-PAYLOAD" {
		t.Fatalf("V4 headers missing: %v", put.Header)
	}

	u, err := url.Parse(signed)
	if err != nil {
		t.Fatalf("signed URL %q: %v", signed, err)
	}
	if got := u.Scheme + "://" + u.Host + u.Path; got != srv.URL+"/bucket/dir/a.png" {
		t.Fatalf("signed URL base = %q", got)
	}
	q := u.Query()
	if q.Get("x-oss-signature-version") != "OSS4-HMAC-SHA256" ||
		!strings.HasPrefix(q.Get("x-oss-credential"), "test-ak/") ||
		!strings.HasSuffix(q.Get("x-oss-credential"), "/cn-beijing/oss/aliyun_v4_request") ||
		q.Get("x-oss-date") == "" || q.Get("x-oss-expires") != "3600" || q.Get("x-oss-signature") == "" {
		t.Fatalf("signed URL query = %v", q)
	}
	if q.Get("OSSAccessKeyId") != "" || q.Get("Signature") != "" {
		t.Fatalf("signed URL still carries V1 parameters: %v", q)
	}
	if strings.Contains(signed, "test-sk") {
		t.Fatal("the signed URL must not contain the secret")
	}
}

func TestAliOSSUploadWithoutRegionFailsBeforeSending(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL)
	}))
	defer srv.Close()

	up := NewAliOSSUpload(srv.URL, "", "test-ak", "test-sk", "bucket")
	if _, err := up.Upload([]byte("hello"), "a.png"); err == nil || !strings.Contains(err.Error(), "region") {
		t.Fatalf("err = %v", err)
	}
}

func TestAliOSSRegionFromEndpoint(t *testing.T) {
	cases := map[string]string{
		"oss-cn-beijing.aliyuncs.com":                  "cn-beijing",
		"https://oss-cn-hangzhou.aliyuncs.com":         "cn-hangzhou",
		"http://oss-ap-southeast-1.aliyuncs.com/":      "ap-southeast-1",
		"oss-cn-shanghai-internal.aliyuncs.com":        "cn-shanghai",
		"OSS-US-WEST-1.ALIYUNCS.COM":                   "us-west-1",
		"oss-cn-beijing.aliyuncs.com:443":              "cn-beijing",
		"oss-accelerate.aliyuncs.com":                  "",
		"oss-accelerate-overseas.aliyuncs.com":         "",
		"bucket.oss-cn-beijing.aliyuncs.com":           "",
		"img.example.com":                              "",
		"oss-cn-beijing.aliyuncs.com.evil.example.com": "",
		"": "",
	}
	for endpoint, want := range cases {
		if got := AliOSSRegionFromEndpoint(endpoint); got != want {
			t.Errorf("AliOSSRegionFromEndpoint(%q) = %q, want %q", endpoint, got, want)
		}
	}
	if got := NewAliOSSUpload("oss-cn-beijing.aliyuncs.com", " eu-central-1 ", "ak", "sk", "b").Region; got != "eu-central-1" {
		t.Errorf("explicit region = %q", got)
	}
}
