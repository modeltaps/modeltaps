package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/model"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// OIDC 标准 claim 名，管理员留空时按标准值兜底（对接只下发标准 claim 的 IdP 时开箱可用）。
const (
	oidcProviderDefaultUsernameClaim    = "preferred_username"
	oidcProviderDefaultDisplayNameClaim = "name"
	oidcProviderDefaultAvatarClaim      = "picture"
)

// oidcProviderDiscoveryTimeout 保存时同步做 discovery 的超时：太长会把后台请求挂住。
const oidcProviderDiscoveryTimeout = 10 * time.Second

// oidcProviderDiscoveryErrorLimit discovery 错误摘要的截断长度，避免把整页 HTML 回给前端。
const oidcProviderDiscoveryErrorLimit = 300

// OidcProviderResponse 后台读接口的响应体。client_secret 靠 model 的 json:"-" 永不下发，
// 这里只额外告诉前端「有没有配过密钥」，便于编辑表单区分「不改」与「未配置」。
type OidcProviderResponse struct {
	*model.OidcProvider
	HasClientSecret bool `json:"has_client_secret"`
}

func newOidcProviderResponse(provider *model.OidcProvider) *OidcProviderResponse {
	return &OidcProviderResponse{
		OidcProvider:    provider,
		HasClientSecret: provider.ClientSecret != "",
	}
}

// OidcProviderRequest 新建 / 更新入参。ClientSecret 为空表示不修改（读接口不回显密钥，
// 若把空值当有效值会在编辑后清空已配置的密钥）；Slug 在更新时被忽略，回调地址不可变。
type OidcProviderRequest struct {
	Slug                string `json:"slug"`
	DisplayName         string `json:"display_name"`
	Issuer              string `json:"issuer"`
	ClientId            string `json:"client_id"`
	ClientSecret        string `json:"client_secret"`
	Scopes              string `json:"scopes"`
	UsernameClaim       string `json:"username_claim"`
	DisplayNameClaim    string `json:"display_name_claim"`
	AvatarClaim         string `json:"avatar_claim"`
	LinkByVerifiedEmail bool   `json:"link_by_verified_email"`
	LinkByVerifiedPhone bool   `json:"link_by_verified_phone"`
	DisableAutoRegister bool   `json:"disable_auto_register"`
	FirstParty          bool   `json:"first_party"`
	AccountSettingsUrl  string `json:"account_settings_url"`
	PasswordUrl         string `json:"password_url"`
	MfaUrl              string `json:"mfa_url"`
	PasskeyUrl          string `json:"passkey_url"`
	IdentityUrl         string `json:"identity_url"`
	Enabled             bool   `json:"enabled"`
	Sort                int    `json:"sort"`
}

func oidcProviderError(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": message,
	})
}

// normalize 去掉两侧空白并给缺省 claim 名填标准值。
func (req *OidcProviderRequest) normalize() {
	req.Slug = strings.TrimSpace(req.Slug)
	req.DisplayName = strings.TrimSpace(req.DisplayName)
	req.Issuer = strings.TrimRight(strings.TrimSpace(req.Issuer), "/")
	req.ClientId = strings.TrimSpace(req.ClientId)
	req.Scopes = strings.TrimSpace(req.Scopes)
	req.UsernameClaim = strings.TrimSpace(req.UsernameClaim)
	req.DisplayNameClaim = strings.TrimSpace(req.DisplayNameClaim)
	req.AvatarClaim = strings.TrimSpace(req.AvatarClaim)
	req.AccountSettingsUrl = strings.TrimSpace(req.AccountSettingsUrl)
	req.PasswordUrl = strings.TrimSpace(req.PasswordUrl)
	req.MfaUrl = strings.TrimSpace(req.MfaUrl)
	req.PasskeyUrl = strings.TrimSpace(req.PasskeyUrl)
	req.IdentityUrl = strings.TrimSpace(req.IdentityUrl)
	if req.UsernameClaim == "" {
		req.UsernameClaim = oidcProviderDefaultUsernameClaim
	}
	if req.DisplayNameClaim == "" {
		req.DisplayNameClaim = oidcProviderDefaultDisplayNameClaim
	}
	if req.AvatarClaim == "" {
		req.AvatarClaim = oidcProviderDefaultAvatarClaim
	}
	if req.DisplayName == "" {
		req.DisplayName = req.Slug
	}
}

