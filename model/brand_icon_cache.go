package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/brandicon"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
)

const (
	BrandIconStatusOK     = "ok"
	BrandIconStatusFailed = "failed"

	brandIconOKTTL       = 30 * 24 * time.Hour
	brandIconNegativeTTL = 7 * 24 * time.Hour
	brandIconConcurrency = 2
)

// BrandIconCache 是抓取层缓存：按可注册域名存站点图标，失败行作为负缓存。
type BrandIconCache struct {
	Domain    string `json:"domain" gorm:"primaryKey;type:varchar(253)"`
	Status    string `json:"status" gorm:"type:varchar(16);not null"`
	MimeType  string `json:"mime_type" gorm:"type:varchar(64)"`
	Data      []byte `json:"-"`
	Sha256    string `json:"sha256" gorm:"type:varchar(64)"`
	FetchedAt int64  `json:"fetched_at" gorm:"bigint"`
	ExpiresAt int64  `json:"expires_at" gorm:"bigint;index"`
}

func (BrandIconCache) TableName() string { return "brand_icon_cache" }

// GetBrandIconCache 读取域名缓存；只返回有图的行（过期仍返回，等待后台刷新）。
func GetBrandIconCache(domain string) (*BrandIconCache, error) {
	var row BrandIconCache
	err := DB.Where("domain = ? AND status = ?", domain, BrandIconStatusOK).First(&row).Error
	if err != nil {
		return nil, err
	}
	return &row, nil
}

var (
	brandIconFetcher  = brandicon.FetchFavicon
	brandIconInflight sync.Map
	brandIconSem      = make(chan struct{}, brandIconConcurrency)
	brandIconNow      = time.Now
)

// EnqueueBrandIconFetch 异步抓取域名图标；缓存未过期或同域名已在抓取时直接返回。
func EnqueueBrandIconFetch(domain string) {
	domain, ok := brandicon.RegistrableDomain(domain)
	if !ok || DB == nil || !config.BrandIconFaviconFetchEnabled {
		return
	}
	var existing BrandIconCache
	if err := DB.Where("domain = ?", domain).Limit(1).Find(&existing).Error; err != nil ||
		(existing.Domain != "" && existing.ExpiresAt > brandIconNow().Unix()) {
		return
	}
	if _, loaded := brandIconInflight.LoadOrStore(domain, struct{}{}); loaded {
		return
	}
	go func() {
		defer brandIconInflight.Delete(domain)
		defer func() {
			if r := recover(); r != nil {
				logger.SysError(fmt.Sprintf("brand icon fetch panic for %s: %v", domain, r))
			}
		}()
		brandIconSem <- struct{}{}
		defer func() { <-brandIconSem }()
		refreshBrandIcon(domain)
	}()
}

func refreshBrandIcon(domain string) {
	data, mime, err := brandIconFetcher(context.Background(), domain)
	now := brandIconNow()
	if err != nil {
		var prev BrandIconCache
		if DB.Where("domain = ?", domain).Limit(1).Find(&prev).Error == nil && prev.Status == BrandIconStatusOK {
			// 刷新失败不覆盖已有的图，只推迟下次重试。
			DB.Model(&prev).Update("expires_at", now.Add(brandIconNegativeTTL).Unix())
			return
		}
		saveBrandIconCache(&BrandIconCache{Domain: domain, Status: BrandIconStatusFailed,
			FetchedAt: now.Unix(), ExpiresAt: now.Add(brandIconNegativeTTL).Unix()})
		return
	}
	sum := sha256.Sum256(data)
	saveBrandIconCache(&BrandIconCache{Domain: domain, Status: BrandIconStatusOK, MimeType: mime, Data: data,
		Sha256: hex.EncodeToString(sum[:]), FetchedAt: now.Unix(), ExpiresAt: now.Add(brandIconOKTTL).Unix()})
}

func saveBrandIconCache(row *BrandIconCache) {
	if err := DB.Save(row).Error; err != nil {
		logger.SysError(fmt.Sprintf("save brand icon cache for %s: %v", row.Domain, err))
	}
}

// TriggerChannelBrandIconFetch 渠道保存后调用：base_url 未命中内置图标时按其域名抓取。
// 未填 base_url 且渠道类型有内置图标时走官方地址，无需抓取。
func TriggerChannelBrandIconFetch(channel *Channel) {
	if channel == nil {
		return
	}
	host := brandicon.HostFromURL(channel.GetBaseURL())
	if host == "" {
		return
	}
	if _, ok := brandicon.DomainKey(host); ok {
		return
	}
	EnqueueBrandIconFetch(host)
}

// TriggerOwnedByBrandIconFetch 厂商保存后调用：slug/名称未命中内置 key、且本身形如域名时抓取。
func TriggerOwnedByBrandIconFetch(owned *ModelOwnedBy) {
	if owned == nil {
		return
	}
	slug := strings.TrimSpace(string(owned.Slug))
	for _, key := range []string{slug, owned.Name} {
		if _, ok := brandicon.Resolve(key); ok {
			return
		}
	}
	for _, candidate := range []string{slug, owned.Name} {
		if _, ok := brandicon.RegistrableDomain(candidate); ok {
			if _, known := brandicon.DomainKey(candidate); !known {
				EnqueueBrandIconFetch(candidate)
			}
			return
		}
	}
}

var (
	ErrBrandIconRefreshBusy   = errors.New("brand icon refresh already in progress")
	ErrInvalidBrandIconDomain = errors.New("invalid domain")
	ErrBrandIconFetchDisabled = errors.New("favicon fetching is disabled")
)

// RefreshBrandIconNow 管理员手动刷新：忽略 TTL 同步重新抓取域名图标，返回规范化后的域名。
func RefreshBrandIconNow(domain string) (string, error) {
	domain, ok := brandicon.RegistrableDomain(domain)
	if !ok {
		return "", ErrInvalidBrandIconDomain
	}
	if !config.BrandIconFaviconFetchEnabled {
		return domain, ErrBrandIconFetchDisabled
	}
	if _, loaded := brandIconInflight.LoadOrStore(domain, struct{}{}); loaded {
		return domain, ErrBrandIconRefreshBusy
	}
	defer brandIconInflight.Delete(domain)
	brandIconSem <- struct{}{}
	defer func() { <-brandIconSem }()
	refreshBrandIcon(domain)
	return domain, nil
}
