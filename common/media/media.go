// Package media keeps generated media (images an upstream returned as bytes) for a short time
// and serves it from the gateway itself, so a request that asks for a URL gets one on the site's
// own address without any third-party storage. A link lives for TTL, may be fetched MaxDownloads
// times, and each download is rate limited, so links cannot be passed around as a free host.
//
// With Redis the files live there (every node can serve every link); without it they are written
// to the data directory of the node that made them, which suits a single node.
package media

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/spf13/viper"
)

const (
	// TTL is how long a link works. Clients fetch generated media right away; an hour leaves
	// room for retries and slow consumers.
	TTL = time.Hour
	// MaxDownloads is how many times one link can be fetched: enough for retries and a
	// preview, too few to publish it.
	MaxDownloads = 5
	// MaxSize is the largest file kept (an image is a few MB at most).
	MaxSize = 20 << 20
	// BytesPerSecond is the speed of one download.
	BytesPerSecond = 2 << 20
	// MaxConcurrent is how many downloads one node serves at once; the rest get 429.
	MaxConcurrent = 16

	cleanupEvery = 10 * time.Minute
	redisPrefix  = "media:f:"
	redisCount   = "media:n:"
	routePrefix  = "/media/"
)

var (
	ErrNoAddress = errors.New("the server address is not set, so media links cannot be built")
	ErrTooLarge  = errors.New("the file is larger than the gateway keeps")
	ErrType      = errors.New("unsupported media type")
	ErrNotFound  = errors.New("not found or expired")
	ErrExhausted = errors.New("download limit reached")
)

// Allowed types and the extension their links carry.
var extByType = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

var typeByExt = func() map[string]string {
	m := map[string]string{}
	for t, e := range extByType {
		m[e] = t
	}
	return m
}()

var namePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{32}\.[a-z]{3,4}$`)

// Dir is where files go without Redis.
func Dir() string {
	if d := strings.TrimSpace(viper.GetString("media_dir")); d != "" {
		return d
	}
	return filepath.Join("data", "media")
}

func useRedis() bool { return config.RedisEnabled && redis.RDB != nil }

func newName(ext string) (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b) + ext, nil
}

// LinkFor returns the public link of a stored file.
func LinkFor(name string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(config.ServerAddress), "/")
	if base == "" {
		return "", ErrNoAddress
	}
	return base + routePrefix + name, nil
}

// Put keeps data for TTL and returns its link.
func Put(data []byte) (string, error) {
	if len(data) > MaxSize {
		return "", ErrTooLarge
	}
	ext, ok := extByType[http.DetectContentType(data)]
	if !ok {
		return "", ErrType
	}
	if strings.TrimSpace(config.ServerAddress) == "" {
		return "", ErrNoAddress
	}
	name, err := newName(ext)
	if err != nil {
		return "", err
	}
	if useRedis() {
		if err := redis.RDB.Set(context.Background(), redisPrefix+name, data, TTL).Err(); err != nil {
			return "", fmt.Errorf("saving media to Redis: %w", err)
		}
	} else {
		if err := os.MkdirAll(Dir(), 0o750); err != nil {
			return "", fmt.Errorf("creating the media directory: %w", err)
		}
		if err := os.WriteFile(filepath.Join(Dir(), name), data, 0o640); err != nil {
			return "", fmt.Errorf("writing media: %w", err)
		}
	}
	return LinkFor(name)
}

// local download counts; they reset on restart, which only gives a link a few more downloads.
var (
	countMu sync.Mutex
	counts  = map[string]int{}
)

// take counts one download of name and reports whether it is still allowed.
func take(name string) (bool, error) {
	if useRedis() {
		ctx := context.Background()
		n, err := redis.RDB.Incr(ctx, redisCount+name).Result()
		if err != nil {
			return false, err
		}
		if n == 1 {
			redis.RDB.Expire(ctx, redisCount+name, TTL)
		}
		return n <= MaxDownloads, nil
	}
	countMu.Lock()
	defer countMu.Unlock()
	counts[name]++
	return counts[name] <= MaxDownloads, nil
}

// Get returns a stored file, its type and how long it still lives, counting one download.
func Get(name string) ([]byte, string, time.Duration, error) {
	if !namePattern.MatchString(name) {
		return nil, "", 0, ErrNotFound
	}
	ctype := typeByExt[filepath.Ext(name)]
	if ctype == "" {
		return nil, "", 0, ErrNotFound
	}
	var data []byte
	var left time.Duration
	if useRedis() {
		ctx := context.Background()
		b, err := redis.RDB.Get(ctx, redisPrefix+name).Bytes()
		if err != nil {
			return nil, "", 0, ErrNotFound
		}
		data = b
		left, _ = redis.RDB.TTL(ctx, redisPrefix+name).Result()
	} else {
		path := filepath.Join(Dir(), name)
		info, err := os.Stat(path)
		if err != nil {
			return nil, "", 0, ErrNotFound
		}
		left = time.Until(info.ModTime().Add(TTL))
		if left <= 0 {
			return nil, "", 0, ErrNotFound
		}
		if data, err = os.ReadFile(path); err != nil {
			return nil, "", 0, ErrNotFound
		}
	}
	ok, err := take(name)
	if err != nil {
		return nil, "", 0, err
	}
	if !ok {
		return nil, "", 0, ErrExhausted
	}
	return data, ctype, left, nil
}

// Cleanup removes files past TTL from the media directory (Redis expires its own).
func Cleanup(now time.Time) int {
	entries, err := os.ReadDir(Dir())
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || e.IsDir() || now.Sub(info.ModTime()) < TTL {
			continue
		}
		if os.Remove(filepath.Join(Dir(), e.Name())) == nil {
			removed++
		}
	}
	countMu.Lock()
	for name := range counts {
		if _, err := os.Stat(filepath.Join(Dir(), name)); err != nil {
			delete(counts, name)
		}
	}
	countMu.Unlock()
	return removed
}

// StartCleanup removes expired files every ten minutes.
func StartCleanup() {
	go func() {
		for {
			if n := Cleanup(time.Now()); n > 0 {
				logger.SysLog(fmt.Sprintf("media: removed %d expired files", n))
			}
			time.Sleep(cleanupEvery)
		}
	}()
}