// validateOidcIssuer issuer 必须是 https URL；host 为本机时放行 http，方便本地对接调试。
func validateOidcIssuer(issuer string) error {
	if issuer == "" {
		return errors.New("issuer must not be empty")
	}
	parsed, err := url.Parse(issuer)
	if err != nil || parsed.Host == "" {
		return errors.New("issuer is not a valid URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("issuer must not contain query parameters or a fragment")
	}
	switch parsed.Scheme {
	case "https":
	case "http":
		switch parsed.Hostname() {
		case "localhost", "127.0.0.1", "::1":
		default:
			return errors.New("issuer must use https (http is only allowed for local addresses)")
		}
	default:
		return errors.New("issuer must start with https://")
	}
	return nil
}

// validateOidcScopes scopes 非空且必须含 openid，否则 IdP 不会下发 id_token。
func validateOidcScopes(scopes string) error {
	fields := strings.FieldsFunc(scopes, func(r rune) bool {
		return r == ' ' || r == ',' || r == '\t'
	})
	if len(fields) == 0 {
		return errors.New("scopes must not be empty")
	}
	for _, scope := range fields {
		if scope == "openid" {
			return nil
		}
	}
	return errors.New("scopes must include openid")
}

// validateOidcAccountSettingsUrl 账号设置页可留空；填了就必须是 https 绝对 URL，
// 这个地址会作为外链下发到前端，相对路径或 http 都不可用。
func validateOidcAccountSettingsUrl(rawUrl string) error {
	if rawUrl == "" {
		return nil
	}
	parsed, err := url.Parse(rawUrl)
	if err != nil || parsed.Host == "" {
		return errors.New("account settings URL is not a valid URL")
	}
	if parsed.Scheme != "https" {
		return errors.New("account settings URL must start with https://")
	}
	return nil
}

func (req *OidcProviderRequest) validate() error {
	if req.ClientId == "" {
		return errors.New("client_id must not be empty")
	}
	if err := validateOidcIssuer(req.Issuer); err != nil {
		return err
	}
	for _, rawUrl := range []string{req.AccountSettingsUrl, req.PasswordUrl, req.MfaUrl, req.PasskeyUrl, req.IdentityUrl} {
		if err := validateOidcAccountSettingsUrl(rawUrl); err != nil {
			return err
		}
	}
	return validateOidcScopes(req.Scopes)
}

// runOidcDiscovery 按 issuer 跑一次标准 discovery。这里直接用 go-oidc，不经过
// common/oidc 注册表：注册表按 slug 每次读表，后台增删改无需通知它失效。
func runOidcDiscovery(issuer string) error {
	ctx, cancel := context.WithTimeout(context.Background(), oidcProviderDiscoveryTimeout)
	defer cancel()
	if _, err := oidc.NewProvider(ctx, issuer); err != nil {
		summary := err.Error()
		if len(summary) > oidcProviderDiscoveryErrorLimit {
			summary = summary[:oidcProviderDiscoveryErrorLimit] + "..."
		}
		return fmt.Errorf("OIDC discovery failed (issuer: %s): %s", issuer, summary)
	}
	return nil
}

func oidcProviderIdParam(c *gin.Context) (int, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		oidcProviderError(c, http.StatusBadRequest, "Invalid ID parameter")
		return 0, false
	}
	return id, true
}

// loadOidcProvider 取出目标提供方，不存在返回 404。
func loadOidcProvider(c *gin.Context, id int) (*model.OidcProvider, bool) {
	provider, err := model.GetOidcProviderById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			oidcProviderError(c, http.StatusNotFound, "Identity provider not found")
		} else {
			oidcProviderError(c, http.StatusInternalServerError, "Failed to get identity provider: "+err.Error())
		}
		return nil, false
	}
	return provider, true
}

