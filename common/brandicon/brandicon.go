// Package brandicon 内嵌 lobehub 品牌图标白名单（由 scripts/sync-brand-icons 生成），
// 供本站接口下发，避免前端依赖外部 CDN。
package brandicon

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

//go:embed assets/*.svg
var assetsFS embed.FS

//go:embed manifest.json
var manifestBytes []byte

const (
	VariantColor = "color"
	VariantMono  = "mono"
)

type Icon struct {
	Mono  bool    `json:"mono"`
	Color bool    `json:"color"`
	Scale float64 `json:"scale"`
}

type Manifest struct {
	Version       string            `json:"version"`
	Source        string            `json:"source"`
	Icons         map[string]Icon   `json:"icons"`
	Aliases       map[string]string `json:"aliases"`
	ChannelTypes  map[string]string `json:"channelTypes"`
	ModelPrefixes map[string]string `json:"modelPrefixes"`
	Domains       map[string]string `json:"domains"`
}

var (
	manifest Manifest
	etag     string
)

func init() {
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		panic(fmt.Sprintf("brandicon: invalid manifest.json: %v", err))
	}
	sum := sha256.Sum256(manifestBytes)
	etag = fmt.Sprintf(`"%s-%s"`, manifest.Version, hex.EncodeToString(sum[:])[:12])
}

// ManifestJSON 返回内嵌的 manifest 原始字节。
func ManifestJSON() []byte { return manifestBytes }

// GetManifest 返回解析后的 manifest。
func GetManifest() Manifest { return manifest }

// ETag 返回基于图标库版本与 manifest 内容的强校验值（含引号），同步层切换版本后随之变化。
func ETag() string {
	if l := synced.Load(); l != nil {
		return l.etag
	}
	return etag
}

// Resolve 把 key 或别名解析成规范图标 key，未知时返回 false。
func Resolve(key string) (string, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	if _, ok := manifest.Icons[key]; ok {
		return key, true
	}
	if target, ok := manifest.Aliases[key]; ok {
		if _, ok := manifest.Icons[target]; ok {
			return target, true
		}
	}
	return "", false
}

// SVG 返回指定 key 的 SVG。variant 为空时优先彩色、无彩色则回落单色；
// 显式请求 color 但该图标无彩色版本时同样回落单色。同步层有该文件时优先于内嵌层。
func SVG(key, variant string) ([]byte, error) {
	canonical, ok := Resolve(key)
	if !ok {
		return nil, ErrNotFound
	}
	icon := manifest.Icons[canonical]
	file := canonical + ".svg"
	switch variant {
	case "", VariantColor:
		if icon.Color {
			file = canonical + "-color.svg"
		}
	case VariantMono:
	default:
		return nil, ErrBadVariant
	}
	if l := synced.Load(); l != nil {
		if data, ok := l.files[file]; ok {
			return data, nil
		}
	}
	data, err := assetsFS.ReadFile("assets/" + file)
	if err != nil {
		return nil, ErrNotFound
	}
	return data, nil
}

var (
	ErrNotFound   = errors.New("brand icon not found")
	ErrBadVariant = errors.New("variant must be color or mono")
)
