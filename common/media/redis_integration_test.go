//go:build integration

// Runs the Redis path against a real server: MEDIA_TEST_REDIS=redis://127.0.0.1:16399 go test
// -tags integration ./common/media/
package media

import (
	"bytes"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/redis"
	goredis "github.com/redis/go-redis/v9"
)

func TestRedisPath(t *testing.T) {
	url := os.Getenv("MEDIA_TEST_REDIS")
	if url == "" {
		t.Skip("MEDIA_TEST_REDIS not set")
	}
	dir := setup(t)
	opt, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	redis.RDB = goredis.NewClient(opt)
	config.RedisEnabled = true
	t.Cleanup(func() { config.RedisEnabled = false; redis.RDB = nil })

	data := testPNG(t)
	link, err := Put(data)
	if err != nil {
		t.Fatal(err)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Fatalf("wrote %d files to disk with Redis on", len(entries))
	}
	name := filepath.Base(link)
	for i := 0; i < MaxDownloads; i++ {
		if w := serve(name); w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), data) {
			t.Fatalf("download %d: %d", i+1, w.Code)
		}
	}
	if w := serve(name); w.Code != http.StatusGone {
		t.Fatalf("after limit: %d", w.Code)
	}
}

func TestRedisDeferredPath(t *testing.T) {
	url := os.Getenv("MEDIA_TEST_REDIS")
	if url == "" {
		t.Skip("MEDIA_TEST_REDIS not set")
	}
	dir := setup(t)
	opt, err := goredis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	redis.RDB = goredis.NewClient(opt)
	config.RedisEnabled = true
	t.Cleanup(func() { config.RedisEnabled = false; redis.RDB = nil })

	video := bytes.Repeat([]byte("v"), 2048)
	srv, hits := fakeUpstream(t, video, "video/mp4")
	link, err := Defer(srv.URL+"/a.mp4", 0)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := Defer(srv.URL+"/a.mp4", 0); again != link {
		t.Fatal("polling again gave a new link")
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.Base(link)+metaSuffix)); err == nil {
		t.Fatal("meta written to disk with Redis on")
	}
	for i := 0; i < 2; i++ {
		if w := serve(filepath.Base(link)); w.Code != http.StatusOK || !bytes.Equal(w.Body.Bytes(), video) {
			t.Fatalf("download %d: %d", i+1, w.Code)
		}
	}
	if *hits != 1 {
		t.Fatalf("upstream fetched %d times", *hits)
	}
}
