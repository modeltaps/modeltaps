package model

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"slices"
	"strings"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Channel struct {
	Id                 int      `json:"id"`
	Type               int      `json:"type" form:"type" gorm:"default:0"`
	Key                string   `json:"key" form:"key" gorm:"type:text;serializer:encrypted"` // encrypted at rest, see secret_storage.go
	Status             int      `json:"status" form:"status" gorm:"default:1"`
	Name               string   `json:"name" form:"name" gorm:"index"`
	Weight             *uint    `json:"weight" gorm:"default:1"`
	CreatedTime        int64    `json:"created_time" gorm:"bigint"`
	TestTime           int64    `json:"test_time" gorm:"bigint"`
	ResponseTime       int      `json:"response_time"` // in milliseconds
	BaseURL            *string  `json:"base_url" gorm:"column:base_url;default:''"`
	Other              string   `json:"other" form:"other"`
	Balance            float64  `json:"balance"` // in USD
	BalanceUpdatedTime int64    `json:"balance_updated_time" gorm:"bigint"`
	Models             string   `json:"models" form:"models"`
	Group              string   `json:"group" form:"group" gorm:"type:varchar(255);default:'default'"`
	Tag                string   `json:"tag" form:"tag" gorm:"type:varchar(32);default:''"`
	UsedQuota          int64    `json:"used_quota" gorm:"bigint;default:0"`
	ModelMapping       *string  `json:"model_mapping" gorm:"type:text"`
	ModelHeaders       *string  `json:"model_headers" gorm:"type:varchar(1024);default:''"`
	HeaderOverride     *string  `json:"header_override" gorm:"type:text"`
	CustomParameter    *string  `json:"custom_parameter" gorm:"type:text"`
	Priority           *int64   `json:"priority" gorm:"bigint;default:0"`
	Proxy              *string  `json:"proxy" gorm:"type:varchar(255);default:''"`
	TestModel          string   `json:"test_model" form:"test_model" gorm:"type:varchar(50);default:''"`
	OnlyChat           bool     `json:"only_chat" form:"only_chat" gorm:"default:false"`
	PreCost            int      `json:"pre_cost" form:"pre_cost" gorm:"default:1"`
	CompatibleResponse bool     `json:"compatible_response" gorm:"default:false"`
	AllowExtraBody     bool     `json:"allow_extra_body" form:"allow_extra_body" gorm:"default:false"`
	PassThroughBody    bool     `json:"pass_through_body" form:"pass_through_body" gorm:"default:false"`
	CostRatio          *float64 `json:"cost_ratio" form:"cost_ratio" gorm:"type:decimal(10,4);default:0"`
	// FeeRate is what the channel's provider charges on top of its listed prices, as a fraction
	// (0.05 = 5%), such as a top-up fee. It raises the channel cost that billing never goes below
	// (see SupplyCostPrice). Nil in an update keeps the stored value; a new channel without one
	// takes its type's default (channelFeeDefaults).
	FeeRate *float64 `json:"fee_rate" form:"fee_rate" gorm:"type:decimal(10,4);default:0"`
	// FeeNote says where the fee comes from. Admins only: users see a generic label.
	FeeNote string `json:"fee_note" form:"fee_note" gorm:"type:varchar(255);default:''"`
	// UnifiedRequestResponseModel 渠道级「响应模型名统一为请求模型名」开关；全局开关开启时对所有渠道生效。
	UnifiedRequestResponseModel bool `json:"unified_request_response_model" form:"unified_request_response_model" gorm:"default:false"`
	// ManageKey is an optional key of the same upstream account with management permission. It is
	// never used to send requests: providers read the account balance (and later the cost reports)
	// with it. Nil in an update keeps the stored value; an empty string clears it.
	ManageKey *string `json:"manage_key,omitempty" gorm:"type:text;serializer:encrypted"`
	// Remark is a free-text note for admins; it is kept per channel and not shared across a tag.
	Remark string `json:"remark" form:"remark" gorm:"type:text"`
	// Need2ResponseModels adjusts which chat models go to the Responses API on this channel
	// (see ShouldUseResponsesForModel). Empty means the built-in list.
	Need2ResponseModels *string `json:"need2response_models" form:"need2response_models" gorm:"type:text"`
	// AllowServiceTier forwards request fields that select a pricier upstream tier (OpenAI
	// service_tier priority / fast / ultrafast, Claude speed fast and inference_geo us).
	// Off by default: the gateway drops them so a client cannot raise the upstream price.
	AllowServiceTier bool `json:"allow_service_tier" form:"allow_service_tier" gorm:"default:false"`
	// CancelOnDisconnect closes the upstream stream when the client disconnects, so the upstream
	// stops generating (and billing) output nobody reads. Nil means automatic: see
	// CancelOnDisconnectDefault. Upstreams that keep billing the full output after a disconnect
	// must keep this off, or the gateway loses the final usage and undercharges.
	CancelOnDisconnect *bool `json:"cancel_on_disconnect" form:"cancel_on_disconnect" gorm:"default:null"`

	DisabledStream *datatypes.JSONSlice[string] `json:"disabled_stream,omitempty" gorm:"type:json"`
	// DisabledModels are names from Models an admin turned off: they stay listed (and keep their
	// supplies) but are not routed until turned back on. nil in an update keeps the stored list.
	DisabledModels *datatypes.JSONSlice[string] `json:"disabled_models,omitempty" gorm:"type:json"`

	Plugin     *datatypes.JSONType[PluginType]       `json:"plugin" form:"plugin" gorm:"type:json"`
	ModelDrift *datatypes.JSONType[ModelDriftResult] `json:"model_drift,omitempty" gorm:"type:json"`
	// UsageWindows is the last known usage of a subscription account (Codex, Claude Code).
	UsageWindows *datatypes.JSONType[ChannelUsageWindows] `json:"usage_windows,omitempty" gorm:"type:json"`
	// DriftIgnoreRules lists upstream models the drift check never reports as new, one per line:
	// an exact name, a glob with * (for example *-preview), or a regular expression after "re:".
	DriftIgnoreRules string `json:"drift_ignore_rules" form:"drift_ignore_rules" gorm:"type:text"`
	// PauseRules are this channel's own pause rules as JSON (see PauseRule). They are tried
	// before the site-wide rules; empty means only the site-wide rules apply. Nil in an update
	// keeps the stored value, since the console saves channels whole.
	PauseRules *string `json:"pause_rules,omitempty" gorm:"type:text"`
	// DriftAutoAdd adds new upstream models (not ignored) to the channel's models when the
	// drift check finds them, instead of only reporting them.
	DriftAutoAdd bool `json:"drift_auto_add" form:"drift_auto_add" gorm:"default:false"`
	// DriftKeepMissing keeps supplies routed when the upstream list stops showing their model:
	// for upstreams whose list is incomplete. Off, the model check marks them gone (see
	// ModelSupply.UpstreamGoneAt).
	DriftKeepMissing bool `json:"drift_keep_missing" form:"drift_keep_missing" gorm:"default:false"`
	// SupplyIntakeRules are this channel's own intake rules as JSON (see SupplyIntakeRule),
	// used instead of the site-wide ones; empty means the site-wide rules. Nil in an update keeps
	// the stored value, since the console saves channels whole.
	SupplyIntakeRules *string `json:"supply_intake_rules,omitempty" gorm:"type:text"`

	// BalanceThreshold 低余额提醒阈值（USD）：余额刷新后低于该值发通知；0 = 不提醒。
	BalanceThreshold float64 `json:"balance_threshold" form:"balance_threshold" gorm:"type:decimal(10,4);default:0"`
	// BalanceAlertedTime 上次发出低余额提醒的时间戳；余额回到阈值之上时清零，避免每轮刷新重复提醒。
	BalanceAlertedTime int64 `json:"balance_alerted_time" gorm:"bigint;default:0"`
	// UsageAlertTiers are this subscription channel's usage alert tiers in percent ("50,75,100";
	// "0" turns them off); empty uses the site's (config.UsageAlertTiers). Nil in an update keeps
	// the stored value, since the console saves channels whole.
	UsageAlertTiers *string `json:"usage_alert_tiers,omitempty" gorm:"type:varchar(64)"`
	// UsageAlerts records, per usage window, the highest tier alerted in its current period, so a
	// tier is sent once per period. Kept by the usage refresh.
	UsageAlerts *datatypes.JSONType[ChannelUsageAlerts] `json:"usage_alerts,omitempty" gorm:"type:json"`
	// QuotaSnapshot 上游 key 级配额快照（上限 / 剩余 / 重置周期 / 各窗口用量），由余额刷新写入；
	// 上游不提供这类信息的渠道为空。
	QuotaSnapshot *datatypes.JSONType[ChannelQuotaSnapshot] `json:"quota_snapshot,omitempty" gorm:"type:json"`

	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	KeyStatus *ChannelKeyStatus `json:"key_status,omitempty" gorm:"-"`
	// CancelOnDisconnectAuto is the automatic value of CancelOnDisconnect for the stored type and
	// base URL, so the console can show what "automatic" means for this channel.
	CancelOnDisconnectAuto bool `json:"cancel_on_disconnect_auto" gorm:"-"`
}

