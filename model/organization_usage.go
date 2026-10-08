package model

import (
	"errors"
	"strings"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/gorm"
)

// orgMemberIDExpr 返回从 logs.metadata 提取 org_member_id 的 SQL 表达式(按数据库方言)。
// 组织令牌的消费日志记在影子账户名下,实际成员 ID 存于 metadata(见 relay_util.Quota.GetLogMeta)。
func orgMemberIDExpr() string {
	if common.UsingPostgreSQL {
		return "(metadata ->> 'org_member_id')::int"
	}
	// MySQL 与 SQLite 均支持 JSON_EXTRACT(SQLite 函数名大小写不敏感)
	return "JSON_EXTRACT(metadata, '$.org_member_id')"
}

// cachedReadTokensExpr 返回从 logs.metadata 提取 cached_read_tokens 的 SQL 表达式(按数据库方言)。
// 缓存读取额外 token 记在 metadata(见 relay_util.Quota.GetLogMeta);缺失时归零以便 sum 累加。
// 命中率口径:cached_read_tokens / prompt_tokens,分母(prompt_tokens)为 logs 专用列。
func cachedReadTokensExpr() string {
	if common.UsingPostgreSQL {
		return "COALESCE((metadata ->> 'cached_read_tokens')::bigint, 0)"
	}
	// MySQL 与 SQLite 均支持 JSON_EXTRACT(SQLite 函数名大小写不敏感)
	return "COALESCE(JSON_EXTRACT(metadata, '$.cached_read_tokens'), 0)"
}

// savedQuotaExpr 返回单行"折扣节省额"的 SQL 表达式(按数据库方言),外层需以 SUM() 聚合。
// 口径与日志单行划线原价一致:group_ratio<1 时 original(=quota/group_ratio) 与实收 quota 的差额;
// 无折扣(group_ratio>=1、缺失或为 0)时归零,不产生负节省。gr>0 守卫兼作除零保护。
func savedQuotaExpr() string {
	var gr string
	if common.UsingPostgreSQL {
		gr = "(metadata ->> 'group_ratio')::numeric"
	} else {
		// MySQL 与 SQLite 均支持 JSON_EXTRACT(SQLite 函数名大小写不敏感)
		gr = "JSON_EXTRACT(metadata, '$.group_ratio')"
	}
	return "CASE WHEN " + gr + " > 0 AND " + gr + " < 1 THEN quota / " + gr + " - quota ELSE 0 END"
}

// OrgLogsListParams 组织日志查询参数;MemberId 按 metadata 中实际成员过滤
type OrgLogsListParams struct {
	PaginationParams
	LogType        int    `form:"log_type"`
	StartTimestamp int64  `form:"start_timestamp"`
	EndTimestamp   int64  `form:"end_timestamp"`
	ModelName      string `form:"model_name"`
	TokenName      string `form:"token_name"`
	MemberId       int    `form:"member_id"`
	// W8-E 新增过滤:与 admin/self 列表路径(model/log.go LogsListParams)语义一致。
	FinishReason string `form:"finish_reason"`
	MinQuota     int    `form:"min_quota"`
	MinTokens    int    `form:"min_tokens"`
	// W9-M10 App 归因过滤:按 metadata.app_name 精确过滤(空表示不过滤)。
	AppName string `form:"app_name"`

	// W9-D 级联筛选:多值包含(IN)与排除(NOT IN),口径同 admin/self 路径。member 走 metadata 表达式。
	ModelNames    []string `form:"model_names"`
	TokenNames    []string `form:"token_names"`
	MemberIds     []int    `form:"member_ids"`
	FinishReasons []string `form:"finish_reasons"`
	AppNames      []string `form:"app_names"`
	// 消费模态过滤:口径同 admin/self 路径(metadata.relay_mode 多值包含/排除)。
	RelayModes []string `form:"relay_modes"`

	ExcludeModelNames    []string `form:"exclude_model_name"`
	ExcludeTokenNames    []string `form:"exclude_token_name"`
	ExcludeMemberIds     []int    `form:"exclude_member_id"`
	ExcludeFinishReasons []string `form:"exclude_finish_reason"`
	ExcludeAppNames      []string `form:"exclude_app_name"`
	ExcludeRelayModes    []string `form:"exclude_relay_mode"`

	// W9-D ID 查找:按日志主键精确定位单条(0 表示不过滤)。
	Id int `form:"id"`
}