func GetOidcProvidersList(c *gin.Context) {
	providers, err := model.GetOidcProviders()
	if err != nil {
		oidcProviderError(c, http.StatusInternalServerError, "Failed to list identity providers: "+err.Error())
		return
	}
	responses := make([]*OidcProviderResponse, 0, len(providers))
	for _, provider := range providers {
		responses = append(responses, newOidcProviderResponse(provider))
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    responses,
	})
}

func GetOidcProvider(c *gin.Context) {
	id, ok := oidcProviderIdParam(c)
	if !ok {
		return
	}
	provider, ok := loadOidcProvider(c, id)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    newOidcProviderResponse(provider),
	})
}

// CreateOidcProvider 新建提供方。discovery 失败时返回 4xx；带 force=true 可强行保存，
// 但一律落成 enabled=false，避免把不可用的提供方挂到登录页上。
func CreateOidcProvider(c *gin.Context) {
	var req OidcProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		oidcProviderError(c, http.StatusBadRequest, err.Error())
		return
	}
	req.normalize()
	if err := model.ValidateOidcProviderSlug(req.Slug); err != nil {
		oidcProviderError(c, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.validate(); err != nil {
		oidcProviderError(c, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := model.GetOidcProviderBySlug(req.Slug); err == nil {
		oidcProviderError(c, http.StatusConflict, "slug already exists")
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		oidcProviderError(c, http.StatusInternalServerError, "Failed to validate slug: "+err.Error())
		return
	}

	message := ""
	if err := runOidcDiscovery(req.Issuer); err != nil {
		if c.Query("force") != "true" {
			oidcProviderError(c, http.StatusBadRequest, err.Error())
			return
		}
		req.Enabled = false
		message = err.Error() + "; saved as disabled because force was set"
	}

	provider := &model.OidcProvider{
		Slug:                req.Slug,
		DisplayName:         req.DisplayName,
		Issuer:              req.Issuer,
		ClientId:            req.ClientId,
		ClientSecret:        req.ClientSecret,
		Scopes:              req.Scopes,
		UsernameClaim:       req.UsernameClaim,
		DisplayNameClaim:    req.DisplayNameClaim,
		AvatarClaim:         req.AvatarClaim,
		LinkByVerifiedEmail: req.LinkByVerifiedEmail,
		LinkByVerifiedPhone: req.LinkByVerifiedPhone,
		DisableAutoRegister: req.DisableAutoRegister,
		FirstParty:          req.FirstParty,
		AccountSettingsUrl:  req.AccountSettingsUrl,
		PasswordUrl:         req.PasswordUrl,
		MfaUrl:              req.MfaUrl,
		PasskeyUrl:          req.PasskeyUrl,
		IdentityUrl:         req.IdentityUrl,
		Enabled:             req.Enabled,
		Sort:                req.Sort,
	}
	if err := provider.Insert(); err != nil {
		oidcProviderError(c, http.StatusInternalServerError, "Failed to create identity provider: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    newOidcProviderResponse(provider),
	})
}

// UpdateOidcProvider 更新提供方。slug 不可修改（IdP 侧已登记回调地址 /oauth/oidc/{slug}），
// 入参里的 slug 被忽略；client_secret 为空表示不修改。
func UpdateOidcProvider(c *gin.Context) {
	id, ok := oidcProviderIdParam(c)
	if !ok {
		return
	}
	var req OidcProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		oidcProviderError(c, http.StatusBadRequest, err.Error())
		return
	}
	provider, ok := loadOidcProvider(c, id)
	if !ok {
		return
	}
	req.Slug = provider.Slug
	req.normalize()
	if err := req.validate(); err != nil {
		oidcProviderError(c, http.StatusBadRequest, err.Error())
		return
	}

	message := ""
	if err := runOidcDiscovery(req.Issuer); err != nil {
		if c.Query("force") != "true" {
			oidcProviderError(c, http.StatusBadRequest, err.Error())
			return
		}
		req.Enabled = false
		message = err.Error() + "; saved as disabled because force was set"
	}

	provider.DisplayName = req.DisplayName
	provider.Issuer = req.Issuer
	provider.ClientId = req.ClientId
	provider.ClientSecret = req.ClientSecret
	provider.Scopes = req.Scopes
	provider.UsernameClaim = req.UsernameClaim
	provider.DisplayNameClaim = req.DisplayNameClaim
	provider.AvatarClaim = req.AvatarClaim
	provider.LinkByVerifiedEmail = req.LinkByVerifiedEmail
	provider.LinkByVerifiedPhone = req.LinkByVerifiedPhone
	provider.DisableAutoRegister = req.DisableAutoRegister
	provider.FirstParty = req.FirstParty
	provider.AccountSettingsUrl = req.AccountSettingsUrl
	provider.PasswordUrl = req.PasswordUrl
	provider.MfaUrl = req.MfaUrl
	provider.PasskeyUrl = req.PasskeyUrl
	provider.IdentityUrl = req.IdentityUrl
	provider.Enabled = req.Enabled
	provider.Sort = req.Sort
	if err := provider.Update(); err != nil {
		oidcProviderError(c, http.StatusInternalServerError, "Failed to update identity provider: "+err.Error())
		return
	}

	updated, err := model.GetOidcProviderById(id)
	if err != nil {
		oidcProviderError(c, http.StatusInternalServerError, "Failed to update identity provider: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": message,
		"data":    newOidcProviderResponse(updated),
	})
}

// UpdateOidcProviderStatus 只切换 enabled，不动其余字段，也不做 discovery：
// 停用是排障手段，必须在 IdP 已经不可达时也能生效。
func UpdateOidcProviderStatus(c *gin.Context) {
	id, ok := oidcProviderIdParam(c)
	if !ok {
		return
	}
	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		oidcProviderError(c, http.StatusBadRequest, err.Error())
		return
	}
	provider, ok := loadOidcProvider(c, id)
	if !ok {
		return
	}
	if err := model.DB.Model(&model.OidcProvider{}).Where("id = ?", id).
		Updates(map[string]interface{}{"enabled": req.Enabled, "updated_time": time.Now().Unix()}).Error; err != nil {
		oidcProviderError(c, http.StatusInternalServerError, "Failed to update identity provider status: "+err.Error())
		return
	}
	provider.Enabled = req.Enabled
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    newOidcProviderResponse(provider),
	})
}

