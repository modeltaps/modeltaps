package model

import (
	"errors"
	"regexp"

	"github.com/modeltaps/modeltaps/common/utils"

	"gorm.io/gorm"
)

// OidcProvider 一个可登录的 OIDC 提供方。存量单提供方部署由迁移 202609030001
// 转成 slug=oidc 的第一行，其余由管理员在后台增删改。
//
// ClientSecret 的 json tag 固定为 "-"：该结构会被后台接口直接序列化下发，
// 密钥永远不得出现在任何响应体里。
type OidcProvider struct {
	Id                  int    `json:"id"`
	Slug                string `json:"slug" gorm:"type:varchar(32);not null;uniqueIndex:idx_oidc_providers_slug"`
	DisplayName         string `json:"display_name" gorm:"type:varchar(64);default:''"`
	Issuer              string `json:"issuer" gorm:"type:varchar(500);default:''"`
	ClientId            string `json:"client_id" gorm:"type:varchar(255);default:''"`
	ClientSecret        string `json:"-" gorm:"type:varchar(500);default:''"`
	Scopes              string `json:"scopes" gorm:"type:varchar(255);default:''"`
	UsernameClaim       string `json:"username_claim" gorm:"type:varchar(64);default:''"`
	DisplayNameClaim    string `json:"display_name_claim" gorm:"type:varchar(64);default:''"`
	AvatarClaim         string `json:"avatar_claim" gorm:"type:varchar(64);default:''"`
	LinkByVerifiedEmail bool   `json:"link_by_verified_email" gorm:"default:false"`
	// LinkByVerifiedPhone 允许按 IdP 下发的已验证手机号关联已有账号，语义与
	// LinkByVerifiedEmail 完全对称；仅在 IdP 对手机号做过所有权验证时才应开启。
	LinkByVerifiedPhone bool `json:"link_by_verified_phone" gorm:"default:false"`
	DisableAutoRegister bool `json:"disable_auto_register" gorm:"default:false"`
	// FirstParty 该提供方即本站账号体系：登录入口按站点名称与站点图标呈现，不露出提供方名称与图标。
	FirstParty bool `json:"first_party" gorm:"default:false"`
	// AccountSettingsUrl 提供方侧的账号设置页，供本站在不持有密码时给出改密出口；空串表示没有。
	AccountSettingsUrl string `json:"account_settings_url" gorm:"type:varchar(500);default:''"`
	// 提供方托管设置页的四个深链：账号安全页的密码 / 两步验证 / 通行密钥 / 第三方登录与联系信息各行
	// 按此跳转，留空的行不渲染。只在承担本站身份的提供方上有意义。
	PasswordUrl string `json:"password_url" gorm:"type:varchar(500);default:''"`
	MfaUrl      string `json:"mfa_url" gorm:"type:varchar(500);default:''"`
	PasskeyUrl  string `json:"passkey_url" gorm:"type:varchar(500);default:''"`
	IdentityUrl string `json:"identity_url" gorm:"type:varchar(500);default:''"`
	Enabled     bool   `json:"enabled" gorm:"default:false"`
	Sort        int    `json:"sort" gorm:"type:int;default:0"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

// LegacyOidcProviderSlug 存量单提供方迁移后使用的 slug，同时是 /oauth/oidc 旧回调的别名。
const LegacyOidcProviderSlug = "oidc"

// oidcProviderSlugPattern slug 进 URL（/oauth/oidc/{slug}），只允许小写字母、数字与连字符。
var oidcProviderSlugPattern = regexp.MustCompile(`^[a-z0-9-]{1,32}$`)

// ValidateOidcProviderSlug 校验 slug 合法性。LegacyOidcProviderSlug 本身合法（存量迁移
// 就用它），故不额外设黑名单，否则后台无法编辑迁移出来的那一行。
func ValidateOidcProviderSlug(slug string) error {
	if !oidcProviderSlugPattern.MatchString(slug) {
		return errors.New("slug may only contain lowercase letters, digits and hyphens, 1-32 characters long")
	}
	return nil
}

// oidcProviderUpdateColumns Update 时显式列出的列：Updates(struct) 会跳过零值，
// 只有 Select 出来才能把 enabled=false / sort=0 这类零值写回去。
var oidcProviderUpdateColumns = []string{
	"slug", "display_name", "issuer", "client_id", "scopes",
	"username_claim", "display_name_claim", "avatar_claim",
	"link_by_verified_email", "link_by_verified_phone", "disable_auto_register", "first_party", "account_settings_url",
	"password_url", "mfa_url", "passkey_url", "identity_url",
	"enabled", "sort", "updated_time",
}

// GetOidcProviders 返回全部提供方，按 sort、id 排序。
func GetOidcProviders() ([]*OidcProvider, error) {
	var providers []*OidcProvider
	err := DB.Order("sort asc, id asc").Find(&providers).Error
	return providers, err
}

// GetEnabledOidcProviders 返回已启用的提供方，按 sort、id 排序。
func GetEnabledOidcProviders() ([]*OidcProvider, error) {
	var providers []*OidcProvider
	err := DB.Where("enabled = ?", true).Order("sort asc, id asc").Find(&providers).Error
	return providers, err
}

// PickFirstPartyOidcProvider 从已启用的提供方里挑出承担本站账号体系的那一个：
// 标记为本站身份的第一行优先；没有标记时，恰好只启用了一个就用它；其余情形返回 nil（配置不完整）。
func PickFirstPartyOidcProvider(providers []*OidcProvider) *OidcProvider {
	for _, provider := range providers {
		if provider != nil && provider.Enabled && provider.FirstParty {
			return provider
		}
	}
	if len(providers) == 1 && providers[0] != nil && providers[0].Enabled {
		return providers[0]
	}
	return nil
}

// GetFirstPartyOidcProvider 读库后按 PickFirstPartyOidcProvider 的规则取本站身份提供方；没有时返回 (nil, nil)。
func GetFirstPartyOidcProvider() (*OidcProvider, error) {
	providers, err := GetEnabledOidcProviders()
	if err != nil {
		return nil, err
	}
	return PickFirstPartyOidcProvider(providers), nil
}

func GetOidcProviderById(id int) (*OidcProvider, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var provider OidcProvider
	err := DB.First(&provider, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func GetOidcProviderBySlug(slug string) (*OidcProvider, error) {
	if slug == "" {
		return nil, errors.New("slug is empty")
	}
	var provider OidcProvider
	err := DB.First(&provider, "slug = ?", slug).Error
	if err != nil {
		return nil, err
	}
	return &provider, nil
}

func (provider *OidcProvider) Insert() error {
	if err := ValidateOidcProviderSlug(provider.Slug); err != nil {
		return err
	}
	now := utils.GetTimestamp()
	provider.CreatedTime = now
	provider.UpdatedTime = now
	return DB.Create(provider).Error
}

// Update 更新提供方配置。ClientSecret 为空表示不改：后台读接口不下发密钥，
// 回写时若把空值当有效值会直接清空已配置的密钥。
func (provider *OidcProvider) Update() error {
	if err := ValidateOidcProviderSlug(provider.Slug); err != nil {
		return err
	}
	provider.UpdatedTime = utils.GetTimestamp()
	columns := oidcProviderUpdateColumns
	if provider.ClientSecret != "" {
		columns = append(append([]string{}, columns...), "client_secret")
	}
	return DB.Model(provider).Select(columns).Updates(provider).Error
}

// Delete 删除提供方及其下所有用户身份（身份行没有外键，须显式清理）。
func (provider *OidcProvider) Delete() error {
	if provider.Id == 0 {
		return errors.New("id is empty")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("provider_id = ?", provider.Id).Delete(&UserOidcIdentity{}).Error; err != nil {
			return err
		}
		if err := DeleteUserSessionsByProvider(tx, provider.Id); err != nil {
			return err
		}
		return tx.Delete(provider).Error
	})
}

func DeleteOidcProviderById(id int) error {
	if id == 0 {
		return errors.New("id is empty")
	}
	provider := OidcProvider{Id: id}
	return provider.Delete()
}
