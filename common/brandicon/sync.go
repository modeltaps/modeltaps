package brandicon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/safefetch"
)

const (
	PackageName     = "@lobehub/icons-static-svg"
	DefaultRegistry = "https://registry.npmmirror.com"

	currentFile      = "current"
	syncInitialDelay = time.Minute
	metaMaxBytes     = 1 << 20
	tarballMaxBytes  = 32 << 20
	svgMaxBytes      = 256 << 10
	metaTimeout      = 15 * time.Second
	tarballTimeout   = 2 * time.Minute
	keepSyncVersions = 2
)

var (
	ErrIntegrity   = errors.New("brandicon: tarball integrity mismatch")
	ErrBadRegistry = errors.New("brandicon: registry must be an absolute http(s) url")

	versionRe = regexp.MustCompile(`^[0-9]{1,6}\.[0-9]{1,6}\.[0-9]{1,6}$`)

	// syncFetch 便于测试替换同步层的底层抓取。
	syncFetch = safefetch.Fetch
	syncMu    sync.Mutex
	synced    atomic.Pointer[syncLayer]
)

// syncLayer 是后台同步下来的一版图标，只覆盖内嵌 manifest 已映射的文件。
type syncLayer struct {
	version string
	etag    string
	files   map[string][]byte
}

// SyncedVersion 返回当前生效的同步层版本，未启用同步层时为空。
func SyncedVersion() string {
	if l := synced.Load(); l != nil {
		return l.version
	}
	return ""
}

func setSyncLayer(version string, files map[string][]byte) {
	synced.Store(&syncLayer{
		version: version,
		etag:    fmt.Sprintf(`"%s+%s"`, strings.Trim(etag, `"`), version),
		files:   files,
	})
}

// NormalizeRegistry 校验并规范化 registry 地址，空值取默认 npmmirror。
func NormalizeRegistry(raw string) (string, error) {
	raw = strings.TrimRight(strings.TrimSpace(raw), "/")
	if raw == "" {
		return DefaultRegistry, nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", ErrBadRegistry
	}
	return raw, nil
}

// wantedFiles 是 manifest 已映射 key 对应的 SVG 文件名集合。
func wantedFiles() map[string]bool {
	out := make(map[string]bool, len(manifest.Icons)*2)
	for key, icon := range manifest.Icons {
		out[key+".svg"] = true
		if icon.Color {
			out[key+"-color.svg"] = true
		}
	}
	return out
}

// newerVersion 判断 a 是否严格新于 b（均为 x.y.z）。
func newerVersion(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	if len(pa) != 3 || len(pb) != 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		x, _ := strconv.Atoi(pa[i])
		y, _ := strconv.Atoi(pb[i])
		if x != y {
			return x > y
		}
	}
	return false
}

// activeVersion 是当前实际下发的图标库版本（同步层优先）。
func activeVersion() string {
	if v := SyncedVersion(); v != "" {
		return v
	}
	return manifest.Version
}