// applyOrgLogsFilters 组织日志查询的统一过滤:锁定影子账户 + 各维度条件
func applyOrgLogsFilters(tx *gorm.DB, shadowUserId int, params *OrgLogsListParams) *gorm.DB {
	tx = tx.Where("user_id = ?", shadowUserId)
	if params.LogType != LogTypeUnknown {
		tx = tx.Where("type = ?", params.LogType)
	}
	if params.ModelName != "" {
		tx = tx.Where("model_name = ?", params.ModelName)
	}
	if params.TokenName != "" {
		tx = tx.Where("token_name = ?", params.TokenName)
	}
	if params.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", params.EndTimestamp)
	}
	if params.MemberId != 0 {
		tx = tx.Where(orgMemberIDExpr()+" = ?", params.MemberId)
	}
	if params.AppName != "" {
		tx = tx.Where(appNameExpr()+" = ?", params.AppName)
	}
	// W9-D 多值/排除:model_name、token_name 直列;member 走 metadata 表达式;finish_reason 见 metric 过滤。
	tx = applyInNotIn(tx, "model_name", params.ModelNames, params.ExcludeModelNames)
	tx = applyInNotIn(tx, "token_name", params.TokenNames, params.ExcludeTokenNames)
	if len(params.MemberIds) > 0 {
		tx = tx.Where(orgMemberIDExpr()+" IN ?", params.MemberIds)
	}
	if len(params.ExcludeMemberIds) > 0 {
		tx = tx.Where(orgMemberIDExpr()+" NOT IN ?", params.ExcludeMemberIds)
	}
	tx = applyAppNameInNotIn(tx, params.AppNames, params.ExcludeAppNames)
	tx = applyRelayModeInNotIn(tx, params.RelayModes, params.ExcludeRelayModes)
	if params.Id > 0 {
		tx = tx.Where("id = ?", params.Id)
	}
	return tx
}

// applyOrgLogMetricFilters 施加 W8-E 三个新过滤(finish_reason / min_quota / min_tokens),
// 与 admin/self 的 model/log.go applyLogMetricFilters 语义完全一致:复用同包 finishReasonExpr
// 的方言逻辑,quota/tokens 为普通列比较;空/零值表示不过滤(旧日志容错)。
// 只在列表、导出与总消费三条路径施加,聚合统计(GetOrgUsageStatistics)与 admin/self 一致不施加。
func applyOrgLogMetricFilters(tx *gorm.DB, params *OrgLogsListParams) *gorm.DB {
	if params.FinishReason != "" {
		tx = tx.Where(finishReasonExpr()+" = ?", params.FinishReason)
	}
	if params.MinQuota > 0 {
		tx = tx.Where("quota >= ?", params.MinQuota)
	}
	if params.MinTokens > 0 {
		tx = tx.Where("prompt_tokens + completion_tokens >= ?", params.MinTokens)
	}
	if len(params.FinishReasons) > 0 {
		tx = tx.Where(finishReasonExpr()+" IN ?", params.FinishReasons)
	}
	if len(params.ExcludeFinishReasons) > 0 {
		tx = tx.Where(finishReasonExpr()+" NOT IN ?", params.ExcludeFinishReasons)
	}
	return tx
}

// GetOrgLogsList 组织日志分页列表(仅影子账户名下日志)
func GetOrgLogsList(shadowUserId int, params *OrgLogsListParams) (*DataResult[Log], error) {
	var logs []*Log
	tx := applyOrgLogsFilters(DB, shadowUserId, params)
	tx = applyOrgLogMetricFilters(tx, params)
	result, err := PaginateAndOrder[Log](tx, &params.PaginationParams, &logs, allowedLogsOrderFields)
	if err != nil {
		return nil, err
	}
	if err := FillLogsHasDetail(*result.Data); err != nil {
		return nil, err
	}
	return result, nil
}

// GetAllOrgLogsList 组织日志全量列表(CSV 导出用,不分页)
func GetAllOrgLogsList(shadowUserId int, params *OrgLogsListParams) ([]*Log, error) {
	var logs []*Log
	tx := applyOrgLogsFilters(DB, shadowUserId, params)
	tx = applyOrgLogMetricFilters(tx, params)
	err := tx.Order("id DESC").Find(&logs).Error
	return logs, err
}

// SumOrgUsedQuota 组织消费总额(只统计 LogTypeConsume,口径与 SumUsedQuota 一致)
func SumOrgUsedQuota(shadowUserId int, params *OrgLogsListParams) (quota int64) {
	p := *params
	p.LogType = LogTypeConsume
	tx := DB.Table("logs").Select(assembleSumSelectStr("quota"))
	tx = applyOrgLogsFilters(tx, shadowUserId, &p)
	tx = applyOrgLogMetricFilters(tx, &p)
	tx.Scan(&quota)
	return quota
}

// 组织用量聚合维度(规格 §3.6:成员 / 令牌 / 模型 / 时间 / App)
const (
	OrgUsageGroupMember = "member"
	OrgUsageGroupToken  = "token"
	OrgUsageGroupModel  = "model"
	OrgUsageGroupDate   = "date"
	OrgUsageGroupApp    = "app"
)

