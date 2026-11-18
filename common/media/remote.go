package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/redis"
	"github.com/modeltaps/modeltaps/common/safefetch"
	"golang.org/x/time/rate"
)

// Media an upstream returns as a link (a finished video task) is taken over lazily: Defer hands
// out a gateway link at once, and the first download of it fetches the upstream file through the
// channel's proxy, streaming it to the client and to disk at the same time; later downloads on
// that node read the disk copy. The link lives for TTL from Defer, with the same download limit
// and speed as stored media. With Redis the link is known to every node and each node fetches its
// own copy; without Redis everything is on the one node's disk.

// MaxRemoteSize is the largest upstream file taken over (a video).
const MaxRemoteSize = 512 << 20

const (
	redisRemote = "media:r:"
	redisRef    = "media:u:"
	metaSuffix  = ".meta"
	cacheSuffix = ".cache"
	refSuffix   = ".ref"
)

var remoteExt = map[string]string{
	".mp4": ".mp4", ".webm": ".webm", ".mov": ".mov",
	".png": ".png", ".jpg": ".jpg", ".jpeg": ".jpg", ".webp": ".webp", ".gif": ".gif",
}

func init() {
	typeByExt[".mp4"] = "video/mp4"
	typeByExt[".webm"] = "video/webm"
	typeByExt[".mov"] = "video/quicktime"
}

// remoteMeta is kept for a deferred link. It names the channel, not its proxy, so proxy
// credentials are not written to Redis or disk.
type remoteMeta struct {
	URL       string `json:"url"`
	ChannelID int    `json:"channel_id,omitempty"`
	Expires   int64  `json:"expires"`
}

// ProxyForChannel returns the proxy a channel's upstream requests use ("" for none); set by the
// model layer.
var ProxyForChannel = func(int) string { return "" }

func refKey(rawURL string) string {
	sum := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(sum[:20])
}

// Defer returns a gateway link for an upstream file. Asking again for the same upstream link
// while its gateway link lives returns the same link.
func Defer(rawURL string, channelID int) (string, error) {
	u, err := safefetch.CheckURL(rawURL)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(serverAddress()) == "" {
		return "", ErrNoAddress
	}
	ref := refKey(rawURL)
	if name := lookupRef(ref); name != "" {
		return LinkFor(name)
	}
	ext := remoteExt[strings.ToLower(path.Ext(u.Path))]
	if ext == "" {
		ext = ".mp4"
	}
	name, err := newName(ext)
	if err != nil {
		return "", err
	}
	meta := remoteMeta{URL: rawURL, ChannelID: channelID, Expires: time.Now().Add(TTL).Unix()}
	if err := saveRemote(name, ref, meta); err != nil {
		return "", err
	}
	return LinkFor(name)
}

