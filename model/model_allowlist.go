package model

import (
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
)

// ModelAllowlistPermits 是令牌模型限制与组织 / 成员白名单共用的匹配规则(ORG-14):
//   - 空列表不额外限制;空白条目跳过;
//   - 不区分大小写;
//   - 条目以 "*" 结尾时做前缀匹配(如 gpt-4*);
//   - 别名与主名同等生效:请求名或其主名命中条目、或条目解析到的主名与请求的主名相同都放行
//     (别名解析按目录里的原名精确查找,与路由一致)。
//
// 令牌上「开启限制但列表为空」的历史拒绝语义由调用方单独处理,不在这里表达。
func ModelAllowlistPermits(allowlist []string, modelName string) bool {
	if len(allowlist) == 0 {
		return true
	}
	requested := strings.TrimSpace(modelName)
	canonical, _ := ResolveCanonicalModel(requested)
	name := strings.ToLower(requested)
	canonicalLower := strings.ToLower(canonical)
	for _, raw := range allowlist {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		item := strings.ToLower(entry)
		if prefix, ok := strings.CutSuffix(item, "*"); ok {
			if strings.HasPrefix(name, prefix) || strings.HasPrefix(canonicalLower, prefix) {
				return true
			}
			continue
		}
		if item == name || item == canonicalLower {
			return true
		}
		if itemCanonical, _ := ResolveCanonicalModel(entry); strings.ToLower(itemCanonical) == canonicalLower {
			return true
		}
	}
	return false
}

// FilterModelsByAllowlist 保留白名单允许的模型,规则见 ModelAllowlistPermits;空白名单原样返回。
func FilterModelsByAllowlist(allowlist []string, models []string) []string {
	if len(allowlist) == 0 {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, modelName := range models {
		if ModelAllowlistPermits(allowlist, modelName) {
			filtered = append(filtered, modelName)
		}
	}
	return filtered
}

// FilterModelsByTokenSetting 按令牌模型限制过滤:未开启原样返回;开启且列表为空(写入归一与迁移后
// 只会出现在迁移前的缓存里)按历史拒绝语义返回空,与 CheckLimitModel 一致。
func FilterModelsByTokenSetting(setting *TokenSetting, models []string) []string {
	if setting == nil || !setting.Limits.LimitModelSetting.Enabled {
		return models
	}
	if len(setting.Limits.LimitModelSetting.Models) == 0 {
		return []string{}
	}
	return FilterModelsByAllowlist(setting.Limits.LimitModelSetting.Models, models)
}

// normalizeLimitModelSetting 统一令牌模型限制的存储口径(ORG-14):列表为空即不限制,关闭限制即清空列表,
// 使 Enabled 恒等于「列表非空」,不再出现「开启但列表为空=拒绝全部」的状态。
func normalizeLimitModelSetting(s LimitModelSetting) LimitModelSetting {
	models := make([]string, 0, len(s.Models))
	if s.Enabled {
		seen := make(map[string]bool, len(s.Models))
		for _, m := range s.Models {
			m = strings.TrimSpace(m)
			if m == "" || seen[m] {
				continue
			}
			seen[m] = true
			models = append(models, m)
		}
	}
	return LimitModelSetting{Enabled: len(models) > 0, Models: models}
}

// TokenAvailableModels 返回令牌实际可调用的模型,口径与 /v1/models 相同:分组内可路由模型 ∩ 令牌模型限制
// ∩ 组织成员生效白名单(成员覆盖,空则组织缺省),再按目录规则去掉别名与隐藏模型。
func TokenAvailableModels(token *Token) ([]string, error) {
	group := token.Group
	if group == "" {
		userGroup, err := CacheGetUserGroup(token.UserId)
		if err != nil {
			return nil, err
		}
		group = userGroup
	}
	models, err := ChannelGroup.GetGroupModels(group)
	if err != nil {
		return []string{}, nil
	}
	sort.Strings(models)
	setting := token.Setting.Data()
	models = FilterModelsByTokenSetting(&setting, models)
	if token.CreatedBy > 0 && token.CreatedBy != token.UserId {
		guardrail, err := CacheGetOrgMemberGuardrail(token.UserId, token.CreatedBy)
		if err != nil {
			return nil, err
		}
		models = FilterModelsByAllowlist(guardrail.ModelWhitelist, models)
	}
	if config.CatalogEnforceHidden() {
		visible := make([]string, 0, len(models))
		for _, modelName := range models {
			if IsCatalogVisible(modelName) {
				visible = append(visible, modelName)
			}
		}
		models = visible
	}
	return models, nil
}
