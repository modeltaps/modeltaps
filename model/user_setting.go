package model

import (
	"errors"

	"github.com/modeltaps/modeltaps/common/database"
)

// UserSetting 用户级偏好设置，存储于 users.setting JSON 列（复用 TokenSetting 的 JSONType 范式）。
// 所有字段均可为空：空值表示未设置，前端回退到本地（localStorage）行为。
type UserSetting struct {
	Theme    string `json:"theme,omitempty"`    // system / light / dark
	Language string `json:"language,omitempty"` // zh_CN / zh_HK / en_US / ja_JP
	// LogIODefault 个人令牌「完整请求/响应留存」继承默认;nil 时取站点默认 config.LogIODefaultUser
	LogIODefault *bool `json:"log_io_default,omitempty"`
	// EmailCodeLogin 是否允许用邮箱验证码登录本账号；nil 表示未设置，按开启处理
	EmailCodeLogin *bool `json:"email_code_login,omitempty"`
}

// EmailCodeLoginAllowed 用户是否允许邮箱验证码登录（默认允许）。
func (s *UserSetting) EmailCodeLoginAllowed() bool {
	return s.EmailCodeLogin == nil || *s.EmailCodeLogin
}

// UserSettingUpdate PUT /api/user/setting 的请求体;nil 字段表示不修改(部分更新)。
// LogIODefault 用三态哨兵字符串 "inherit"|"on"|"off",使「跟随站点」(nil)可经接口重置。
type UserSettingUpdate struct {
	Theme          *string `json:"theme"`
	Language       *string `json:"language"`
	LogIODefault   *string `json:"log_io_default"`
	EmailCodeLogin *bool   `json:"email_code_login"`
}

// LogIOTriStateString 把 LogIO 三态 *bool 编码为 API 哨兵字符串(nil→inherit / true→on / false→off)。
func LogIOTriStateString(b *bool) string {
	if b == nil {
		return "inherit"
	}
	if *b {
		return "on"
	}
	return "off"
}

// ParseLogIOTriState 把 API 哨兵字符串解码为三态 *bool;ok=false 表示非法值。
func ParseLogIOTriState(s string) (value *bool, ok bool) {
	switch s {
	case "inherit":
		return nil, true
	case "on":
		t := true
		return &t, true
	case "off":
		f := false
		return &f, true
	default:
		return nil, false
	}
}

// APIView 以 API 契约形状返回用户设置(LogIODefault 回显三态哨兵字符串)。
func (s *UserSetting) APIView() map[string]any {
	return map[string]any{
		"theme":            s.Theme,
		"language":         s.Language,
		"log_io_default":   LogIOTriStateString(s.LogIODefault),
		"email_code_login": s.EmailCodeLoginAllowed(),
	}
}

var userSettingThemes = map[string]bool{
	"":       true,
	"system": true,
	"light":  true,
	"dark":   true,
}

var userSettingLanguages = map[string]bool{
	"":      true,
	"zh_CN": true,
	"zh_HK": true,
	"en_US": true,
	"ja_JP": true,
}

// ApplyUpdate 合并部分更新:仅覆盖非 nil 字段。LogIODefault 哨兵字符串 "inherit" 解码为 nil(重置回跟随站点)。
func (s *UserSetting) ApplyUpdate(update UserSettingUpdate) error {
	if update.Theme != nil {
		s.Theme = *update.Theme
	}
	if update.Language != nil {
		s.Language = *update.Language
	}
	if update.LogIODefault != nil {
		v, ok := ParseLogIOTriState(*update.LogIODefault)
		if !ok {
			return errors.New("invalid log_io_default value")
		}
		s.LogIODefault = v
	}
	if update.EmailCodeLogin != nil {
		value := *update.EmailCodeLogin
		s.EmailCodeLogin = &value
	}
	return nil
}

// Validate 校验字段取值；空值合法（表示未设置）。
func (s *UserSetting) Validate() error {
	if !userSettingThemes[s.Theme] {
		return errors.New("invalid theme setting")
	}
	if !userSettingLanguages[s.Language] {
		return errors.New("invalid language setting")
	}
	return nil
}

// GetUserSettingById 仅读取 setting 列；老用户列值为 NULL 时返回零值（全空）。
func GetUserSettingById(id int) (*UserSetting, error) {
	if id == 0 {
		return nil, errors.New("id is empty")
	}
	var user User
	if err := DB.Select("setting").First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	setting := user.Setting.Data()
	return &setting, nil
}

// UpdateUserSettingById 整体写入 setting 列（调用方先合并好字段）。
func UpdateUserSettingById(id int, setting UserSetting) error {
	if id == 0 {
		return errors.New("id is empty")
	}
	var value database.JSONType[UserSetting]
	value.Set(setting)
	err := DB.Model(&User{}).Where("id = ?", id).Update("setting", value).Error
	if err == nil {
		// 个人默认变更后失效解析缓存,确保下一次令牌「继承」按新默认回退
		InvalidateOwnerLogIODefaultCache(id)
	}
	return err
}
