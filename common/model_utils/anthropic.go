package model_utils

import "strings"

// IsAnthropicModelName 判定上游模型名（model_mapping 之后）是否属于 Anthropic Claude 家族。
// 规则：小写后，字符串包含 anthropic 或 claude；或任一分段（按 / . - : @ _ 及空白切分）
// 精确等于 opus / sonnet / haiku（分段相等而非子串，避免 corpus 等误命中）。
func IsAnthropicModelName(name string) bool {
	n := strings.ToLower(name)
	if strings.Contains(n, "anthropic") || strings.Contains(n, "claude") {
		return true
	}
	segs := strings.FieldsFunc(n, func(r rune) bool {
		switch r {
		case '/', '.', '-', ':', '@', '_', ' ', '\t', '\n', '\r':
			return true
		}
		return false
	})
	for _, seg := range segs {
		switch seg {
		case "opus", "sonnet", "haiku":
			return true
		}
	}
	return false
}