// LoadSynced 从 dir 读取上次同步成功的版本并启用；无同步层或版本不新于内嵌版本时保持内嵌层，
// 读取出错时返回错误，同样保持内嵌层。
func LoadSynced(dir string) error {
	raw, err := os.ReadFile(filepath.Join(dir, currentFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	version := strings.TrimSpace(string(raw))
	if !versionRe.MatchString(version) {
		return fmt.Errorf("brandicon: invalid synced version %q", version)
	}
	if !newerVersion(version, manifest.Version) {
		return nil
	}
	files := make(map[string][]byte)
	for name := range wantedFiles() {
		data, err := os.ReadFile(filepath.Join(dir, version, name))
		if err == nil && len(data) <= svgMaxBytes {
			files[name] = data
		}
	}
	if len(files) == 0 {
		return fmt.Errorf("brandicon: synced version %s has no icons", version)
	}
	setSyncLayer(version, files)
	return nil
}

type npmLatest struct {
	Version string `json:"version"`
	Dist    struct {
		Tarball   string `json:"tarball"`
		Integrity string `json:"integrity"`
	} `json:"dist"`
}

// Sync 查询 registry 上的最新版本，新于当前下发版本时下载 tarball、校验 sha512、
// 只解出 manifest 已映射的 SVG 写入 dir/{version}/ 并原子切换。任何一步失败都不影响现有版本。
// 返回 registry 上的最新版本号与是否发生了切换。
func Sync(ctx context.Context, registry, dir string) (string, bool, error) {
	syncMu.Lock()
	defer syncMu.Unlock()

	registry, err := NormalizeRegistry(registry)
	if err != nil {
		return "", false, err
	}
	res, err := syncFetch(ctx, registry+"/"+PackageName+"/latest", safefetch.Options{
		MaxBytes: metaMaxBytes, Timeout: metaTimeout, Accept: "application/json",
	})
	if err != nil {
		return "", false, fmt.Errorf("brandicon: fetch registry metadata: %w", err)
	}
	var meta npmLatest
	if err := json.Unmarshal(res.Body, &meta); err != nil {
		return "", false, fmt.Errorf("brandicon: parse registry metadata: %w", err)
	}
	if !versionRe.MatchString(meta.Version) {
		return "", false, fmt.Errorf("brandicon: unsupported version %q", meta.Version)
	}
	if !newerVersion(meta.Version, activeVersion()) {
		return meta.Version, false, nil
	}
	want, ok := strings.CutPrefix(meta.Dist.Integrity, "sha512-")
	expected, err := base64.StdEncoding.DecodeString(want)
	if !ok || err != nil || len(expected) != sha512.Size {
		return meta.Version, false, fmt.Errorf("brandicon: missing sha512 integrity for %s", meta.Version)
	}
	res, err = syncFetch(ctx, meta.Dist.Tarball, safefetch.Options{MaxBytes: tarballMaxBytes, Timeout: tarballTimeout})
	if err != nil {
		return meta.Version, false, fmt.Errorf("brandicon: download tarball: %w", err)
	}
	sum := sha512.Sum512(res.Body)
	if !bytes.Equal(sum[:], expected) {
		return meta.Version, false, ErrIntegrity
	}
	files, err := extractIcons(res.Body)
	if err != nil {
		return meta.Version, false, err
	}
	if err := writeVersion(dir, meta.Version, files); err != nil {
		return meta.Version, false, err
	}
	setSyncLayer(meta.Version, files)
	pruneVersions(dir, meta.Version)
	return meta.Version, true, nil
}

// extractIcons 只取 package/icons/ 下 manifest 已映射的普通文件并逐个清洗；
// 清洗失败的文件跳过，该文件继续回落内嵌层。
func extractIcons(tgz []byte) (map[string][]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(tgz))
	if err != nil {
		return nil, fmt.Errorf("brandicon: open tarball: %w", err)
	}
	defer gz.Close()
	want := wantedFiles()
	files := make(map[string][]byte)
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("brandicon: read tarball: %w", err)
		}
		name, ok := strings.CutPrefix(hdr.Name, "package/icons/")
		if !ok || hdr.Typeflag != tar.TypeReg || !want[name] || hdr.Size > svgMaxBytes {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, svgMaxBytes+1))
		if err != nil || len(data) > svgMaxBytes {
			continue
		}
		clean, err := safefetch.SanitizeSVG(data)
		if err != nil {
			continue
		}
		files[name] = clean
	}
	if len(files) == 0 {
		return nil, errors.New("brandicon: tarball contains no mapped icons")
	}
	return files, nil
}

// writeVersion 先写临时目录再改名为 dir/{version}，最后以临时文件改名的方式更新 current 指针。
func writeVersion(dir, version string, files map[string][]byte) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(dir, "."+version+"-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(tmp, name), data, 0o644); err != nil {
			return err
		}
	}
	final := filepath.Join(dir, version)
	if err := os.RemoveAll(final); err != nil {
		return err
	}
	if err := os.Rename(tmp, final); err != nil {
		return err
	}
	ptr, err := os.CreateTemp(dir, ".current-")
	if err != nil {
		return err
	}
	defer os.Remove(ptr.Name())
	if _, err := ptr.WriteString(version + "\n"); err != nil {
		ptr.Close()
		return err
	}
	if err := ptr.Close(); err != nil {
		return err
	}
	return os.Rename(ptr.Name(), filepath.Join(dir, currentFile))
}

// StartSyncLoop 同步加载磁盘上的同步层，再在后台按 interval 检查新版本（首轮延迟 syncInitialDelay）。
// enabled 与 registry 每轮重新读取，系统设置修改后下一轮生效；失败只记日志。
func StartSyncLoop(dir string, interval time.Duration, enabled func() bool, registry func() string) {
	if err := LoadSynced(dir); err != nil {
		logger.SysError("brand icon: load synced icons failed, using embedded icons: " + err.Error())
	} else if v := SyncedVersion(); v != "" {
		logger.SysLog("brand icon: using synced icons " + v)
	}
	go func() {
		timer := time.NewTimer(syncInitialDelay)
		for range timer.C {
			if enabled() {
				runSync(dir, registry())
			}
			timer.Reset(interval)
		}
	}()
}

func runSync(dir, registry string) {
	defer func() {
		if r := recover(); r != nil {
			logger.SysError(fmt.Sprintf("brand icon: sync panic: %v", r))
		}
	}()
	version, changed, err := Sync(context.Background(), registry, dir)
	switch {
	case err != nil:
		logger.SysError("brand icon: sync failed, keeping current icons: " + err.Error())
	case changed:
		logger.SysLog("brand icon: switched to " + PackageName + "@" + version)
	}
}

// pruneVersions 只保留最新的 keepSyncVersions 个版本目录（含当前版本）。
func pruneVersions(dir, current string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var others []string
	for _, e := range entries {
		if e.IsDir() && versionRe.MatchString(e.Name()) && e.Name() != current {
			others = append(others, e.Name())
		}
	}
	for len(others) > keepSyncVersions-1 {
		oldest := 0
		for i := range others {
			if newerVersion(others[oldest], others[i]) {
				oldest = i
			}
		}
		_ = os.RemoveAll(filepath.Join(dir, others[oldest]))
		others = append(others[:oldest], others[oldest+1:]...)
	}
}