// DeleteOidcProvider 删除提供方。仍有用户身份行时拒绝：删除会连带清掉这些绑定，
// 相关用户将无法再用该 IdP 登录，须管理员先解绑或改用 enabled=false 停用。
func DeleteOidcProvider(c *gin.Context) {
	id, ok := oidcProviderIdParam(c)
	if !ok {
		return
	}
	if _, ok := loadOidcProvider(c, id); !ok {
		return
	}
	var count int64
	if err := model.DB.Model(&model.UserOidcIdentity{}).Where("provider_id = ?", id).Count(&count).Error; err != nil {
		oidcProviderError(c, http.StatusInternalServerError, "Failed to count linked identities: "+err.Error())
		return
	}
	if count > 0 {
		oidcProviderError(c, http.StatusConflict, fmt.Sprintf(
			"This identity provider still has %d linked user identities and cannot be deleted. Have users unlink first, or disable it (enabled=false)", count))
		return
	}
	if err := model.DeleteOidcProviderById(id); err != nil {
		oidcProviderError(c, http.StatusInternalServerError, "Failed to delete identity provider: "+err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

// TestOidcProvider 按已保存的 issuer 重新跑一次 discovery，用于排查 IdP 侧变更。
func TestOidcProvider(c *gin.Context) {
	id, ok := oidcProviderIdParam(c)
	if !ok {
		return
	}
	provider, ok := loadOidcProvider(c, id)
	if !ok {
		return
	}
	if err := runOidcDiscovery(provider.Issuer); err != nil {
		oidcProviderError(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OIDC discovery succeeded",
		"data":    gin.H{"issuer": provider.Issuer},
	})
}
