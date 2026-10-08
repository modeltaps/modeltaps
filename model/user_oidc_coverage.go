package model

import (
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/gorm"
)

// LoginMethodFilterNoOidc 用户列表 login_method 参数值：只看未绑任何启用提供方身份的用户。
const LoginMethodFilterNoOidc = "no_oidc"

// OidcProviderCoverage 单个启用提供方下已绑定身份的活跃用户数。
type OidcProviderCoverage struct {
	Id          int    `json:"id"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name"`
	BoundUsers  int64  `json:"bound_users"`
}

// OidcCoverage 迁移进度核查结果：活跃用户中已绑 / 未绑启用提供方身份的人数。
type OidcCoverage struct {
	TotalActive int64                  `json:"total_active"`
	WithOidc    int64                  `json:"with_oidc"`
	WithoutOidc int64                  `json:"without_oidc"`
	ByProvider  []OidcProviderCoverage `json:"by_provider"`
}

// activeUsersQuery 活跃用户口径：未软删（gorm 自动过滤 deleted_at）+ status 启用 + 非影子账户。
func activeUsersQuery() *gorm.DB {
	return ExcludeShadowUsers(DB.Model(&User{})).Where("status = ?", config.UserStatusEnabled)
}

// enabledOidcIdentityExists 关联子查询：该用户在某个启用提供方下至少有一行身份。
// 只用标准 EXISTS 语法，SQLite / MySQL / PostgreSQL 通用。
func enabledOidcIdentityExists() *gorm.DB {
	return DB.Table("user_oidc_identities").
		Select("1").
		Joins("JOIN oidc_providers ON oidc_providers.id = user_oidc_identities.provider_id").
		Where("user_oidc_identities.user_id = users.id").
		Where("oidc_providers.enabled = ?", true)
}

// GetOidcCoverage 统计活跃用户的 OIDC 绑定覆盖情况，供管理员核查迁移进度。
// 只绑了已停用提供方的用户计入未绑。
func GetOidcCoverage() (*OidcCoverage, error) {
	coverage := &OidcCoverage{ByProvider: []OidcProviderCoverage{}}
	if err := activeUsersQuery().Count(&coverage.TotalActive).Error; err != nil {
		return nil, err
	}
	if err := activeUsersQuery().Where("EXISTS (?)", enabledOidcIdentityExists()).Count(&coverage.WithOidc).Error; err != nil {
		return nil, err
	}
	coverage.WithoutOidc = coverage.TotalActive - coverage.WithOidc

	var providers []*OidcProvider
	if err := DB.Where("enabled = ?", true).Order("sort asc, id asc").Find(&providers).Error; err != nil {
		return nil, err
	}
	for _, provider := range providers {
		bound := DB.Table("user_oidc_identities").
			Select("1").
			Where("user_oidc_identities.user_id = users.id").
			Where("user_oidc_identities.provider_id = ?", provider.Id)
		var boundUsers int64
		if err := activeUsersQuery().Where("EXISTS (?)", bound).Count(&boundUsers).Error; err != nil {
			return nil, err
		}
		coverage.ByProvider = append(coverage.ByProvider, OidcProviderCoverage{
			Id:          provider.Id,
			Slug:        provider.Slug,
			DisplayName: provider.DisplayName,
			BoundUsers:  boundUsers,
		})
	}
	return coverage, nil
}
