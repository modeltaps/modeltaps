package controller

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common/brandicon"
	"github.com/modeltaps/modeltaps/common/safefetch"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// setBrandIconCacheHeaders 覆盖 /api 默认的 NoCache，按图标库版本做强缓存；
// If-None-Match 命中时返回 true，调用方直接结束。
func setBrandIconCacheHeaders(c *gin.Context) bool {
	etag := brandicon.ETag()
	c.Header("Cache-Control", "public, max-age=86400")
	c.Writer.Header().Del("Pragma")
	c.Writer.Header().Del("Expires")
	c.Header("ETag", etag)
	c.Header("X-Content-Type-Options", "nosniff")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return true
	}
	return false
}

// GetBrandIconManifest GET /api/brand-icon/manifest
func GetBrandIconManifest(c *gin.Context) {
	if setBrandIconCacheHeaders(c) {
		return
	}
	c.Data(http.StatusOK, "application/json; charset=utf-8", brandicon.ManifestJSON())
}

// GetBrandIcon GET /api/brand-icon/:key?variant=color|mono
func GetBrandIcon(c *gin.Context) {
	data, err := brandicon.SVG(c.Param("key"), c.Query("variant"))
	if err != nil {
		status := http.StatusNotFound
		if errors.Is(err, brandicon.ErrBadVariant) {
			status = http.StatusBadRequest
		}
		c.JSON(status, gin.H{"success": false, "message": err.Error()})
		return
	}
	if setBrandIconCacheHeaders(c) {
		return
	}
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	c.Data(http.StatusOK, "image/svg+xml", data)
}

// GetBrandIconByDomain GET /api/brand-icon/domain/:domain
// 只读抓取层缓存，不触发抓取；子域名按可注册域名归一，未命中返回 404。
func GetBrandIconByDomain(c *gin.Context) {
	domain, ok := brandicon.RegistrableDomain(c.Param("domain"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid domain"})
		return
	}
	row, err := model.GetBrandIconCache(domain)
	if err != nil || len(row.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": brandicon.ErrNotFound.Error()})
		return
	}
	etag := `"` + row.Sha256 + `"`
	c.Header("Cache-Control", "public, max-age=86400")
	c.Writer.Header().Del("Pragma")
	c.Writer.Header().Del("Expires")
	c.Header("ETag", etag)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, row.MimeType, row.Data)
}

// GetBrandIconUpload GET /api/brand-icon/upload/:assetId
// 公开只读：返回管理员设置的厂商自定义图标（按内容 sha256 寻址，内容不可变）。
func GetBrandIconUpload(c *gin.Context) {
	row, err := model.GetBrandIconUpload(c.Param("assetId"))
	if err != nil || len(row.Data) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "message": brandicon.ErrNotFound.Error()})
		return
	}
	etag := `"` + row.Id + `"`
	c.Header("Cache-Control", "public, max-age=86400")
	c.Writer.Header().Del("Pragma")
	c.Writer.Header().Del("Expires")
	c.Header("ETag", etag)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, row.MimeType, row.Data)
}

// BrandIconSync 执行图标库同步层（由同步模块在启动时注入）；未注入时刷新接口只支持按域名刷新。
var BrandIconSync func(ctx context.Context) error

// 上传请求体上限：图标本身不超过 safefetch.DefaultMaxBytes，另留 multipart 头部余量。
const brandIconUploadBodyLimit = safefetch.DefaultMaxBytes + 16<<10

// UploadBrandIcon POST /api/brand-icon/upload（管理员，multipart 字段 file）
// 按文件头校验类型、限制大小、SVG 清洗后落库，返回可写入厂商 icon 的 upload:{assetId}。
func UploadBrandIcon(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, brandIconUploadBodyLimit)
	file, err := c.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": safefetch.ErrTooLarge.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "file is required"})
		return
	}
	if file.Size > safefetch.DefaultMaxBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"success": false, "message": safefetch.ErrTooLarge.Error()})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, safefetch.DefaultMaxBytes+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	id, err := model.SaveBrandIconUpload(data)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, safefetch.ErrTooLarge) {
			status = http.StatusRequestEntityTooLarge
		}
		c.JSON(status, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
		"id":   id,
		"icon": model.VendorIconUploadPrefix + id,
	}})
}

type brandIconRefreshRequest struct {
	Domain string `json:"domain"`
}

// RefreshBrandIcon POST /api/brand-icon/refresh（管理员）
// 带 domain（可为完整 URL）：忽略 TTL 立即重新抓取该域名的站点图标；不带：立即执行图标库同步层。
func RefreshBrandIcon(c *gin.Context) {
	var req brandIconRefreshRequest
	if c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid request body"})
			return
		}
	}
	raw := strings.TrimSpace(req.Domain)
	if raw == "" {
		raw = strings.TrimSpace(c.Query("domain"))
	}
	if raw != "" {
		domain, err := model.RefreshBrandIconNow(brandicon.HostFromURL(raw))
		switch {
		case errors.Is(err, model.ErrInvalidBrandIconDomain):
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		case errors.Is(err, model.ErrBrandIconRefreshBusy), errors.Is(err, model.ErrBrandIconFetchDisabled):
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": err.Error()})
			return
		}
		_, cacheErr := model.GetBrandIconCache(domain)
		c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
			"domain": domain,
			"found":  cacheErr == nil,
		}})
		return
	}
	if BrandIconSync == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "message": "icon library sync is not available"})
		return
	}
	if err := BrandIconSync(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}
