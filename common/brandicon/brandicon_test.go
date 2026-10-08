package brandicon

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/safefetch"
)

// stubFetch 用内存表替换真实抓取，记录请求顺序。
func stubFetch(t *testing.T, pages map[string]string) *[]string {
	t.Helper()
	var calls []string
	orig := fetchFunc
	fetchFunc = func(_ context.Context, raw string, _ safefetch.Options) (*safefetch.Result, error) {
		calls = append(calls, raw)
		body, ok := pages[raw]
		if !ok {
			return nil, errors.New("not found")
		}
		u, _ := url.Parse(raw)
		return &safefetch.Result{FinalURL: u, Body: []byte(body)}, nil
	}
	t.Cleanup(func() { fetchFunc = orig })
	return &calls
}

const pngHeader = "\x89PNG\r\n\x1a\n...."

func TestFetchFaviconPrefersHTMLLinks(t *testing.T) {
	stubFetch(t, map[string]string{
		"https://foo.com/": `<html><head><link rel="icon" href="/i.png"><link rel="apple-touch-icon" href="https://cdn.foo.com/t.svg">` +
			`<link rel="icon" href="javascript:alert(1)"></head><body><link rel="icon" href="/late.png"></body></html>`,
		"https://cdn.foo.com/t.svg": `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script><path d="M0 0"/></svg>`,
		"https://foo.com/i.png":     pngHeader,
	})
	data, mime, err := FetchFavicon(context.Background(), "api.foo.com")
	if err != nil || mime != safefetch.MimeSVG || strings.Contains(string(data), "script") {
		t.Fatalf("got mime=%q err=%v data=%s", mime, err, data)
	}
}

func TestFetchFaviconFallsBackAndRejectsNonImages(t *testing.T) {
	calls := stubFetch(t, map[string]string{
		"https://foo.com/":                     `<link rel="icon" href="/fake.png">`,
		"https://foo.com/fake.png":             "<html>not an image</html>",
		"https://foo.com/apple-touch-icon.png": "garbage",
		"https://foo.com/favicon.ico":          "\x00\x00\x01\x00ico",
	})
	_, mime, err := FetchFavicon(context.Background(), "foo.com")
	if err != nil || mime != safefetch.MimeICO {
		t.Fatalf("got mime=%q err=%v calls=%v", mime, err, *calls)
	}
	stubFetch(t, map[string]string{})
	if _, _, err := FetchFavicon(context.Background(), "foo.com"); !errors.Is(err, ErrFaviconNotFound) {
		t.Fatalf("want ErrFaviconNotFound, got %v", err)
	}
	if _, _, err := FetchFavicon(context.Background(), "127.0.0.1"); !errors.Is(err, ErrFaviconNotFound) {
		t.Fatalf("ip literal: want ErrFaviconNotFound, got %v", err)
	}
}

// 默认厂商表中带图标的渠道类型（model.GetDefaultModelOwnedBy，除 Rerank）。
var baselineChannelTypes = []int{
	1, 11, 14, 15, 16, 17, 18, 19, 20, 23, 25, 26, 27, 28, 29, 30, 31, 33, 34, 35, 36, 37, 38,
	39, 40, 41, 43, 44, 45, 46, 47, 51, 53, 56,
}

// 默认厂商 slug（model.defaultVendorSlugs）与目录种子 vendor_slug。
var baselineVendorSlugs = []string{
	"openai", "anthropic", "google", "x-ai", "deepseek", "qwen", "moonshotai", "z-ai",
	"meta-llama", "mistralai", "baidu", "cohere", "groq", "minimax", "baichuan", "tencent", "01-ai",
}

func TestManifestMetadata(t *testing.T) {
	m := GetManifest()
	if m.Version == "" || m.Source == "" {
		t.Fatalf("manifest 缺少 version/source: %q %q", m.Version, m.Source)
	}
	if len(ETag()) < 3 || ETag()[0] != '"' {
		t.Fatalf("ETag 格式不对: %q", ETag())
	}
}

func TestBaselineCoverage(t *testing.T) {
	m := GetManifest()
	for _, id := range baselineChannelTypes {
		key, ok := m.ChannelTypes[strconv.Itoa(id)]
		if !ok {
			t.Errorf("渠道类型 %d 未映射图标", id)
			continue
		}
		if _, err := SVG(key, ""); err != nil {
			t.Errorf("渠道类型 %d -> %s 无法取到 SVG: %v", id, key, err)
		}
	}
	for _, slug := range baselineVendorSlugs {
		if _, ok := Resolve(slug); !ok {
			t.Errorf("厂商 slug %q 无法解析", slug)
		}
	}
}

