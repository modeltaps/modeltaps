package model

import (
	"errors"

	"github.com/modeltaps/modeltaps/common/utils"
)

// UserOidcIdentity 用户在某个 OIDC 提供方下的身份（provider_id + subject）。
// 一个用户可绑定多个提供方，同一 (provider_id, subject) 只能归属一个用户。
//
// 唯一索引必须由 gorm 标签声明：MySQL driver 的 MigrateColumnUnique 会在 AutoMigrate
// 时把「schema 不认识」的唯一索引当冗余 DROP 掉，手工 CREATE INDEX 建出来的约束
// 会在第二次启动后消失（详见 model/user.go 上 Email 字段的同类注释）。
type UserOidcIdentity struct {
	Id         int    `json:"id"`
	UserId     int    `json:"user_id" gorm:"not null;index"`
	ProviderId int    `json:"provider_id" gorm:"not null;uniqueIndex:idx_user_oidc_identities_provider_subject,priority:1"`
	Subject    string `json:"subject" gorm:"type:varchar(255);not null;uniqueIndex:idx_user_oidc_identities_provider_subject,priority:2"`
	// IdpUsername / IdpEmail 是 IdP 侧账户的展示快照（登录 / 绑定时刷新），只用于在
	// 「账号绑定」页说明这条绑定对应对方的哪个账户，不参与任何匹配与关联判定。
	IdpUsername string `json:"idp_username" gorm:"type:varchar(255)"`
	IdpEmail    string `json:"idp_email" gorm:"type:varchar(255)"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
}

// FindUserOidcIdentity 按 (provider_id, subject) 查身份，未命中返回 gorm.ErrRecordNotFound。
func FindUserOidcIdentity(providerId int, subject string) (*UserOidcIdentity, error) {
	if providerId == 0 || subject == "" {
		return nil, errors.New("identity provider or subject is empty")
	}
	var identity UserOidcIdentity
	err := DB.Where("provider_id = ? AND subject = ?", providerId, subject).First(&identity).Error
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

// FindUserOidcIdentityByUserAndProvider 按 (user_id, provider_id) 查身份，
// 未命中返回 gorm.ErrRecordNotFound。用于判断某账号在某提供方下是否已有身份。
func FindUserOidcIdentityByUserAndProvider(userId int, providerId int) (*UserOidcIdentity, error) {
	if userId == 0 || providerId == 0 {
		return nil, errors.New("user id or identity provider is empty")
	}
	var identity UserOidcIdentity
	err := DB.Where("user_id = ? AND provider_id = ?", userId, providerId).Order("id asc").First(&identity).Error
	if err != nil {
		return nil, err
	}
	return &identity, nil
}

// ListUserOidcIdentities 列出某个用户已绑定的全部身份。
func ListUserOidcIdentities(userId int) ([]*UserOidcIdentity, error) {
	if userId == 0 {
		return nil, errors.New("user id is empty")
	}
	var identities []*UserOidcIdentity
	err := DB.Where("user_id = ?", userId).Order("id asc").Find(&identities).Error
	return identities, err
}

func (identity *UserOidcIdentity) Insert() error {
	if identity.UserId == 0 || identity.ProviderId == 0 || identity.Subject == "" {
		return errors.New("user id, identity provider or subject is empty")
	}
	identity.CreatedTime = utils.GetTimestamp()
	return DB.Create(identity).Error
}

// UpdateUserOidcIdentitySnapshot 刷新 (provider_id, subject) 身份行上的 IdP 账户展示快照。
// 只写展示列，不碰归属关系；claim 缺失时写空串，展示端据此省略对应项。
func UpdateUserOidcIdentitySnapshot(providerId int, subject string, idpUsername string, idpEmail string) error {
	if providerId == 0 || subject == "" {
		return errors.New("identity provider or subject is empty")
	}
	return DB.Model(&UserOidcIdentity{}).
		Where("provider_id = ? AND subject = ?", providerId, subject).
		Updates(map[string]interface{}{"idp_username": idpUsername, "idp_email": idpEmail}).Error
}

// DeleteUserOidcIdentity 解绑某个用户在某个提供方下的身份。
func DeleteUserOidcIdentity(userId int, providerId int) error {
	if userId == 0 || providerId == 0 {
		return errors.New("user id or identity provider is empty")
	}
	return DB.Where("user_id = ? AND provider_id = ?", userId, providerId).Delete(&UserOidcIdentity{}).Error
}