// OrgUsageStatistic 组织用量聚合行;按聚合维度只填充对应维度字段
type OrgUsageStatistic struct {
	MemberId  int    `json:"member_id,omitempty" gorm:"column:org_member_id"`
	Username  string `json:"username,omitempty" gorm:"-"`
	TokenName string `json:"token_name,omitempty" gorm:"column:token_name"`
	ModelName string `json:"model_name,omitempty" gorm:"column:model_name"`
	AppName   string `json:"app_name,omitempty" gorm:"column:app_name"`
	// AppDomain 仅 group_by=app 时填充:同名 app 组内取任一非空 app_domain(MAX),驱动前端 favicon;
	// 其他维度不 select,保持零值 + omitempty 省略,向后兼容。
	AppDomain        string `json:"app_domain,omitempty" gorm:"column:app_domain"`
	Date             string `json:"date,omitempty" gorm:"column:date"`
	RequestCount     int64  `json:"request_count" gorm:"column:request_count"`
	Quota            int64  `json:"quota" gorm:"column:quota"`
	PromptTokens     int64  `json:"prompt_tokens" gorm:"column:prompt_tokens"`
	CompletionTokens int64  `json:"completion_tokens" gorm:"column:completion_tokens"`
	CachedReadTokens int64  `json:"cached_read_tokens" gorm:"column:cached_read_tokens"`
	// SavedQuota 折扣节省额(原始 quota 单位),口径同日志单行划线原价:sum(original-实收)。
	SavedQuota  float64 `json:"saved_quota" gorm:"column:saved_quota"`
	RequestTime int64   `json:"request_time" gorm:"column:request_time"`
}

// GetOrgUsageStatistics 组织用量聚合查询(只统计消费日志)。
// groupBy 取 OrgUsageGroup* 常量;params.MemberId 等过滤条件在聚合前生效。
func GetOrgUsageStatistics(shadowUserId int, groupBy string, params *OrgLogsListParams) ([]*OrgUsageStatistic, error) {
	var groupExpr, dimensionSelect, orderExpr string
	switch groupBy {
	case OrgUsageGroupMember:
		groupExpr = orgMemberIDExpr()
		dimensionSelect = groupExpr + " as org_member_id"
		orderExpr = "quota DESC"
	case OrgUsageGroupToken:
		groupExpr = "token_name"
		dimensionSelect = "token_name"
		orderExpr = "quota DESC"
	case OrgUsageGroupModel:
		groupExpr = "model_name"
		dimensionSelect = "model_name"
		orderExpr = "quota DESC"
	case OrgUsageGroupApp:
		groupExpr = appNameExpr()
		dimensionSelect = appNameExpr() + " as app_name, MAX(" + appDomainExpr() + ") as app_domain"
		orderExpr = "quota DESC"
	case OrgUsageGroupDate:
		groupExpr = "date"
		dimensionSelect = getTimestampGroupsSelect("created_at", "day", "date")
		orderExpr = "date ASC"
	default:
		return nil, errors.New("unsupported group-by dimension: " + groupBy)
	}

	p := *params
	p.LogType = LogTypeConsume
	selectStr := strings.Join([]string{
		dimensionSelect,
		"count(*) as request_count",
		"sum(quota) as quota",
		"sum(prompt_tokens) as prompt_tokens",
		"sum(completion_tokens) as completion_tokens",
		"sum(" + cachedReadTokensExpr() + ") as cached_read_tokens",
		"sum(" + savedQuotaExpr() + ") as saved_quota",
		"sum(request_time) as request_time",
	}, ", ")

	statistics := make([]*OrgUsageStatistic, 0)
	tx := DB.Table("logs").Select(selectStr)
	tx = applyOrgLogsFilters(tx, shadowUserId, &p)
	err := tx.Group(groupExpr).Order(orderExpr).Scan(&statistics).Error
	return statistics, err
}

// GetOrgLogsRequestHistogram 组织上下文的请求量直方图,过滤口径与 GetOrgLogsList 完全一致
// (锁定影子账户 + member/model/token/finish_reason 等)。单条聚合 SQL、无 N+1。
func GetOrgLogsRequestHistogram(shadowUserId int, params *OrgLogsListParams, bucketType string) ([]*LogRequestHistogramBucket, error) {
	tx := DB.Table("logs")
	tx = applyOrgLogsFilters(tx, shadowUserId, params)
	tx = applyOrgLogMetricFilters(tx, params)
	return queryLogRequestHistogram(tx, bucketType)
}

// GetUsernamesByIds 批量查询用户名(组织日志/聚合按成员展示用)
func GetUsernamesByIds(ids []int) (map[int]string, error) {
	result := make(map[int]string)
	if len(ids) == 0 {
		return result, nil
	}
	var rows []struct {
		Id       int
		Username string
	}
	err := DB.Model(&User{}).Where("id IN ?", ids).Select("id, username").Find(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.Id] = row.Username
	}
	return result, nil
}
