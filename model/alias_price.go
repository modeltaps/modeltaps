package model

import (
	"encoding/json"
	"strings"
)

// mappingTargetFor 解析单条渠道 model_mapping JSON，返回 modelName 的上游映射目标。
// 目标带 "+" 前缀(按传入/别名计费)时，别名用自己的价，不解析目标 → ok=false。
func mappingTargetFor(mappingJSON, modelName string) (string, bool) {
	if mappingJSON == "" {
		return "", false
	}
	m := map[string]string{}
	if err := json.Unmarshal([]byte(mappingJSON), &m); err != nil {
		return "", false
	}
	target, ok := m[modelName]
	if !ok || target == "" || strings.HasPrefix(target, "+") {
		return "", false
	}
	return target, true
}

// ResolveMappedPrice 对「价格未配置」的别名，顺着各渠道 model_mapping 找到映射目标，
// 返回首个已配置价格目标的 Price 副本（含其 ChannelType，以便前端显示真实供应商/价）。
// 找不到返回 nil。多渠道映射不一致时取首个有价者（代表值）。
func (cc *ChannelsChooser) ResolveMappedPrice(modelName string) *Price {
	cc.RLock()
	channels := make([]*Channel, 0, len(cc.Channels))
	for _, choice := range cc.Channels {
		if choice != nil && choice.Channel != nil {
			channels = append(channels, choice.Channel)
		}
	}
	cc.RUnlock()

	seen := map[string]bool{}
	for _, ch := range channels {
		target, ok := mappingTargetFor(ch.GetModelMapping(), modelName)
		if !ok || seen[target] {
			continue
		}
		seen[target] = true
		p := PricingInstance.GetPrice(target)
		if p != nil && !p.Unconfigured {
			cp := *p
			return &cp
		}
	}
	return nil
}