// ChannelKeyStatus 是渠道 key 的安全投影：是否已配置、条数、每条的回显（utils.MaskSecret），不含完整 key。
type ChannelKeyStatus struct {
	Configured bool     `json:"configured"`
	Count      int      `json:"count"`
	Masked     []string `json:"masked"`
	// ManageMasked is the masked management key; empty when the channel has none.
	ManageMasked string `json:"manage_masked,omitempty"`
}

const keyMaskMaxEntries = 10

// MaskChannelKey 按换行拆分 key，统计非空条数并生成掩码（最多 keyMaskMaxEntries 条）。
func MaskChannelKey(key string) ChannelKeyStatus {
	status := ChannelKeyStatus{Masked: []string{}}
	for _, line := range strings.Split(key, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		status.Count++
		if len(status.Masked) < keyMaskMaxEntries {
			status.Masked = append(status.Masked, utils.MaskSecret(line))
		}
	}
	status.Configured = status.Count > 0
	return status
}

// HideKey 用掩码状态替换完整 key，供接口返回前调用。
func (channel *Channel) HideKey() {
	status := MaskChannelKey(channel.Key)
	if manageKey := channel.GetManageKey(); manageKey != "" {
		status.ManageMasked = utils.MaskSecret(manageKey)
	}
	channel.KeyStatus = &status
	channel.Key = ""
	channel.ManageKey = nil
}

// GetManageKey returns the channel's management key, or "" when it has none.
func (channel *Channel) GetManageKey() string {
	if channel.ManageKey == nil {
		return ""
	}
	return strings.TrimSpace(*channel.ManageKey)
}

