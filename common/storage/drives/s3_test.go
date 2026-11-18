package drives

import (
	"io"
	"net/http"
	"net/http/httptest"
	neturl "net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeS3 struct {
	mu       sync.Mutex
	existing bool
	puts     []*http.Request
	bodies   []string
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.Method {
	case http.MethodHead:
		if f.existing {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	case http.MethodPut:
		body, _ := io.ReadAll(r.Body)
		f.puts = append(f.puts, r)
		f.bodies = append(f.bodies, string(body))
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

// 新对象：路径风格寻址上传到 /<bucket>/<日期>/<key>，不带默认的 CRC 校验头，返回自定义域名 URL。
func TestS3UploadPutsNewObject(t *testing.T) {
	fake := &fakeS3{}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	up := NewS3Upload(srv.URL, "", "ak", "sk", "bucket", "https://cdn.example.com", 0)
	url, err := up.Upload([]byte("hello"), "a.png")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	datePrefix := time.Now().Format("2006-01-02") + "/"
	if want := "https://cdn.example.com/" + datePrefix + "a.png"; url != want {
		t.Fatalf("url = %q, want %q", url, want)
	}
	if len(fake.puts) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(fake.puts))
	}
	put := fake.puts[0]
	if want := "/bucket/" + datePrefix + "a.png"; put.URL.Path != want {
		t.Fatalf("path = %q, want %q", put.URL.Path, want)
	}
	if fake.bodies[0] != "hello" {
		t.Fatalf("body = %q", fake.bodies[0])
	}
	for name := range put.Header {
		if strings.HasPrefix(strings.ToLower(name), "x-amz-checksum-") {
			t.Fatalf("unexpected checksum header %s", name)
		}
	}
	if !strings.Contains(put.Header.Get("Authorization"), "Credential=ak/") {
		t.Fatalf("request not signed with the access key: %q", put.Header.Get("Authorization"))
	}
	// 未配置 region 且 endpoint 不是已知厂商：按 R2 的 auto 签名。
	if !strings.Contains(put.Header.Get("Authorization"), "/auto/s3/aws4_request") {
		t.Fatalf("authorization = %q, want the auto region", put.Header.Get("Authorization"))
	}
}

// 配置了 region 时签名范围用它：AWS S3、Backblaze B2 会拒绝 auto。
func TestS3UploadSignsWithConfiguredRegion(t *testing.T) {
	fake := &fakeS3{}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	up := NewS3Upload(srv.URL, " eu-central-1 ", "ak", "sk", "bucket", "https://cdn.example.com", 0)
	if _, err := up.Upload([]byte("hello"), "a.png"); err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if len(fake.puts) != 1 {
		t.Fatalf("expected 1 PUT, got %d", len(fake.puts))
	}
	if auth := fake.puts[0].Header.Get("Authorization"); !strings.Contains(auth, "/eu-central-1/s3/aws4_request") {
		t.Fatalf("authorization = %q", auth)
	}
}

func TestS3RegionFromEndpoint(t *testing.T) {
	cases := map[string]string{
		"https://s3.us-west-2.amazonaws.com":                  "us-west-2",
		"s3.eu-central-1.amazonaws.com":                       "eu-central-1",
		"https://s3.dualstack.ap-northeast-1.amazonaws.com":   "ap-northeast-1",
		"https://s3-ap-southeast-1.amazonaws.com":             "ap-southeast-1",
		"https://s3.amazonaws.com":                            "us-east-1",
		"https://s3.us-west-004.backblazeb2.com":              "us-west-004",
		"https://s3.eu-central-2.wasabisys.com":               "eu-central-2",
		"https://s3.wasabisys.com":                            "us-east-1",
		"https://0123456789abcdef.r2.cloudflarestorage.com":   "auto",
		"https://fsn1.your-objectstorage.com":                 "auto",
		"https://s3.us-west-2.amazonaws.com.evil.example.com": "auto",
		"": "auto",
	}
	for endpoint, want := range cases {
		if got := S3RegionFromEndpoint(endpoint); got != want {
			t.Errorf("S3RegionFromEndpoint(%q) = %q, want %q", endpoint, got, want)
		}
	}
}

// 已存在的对象直接返回 URL，不再上传；未配置 CDN 时按路径风格拼 <endpoint>/<bucket>/<key>。
func TestS3UploadSkipsExistingObject(t *testing.T) {
	fake := &fakeS3{existing: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	up := NewS3Upload(srv.URL, "", "ak", "sk", "bucket", "", 0)
	url, err := up.Upload([]byte("hello"), "a.png")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if want := srv.URL + "/bucket/" + time.Now().Format("2006-01-02") + "/a.png?"; !strings.HasPrefix(url, want) {
		t.Fatalf("url = %q, want it to start with %q", url, want)
	}
	if len(fake.puts) != 0 {
		t.Fatalf("expected no PUT for an existing object, got %d", len(fake.puts))
	}
}

func TestS3UploadLinkBase(t *testing.T) {
	cases := []struct{ endpoint, cdnurl, want string }{
		{"https://acct.r2.cloudflarestorage.com", "https://pub-x.r2.dev/", "https://pub-x.r2.dev"},
		{"https://s3.us-west-2.amazonaws.com/", "", "https://s3.us-west-2.amazonaws.com/bucket"},
		{"s3.us-west-004.backblazeb2.com", "", "https://s3.us-west-004.backblazeb2.com/bucket"},
	}
	for _, c := range cases {
		if got := NewS3Upload(c.endpoint, "", "ak", "sk", "bucket", c.cdnurl, 0).CustomDomain; got != c.want {
			t.Errorf("endpoint %q cdnurl %q: link base = %q, want %q", c.endpoint, c.cdnurl, got, c.want)
		}
	}
}

// Without a CDN URL the link is signed for an hour, so the bucket can stay private; with one it
// is a plain link under it.
func TestS3UploadSignsLinksWithoutCDN(t *testing.T) {
	fake := &fakeS3{}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	link, err := NewS3Upload(srv.URL, "us-east-1", "ak", "sk", "bucket", "", 0).Upload([]byte("x"), "a.png")
	if err != nil {
		t.Fatal(err)
	}
	u, err := neturl.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("X-Amz-Expires") != "3600" || q.Get("X-Amz-Signature") == "" || !strings.HasPrefix(u.Path, "/bucket/") {
		t.Fatalf("not a link signed for an hour: %s", link)
	}

	plain, err := NewS3Upload(srv.URL, "us-east-1", "ak", "sk", "bucket", "https://img.example.com/", 0).Upload([]byte("x"), "b.png")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "X-Amz-") || !strings.HasPrefix(plain, "https://img.example.com/") {
		t.Fatalf("a CDN link must be plain: %s", plain)
	}
}
