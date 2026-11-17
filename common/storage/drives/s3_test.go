package drives

import (
	"io"
	"net/http"
	"net/http/httptest"
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

// 已存在的对象直接返回 URL，不再上传；未配置 CDN 时用 endpoint 拼 URL。
func TestS3UploadSkipsExistingObject(t *testing.T) {
	fake := &fakeS3{existing: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	up := NewS3Upload(srv.URL, "", "ak", "sk", "bucket", "", 0)
	url, err := up.Upload([]byte("hello"), "a.png")
	if err != nil {
		t.Fatalf("upload failed: %v", err)
	}
	if !strings.HasPrefix(url, srv.URL+"/") {
		t.Fatalf("url = %q, want endpoint prefix", url)
	}
	if len(fake.puts) != 0 {
		t.Fatalf("expected no PUT for an existing object, got %d", len(fake.puts))
	}
}