// ModelDriftResult 记录一次「渠道 models vs 上游实时模型列表」对比结果。
// MissingModels 为渠道配置了、但上游已不再返回的模型名（对外别名按 model_mapping 解析为上游名后对比）。
// NewModels 为上游相对「上次检测快照」新出现的模型名：检测会把它们加入渠道列表，标记保留到管理员忽略、开关或移除为止。
// UpstreamSnapshot 为本次上游返回的完整模型 id 列表，作为下次检测判定「新增」的基线。
type ModelDriftResult struct {
	CheckedAt        int64    `json:"checked_at"`
	MissingModels    []string `json:"missing_models"`
	NewModels        []string `json:"new_models,omitempty"`
	UpstreamSnapshot []string `json:"upstream_snapshot,omitempty"`
	// DismissedMissing are missing models an admin dismissed: they are no longer flagged. The
	// check keeps only the ones still missing, so a model that comes back and goes again is
	// flagged again, and so is any model that goes missing after the dismissal.
	DismissedMissing []string `json:"dismissed_missing,omitempty"`
	// Removed are upstream names an admin removed from the channel's list: the check does not
	// add them back while they stay removed (see JoinUpstreamModels). Listing one again forgets it.
	Removed []string `json:"removed,omitempty"`
	OK      bool     `json:"ok"`
	// Supplies is what the check changed on the channel's supplies while they route requests:
	// supplies marked gone or back (see MarkSupplyUpstreams); nil when nothing changed.
	Supplies *SupplyUpstreamChange `json:"supplies,omitempty"`
	// Intake is what the intake rules did with the new upstream models at this check.
	Intake []IntakeOutcome `json:"intake,omitempty"`
}

func (c *Channel) AllowStream(modelName string) bool {
	if c.DisabledStream == nil {
		return true
	}

	return !slices.Contains(*c.DisabledStream, modelName)
}

// DisabledModelSet is the set of listed models the channel does not route. Names the model
// list no longer has are left out.
func (c *Channel) DisabledModelSet() map[string]bool {
	if c.DisabledModels == nil || len(*c.DisabledModels) == 0 {
		return nil
	}
	listed := map[string]bool{}
	for _, name := range strings.Split(c.Models, ",") {
		listed[strings.TrimSpace(name)] = true
	}
	set := make(map[string]bool, len(*c.DisabledModels))
	for _, name := range *c.DisabledModels {
		if name = strings.TrimSpace(name); name != "" && listed[name] {
			set[name] = true
		}
	}
	return set
}

// DisabledUpstreams are the upstream names that only disabled models of the channel send: a
// supply of the channel with one of them as its upstream model is not routed. An upstream name
// that an enabled model also maps to stays routed.
func (c *Channel) DisabledUpstreams() map[string]bool {
	disabled := c.DisabledModelSet()
	if len(disabled) == 0 {
		return nil
	}
	mapping := channelMappingOf(c)
	upstreamOf := func(name string) string {
		if up, ok := mapping[name]; ok {
			return up
		}
		return name
	}
	enabled := map[string]bool{}
	for _, name := range strings.Split(c.Models, ",") {
		if name = strings.TrimSpace(name); name != "" && !disabled[name] {
			enabled[upstreamOf(name)] = true
		}
	}
	out := map[string]bool{}
	for name := range disabled {
		if up := upstreamOf(name); !enabled[up] {
			out[up] = true
		}
	}
	return out
}

// pruneDisabledModels drops disabled names the model list no longer has, so a model removed and
// added again comes back enabled. A partial update without a model list keeps them all.
func (c *Channel) pruneDisabledModels() {
	if c.DisabledModels == nil || strings.TrimSpace(c.Models) == "" {
		return
	}
	listed := map[string]bool{}
	for _, name := range strings.Split(c.Models, ",") {
		listed[strings.TrimSpace(name)] = true
	}
	kept := datatypes.JSONSlice[string]{}
	for _, name := range *c.DisabledModels {
		if name = strings.TrimSpace(name); name != "" && listed[name] && !slices.Contains(kept, name) {
			kept = append(kept, name)
		}
	}
	c.DisabledModels = &kept
}

type PluginType map[string]map[string]interface{}

var allowedChannelOrderFields = map[string]bool{
	"id":            true,
	"name":          true,
	"group":         true,
	"type":          true,
	"status":        true,
	"response_time": true,
	"balance":       true,
	"used_quota":    true,
	"priority":      true,
	"weight":        true,
	"cost_ratio":    true,
}

// likeEscaper escapes LIKE wildcards for use with ESCAPE '!', which MySQL, PostgreSQL and SQLite all support.
var likeEscaper = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

type SearchChannelsParams struct {
	Channel
	PaginationParams
	FilterTag int    `json:"filter_tag" form:"filter_tag"`
	BaseURL   string `json:"base_url" form:"base_url"`
	// ExactModel matches channels whose comma-separated models list contains exactly this model.
	ExactModel string `json:"exact_model" form:"exact_model"`
}

