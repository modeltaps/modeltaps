package model

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

// 个人用量聚合维度(令牌 / 模型 / 时间 / App),取值与组织端保持一致
const (
	UserUsageGroupToken = OrgUsageGroupToken
	UserUsageGroupModel = OrgUsageGroupModel
	UserUsageGroupDate  = OrgUsageGroupDate
	UserUsageGroupApp   = OrgUsageGroupApp
)

// UserUsageStatistic 个人用量聚合行;按聚合维度只填充对应维度字段
type UserUsageStatistic struct {
	TokenName string `json:"token_name,omitempty" gorm:"column:token_name"`
	ModelName string `json:"model_name,omitempty" gorm:"column:model_name"`
	AppName   string `json:"app_name,omitempty" gorm:"column:app_name"`
	// AppDomain 仅 group_by=app 时填充:同名 app 组内取任一非空 app_domain(MAX),驱动前端 favicon;
	// 其他维度不 select,保持零值 + omitempty 省略,向后兼容(Usage 页 Top Apps/图表不受影响)。
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

// applyUserUsageFilters 个人用量查询的统一过滤:锁定当前用户 + 只统计消费日志 + 各维度条件
func applyUserUsageFilters(tx *gorm.DB, userId int, params *LogsListParams) *gorm.DB {
	tx = tx.Where("user_id = ?", userId).Where("type = ?", LogTypeConsume)
	if params.ModelName != "" {
		tx = tx.Where("model_name = ?", params.ModelName)
	}
	if params.TokenName != "" {
		tx = tx.Where("token_name = ?", params.TokenName)
	}
	if params.AppName != "" {
		tx = tx.Where(appNameExpr()+" = ?", params.AppName)
	}
	if params.StartTimestamp != 0 {
		tx = tx.Where("created_at >= ?", params.StartTimestamp)
	}
	if params.EndTimestamp != 0 {
		tx = tx.Where("created_at <= ?", params.EndTimestamp)
	}
	return tx
}

// GetUserUsageStatistics 个人用量聚合查询(只统计消费日志,形态与 GetOrgUsageStatistics 同构)。
// groupBy 取 UserUsageGroup* 常量;params 的 model_name/token_name/时间范围在聚合前生效。
func GetUserUsageStatistics(userId int, groupBy string, params *LogsListParams) ([]*UserUsageStatistic, error) {
	var groupExpr, dimensionSelect, orderExpr string
	switch groupBy {
	case UserUsageGroupToken:
		groupExpr = "token_name"
		dimensionSelect = "token_name"
		orderExpr = "quota DESC"
	case UserUsageGroupModel:
		groupExpr = "model_name"
		dimensionSelect = "model_name"
		orderExpr = "quota DESC"
	case UserUsageGroupApp:
		groupExpr = appNameExpr()
		dimensionSelect = appNameExpr() + " as app_name, MAX(" + appDomainExpr() + ") as app_domain"
		orderExpr = "quota DESC"
	case UserUsageGroupDate:
		groupExpr = "date"
		dimensionSelect = getTimestampGroupsSelect("created_at", "day", "date")
		orderExpr = "date ASC"
	default:
		return nil, errors.New("unsupported group-by dimension: " + groupBy)
	}

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

	statistics := make([]*UserUsageStatistic, 0)
	tx := DB.Table("logs").Select(selectStr)
	tx = applyUserUsageFilters(tx, userId, params)
	err := tx.Group(groupExpr).Order(orderExpr).Scan(&statistics).Error
	return statistics, err
}
