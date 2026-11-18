package media

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/safefetch"
)

func fakeUpstream(t *testing.T, body []byte, ctype string) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", ctype)
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	restore := safefetch.SetAllowAddrForTest(func(netip.AddrPort) bool { return true })
	t.Cleanup(restore)
	return srv, &hits
}

func TestDeferFetchesOnceAndServesTheCopy(t *testing.T) {
	dir := setup(t)
	video := bytes.Repeat([]byte("v"), 4096)
	srv, hits := fakeUpstream(t, video, "video/mp4")

	link, err := Defer(srv.URL+"/out/video.mp4?sig=abc", 7)
	if err != nil {
		t.Fatal(err)
	}
	again, _ := Defer(srv.URL+"/out/video.mp4?sig=abc", 7)
	if again != link {
		t.Fatalf("polling again gave a new link: %s vs %s", again, link)
	}
	name := filepath.Base(link)
	if filepath.Ext(name) != ".mp4" {
		t.Fatalf("name %s", name)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, name+metaSuffix)); bytes.Contains(b, []byte("proxy")) {
		t.Fatal("the meta must not hold a proxy")
	}
	for i := 0; i < 2; i++ {
		w := serve(name)
		if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), video) || w.Header().Get("Content-Type") != "video/mp4" {
			t.Fatalf("download %d: %d %q", i+1, w.Code, w.Header().Get("Content-Type"))
		}
	}
	if n := atomic.LoadInt32(hits); n != 1 {
		t.Fatalf("upstream fetched %d times, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(dir, name+cacheSuffix)); err != nil {
		t.Fatalf("no disk copy: %v", err)
	}
	for i := 2; i < MaxDownloads; i++ {
		serve(name)
	}
	if w := serve(name); w.Code != http.StatusGone {
		t.Fatalf("over the limit: %d", w.Code)
	}
}

func TestDeferredLinkExpires(t *testing.T) {
	dir := setup(t)
	srv, _ := fakeUpstream(t, []byte("x"), "video/mp4")
	link, err := Defer(srv.URL+"/v.mp4", 0)
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(link)
	meta, _ := json.Marshal(remoteMeta{URL: srv.URL + "/v.mp4", Expires: time.Now().Add(-time.Second).Unix()})
	if err := os.WriteFile(filepath.Join(dir, name+metaSuffix), meta, 0o640); err != nil {
		t.Fatal(err)
	}
	if w := serve(name); w.Code != http.StatusNotFound {
		t.Fatalf("expired: %d", w.Code)
	}
	if again, _ := Defer(srv.URL+"/v.mp4", 0); again == link {
		t.Fatal("an expired link must be replaced")
	}
}

func TestDeferredLinkRefusesNonMedia(t *testing.T) {
	setup(t)
	srv, _ := fakeUpstream(t, []byte("<html></html>"), "text/html")
	link, err := Defer(srv.URL+"/v.mp4", 0)
	if err != nil {
		t.Fatal(err)
	}
	if w := serve(filepath.Base(link)); w.Code != http.StatusBadGateway {
		t.Fatalf("html upstream: %d", w.Code)
	}
	if _, err := Defer("file:///etc/passwd", 0); err == nil {
		t.Fatal("a non-http link must be refused")
	}
}