func GetChannelsList(params *SearchChannelsParams) (*DataResult[Channel], error) {
	var channels []*Channel

	db := DB
	// 代表行取组内最小 id，与 GetChannelsTag 取 channels[0]（Order id ASC）保持一致
	tagDB := DB.Model(&Channel{}).Select("MIN(id) as id").Where("tag != ''").Group("tag")

	if params.Type != 0 {
		db = db.Where("type = ?", params.Type)
		tagDB = tagDB.Where("type = ?", params.Type)
	}

	if params.Status != 0 {
		db = db.Where("status = ?", params.Status)
		tagDB = tagDB.Where("status = ?", params.Status)
	}

	if params.Name != "" {
		like := "%" + params.Name + "%"
		db = db.Where(caseInsensitiveLike("name LIKE ? OR tag LIKE ?"), like, like)
		tagDB = tagDB.Where(caseInsensitiveLike("name LIKE ? OR tag LIKE ?"), like, like)
	}

	if params.Group != "" {
		groupKey := quotePostgresField("group")
		// A channel open to all groups matches every group filter.
		db = db.Where("( "+groupKey+" LIKE ? OR "+groupKey+" LIKE ? OR "+groupKey+" LIKE ? OR "+groupKey+" = ? OR "+groupKey+" = ?)",
			"%,"+params.Group+",%", params.Group+",%", "%,"+params.Group, params.Group, ChannelGroupAll)
		tagDB = tagDB.Where("( "+groupKey+" LIKE ? OR "+groupKey+" LIKE ? OR "+groupKey+" LIKE ? OR "+groupKey+" = ? OR "+groupKey+" = ?)",
			"%,"+params.Group+",%", params.Group+",%", "%,"+params.Group, params.Group, ChannelGroupAll)
	}

	if params.Models != "" {
		db = db.Where(caseInsensitiveLike("models LIKE ?"), "%"+params.Models+"%")
		tagDB = tagDB.Where(caseInsensitiveLike("models LIKE ?"), "%"+params.Models+"%")
	}

	// Match a whole entry of the comma-separated models list, so gemini-2.5-flash does not also hit
	// gemini-2.5-flash-image and push the channels actually wanted onto later pages. _ is a LIKE
	// wildcard and must be escaped, or mj_imagine would also match mj-imagine.
	if params.ExactModel != "" {
		escaped := likeEscaper.Replace(params.ExactModel)
		cond := "(models = ? OR models LIKE ? ESCAPE '!' OR models LIKE ? ESCAPE '!' OR models LIKE ? ESCAPE '!')"
		args := []any{params.ExactModel, escaped + ",%", "%," + escaped, "%," + escaped + ",%"}
		db = db.Where(cond, args...)
		tagDB = tagDB.Where(cond, args...)
	}

	if params.Other != "" {
		db = db.Where(caseInsensitiveLike("other LIKE ?"), params.Other+"%")
		tagDB = tagDB.Where(caseInsensitiveLike("other LIKE ?"), params.Other+"%")
	}

	if params.Key != "" {
		ids, err := channelIDsWithKey(params.Key)
		if err != nil {
			return nil, err
		}
		db = db.Where("id IN ?", ids)
		tagDB = tagDB.Where("id IN ?", ids)
	}

	if params.TestModel != "" {
		db = db.Where(caseInsensitiveLike("test_model LIKE ?"), params.TestModel+"%")
		tagDB = tagDB.Where(caseInsensitiveLike("test_model LIKE ?"), params.TestModel+"%")
	}

	if params.BaseURL != "" {
		db = db.Where(caseInsensitiveLike("base_url LIKE ?"), "%"+params.BaseURL+"%")
		tagDB = tagDB.Where(caseInsensitiveLike("base_url LIKE ?"), "%"+params.BaseURL+"%")
	}

	if params.Tag != "" {
		db = db.Where("tag = ?", params.Tag)
		tagDB = tagDB.Where("tag = ?", params.Tag)
	}

	switch params.FilterTag {
	case 1:
		db = db.Where("tag = ''")
	case 2:
		db = db.Where("id IN (?)", tagDB)
	default:
		db = db.Where("tag = '' OR id IN (?)", tagDB)
	}

	// 标签代表行只是组内最小 id 的那条渠道，默认排序会按它「自身」的列值排位；
	// 但「已使用/余额/响应时间」展示的是整组合计/平均、「名称」展示的是标签名，
	// 直接按自身值排会与显示值错位。这里对这几列改用标签感知表达式：
	// 代表行按整组聚合排序、普通渠道仍按自身值，使排序与所见一致。
	if order := strings.TrimSpace(params.Order); order != "" {
		field, dir := order, "ASC"
		if strings.HasPrefix(field, "-") {
			field, dir = field[1:], "DESC"
		}
		if expr := tagAwareOrderExpr(field, dir); expr != "" {
			db = db.Order(expr)
			params.Order = "" // 已自定义排序，避免 PaginateAndOrder 对该列重复套用
		}
	}

	result, err := PaginateAndOrder(db, &params.PaginationParams, &channels, allowedChannelOrderFields)
	if err != nil {
		return nil, err
	}
	for _, channel := range channels {
		channel.HideKey()
	}
	return result, nil
}

// tagAwareOrderExpr 为「展示值=整组聚合」的列返回标签感知的排序表达式：标签代表行
// (tag != ”) 按整组聚合排序，普通渠道 (tag = ”) 仍按自身列值；聚合口径与
// GetChannelsTagAllList 的 _all 统计保持一致（SUM/SUM/已测平均）。返回 "" 表示该列
// 代表行展示的就是自身值，无需特殊处理，交由通用排序。dir 取 "ASC"/"DESC"。
func tagAwareOrderExpr(field, dir string) string {
	var expr string
	switch field {
	case "used_quota":
		expr = "CASE WHEN channels.tag = '' THEN channels.used_quota " +
			"ELSE (SELECT SUM(c2.used_quota) FROM channels c2 WHERE c2.tag = channels.tag) END"
	case "balance":
		expr = "CASE WHEN channels.tag = '' THEN channels.balance " +
			"ELSE (SELECT SUM(c2.balance) FROM channels c2 WHERE c2.tag = channels.tag) END"
	case "response_time":
		expr = "CASE WHEN channels.tag = '' THEN channels.response_time " +
			"ELSE (SELECT AVG(CASE WHEN c2.response_time > 0 THEN c2.response_time ELSE NULL END) " +
			"FROM channels c2 WHERE c2.tag = channels.tag) END"
	case "name":
		expr = "CASE WHEN channels.tag = '' THEN channels.name ELSE channels.tag END"
	default:
		return ""
	}
	return expr + " " + dir
}

