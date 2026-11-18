package media

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/spf13/viper"
)

func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func setup(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	viper.Set("media_dir", dir)
	saved := config.ServerAddress
	config.ServerAddress = "https://gw.example.com/"
	t.Cleanup(func() { viper.Set("media_dir", nil); config.ServerAddress = saved; counts = map[string]int{} })
	return dir
}

func serve(name string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/media/:name", Serve)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/media/"+name, nil))
	return w
}

func TestPutAndServeOnDisk(t *testing.T) {
	setup(t)
	data := testPNG(t)
	link, err := Put(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(link, "https://gw.example.com/media/") || !strings.HasSuffix(link, ".png") {
		t.Fatalf("link = %s", link)
	}
	name := strings.TrimPrefix(link, "https://gw.example.com/media/")
	for i := 0; i < MaxDownloads; i++ {
		w := serve(name)
		if w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), data) || w.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("download %d: %d %s", i+1, w.Code, w.Header().Get("Content-Type"))
		}
	}
	if w := serve(name); w.Code != http.StatusGone {
		t.Fatalf("after %d downloads: %d, want 410", MaxDownloads, w.Code)
	}
}

func TestRejectsAndNotFound(t *testing.T) {
	setup(t)
	if _, err := Put([]byte("<html>not an image</html>")); err != ErrType {
		t.Fatalf("html: %v", err)
	}
	if _, err := Put(make([]byte, MaxSize+1)); err != ErrTooLarge {
		t.Fatalf("large: %v", err)
	}
	for _, name := range []string{"../../etc/passwd", "abc.png", strings.Repeat("a", 32) + ".exe"} {
		if w := serve(name); w.Code != http.StatusNotFound {
			t.Fatalf("%s: %d", name, w.Code)
		}
	}
	config.ServerAddress = ""
	if _, err := Put(testPNG(t)); err != ErrNoAddress {
		t.Fatalf("no address: %v", err)
	}
}

func TestExpiredIsGoneAndCleanedUp(t *testing.T) {
	dir := setup(t)
	link, err := Put(testPNG(t))
	if err != nil {
		t.Fatal(err)
	}
	name := filepath.Base(link)
	old := time.Now().Add(-TTL - time.Minute)
	if err := os.Chtimes(filepath.Join(dir, name), old, old); err != nil {
		t.Fatal(err)
	}
	if w := serve(name); w.Code != http.StatusNotFound {
		t.Fatalf("expired: %d, want 404", w.Code)
	}
	if n := Cleanup(time.Now()); n != 1 {
		t.Fatalf("removed %d, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
		t.Fatalf("file still there: %v", err)
	}
}

func TestDownloadIsThrottled(t *testing.T) {
	setup(t)
	// A file of one second's worth plus the burst takes about a second.
	data := append(testPNG(t), make([]byte, BytesPerSecond)...)
	link, err := Put(data)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if w := serve(filepath.Base(link)); w.Code != http.StatusOK || w.Body.Len() != len(data) {
		t.Fatalf("code %d len %d", w.Code, w.Body.Len())
	}
	if d := time.Since(start); d < 800*time.Millisecond {
		t.Fatalf("took %s, want about 1s", d)
	}
}