func TestMappingTargetsExist(t *testing.T) {
	m := GetManifest()
	for name, table := range map[string]map[string]string{
		"aliases": m.Aliases, "channelTypes": m.ChannelTypes,
		"modelPrefixes": m.ModelPrefixes, "domains": m.Domains,
	} {
		for from, to := range table {
			if _, ok := m.Icons[to]; !ok {
				t.Errorf("%s.%s -> %s 指向不存在的图标", name, from, to)
			}
		}
	}
}

// PaLM 2 系列模型名（text-bison-001 / chat-bison 等）按前缀映射到 google。
func TestBisonModelPrefixes(t *testing.T) {
	m := GetManifest()
	for _, prefix := range []string{"bison", "chat-bison", "text-bison", "code-bison", "codechat-bison"} {
		if got := m.ModelPrefixes[prefix]; got != "google" {
			t.Errorf("modelPrefixes[%q] = %q, want google", prefix, got)
		}
	}
}

func TestAssetsMatchManifest(t *testing.T) {
	m := GetManifest()
	total := 0
	for key, icon := range m.Icons {
		for variant, want := range map[string]bool{".svg": icon.Mono, "-color.svg": icon.Color} {
			data, err := assetsFS.ReadFile("assets/" + key + variant)
			if want != (err == nil) {
				t.Errorf("%s%s: manifest=%v 文件存在=%v", key, variant, want, err == nil)
			}
			total += len(data)
		}
	}
	entries, _ := fs.ReadDir(assetsFS, "assets")
	count := 0
	for _, icon := range m.Icons {
		count++
		if icon.Color {
			count++
		}
	}
	if len(entries) != count {
		t.Errorf("assets 目录有 %d 个文件，manifest 声明 %d 个", len(entries), count)
	}
	if total >= 1<<20 {
		t.Errorf("白名单 SVG 总大小 %d 字节，超过 1MB", total)
	}
}

func TestSVGVariants(t *testing.T) {
	color, err := SVG("claude", VariantColor)
	if err != nil {
		t.Fatal(err)
	}
	mono, err := SVG("claude", VariantMono)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(color, mono) || !bytes.HasPrefix(bytes.TrimSpace(mono), []byte("<svg")) {
		t.Fatal("claude 的彩色与单色版本应为不同的 SVG")
	}
	if def, _ := SVG("claude", ""); !bytes.Equal(def, color) {
		t.Fatal("未指定 variant 时应优先彩色")
	}
	// openai 无彩色版本，请求 color 回落单色。
	fallback, err := SVG("openai", VariantColor)
	if err != nil {
		t.Fatal(err)
	}
	if m, _ := SVG("openai", VariantMono); !bytes.Equal(fallback, m) {
		t.Fatal("无彩色版本时应回落单色")
	}
	if _, err := SVG("OpenAI", ""); err != nil {
		t.Fatalf("key 应大小写不敏感: %v", err)
	}
	if _, err := SVG("z-ai", ""); err != nil {
		t.Fatalf("别名应可解析: %v", err)
	}
	if _, err := SVG("no-such-brand", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知 key 应返回 ErrNotFound，实际 %v", err)
	}
	if _, err := SVG("openai", "rainbow"); !errors.Is(err, ErrBadVariant) {
		t.Fatalf("非法 variant 应返回 ErrBadVariant，实际 %v", err)
	}
	if _, err := SVG("../manifest", ""); !errors.Is(err, ErrNotFound) {
		t.Fatalf("路径穿越 key 应返回 ErrNotFound，实际 %v", err)
	}
}

func TestRegistrableDomain(t *testing.T) {
	ok := map[string]string{
		"api.foo.com": "foo.com", "FOO.com.": "foo.com", "a.b.foo.co.uk": "foo.co.uk", "x.github.io": "x.github.io",
	}
	for in, want := range ok {
		if got, valid := RegistrableDomain(in); !valid || got != want {
			t.Errorf("RegistrableDomain(%q) = %q,%v want %q", in, got, valid, want)
		}
	}
	for _, in := range []string{"", "localhost", "127.0.0.1", "[::1]", "com", "co.uk", "a.localhost", "svc.internal",
		"foo.local", "bad_host.com", "-a.com", "a..com", "foo.com/evil", "foo.com:80"} {
		if got, valid := RegistrableDomain(in); valid {
			t.Errorf("RegistrableDomain(%q) = %q, want invalid", in, got)
		}
	}
}