func lookupRef(ref string) string {
	if useRedis() {
		name, err := redis.RDB.Get(context.Background(), redisRef+ref).Result()
		if err != nil {
			return ""
		}
		return name
	}
	path := filepath.Join(Dir(), ref+refSuffix)
	info, err := os.Stat(path)
	if err != nil || time.Since(info.ModTime()) >= TTL {
		return ""
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	name := strings.TrimSpace(string(b))
	if _, ok := loadRemote(name); !ok {
		return ""
	}
	return name
}

func saveRemote(name, ref string, meta remoteMeta) error {
	b, _ := json.Marshal(meta)
	if useRedis() {
		ctx := context.Background()
		if err := redis.RDB.Set(ctx, redisRemote+name, b, TTL).Err(); err != nil {
			return fmt.Errorf("saving the media link to Redis: %w", err)
		}
		return redis.RDB.Set(ctx, redisRef+ref, name, TTL).Err()
	}
	if err := os.MkdirAll(Dir(), 0o750); err != nil {
		return fmt.Errorf("creating the media directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(Dir(), name+metaSuffix), b, 0o640); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(Dir(), ref+refSuffix), []byte(name), 0o640)
}

func loadRemote(name string) (remoteMeta, bool) {
	var b []byte
	if useRedis() {
		v, err := redis.RDB.Get(context.Background(), redisRemote+name).Bytes()
		if err != nil {
			return remoteMeta{}, false
		}
		b = v
	} else {
		v, err := os.ReadFile(filepath.Join(Dir(), name+metaSuffix))
		if err != nil {
			return remoteMeta{}, false
		}
		b = v
	}
	var meta remoteMeta
	if json.Unmarshal(b, &meta) != nil || time.Now().Unix() >= meta.Expires {
		return remoteMeta{}, false
	}
	return meta, true
}

// remoteTypeOK reports whether an upstream Content-Type may be passed on as media.
func remoteTypeOK(ct string) bool {
	if strings.TrimSpace(ct) == "" {
		return true
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return strings.HasPrefix(mt, "video/") || mt == "application/octet-stream" || typeByExt[extByType[mt]] != ""
}

// serveRemote answers a deferred link; it reports false when name is not one.
func serveRemote(c *gin.Context, name string) bool {
	meta, ok := loadRemote(name)
	if !ok {
		return false
	}
	allowed, err := take(name)
	if err != nil {
		c.Status(http.StatusServiceUnavailable)
		return true
	}
	if !allowed {
		c.Status(http.StatusGone)
		return true
	}
	ctype := typeByExt[filepath.Ext(name)]
	left := time.Until(time.Unix(meta.Expires, 0))
	setMediaHeaders(c, ctype, left)
	out := throttled(c)

	cache := filepath.Join(Dir(), name+cacheSuffix)
	if f, err := os.Open(cache); err == nil {
		defer f.Close()
		if info, err := f.Stat(); err == nil {
			c.Header("Content-Length", strconv.FormatInt(info.Size(), 10))
		}
		c.Status(http.StatusOK)
		_, _ = io.Copy(out, f)
		return true
	}

	client, err := safefetch.NewClient(safefetch.Options{Timeout: 15 * time.Minute, MaxRedirects: 3, Proxy: ProxyForChannel(meta.ChannelID)})
	if err != nil {
		c.Status(http.StatusBadGateway)
		return true
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, meta.URL, nil)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return true
	}
	resp, err := client.Do(req)
	if err != nil {
		c.Status(http.StatusBadGateway)
		return true
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !remoteTypeOK(resp.Header.Get("Content-Type")) || resp.ContentLength > MaxRemoteSize {
		c.Status(http.StatusBadGateway)
		return true
	}
	if resp.ContentLength > 0 {
		c.Header("Content-Length", strconv.FormatInt(resp.ContentLength, 10))
	}
	c.Status(http.StatusOK)

	part, err := os.CreateTemp(Dir(), name+".*.part")
	if err != nil {
		// No disk copy: still answer this download.
		_, _ = io.Copy(out, io.LimitReader(resp.Body, MaxRemoteSize))
		return true
	}
	n, copyErr := io.Copy(io.MultiWriter(part, out), io.LimitReader(resp.Body, MaxRemoteSize+1))
	closeErr := part.Close()
	complete := copyErr == nil && closeErr == nil && n <= MaxRemoteSize && (resp.ContentLength <= 0 || n == resp.ContentLength)
	if !complete || os.Rename(part.Name(), cache) != nil {
		_ = os.Remove(part.Name())
	}
	return true
}

func setMediaHeaders(c *gin.Context, ctype string, left time.Duration) {
	h := c.Writer.Header()
	h.Set("Content-Type", ctype)
	h.Set("Content-Disposition", "inline")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, max-age="+strconv.Itoa(int(left/time.Second)))
}

func throttled(c *gin.Context) io.Writer {
	return throttledWriter{ctx: c.Request.Context(), w: c.Writer, lim: rate.NewLimiter(rate.Limit(BytesPerSecond), 64<<10)}
}