func GetAllChannels() ([]*Channel, error) {
	var channels []*Channel
	err := DB.Order("id desc").Find(&channels).Error
	return channels, err
}

func GetChannelById(id int) (*Channel, error) {
	channel := Channel{Id: id}
	err := DB.First(&channel, "id = ?", id).Error

	return &channel, err
}

func GetChannelsByTag(tag string) ([]*Channel, error) {
	var channels []*Channel
	err := DB.Where("tag = ?", tag).Find(&channels).Error
	return channels, err
}

func DeleteChannelTag(channelId int) error {
	result := DB.Model(&Channel{}).Where("id = ?", channelId).Update("tag", "")
	if result.Error == nil && result.RowsAffected > 0 {
		ChannelGroup.Load()
	}
	return result.Error
}

func BatchDeleteChannel(ids []int) (int64, error) {
	result := DB.Where("id IN ?", ids).Delete(&Channel{})
	if result.Error == nil && result.RowsAffected > 0 {
		cleanUpDeletedChannels(ids)
		ChannelGroup.Load()
	}
	return result.RowsAffected, result.Error
}

// cleanUpDeletedChannels removes what belongs to deleted channels: their aliases, their price
// lists and their supplies. A supply of a deleted channel never routes, but it would still show
// on the model page and in statistics.
func cleanUpDeletedChannels(ids []int) {
	if len(ids) == 0 {
		return
	}
	deleteChannelAliases(ids)
	deleteChannelPrices(ids)
	if err := DB.Where("channel_id IN ?", ids).Delete(&ModelSupply{}).Error; err != nil {
		logger.SysError("failed to delete supplies of deleted channels: " + err.Error())
	}
}

func BatchInsertChannels(channels []Channel) error {
	for i := range channels {
		channels[i].Group = NormalizeChannelGroup(channels[i].Group)
		channels[i].applyFeeDefault()
	}
	err := DB.Omit("UsedQuota").Create(&channels).Error
	if err != nil {
		return err
	}

	ChannelGroup.Load()
	return nil
}

type BatchChannelsParams struct {
	Value string `json:"value" form:"value" binding:"required"`
	Ids   []int  `json:"ids" form:"ids" binding:"required"`
}

func BatchUpdateChannelsAzureApi(params *BatchChannelsParams) (int64, error) {
	db := DB.Model(&Channel{}).Where("id IN ?", params.Ids).Update("other", params.Value)
	if db.Error != nil {
		return 0, db.Error
	}

	if db.RowsAffected > 0 {
		ChannelGroup.Load()
	}
	return db.RowsAffected, nil
}

func BatchDelModelChannels(params *BatchChannelsParams) (int64, error) {
	var count int64

	var channels []*Channel
	err := DB.Select("id, models, "+quotePostgresField("group")).Find(&channels, "id IN ?", params.Ids).Error
	if err != nil {
		return 0, err
	}

	for _, channel := range channels {
		_, listBefore, listed := loadModelListState(channel.Id)
		modelsSlice := strings.Split(channel.Models, ",")
		for i, m := range modelsSlice {
			if m == params.Value {
				modelsSlice = append(modelsSlice[:i], modelsSlice[i+1:]...)
				break
			}
		}

		channel.Models = strings.Join(modelsSlice, ",")
		channel.UpdateRaw(false)
		if listed {
			if err := noteModelListChange(channel.Id, listBefore); err != nil {
				logger.SysError("failed to note model list change of channel " + channel.Name + ": " + err.Error())
			}
		}
		count++
	}

	if count > 0 {
		ChannelGroup.Load()
	}

	return count, nil
}

// BatchAddUserGroupToChannels 批量添加用户分组到渠道
func BatchAddUserGroupToChannels(params *BatchChannelsParams) (int64, error) {
	var count int64

	var channels []*Channel
	err := DB.Select("id, "+quotePostgresField("group")).Find(&channels, "id IN ?", params.Ids).Error
	if err != nil {
		return 0, err
	}

	for _, channel := range channels {
		// A channel open to all groups already includes the group.
		if ChannelGroupIsAll(channel.Group) {
			continue
		}
		// 获取当前渠道的用户分组列表
		currentGroups := strings.Split(channel.Group, ",")

		// 清理空字符串并去重
		uniqueGroups := make(map[string]bool)
		for _, group := range currentGroups {
			group = strings.TrimSpace(group)
			if group != "" {
				uniqueGroups[group] = true
			}
		}

		// 检查要添加的分组是否已存在
		newGroup := strings.TrimSpace(params.Value)
		if newGroup == ChannelGroupAll {
			// Adding "all groups" replaces the list instead of joining it.
			if err = DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("group", ChannelGroupAll).Error; err != nil {
				return count, err
			}
			count++
			continue
		}
		if newGroup != "" && !uniqueGroups[newGroup] {
			// 分组不存在，添加到渠道
			uniqueGroups[newGroup] = true

			// 重新构建分组字符串
			var groupSlice []string
			for group := range uniqueGroups {
				groupSlice = append(groupSlice, group)
			}

			newGroupString := strings.Join(groupSlice, ",")

			// 更新渠道分组
			err = DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("group", newGroupString).Error
			if err != nil {
				return count, err
			}
			count++
		}
	}

	if count > 0 {
		ChannelGroup.Load()
	}

	return count, nil
}

