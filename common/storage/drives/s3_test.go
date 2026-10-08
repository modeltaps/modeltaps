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

	up := NewS3Upload(srv.URL, "ak", "sk", "bucket", "https://cdn.example.com", 0)
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
}

// 已存在的对象直接返回 URL，不再上传；未配置 CDN 时用 endpoint 拼 URL。
func TestS3UploadSkipsExistingObject(t *testing.T) {
	fake := &fakeS3{existing: true}
	srv := httptest.NewServer(fake)
	defer srv.Close()

	up := NewS3Upload(srv.URL, "ak", "sk", "bucket", "", 0)
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
