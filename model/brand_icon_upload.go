package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common/brandicon"
	"github.com/modeltaps/modeltaps/common/safefetch"

	"gorm.io/gorm"
)

// 厂商图标取值：空串（按厂商名自动解析）、brand:{key}（内置图标）、upload:{assetId}（本站资源）。
const (
	VendorIconBrandPrefix  = "brand:"
	VendorIconUploadPrefix = "upload:"
)

var ErrInvalidVendorIcon = errors.New("icon must be empty, brand:{key} or upload:{assetId}")

// BrandIconUpload 是本站保存的自定义图标资源，按内容 sha256 寻址（相同内容只存一份）。
type BrandIconUpload struct {
	Id        string `json:"id" gorm:"primaryKey;type:varchar(64)"`
	MimeType  string `json:"mime_type" gorm:"type:varchar(64);not null"`
	Data      []byte `json:"-"`
	CreatedAt int64  `json:"created_at" gorm:"bigint"`
}

func (BrandIconUpload) TableName() string { return "brand_icon_upload" }

// SaveBrandIconUpload 校验文件头（SVG 另做清洗）后落库，返回资源 id。
func SaveBrandIconUpload(data []byte) (string, error) {
	return saveBrandIconUpload(DB, data)
}

func saveBrandIconUpload(db *gorm.DB, data []byte) (string, error) {
	if int64(len(data)) > safefetch.DefaultMaxBytes {
		return "", safefetch.ErrTooLarge
	}
	mime := safefetch.SniffImage(data)
	if mime == "" {
		return "", errors.New("unsupported image type")
	}
	if mime == safefetch.MimeSVG {
		clean, err := safefetch.SanitizeSVG(data)
		if err != nil {
			return "", err
		}
		data = clean
	}
	sum := sha256.Sum256(data)
	id := hex.EncodeToString(sum[:])
	var count int64
	if err := db.Model(&BrandIconUpload{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return "", err
	}
	if count == 0 {
		row := &BrandIconUpload{Id: id, MimeType: mime, Data: data, CreatedAt: time.Now().Unix()}
		if err := db.Create(row).Error; err != nil {
			return "", err
		}
	}
	return id, nil
}

// GetBrandIconUpload 按资源 id 读取图标。
func GetBrandIconUpload(id string) (*BrandIconUpload, error) {
	var row BrandIconUpload
	if err := DB.Where("id = ?", id).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// NormalizeVendorIcon 校验厂商图标取值并返回规范形式；外链与未知 key / 资源一律拒绝。
func NormalizeVendorIcon(icon string) (string, error) {
	icon = strings.TrimSpace(icon)
	switch {
	case icon == "":
		return "", nil
	case strings.HasPrefix(icon, VendorIconBrandPrefix):
		key, ok := brandicon.Resolve(strings.TrimPrefix(icon, VendorIconBrandPrefix))
		if !ok {
			return "", ErrInvalidVendorIcon
		}
		return VendorIconBrandPrefix + key, nil
	case strings.HasPrefix(icon, VendorIconUploadPrefix):
		id := strings.TrimPrefix(icon, VendorIconUploadPrefix)
		var count int64
		if id == "" || DB.Model(&BrandIconUpload{}).Where("id = ?", id).Count(&count).Error != nil || count == 0 {
			return "", ErrInvalidVendorIcon
		}
		return icon, nil
	}
	return "", ErrInvalidVendorIcon
}

// legacyDefaultModelIcon 是旧版 DefaultModelIcon 的取值，迁移时置空（交给自动解析）。
const legacyDefaultModelIcon = "https://registry.npmmirror.com/@lobehub/icons-static-svg/latest/files/icons/ai.svg"

// lobehubIconPath 匹配 npm 镜像 / CDN 上 @lobehub/icons-static-* 包内的图标路径，捕获图标名。
var lobehubIconPath = regexp.MustCompile(`/@lobehub/icons-static-(?:svg|png|webp)/[^/]+/files/(?:icons|light|dark)/([a-z0-9-]+)\.(?:svg|png|webp)$`)

var vendorIconFetch = safefetch.Fetch

// convertLegacyVendorIcon 把旧的外链图标转成新取值：lobehub 图标 → brand:{key}，
// 旧默认图 → 空，其他 http(s) 外链下载一次存为 upload:{id}。
// 下载或校验失败时返回空串与错误，由调用方记日志。已是新取值的原样返回。
func convertLegacyVendorIcon(ctx context.Context, db *gorm.DB, icon string) (string, error) {
	icon = strings.TrimSpace(icon)
	if icon == "" || strings.HasPrefix(icon, VendorIconBrandPrefix) || strings.HasPrefix(icon, VendorIconUploadPrefix) {
		return icon, nil
	}
	if icon == legacyDefaultModelIcon {
		return "", nil
	}
	u, err := url.Parse(icon)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("unsupported icon value")
	}
	if m := lobehubIconPath.FindStringSubmatch(u.Path); m != nil {
		name := m[1]
		if strings.TrimSuffix(name, "-color") == "ai" {
			return "", nil
		}
		for _, candidate := range []string{strings.TrimSuffix(name, "-color"), name} {
			if key, ok := brandicon.Resolve(candidate); ok {
				return VendorIconBrandPrefix + key, nil
			}
		}
	}
	res, err := vendorIconFetch(ctx, icon, safefetch.Options{Accept: "image/*"})
	if err != nil {
		return "", err
	}
	id, err := saveBrandIconUpload(db, res.Body)
	if err != nil {
		return "", err
	}
	return VendorIconUploadPrefix + id, nil
}