// BatchAddModelToChannels 批量添加模型到渠道
func BatchAddModelToChannels(params *BatchChannelsParams) (int64, error) {
	var count int64

	var channels []*Channel
	err := DB.Select("id, models").Find(&channels, "id IN ?", params.Ids).Error
	if err != nil {
		return 0, err
	}

	// 解析要添加的模型列表（支持逗号分隔的多个模型）
	newModels := strings.Split(params.Value, ",")
	var trimmedNewModels []string
	for _, model := range newModels {
		model = strings.TrimSpace(model)
		if model != "" {
			trimmedNewModels = append(trimmedNewModels, model)
		}
	}

	if len(trimmedNewModels) == 0 {
		return 0, nil
	}

	for _, channel := range channels {
		// 获取当前渠道的模型列表
		currentModels := strings.Split(channel.Models, ",")

		// 清理空字符串并去重
		uniqueModels := make(map[string]bool)
		for _, model := range currentModels {
			model = strings.TrimSpace(model)
			if model != "" {
				uniqueModels[model] = true
			}
		}

		// 检查要添加的模型，只添加不存在的模型
		hasNewModel := false
		for _, newModel := range trimmedNewModels {
			if !uniqueModels[newModel] {
				uniqueModels[newModel] = true
				hasNewModel = true
			}
		}

		// 如果有新模型添加，则更新渠道
		if hasNewModel {
			// 重新构建模型字符串
			var modelSlice []string
			for model := range uniqueModels {
				modelSlice = append(modelSlice, model)
			}

			newModelString := strings.Join(modelSlice, ",")

			// 更新渠道模型
			err = DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("models", newModelString).Error
			if err != nil {
				return count, err
			}
			count++
		}
	}

	if count > 0 {
		ChannelGroup.Load()
	}

	return count, nil
}

func (c *Channel) SetProxy() {
	if c.Proxy == nil {
		return
	}

	if strings.Contains(*c.Proxy, "%s") {
		md5Str := md5.Sum([]byte(c.Key))
		idStr := hex.EncodeToString(md5Str[:])
		*c.Proxy = strings.Replace(*c.Proxy, "%s", idStr, 1)
	}

}

func (c *Channel) GetProxy() string {
	if c.Proxy == nil {
		return ""
	}
	return *c.Proxy
}

func (channel *Channel) GetPriority() int64 {
	if channel.Priority == nil {
		return 0
	}
	return *channel.Priority
}

// GetWeight 兜住 weight 为 NULL 的渠道：管理员更新渠道不带 weight 时该列会被写成 NULL，
// 按默认权重参与负载均衡，与「0 视为默认权重」的既有语义一致。
func (channel *Channel) GetWeight() int64 {
	if channel.Weight == nil {
		return int64(config.DefaultChannelWeight)
	}
	return int64(*channel.Weight)
}

func (channel *Channel) GetCostRatio() float64 {
	if channel.CostRatio == nil || *channel.CostRatio <= 0 {
		return 0
	}
	return *channel.CostRatio
}

// GetFeeRate is the channel's fee rate, 0 when unset or negative.
func (channel *Channel) GetFeeRate() float64 {
	if channel == nil || channel.FeeRate == nil || *channel.FeeRate <= 0 {
		return 0
	}
	return *channel.FeeRate
}

func (channel *Channel) GetBaseURL() string {
	if channel.BaseURL == nil {
		return ""
	}
	return *channel.BaseURL
}

func (channel *Channel) GetModelMapping() string {
	if channel.ModelMapping == nil {
		return ""
	}
	return *channel.ModelMapping
}

func (channel *Channel) GetCustomParameter() string {
	if channel.CustomParameter == nil {
		return ""
	}
	return *channel.CustomParameter
}

func (channel *Channel) Insert() error {
	channel.Group = NormalizeChannelGroup(channel.Group)
	channel.applyFeeDefault()
	err := DB.Omit("UsedQuota").Create(channel).Error
	if err == nil {
		ChannelGroup.Load()
	}

	return err
}

func (channel *Channel) Update(overwrite bool) error {
	// The numbers before the change, so supplies that follow the channel can move with it.
	before, _ := channelNumbers(DB.Where("id = ?", channel.Id))
	_, listBefore, listed := loadModelListState(channel.Id)

	err := channel.UpdateRaw(overwrite)

	if err == nil {
		// Remember removed models so the model check does not add them back.
		if listed {
			if err := noteModelListChange(channel.Id, listBefore); err != nil {
				logger.SysError("failed to note model list change of channel " + channel.Name + ": " + err.Error())
			}
		}
		if old, ok := before[channel.Id]; ok {
			if _, syncErr := SyncSuppliesWithChannel(channel.Id, old[0], channel.GetPriority(), old[1], channel.GetWeight()); syncErr != nil {
				logger.SysError("failed to sync supplies of channel " + channel.Name + ": " + syncErr.Error())
			}
		}
		ChannelGroup.Load()
		ChannelGroup.ClearChannelCooldowns(channel.Id)
	}

	return err
}