func TestDomainAndChannelTypeKey(t *testing.T) {
	if k, ok := DomainKey("api.deepseek.com"); !ok || k != "deepseek" {
		t.Errorf("DomainKey api.deepseek.com = %q,%v", k, ok)
	}
	if k, ok := DomainKey("dashscope.aliyuncs.com"); !ok || k != "qwen" {
		t.Errorf("DomainKey dashscope = %q,%v", k, ok)
	}
	if _, ok := DomainKey("api.unknown-vendor.example"); ok {
		t.Error("unknown domain should not match")
	}
	if k, ok := ChannelTypeKey(1); !ok || k != "openai" {
		t.Errorf("ChannelTypeKey(1) = %q,%v", k, ok)
	}
	if _, ok := ChannelTypeKey(8); ok {
		t.Error("custom channel type should not match")
	}
	for in, want := range map[string]string{"https://API.Foo.com:8443/v1": "api.foo.com", "foo.com/v1": "foo.com", "": ""} {
		if got := HostFromURL(in); got != want {
			t.Errorf("HostFromURL(%q) = %q want %q", in, got, want)
		}
	}
}

const syncedSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M1 1h22v22H1z"/></svg>`

// buildTarball 生成 npm 风格的 tgz；entries 的 value 为空串时写入软链接。
func buildTarball(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		hdr := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}
		if body == "" {
			hdr = &tar.Header{Name: name, Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink}
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func integrityOf(b []byte) string {
	sum := sha512.Sum512(b)
	return "sha512-" + base64.StdEncoding.EncodeToString(sum[:])
}

// stubRegistry 用内存 registry 替换同步抓取，返回被请求的 URL 列表。
func stubRegistry(t *testing.T, version, integrity string, tgz []byte) *[]string {
	t.Helper()
	var calls []string
	orig := syncFetch
	tarball := "https://registry.example/" + PackageName + "/-/icons-static-svg-" + version + ".tgz"
	meta := `{"version":"` + version + `","dist":{"tarball":"` + tarball + `","integrity":"` + integrity + `"}}`
	syncFetch = func(_ context.Context, raw string, _ safefetch.Options) (*safefetch.Result, error) {
		calls = append(calls, raw)
		u, _ := url.Parse(raw)
		switch raw {
		case "https://registry.example/" + PackageName + "/latest":
			return &safefetch.Result{FinalURL: u, Body: []byte(meta)}, nil
		case tarball:
			return &safefetch.Result{FinalURL: u, Body: tgz}, nil
		}
		return nil, errors.New("offline")
	}
	t.Cleanup(func() { syncFetch = orig; synced.Store(nil) })
	return &calls
}

func TestSyncSwitchesAtomicallyAndFallsBack(t *testing.T) {
	dir := t.TempDir()
	embeddedETag := ETag()
	embeddedClaude, _ := SVG("claude", VariantColor)
	tgz := buildTarball(t, map[string]string{
		"package/icons/openai.svg":     syncedSVG,
		"package/icons/not-mapped.svg": syncedSVG,
		"package/icons/claude.svg":     `<svg><script>alert(1)</script>`,
		"package/icons/deepseek.svg":   "",
		"package/README.md":            "readme",
	})
	calls := stubRegistry(t, "99.0.0", integrityOf(tgz), tgz)

	version, changed, err := Sync(context.Background(), "https://registry.example/", dir)
	if err != nil || !changed || version != "99.0.0" || SyncedVersion() != "99.0.0" {
		t.Fatalf("Sync = %q,%v,%v synced=%q", version, changed, err, SyncedVersion())
	}
	want, _ := safefetch.SanitizeSVG([]byte(syncedSVG))
	if got, _ := SVG("openai", ""); !bytes.Equal(got, want) {
		t.Fatalf("同步层应优先于内嵌层: %s", got)
	}
	if got, _ := SVG("claude", VariantColor); !bytes.Equal(got, embeddedClaude) {
		t.Fatal("同步层缺失的文件应回落内嵌层")
	}
	if ETag() == embeddedETag || !strings.Contains(ETag(), "99.0.0") {
		t.Fatalf("切换版本后 ETag 应变化: %s", ETag())
	}
	entries, _ := os.ReadDir(filepath.Join(dir, "99.0.0"))
	if len(entries) != 1 || entries[0].Name() != "openai.svg" {
		t.Fatalf("只应写入 manifest 已映射且清洗通过的文件: %v", entries)
	}
	if cur, _ := os.ReadFile(filepath.Join(dir, "current")); strings.TrimSpace(string(cur)) != "99.0.0" {
		t.Fatalf("current 指针 = %q", cur)
	}

	*calls = nil
	if _, changed, err := Sync(context.Background(), "https://registry.example", dir); err != nil || changed || len(*calls) != 1 {
		t.Fatalf("同版本不应重复下载: changed=%v err=%v calls=%v", changed, err, *calls)
	}

	// 重启：从磁盘恢复同步层。
	synced.Store(nil)
	if err := LoadSynced(dir); err != nil || SyncedVersion() != "99.0.0" {
		t.Fatalf("LoadSynced = %v, version %q", err, SyncedVersion())
	}
}

func TestSyncIntegrityMismatchKeepsOldVersion(t *testing.T) {
	dir := t.TempDir()
	good := buildTarball(t, map[string]string{"package/icons/openai.svg": syncedSVG})
	stubRegistry(t, "99.0.0", integrityOf(good), good)
	if _, _, err := Sync(context.Background(), "https://registry.example", dir); err != nil {
		t.Fatal(err)
	}
	bad := buildTarball(t, map[string]string{"package/icons/openai.svg": strings.Replace(syncedSVG, "22", "10", 1)})
	stubRegistry(t, "99.0.1", integrityOf(good), bad)
	synced.Store(nil)
	if err := LoadSynced(dir); err != nil {
		t.Fatal(err)
	}
	if _, changed, err := Sync(context.Background(), "https://registry.example", dir); !errors.Is(err, ErrIntegrity) || changed {
		t.Fatalf("校验失败应报 ErrIntegrity 且不切换: changed=%v err=%v", changed, err)
	}
	if SyncedVersion() != "99.0.0" {
		t.Fatalf("校验失败后应保留旧版本，实际 %q", SyncedVersion())
	}
	if _, err := os.Stat(filepath.Join(dir, "99.0.1")); !os.IsNotExist(err) {
		t.Fatal("校验失败不应落盘新版本目录")
	}
	stubRegistry(t, "99.0.2", "sha1-abc", good)
	if _, changed, err := Sync(context.Background(), "https://registry.example", dir); err == nil || changed {
		t.Fatal("缺少 sha512 integrity 时应拒绝")
	}
}

func TestSyncOfflineAndLoadFallbacks(t *testing.T) {
	stubRegistry(t, "99.0.0", "", nil)
	if _, changed, err := Sync(context.Background(), "https://offline.example", t.TempDir()); err == nil || changed {
		t.Fatal("断网时同步应返回错误")
	}
	if err := LoadSynced(filepath.Join(t.TempDir(), "missing")); err != nil || SyncedVersion() != "" {
		t.Fatalf("无同步目录时应静默使用内嵌层: %v", err)
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "current"), []byte("../etc\n"), 0o644)
	if err := LoadSynced(dir); err == nil || SyncedVersion() != "" {
		t.Fatal("非法 current 指针应报错并保持内嵌层")
	}
	_ = os.WriteFile(filepath.Join(dir, "current"), []byte(GetManifest().Version+"\n"), 0o644)
	if err := LoadSynced(dir); err != nil || SyncedVersion() != "" {
		t.Fatal("不新于内嵌版本的同步层应忽略")
	}
	if svg, err := SVG("openai", ""); err != nil || len(svg) == 0 {
		t.Fatal("内嵌图标应正常下发")
	}
}

func TestPruneVersionsAndRegistry(t *testing.T) {
	dir := t.TempDir()
	for _, v := range []string{"1.9.0", "1.10.0", "1.2.0", "2.0.0"} {
		_ = os.MkdirAll(filepath.Join(dir, v), 0o755)
	}
	pruneVersions(dir, "2.0.0")
	entries, _ := os.ReadDir(dir)
	var left []string
	for _, e := range entries {
		left = append(left, e.Name())
	}
	if strings.Join(left, ",") != "1.10.0,2.0.0" {
		t.Fatalf("应只保留当前与上一版本: %v", left)
	}
	for in, want := range map[string]string{"": DefaultRegistry, " https://r.example/npm/ ": "https://r.example/npm"} {
		if got, err := NormalizeRegistry(in); err != nil || got != want {
			t.Errorf("NormalizeRegistry(%q) = %q,%v", in, got, err)
		}
	}
	for _, in := range []string{"ftp://r.example", "registry.npmjs.org", "https://u:p@r.example", "https://r.example/?a=1"} {
		if _, err := NormalizeRegistry(in); err == nil {
			t.Errorf("NormalizeRegistry(%q) should fail", in)
		}
	}
}
