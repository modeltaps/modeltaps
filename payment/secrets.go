package payment

import (
	"encoding/json"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
)

// secretConfigKeys 返回某网关类型的密钥字段名；未知类型没有密钥字段。
func secretConfigKeys(gatewayType string) []string {
	gateway, ok := Gateways[gatewayType]
	if !ok {
		return nil
	}
	return gateway.SecretConfigKeys()
}

// parseConfig 把网关配置 JSON 解析成字段表；空串或非法 JSON 当作空配置。
func parseConfig(raw string) map[string]json.RawMessage {
	config := map[string]json.RawMessage{}
	if raw == "" {
		return config
	}
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return map[string]json.RawMessage{}
	}
	return config
}

func stringField(config map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := config[key]
	if !ok {
		return "", false
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return "", false
	}
	return value, true
}

func setStringField(config map[string]json.RawMessage, key, value string) {
	raw, _ := json.Marshal(value)
	config[key] = raw
}

// MaskConfig 把配置里的密钥字段换成 utils.MaskSecret 的回显值，供后台读接口下发前调用。
// 非密钥字段与非法 JSON 原样返回。
func MaskConfig(gatewayType, rawConfig string) string {
	keys := secretConfigKeys(gatewayType)
	if len(keys) == 0 || rawConfig == "" {
		return rawConfig
	}
	config := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(rawConfig), &config); err != nil {
		return rawConfig
	}
	for _, key := range keys {
		value, ok := stringField(config, key)
		if !ok || value == "" {
			continue
		}
		setStringField(config, key, utils.MaskSecret(value))
	}
	out, err := json.Marshal(config)
	if err != nil {
		return rawConfig
	}
	return string(out)
}

// MergeConfigSecrets 把编辑表单提交的配置与库里的配置合并：密钥字段留空、或原样回传了
// 脱敏回显值时，沿用库里的值；其余字段以提交的为准。
func MergeConfigSecrets(gatewayType, submitted, stored string) string {
	keys := secretConfigKeys(gatewayType)
	if len(keys) == 0 || stored == "" {
		return submitted
	}
	next := parseConfig(submitted)
	prev := parseConfig(stored)
	changed := false
	for _, key := range keys {
		old, hasOld := stringField(prev, key)
		if !hasOld || old == "" {
			continue
		}
		value, _ := stringField(next, key)
		if value == "" || value == utils.MaskSecret(old) {
			setStringField(next, key, old)
			changed = true
		}
	}
	if !changed {
		return submitted
	}
	out, err := json.Marshal(next)
	if err != nil {
		return submitted
	}
	return string(out)
}

// HidePaymentSecrets 把一条支付配置的密钥字段换成回显值，供接口返回前调用。
func HidePaymentSecrets(payment *model.Payment) {
	if payment == nil {
		return
	}
	payment.Config = MaskConfig(payment.Type, payment.Config)
}