func (channel *Channel) UpdateRaw(overwrite bool) error {
	var err error
	// An empty group is left out of a partial update, so only normalize a value that is there.
	if channel.Group != "" {
		channel.Group = NormalizeChannelGroup(channel.Group)
	}

	if overwrite {
		// 运行时字段由刷新 / 检测任务维护，表单保存不得用过期值覆盖。
		// CreatedTime, TestTime and ResponseTime are not form fields either.
		omit := []string{"UsedQuota", "Balance", "BalanceUpdatedTime", "BalanceAlertedTime", "QuotaSnapshot", "ModelDrift", "UsageWindows",
			"UsageAlerts", "CreatedTime", "TestTime", "ResponseTime"}
		// 编辑时 key 留空表示保留原 key，Select("*") 不能把它覆盖成空串
		if channel.Key == "" {
			omit = append(omit, "Key")
		}
		if channel.ManageKey == nil {
			omit = append(omit, "ManageKey")
		}
		if channel.FeeRate == nil {
			omit = append(omit, "FeeRate", "FeeNote")
		}
		if channel.PauseRules == nil {
			omit = append(omit, "PauseRules")
		}
		if channel.UsageAlertTiers == nil {
			omit = append(omit, "UsageAlertTiers")
		}
		if channel.SupplyIntakeRules == nil {
			omit = append(omit, "SupplyIntakeRules")
		}
		if channel.DisabledModels == nil {
			omit = append(omit, "DisabledModels")
		} else {
			channel.pruneDisabledModels()
		}
		err = DB.Model(channel).Select("*").Omit(omit...).Updates(channel).Error
	} else {
		err = DB.Model(channel).Omit("UsedQuota").Updates(channel).Error
		// The fee note comes with the fee rate, and an empty note clears it: a struct update
		// skips empty strings.
		if err == nil && channel.FeeRate != nil {
			err = DB.Model(channel).Update("fee_note", channel.FeeNote).Error
		}
	}
	if err != nil {
		return err
	}
	DB.Model(channel).First(channel, "id = ?", channel.Id)
	return err
}

func (channel *Channel) UpdateResponseTime(responseTime int64) {
	err := DB.Model(channel).Select("response_time", "test_time").Updates(Channel{
		TestTime:     utils.GetTimestamp(),
		ResponseTime: int(responseTime),
	}).Error
	if err != nil {
		logger.SysError("failed to update response time: " + err.Error())
	}
}

// GetModelDrift 返回上次漂移检测结果，未检测过返回 nil。
func (channel *Channel) GetModelDrift() *ModelDriftResult {
	if channel.ModelDrift == nil {
		return nil
	}
	result := channel.ModelDrift.Data()
	return &result
}

// DismissNewModels 清空渠道 NewModels，保留 MissingModels 与上游快照（供管理员忽略新增模型）。
func (channel *Channel) DismissNewModels() error {
	result := channel.GetModelDrift()
	if result == nil {
		return nil
	}
	if len(result.NewModels) == 0 {
		return nil
	}
	result.NewModels = nil
	return channel.UpdateModelDrift(*result)
}

// DismissMissingModels stops flagging the models the last check found missing, keeping the
// rest of the result. Models that go missing later are flagged again.
func (channel *Channel) DismissMissingModels() error {
	result := channel.GetModelDrift()
	if result == nil || len(result.MissingModels) == 0 {
		return nil
	}
	result.DismissedMissing = append([]string(nil), result.MissingModels...)
	return channel.UpdateModelDrift(*result)
}

// RemoveNewModels takes names off the drift check's new models, keeping the rest of the result.
func (channel *Channel) RemoveNewModels(names ...string) error {
	result := channel.GetModelDrift()
	if result == nil || len(result.NewModels) == 0 {
		return nil
	}
	kept := make([]string, 0, len(result.NewModels))
	for _, m := range result.NewModels {
		if !slices.Contains(names, m) {
			kept = append(kept, m)
		}
	}
	if len(kept) == len(result.NewModels) {
		return nil
	}
	if len(kept) == 0 {
		kept = nil
	}
	result.NewModels = kept
	return channel.UpdateModelDrift(*result)
}

// UpdateModelDrift 只更新渠道漂移结果列，不触及其它字段（供漂移检测 cron / 手动触发写回）。
func (channel *Channel) UpdateModelDrift(result ModelDriftResult) error {
	drift := datatypes.NewJSONType(result)
	err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("model_drift", drift).Error
	if err != nil {
		logger.SysError("failed to update channel model drift: " + err.Error())
		return err
	}
	channel.ModelDrift = &drift
	return nil
}

func (channel *Channel) UpdateBalance(balance float64) {
	now := utils.GetTimestamp()
	err := DB.Model(channel).Select("balance_updated_time", "balance").Updates(Channel{
		BalanceUpdatedTime: now,
		Balance:            balance,
	}).Error
	if err != nil {
		logger.SysError("failed to update balance: " + err.Error())
		return
	}
	channel.Balance = balance
	channel.BalanceUpdatedTime = now
}

// ChannelQuotaSnapshot 是上游按 key 返回的配额信息（目前只有 OpenRouter `GET /api/v1/key` 提供）。
// 金额单位 USD；Limit / Remaining 为 nil 表示该 key 没有设上限。
type ChannelQuotaSnapshot struct {
	Limit         *float64 `json:"limit"`
	Remaining     *float64 `json:"remaining"`
	ResetInterval string   `json:"reset_interval"` // daily / weekly / monthly / ""（不重置）
	Usage         float64  `json:"usage"`
	UsageDaily    float64  `json:"usage_daily"`
	UsageWeekly   float64  `json:"usage_weekly"`
	UsageMonthly  float64  `json:"usage_monthly"`
	IsFreeTier    bool     `json:"is_free_tier"`
	// AccountBalance is what the whole upstream account has left, read with the management key;
	// nil when the channel has no management key or the upstream has no such figure.
	AccountBalance *float64 `json:"account_balance,omitempty"`
	FetchedAt      int64    `json:"fetched_at"`
}

// UpdateQuotaSnapshot 只更新配额快照列，不触及其它字段。
func (channel *Channel) UpdateQuotaSnapshot(snapshot ChannelQuotaSnapshot) error {
	jt := datatypes.NewJSONType(snapshot)
	err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("quota_snapshot", jt).Error
	if err != nil {
		logger.SysError("failed to update channel quota snapshot: " + err.Error())
		return err
	}
	channel.QuotaSnapshot = &jt
	return nil
}

