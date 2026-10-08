package common

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

var Validate *validator.Validate

// NormalizeEmail 邮箱归一化：去掉首尾空白并转小写；空串保持空串。
// 所有邮箱写入与按邮箱查询的路径都必须先经过本函数，
// 使 `Foo@X.com` 与 `foo@x.com` 在数据库唯一约束下被视为同一邮箱。
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmailStrict 严格验证邮箱格式，防止恶意别名注册
func ValidateEmailStrict(email string) error {
	// 首先使用标准邮箱验证
	if err := Validate.Var(email, "required,email"); err != nil {
		return err
	}

	// 归一化后进行检查
	email = NormalizeEmail(email)

	// 检查是否包含 '+' 字符
	if strings.Contains(email, "+") {
		return &validator.ValidationErrors{}
	}

	// 检查 '.' 的数量，不能超过2个
	dotCount := strings.Count(email, ".")
	if dotCount > 2 {
		return &validator.ValidationErrors{}
	}

	// 额外的安全检查：确保邮箱格式合理
	// 不允许连续的点号
	if strings.Contains(email, "..") {
		return &validator.ValidationErrors{}
	}

	// 不允许以点号开头或结尾的本地部分
	atIndex := strings.Index(email, "@")
	if atIndex > 0 {
		localPart := email[:atIndex]
		if strings.HasPrefix(localPart, ".") || strings.HasSuffix(localPart, ".") {
			return &validator.ValidationErrors{}
		}
	}

	return nil
}

// IsValidEmailStrict 返回布尔值的邮箱验证
func IsValidEmailStrict(email string) bool {
	return ValidateEmailStrict(email) == nil
}

func init() {
	Validate = validator.New()
}
