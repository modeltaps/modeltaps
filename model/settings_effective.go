package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/settings"
)

// The value a setting takes effect with, and where it comes from: a site setting's stored value
// or its default, or an object field together with the site setting it follows or overrides.
// The admin MCP tools (settings_effective) and the console read the same answer.

// SettingSource is one step of how a value was reached, first the one that decided.
type SettingSource struct {
	// Level is "object", "site" or "default".
	Level  string `json:"level"`
	Key    string `json:"key"`
	Value  string `json:"value"`
	Reason string `json:"reason"`
}

// EffectiveSetting is the value of a setting, for one object when it is an object field.
type EffectiveSetting struct {
	Key      string          `json:"key"`
	ObjectId string          `json:"object_id,omitempty"`
	Value    string          `json:"value"`
	Chain    []SettingSource `json:"chain"`
}

var ErrSettingNotResolvable = errors.New("this object field has no resolver: read it on its object, or use settings_describe")

// SiteSettingValue is a site setting's current value as it may be shown: secrets only say
// whether they are set.
func SiteSettingValue(key string) string {
	if config.GlobalOption.IsSecret(key) {
		if config.GlobalOption.Get(key) == "" {
			return "(unset)"
		}
		return "(set)"
	}
	return config.GlobalOption.Get(key)
}

// SiteSettingModified reports whether a non-secret site setting differs from its default.
func SiteSettingModified(key string) bool {
	if config.GlobalOption.IsSecret(key) {
		return false
	}
	def, ok := config.GlobalOption.Defaults()[key]
	return ok && def != config.GlobalOption.Get(key)
}

func siteSource(key string) SettingSource {
	value := SiteSettingValue(key)
	if SiteSettingModified(key) {
		return SettingSource{Level: "site", Key: key, Value: value, Reason: "stored in the platform settings"}
	}
	return SettingSource{Level: "default", Key: key, Value: value, Reason: "the default (no stored value differs from it)"}
}

// ResolveEffectiveSetting answers for a site setting (objectId empty) or an object field.
func ResolveEffectiveSetting(key, objectId string) (*EffectiveSetting, error) {
	entry, ok := settings.Get(key)
	if !ok {
		return nil, fmt.Errorf("unknown setting %q", key)
	}
	if entry.Scope == settings.ScopeSite {
		src := siteSource(key)
		return &EffectiveSetting{Key: key, Value: src.Value, Chain: []SettingSource{src}}, nil
	}
	if objectId == "" {
		return nil, fmt.Errorf("%s is a field of a %s: give object_id", key, entry.Scope)
	}
	resolve, ok := effectiveResolvers[key]
	if !ok {
		return nil, ErrSettingNotResolvable
	}
	out, err := resolve(objectId)
	if err != nil {
		return nil, err
	}
	out.Key, out.ObjectId = key, objectId
	return out, nil
}

// EffectiveResolvableKeys lists the object fields ResolveEffectiveSetting can answer for.
func EffectiveResolvableKeys() []string {
	keys := make([]string, 0, len(effectiveResolvers))
	for k := range effectiveResolvers {
		keys = append(keys, k)
	}
	return keys
}