// GetQuotaSnapshot 返回上次的配额快照，没有返回 nil。
func (channel *Channel) GetQuotaSnapshot() *ChannelQuotaSnapshot {
	if channel.QuotaSnapshot == nil {
		return nil
	}
	snapshot := channel.QuotaSnapshot.Data()
	if snapshot.FetchedAt == 0 {
		return nil
	}
	return &snapshot
}

// UpdateBalanceAlertedTime 记录（或清零）低余额提醒时间，只动这一列。
func (channel *Channel) UpdateBalanceAlertedTime(ts int64) error {
	err := DB.Model(&Channel{}).Where("id = ?", channel.Id).Update("balance_alerted_time", ts).Error
	if err != nil {
		logger.SysError("failed to update channel balance alerted time: " + err.Error())
		return err
	}
	channel.BalanceAlertedTime = ts
	return nil
}

func (channel *Channel) Delete() error {
	err := DB.Delete(channel).Error
	if err == nil {
		cleanUpDeletedChannels([]int{channel.Id})
		ChannelGroup.Load()
	}
	return err
}

func (channel *Channel) StatusToStr() string {
	switch channel.Status {
	case config.ChannelStatusEnabled:
		return "Enabled"
	case config.ChannelStatusAutoDisabled:
		return "Auto-disabled"
	case config.ChannelStatusManuallyDisabled:
		return "Manually disabled"
	}

	return "Disabled"
}

// EnableAutoDisabledChannel re-enables a channel only while it is still disabled automatically,
// in one conditional update, and reports whether this call changed it. When several nodes run
// the recovery check at the same moment, exactly one of them gets true.
func EnableAutoDisabledChannel(id int) (bool, error) {
	result := DB.Model(&Channel{}).
		Where("id = ? AND status = ?", id, config.ChannelStatusAutoDisabled).
		Update("status", config.ChannelStatusEnabled)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	go ChannelGroup.ChangeStatus(id, true)
	ChannelGroup.ClearChannelCooldowns(id)
	return true, nil
}

func UpdateChannelStatusById(id int, status int) {
	tx := DB.Begin()
	err := tx.Model(&Channel{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		logger.SysError("failed to update channel status: " + err.Error())
		tx.Rollback()
		return
	}

	tx.Commit()

	isEnabled := status == config.ChannelStatusEnabled
	go ChannelGroup.ChangeStatus(id, isEnabled)

	// 启用渠道时清除冻结缓存
	if isEnabled {
		ChannelGroup.ClearChannelCooldowns(id)
	}
}

func UpdateChannelUsedQuota(id int, quota int) {
	if config.BatchUpdateEnabled {
		addNewRecord(BatchUpdateTypeChannelUsedQuota, id, quota)
		return
	}
	updateChannelUsedQuota(id, quota)
}

func updateChannelUsedQuota(id int, quota int) {
	err := DB.Model(&Channel{}).Where("id = ?", id).Update("used_quota", gorm.Expr("used_quota + ?", quota)).Error
	if err != nil {
		logger.SysError("failed to update channel used quota: " + err.Error())
	}
}

func ClearChannelTokenCache(channelId int) {
	cacheKeys := []string{
		fmt.Sprintf("api_token:codex:%d", channelId),
		fmt.Sprintf("api_token:geminicli:%d", channelId),
		fmt.Sprintf("api_token:claudecode:%d", channelId),
		fmt.Sprintf("api_token:vertexai:%d", channelId),
		fmt.Sprintf("api_token:gemini_sa:%d", channelId),
		fmt.Sprintf("api_token:antigravity:%d", channelId),
	}

	for _, key := range cacheKeys {
		if err := cache.DeleteCache(key); err != nil {
			logger.SysError(fmt.Sprintf("failed to clear token cache %s: %v", key, err))
		}
	}
}

// channelIDsWithKey finds the channels whose key equals key. The key column may be encrypted,
// and ciphertext differs on every write, so the keys are compared after loading, not in SQL.
func channelIDsWithKey(key string) ([]int, error) {
	var channels []Channel
	if err := DB.Select("id", "key").Find(&channels).Error; err != nil {
		return nil, err
	}
	ids := []int{}
	for _, channel := range channels {
		if channel.Key == key {
			ids = append(ids, channel.Id)
		}
	}
	return ids, nil
}

func UpdateChannelKey(id int, key string) error {
	// A struct update, so the key goes through the encrypted serializer (a map would not).
	err := DB.Model(&Channel{}).Where("id = ?", id).Select("key").Updates(&Channel{Key: key}).Error
	if err != nil {
		logger.SysError("failed to update channel key: " + err.Error())
		return err
	}

	ClearChannelTokenCache(id)
	ChannelGroup.Load()

	return nil
}

func DeleteDisabledChannel() (int64, error) {
	var ids []int
	if err := DB.Model(&Channel{}).Where("status = ? or status = ?", config.ChannelStatusAutoDisabled, config.ChannelStatusManuallyDisabled).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.Where("id IN ?", ids).Delete(&Channel{})
	if result.Error == nil && result.RowsAffected > 0 {
		cleanUpDeletedChannels(ids)
		ChannelGroup.Load()
	}
	return result.RowsAffected, result.Error
}

type ChannelStatistics struct {
	TotalChannels int `json:"total_channels"`
	Status        int `json:"status"`
}

func GetStatisticsChannel() (statistics []*ChannelStatistics, err error) {
	err = DB.Model(&Channel{}).Select("count(*) as total_channels, status").Group("status").Scan(&statistics).Error
	return statistics, err
}