var effectiveResolvers = map[string]func(objectId string) (*EffectiveSetting, error){
	// A model's supply mode: its own, or the site default; models on the allocation policy share
	// a tier by weight.
	"model.supply_mode": func(name string) (*EffectiveSetting, error) {
		info := lookupModelInfo(name)
		if info == nil {
			return nil, fmt.Errorf("no public model %q", name)
		}
		value := routingSupplyMode(info, currentAllocationActive())
		var chain []SettingSource
		switch {
		case currentAllocationActive() && followsPolicy(info):
			chain = append(chain, SettingSource{Level: "object", Key: "model.allocation", Value: "policy",
				Reason: "the model follows the allocation policy, whose tiers always share requests by weight"})
		case info.SupplyMode == SupplyModeBalance || info.SupplyMode == SupplyModeFailover:
			chain = append(chain, SettingSource{Level: "object", Key: "model.supply_mode", Value: info.SupplyMode,
				Reason: "set on the model's Supplies tab"})
		default:
			chain = append(chain, SettingSource{Level: "object", Key: "model.supply_mode", Value: SupplyModeDefault,
				Reason: "the model follows the site default"}, siteSource("SupplyModeDefault"))
		}
		chain = append(chain, siteSource("SupplyCostOrder"))
		return &EffectiveSetting{Value: value, Chain: chain}, nil
	},
	// A channel's fee rate and where it came from.
	"channel.fee_rate": func(id string) (*EffectiveSetting, error) {
		ch, err := channelByIdString(id)
		if err != nil {
			return nil, err
		}
		rate := strconv.FormatFloat(ch.GetFeeRate()*100, 'f', -1, 64) + "%"
		reason := "set on the channel"
		var n int64
		if DB != nil {
			DB.Model(&ChannelTopup{}).Where("channel_id = ?", ch.Id).Count(&n)
		}
		if n > 0 {
			reason = fmt.Sprintf("derived from %d recorded top-up(s)", n)
		} else if def, note := ChannelFeeDefault(ch.Type); ch.GetFeeRate() == def && note != "" {
			reason = "the default of its channel type"
		}
		if ch.FeeNote != "" {
			reason += ": " + ch.FeeNote
		}
		return &EffectiveSetting{Value: rate, Chain: []SettingSource{{Level: "object", Key: "channel.fee_rate", Value: rate, Reason: reason}}}, nil
	},
	// Whether Claude prompt caching is injected on a channel: its own switch, or the site's.
	"channel.prompt_caching": func(id string) (*EffectiveSetting, error) {
		ch, err := channelByIdString(id)
		if err != nil {
			return nil, err
		}
		own := "inherit"
		if ch.Plugin != nil {
			if pc, ok := ch.Plugin.Data()["prompt_caching"]; ok {
				if v, ok := pc["enabled"].(string); ok && strings.TrimSpace(v) != "" {
					own = strings.ToLower(strings.TrimSpace(v))
				}
			}
		}
		switch own {
		case "on", "off":
			return &EffectiveSetting{Value: own, Chain: []SettingSource{{Level: "object", Key: "channel.prompt_caching", Value: own, Reason: "set on the channel"}}}, nil
		}
		site := siteSource("ClaudePromptCachingEnabled")
		value := "off"
		if config.ClaudePromptCachingEnabled {
			value = "on"
		}
		return &EffectiveSetting{Value: value, Chain: []SettingSource{
			{Level: "object", Key: "channel.prompt_caching", Value: "inherit", Reason: "the channel follows the site switch"}, site,
		}}, nil
	},
	// A user group's ratio: what its users pay over the catalog price.
	"user_group.ratio": func(symbol string) (*EffectiveSetting, error) {
		g := GlobalUserGroupRatio.GetBySymbol(symbol)
		if g == nil {
			return nil, fmt.Errorf("no enabled user group %q", symbol)
		}
		value := strconv.FormatFloat(g.Ratio, 'f', -1, 64)
		reason := "set on the user group"
		if symbol == DefaultUserGroupSymbol {
			reason += "; the default group's ratio is the site's normal price"
		}
		net := strconv.FormatFloat(NetRevenueLineAt(g.Ratio, config.PaymentCostPercent), 'f', 4, 64)
		return &EffectiveSetting{Value: value, Chain: []SettingSource{
			{Level: "object", Key: "user_group.ratio", Value: value, Reason: reason + "; after the payment cost each $1 of catalog price brings in $" + net},
			siteSource("PaymentCostPercent"),
		}}, nil
	},
	// The group an API key bills with: its own, or its user's.
	"token.group": func(id string) (*EffectiveSetting, error) {
		n, err := strconv.Atoi(id)
		if err != nil {
			return nil, errors.New("object_id must be the API key's numeric id")
		}
		token, err := GetTokenById(n)
		if err != nil {
			return nil, err
		}
		if token.Group != "" {
			return &EffectiveSetting{Value: token.Group, Chain: []SettingSource{{Level: "object", Key: "token.group", Value: token.Group, Reason: "set on the API key"}}}, nil
		}
		user, err := GetUserById(token.UserId, false)
		if err != nil {
			return nil, err
		}
		return &EffectiveSetting{Value: user.Group, Chain: []SettingSource{
			{Level: "object", Key: "token.group", Value: "", Reason: "the API key follows its account's group"},
			{Level: "object", Key: "user.group", Value: user.Group, Reason: "the group of the key's account"},
		}}, nil
	},
}

func channelByIdString(id string) (*Channel, error) {
	n, err := strconv.Atoi(id)
	if err != nil {
		return nil, errors.New("object_id must be the channel's numeric id")
	}
	return GetChannelById(n)
}

// SettingOverride counts the objects that set a field of their own instead of following a site
// setting.
type SettingOverride struct {
	Count int    `json:"count"`
	Scope string `json:"scope"`
}

// SettingOverrides tells, per site setting that objects can override, how many do: channels with
// their own prompt caching switch, models with their own supply mode.
func SettingOverrides() (map[string]SettingOverride, error) {
	out := map[string]SettingOverride{}
	var channels []*Channel
	if err := DB.Select("id", "plugin").Find(&channels).Error; err != nil {
		return nil, err
	}
	n := 0
	for _, ch := range channels {
		if ch.Plugin == nil {
			continue
		}
		if pc, ok := ch.Plugin.Data()["prompt_caching"]; ok {
			if v, ok := pc["enabled"].(string); ok {
				if v = strings.ToLower(strings.TrimSpace(v)); v == "on" || v == "off" {
					n++
				}
			}
		}
	}
	out["ClaudePromptCachingEnabled"] = SettingOverride{Count: n, Scope: string(settings.ScopeChannel)}
	var models int64
	if err := DB.Model(&ModelInfo{}).Where("supply_mode IN ?", []string{SupplyModeBalance, SupplyModeFailover}).Count(&models).Error; err != nil {
		return nil, err
	}
	out["SupplyModeDefault"] = SettingOverride{Count: int(models), Scope: string(settings.ScopeModel)}
	return out, nil
}

// inheritedSiteSettings are the site settings object fields follow; admins who edit those objects
// may read them (the rest of the platform settings stay root only).
var inheritedSiteSettings = map[string]bool{"ClaudePromptCachingEnabled": true, "SupplyModeDefault": true}

// InheritedSiteSetting is the value of a site setting an object field follows, for admins.
func InheritedSiteSetting(key string) (string, bool) {
	if !inheritedSiteSettings[key] {
		return "", false
	}
	return config.GlobalOption.Get(key), true
}
