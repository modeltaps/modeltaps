package model

import (
	"context"
	"fmt"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"strings"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Log struct {
	Id               int    `json:"id"`
	UserId           int    `json:"user_id" gorm:"index;index:idx_user_created_at"`
	CreatedAt        int64  `json:"created_at" gorm:"bigint;index:idx_created_at_type;index:idx_user_created_at"`
	Type             int    `json:"type" gorm:"index:idx_created_at_type"`
	Content          string `json:"content"`
	Username         string `json:"username" gorm:"index:index_username_model_name,priority:2;default:''"`
	TokenName        string `json:"token_name" gorm:"index;default:''"`
	ModelName        string `json:"model_name" gorm:"index;index:index_username_model_name,priority:1;default:''"`
	Quota            int    `json:"quota" gorm:"default:0"`
	CostQuota        int    `json:"cost_quota" gorm:"default:0"`
	PromptTokens     int    `json:"prompt_tokens" gorm:"default:0"`
	CompletionTokens int    `json:"completion_tokens" gorm:"default:0"`
	ChannelId        int    `json:"channel_id" gorm:"index"`
	RequestTime      int    `json:"request_time" gorm:"default:0"`
	IsStream         bool   `json:"is_stream" gorm:"default:false"`
	SourceIp         string `json:"source_ip" gorm:"default:''"`
	RequestId        string `json:"request_id" gorm:"type:varchar(64);index;default:''"`
	// 不建索引：管理员偶发排障用，且查询恒带 created_at 范围（前端默认当天），由
	// idx_created_at_type 收窄后再过滤即可；口径同样可过滤的 source_ip。logs 是写入最热的表，
	// 不为低频查询摊索引维护成本。若某部署确实高频按上游 ID 反查，加回 index 即可。
	UpstreamRequestId string                             `json:"upstream_request_id" gorm:"type:varchar(128);default:''"`
	Metadata          datatypes.JSONType[map[string]any] `json:"metadata" gorm:"type:json"`

	Channel *Channel `json:"channel" gorm:"foreignKey:Id;references:ChannelId"`

	// HasDetail 不持久化,仅作列表响应字段:标示该条日志是否在 log_details 表留存了请求/响应明细。
	// 由列表查询对当页 log id 单次批量查 log_details 后内存合并填充(见 FillLogsHasDetail)。
	HasDetail bool `json:"has_detail" gorm:"-"`

	// logIODetail 不持久化到 logs 表,仅作临时载体:LogIOWriteEnabled 为真时由写日志链路填充,
	// 在本行落库后(含 batch insert 逐行回填 Id)经 AfterCreate 派生 log_details(T50d)。
	logIODetail *LogIODetail `gorm:"-"`
}

// AfterCreate 在 logs 行落库后(Id 已回填)派生 log_details。
// 仅当写日志链路填充了 logIODetail 时写入;失败仅记日志,不回滚 logs 行。
func (log *Log) AfterCreate(tx *gorm.DB) error {
	if log.logIODetail == nil {
		return nil
	}
	d := log.logIODetail
	detail := &LogDetail{
		LogId:             log.Id,
		TokenId:           d.TokenId,
		UserId:            log.UserId,
		CreatedBy:         d.CreatedBy,
		RequestBody:       d.RequestBody,
		ResponseBody:      d.ResponseBody,
		RequestTruncated:  d.RequestTruncated,
		ResponseTruncated: d.ResponseTruncated,
		CreatedAt:         log.CreatedAt,
	}
	if err := tx.Create(detail).Error; err != nil {
		logger.SysError("failed to record log_details: " + err.Error())
	}
	return nil
}

const (
	LogTypeUnknown = iota
	LogTypeTopup
	LogTypeConsume
	LogTypeManage
	LogTypeSystem
)

func RecordQuotaLog(userId int, logType int, quota int, ip string, content string) {
	if logType == LogTypeConsume && !config.LogConsumeEnabled {
		return
	}
	username, _ := CacheGetUsername(userId)
	log := &Log{
		UserId:    userId,
		Username:  username,
		Quota:     quota,
		CreatedAt: utils.GetTimestamp(),
		Type:      logType,
		SourceIp:  ip,
		Content:   content,
	}
	err := DB.Create(log).Error
	if err != nil {
		logger.SysError("failed to record log: " + err.Error())
	}
}

func RecordLog(userId int, logType int, content string) {
	if logType == LogTypeConsume && !config.LogConsumeEnabled {
		return
	}
	username, _ := CacheGetUsername(userId)

	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: utils.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err := DB.Create(log).Error
	if err != nil {
		logger.SysError("failed to record log: " + err.Error())
	}
}

// RecordLogWithTx 在指定事务中记录日志
func RecordLogWithTx(tx *gorm.DB, userId int, logType int, content string) {
	if logType == LogTypeConsume && !config.LogConsumeEnabled {
		return
	}

	// 在事务中查询用户名，避免事务未提交时查询不到用户
	var username string
	err := tx.Model(&User{}).Where("id = ?", userId).Select("username").Find(&username).Error
	if err != nil {
		logger.SysError("failed to get username in tx: " + err.Error())
		// 如果事务中查询失败，尝试从缓存获取
		username, _ = CacheGetUsername(userId)
	}

	log := &Log{
		UserId:    userId,
		Username:  username,
		CreatedAt: utils.GetTimestamp(),
		Type:      logType,
		Content:   content,
	}
	err = tx.Create(log).Error
	if err != nil {
		logger.SysError("failed to record log with tx: " + err.Error())
	}
}

func RecordConsumeLog(
	ctx context.Context,
	userId int,
	channelId int,
	promptTokens int,
	completionTokens int,
	modelName string,
	tokenName string,
	quota int,
	costQuota int,
	content string,
	requestTime int,
	isStream bool,
	metadata map[string]any,
	sourceIp string,
	logIODetail *LogIODetail) {
	logger.LogInfo(ctx, fmt.Sprintf("record consume log: userId=%d, channelId=%d, promptTokens=%d, completionTokens=%d, modelName=%s, tokenName=%s, quota=%d, content=%s ,sourceIp=%s", userId, channelId, promptTokens, completionTokens, modelName, tokenName, quota, content, sourceIp))
	if !config.LogConsumeEnabled {
		return
	}

	username, _ := CacheGetUsername(userId)

	// request id 由 middleware 注入 ctx；upstream request id 由 provider 暂存、
	// 经 relay_util.WithUpstreamRequestID 带入。两者缺失时为空串。
	requestId, _ := ctx.Value(logger.RequestIdKey).(string)
	upstreamRequestId, _ := ctx.Value(config.GinUpstreamRequestIdKey).(string)
	// 上游返回的超长 header 会让整条计费日志插入失败，按列宽 varchar(128) 做 rune 安全截断。
	if len(upstreamRequestId) > 128 {
		if r := []rune(upstreamRequestId); len(r) > 128 {
			upstreamRequestId = string(r[:128])
		}
	}

	log := &Log{
		UserId:            userId,
		Username:          username,
		CreatedAt:         utils.GetTimestamp(),
		Type:              LogTypeConsume,
		Content:           content,
		PromptTokens:      promptTokens,
		CompletionTokens:  completionTokens,
		TokenName:         tokenName,
		ModelName:         modelName,
		Quota:             quota,
		CostQuota:         costQuota,
		ChannelId:         channelId,
		RequestTime:       requestTime,
		IsStream:          isStream,
		SourceIp:          sourceIp,
		RequestId:         requestId,
		UpstreamRequestId: upstreamRequestId,
	}

	if metadata != nil {
		log.Metadata = datatypes.NewJSONType(metadata)
	}

	// 仅在写日志链路已按 LogIOWriteEnabled 闸门填充时携带明细,经 AfterCreate 落 log_details。
	log.logIODetail = logIODetail

	if config.BatchUpdateEnabled {
		AddLogToBatch(log)
	} else {
		err := DB.Create(log).Error
		if err != nil {
			logger.LogError(ctx, "failed to record log: "+err.Error())
		}
	}
}

type LogsListParams struct {
	PaginationParams
	LogType        int    `form:"log_type"`
	StartTimestamp int64  `form:"start_timestamp"`
	EndTimestamp   int64  `form:"end_timestamp"`
	ModelName      string `form:"model_name"`
	Username       string `form:"username"`
	TokenName      string `form:"token_name"`
	ChannelId      int    `form:"channel_id"`
	SourceIp       string `form:"source_ip"`
	// 请求 ID 精确过滤:request_id 为本站追踪 ID(用户侧可用),
	// upstream_request_id 为上游厂商 ID(仅管理员维度,口径同 channel_id)。
	RequestId         string `form:"request_id"`
	UpstreamRequestId string `form:"upstream_request_id"`
	// W8-E 新增过滤:finish_reason(metadata JSON 归一值)、成本/tokens 下限。
	FinishReason string `form:"finish_reason"`
	MinQuota     int    `form:"min_quota"`
	MinTokens    int    `form:"min_tokens"`
	// W9-M10 App 归因过滤:按 metadata.app_name 精确过滤(空表示不过滤)。
	AppName string `form:"app_name"`

	// W9-D 级联筛选:多值包含(IN)与排除(NOT IN)。单值旧字段保持不变以兼容 Ledger/导出等调用方;
	// 前端级联筛选改用下列复数/exclude_ 字段,后端以 AND 组合叠加(见 applyLogEnumFilters)。
	ModelNames    []string `form:"model_names"`
	Usernames     []string `form:"usernames"`
	TokenNames    []string `form:"token_names"`
	ChannelIds    []int    `form:"channel_ids"`
	SourceIps     []string `form:"source_ips"`
	FinishReasons []string `form:"finish_reasons"`
	AppNames      []string `form:"app_names"`
	// 消费模态过滤:按 metadata.relay_mode 多值包含/排除(旧日志无该键,不被 IN 命中、不被 NOT IN 误伤)。
	RelayModes []string `form:"relay_modes"`

	ExcludeModelNames    []string `form:"exclude_model_name"`
	ExcludeUsernames     []string `form:"exclude_username"`
	ExcludeTokenNames    []string `form:"exclude_token_name"`
	ExcludeChannelIds    []int    `form:"exclude_channel_id"`
	ExcludeSourceIps     []string `form:"exclude_source_ip"`
	ExcludeFinishReasons []string `form:"exclude_finish_reason"`
	ExcludeAppNames      []string `form:"exclude_app_name"`
	ExcludeRelayModes    []string `form:"exclude_relay_mode"`

	// W9-D ID 查找:按日志主键精确定位单条(0 表示不过滤)。
	Id int `form:"id"`
}

// finishReasonExpr 返回从 logs.metadata 提取归一 finish_reason 的 SQL 表达式(按数据库方言)。
// 与 orgMemberIDExpr 同构:MySQL 需 JSON_UNQUOTE 去引号后再与字符串比较,SQLite 的
// JSON_EXTRACT 直接返回文本,PostgreSQL 用 ->> 取文本。
func finishReasonExpr() string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata ->> 'finish_reason')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$.finish_reason')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.finish_reason'))"
	}
}

// appNameExpr 返回从 logs.metadata 提取 app_name 文本的 SQL 表达式(按数据库方言),
// 与 finishReasonExpr 同构。用于 W9-M10 按 App 归因过滤与 group_by=app 聚合。
func appNameExpr() string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata ->> 'app_name')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$.app_name')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.app_name'))"
	}
}

// appDomainExpr 返回从 logs.metadata 提取 app_domain 文本的 SQL 表达式(按数据库方言),
// 与 appNameExpr 同构。用于 group_by=app 聚合时附带一个代表性 domain 以驱动前端 favicon 渲染。
func appDomainExpr() string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata ->> 'app_domain')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$.app_domain')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.app_domain'))"
	}
}

// relayModeExpr 返回从 logs.metadata 提取 relay_mode 文本的 SQL 表达式(按数据库方言),
// 与 finishReasonExpr 同构。用于按消费模态(chat_completions/image_generations/...)过滤。
func relayModeExpr() string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata ->> 'relay_mode')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$.relay_mode')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.relay_mode'))"
	}
}

// applyRelayModeInNotIn 对 metadata.relay_mode 施加 IN / NOT IN 过滤;空切片不过滤。
// NOT IN 对 NULL 恒为 NULL,会误伤无该键的旧日志,故补 OR ... IS NULL 显式保留(口径同 app_name)。
func applyRelayModeInNotIn(tx *gorm.DB, include, exclude []string) *gorm.DB {
	expr := relayModeExpr()
	if len(include) > 0 {
		tx = tx.Where(expr+" IN ?", include)
	}
	if len(exclude) > 0 {
		tx = tx.Where("("+expr+" NOT IN ? OR "+expr+" IS NULL)", exclude)
	}
	return tx
}

// requestIdExpr 返回从 logs.metadata 提取 request_id 文本的 SQL 表达式(按数据库方言),
// 与 finishReasonExpr 同构。用于 W9-M6 按 request_id 定位消费日志行回填 generation 统计。
func requestIdExpr() string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata ->> 'request_id')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$.request_id')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.request_id'))"
	}
}

// UpdateLogMetadataByRequestId 按 metadata.request_id 定位消费日志行,把 extra 中的键并入其 metadata
// (读-改-写,additive,不覆盖既有键)。W9-M6 异步回填 OpenRouter generation 统计专用。
// 找不到对应行时返回 gorm.ErrRecordNotFound,供调用方按批量落库延迟重试。
func UpdateLogMetadataByRequestId(requestId string, extra map[string]any) error {
	if requestId == "" || len(extra) == 0 {
		return nil
	}

	var log Log
	if err := DB.Where(requestIdExpr()+" = ?", requestId).Order("id DESC").First(&log).Error; err != nil {
		return err
	}

	meta := log.Metadata.Data()
	if meta == nil {
		meta = map[string]any{}
	}
	for k, v := range extra {
		meta[k] = v
	}

	return DB.Model(&Log{}).Where("id = ?", log.Id).Update("metadata", datatypes.NewJSONType(meta)).Error
}

// applyLogMetricFilters 统一施加 W8-E 三个新过滤(finish_reason / min_quota / min_tokens),
// 保证列表、导出与总消费统计三条路径语义一致。空/零值表示不过滤(旧日志容错)。
func applyLogMetricFilters(tx *gorm.DB, params *LogsListParams) *gorm.DB {
	if params.FinishReason != "" {
		tx = tx.Where(finishReasonExpr()+" = ?", params.FinishReason)
	}
	if params.MinQuota > 0 {
		tx = tx.Where("quota >= ?", params.MinQuota)
	}
	if params.MinTokens > 0 {
		tx = tx.Where("prompt_tokens + completion_tokens >= ?", params.MinTokens)
	}
	return tx
}

// applyInNotIn 对指定列施加 IN(include) 与 NOT IN(exclude) 过滤;空切片表示不过滤。
// 泛型以同时服务字符串列(model_name 等)与整型列(channel_id)。
func applyInNotIn[T any](tx *gorm.DB, col string, include, exclude []T) *gorm.DB {
	if len(include) > 0 {
		tx = tx.Where(col+" IN ?", include)
	}
	if len(exclude) > 0 {
		tx = tx.Where(col+" NOT IN ?", exclude)
	}
	return tx
}

// applyAppNameInNotIn 对 metadata.app_name(JSON 提取表达式)施加 IN / NOT IN 过滤;空切片不过滤。
// NOT IN 时表达式对 NULL 恒为 NULL(被过滤),会误伤无归因(app_name 缺失)日志,故补 OR ... IS NULL
// 显式保留无归因日志;IN 不受此影响(排除 NULL 符合按具体 App 包含的语义)。
func applyAppNameInNotIn(tx *gorm.DB, include, exclude []string) *gorm.DB {
	expr := appNameExpr()
	if len(include) > 0 {
		tx = tx.Where(expr+" IN ?", include)
	}
	if len(exclude) > 0 {
		tx = tx.Where("("+expr+" NOT IN ? OR "+expr+" IS NULL)", exclude)
	}
	return tx
}

// applyLogEnumFilters 施加 W9-D 级联筛选的多值(IN)/排除(NOT IN)条件,语义与单值旧字段一致。
// model_name / token_name / finish_reason / app_name 各路径通用;username / channel_id / source_ip 仅在
// admin 上下文(admin=true)施加,与旧单值逻辑在 self 列表中被忽略的口径保持一致。
func applyLogEnumFilters(tx *gorm.DB, params *LogsListParams, admin bool) *gorm.DB {
	tx = applyInNotIn(tx, "model_name", params.ModelNames, params.ExcludeModelNames)
	tx = applyInNotIn(tx, "token_name", params.TokenNames, params.ExcludeTokenNames)
	if len(params.FinishReasons) > 0 {
		tx = tx.Where(finishReasonExpr()+" IN ?", params.FinishReasons)
	}
	if len(params.ExcludeFinishReasons) > 0 {
		tx = tx.Where(finishReasonExpr()+" NOT IN ?", params.ExcludeFinishReasons)
	}
	tx = applyAppNameInNotIn(tx, params.AppNames, params.ExcludeAppNames)
	tx = applyRelayModeInNotIn(tx, params.RelayModes, params.ExcludeRelayModes)
	if params.RequestId != "" {
		tx = tx.Where("request_id = ?", params.RequestId)
	}
	if admin {
		tx = applyInNotIn(tx, "username", params.Usernames, params.ExcludeUsernames)
		tx = applyInNotIn(tx, "channel_id", params.ChannelIds, params.ExcludeChannelIds)
		tx = applyInNotIn(tx, "source_ip", params.SourceIps, params.ExcludeSourceIps)
		// upstream_request_id 属管理员维度,不在用户侧过滤(口径同 channel_id,
		// 由 GetLogsSelfStat 清空保证列表与总消费数字对齐)。
		if params.UpstreamRequestId != "" {
			tx = tx.Where("upstream_request_id = ?", params.UpstreamRequestId)
		}
	}
	if params.Id > 0 {
		tx = tx.Where("id = ?", params.Id)
	}
	return tx
}

var allowedLogsOrderFields = map[string]bool{
	"created_at": true,
	"channel_id": true,
	"user_id":    true,
	"token_name": true,
	"model_name": true,
	"type":       true,
	"source_ip":  true,
}

// GetLogById 按主键读取单条日志(T50f 读取端在无明细空态时用于鉴权归属与判定保留期)。
func GetLogById(id int) (*Log, error) {
	if id == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var log Log
	if err := DB.First(&log, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

func GetLogsList(params *LogsListParams) (*DataResult[Log], error) {
	var tx *gorm.DB
	var logs []*Log

	tx = DB.Preload("Channel", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, name, type, base_url")
	})

	if params.LogType != LogTypeUnknown {
		tx = tx.Where("type = ?", params.LogType)
	}
	if params.ModelName != "" {
		tx = tx.Where("model_name = ?", params.ModelName)
	}
	if params.Username != "" {
		tx = tx.Where("username = ?", params.Username)
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
	if params.ChannelId != 0 {
		tx = tx.Where("channel_id = ?", params.ChannelId)
	}
	if params.SourceIp != "" {
		tx = tx.Where("source_ip = ?", params.SourceIp)
	}
	tx = applyLogMetricFilters(tx, params)
	tx = applyLogEnumFilters(tx, params, true)

	result, err := PaginateAndOrder[Log](tx, &params.PaginationParams, &logs, allowedLogsOrderFields)
	if err != nil {
		return nil, err
	}

	if err := FillLogsHasDetail(*result.Data); err != nil {
		return nil, err
	}

	return result, nil
}

// GetAllLogsList returns all logs matching the criteria without pagination (for export)
func GetAllLogsList(params *LogsListParams) ([]*Log, error) {
	var logs []*Log

	tx := DB.Preload("Channel", func(db *gorm.DB) *gorm.DB {
		return db.Select("id, name, type, base_url")
	})

	if params.LogType != LogTypeUnknown {
		tx = tx.Where("type = ?", params.LogType)
	}
	if params.ModelName != "" {
		tx = tx.Where("model_name = ?", params.ModelName)
	}
	if params.Username != "" {
		tx = tx.Where("username = ?", params.Username)
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
	if params.ChannelId != 0 {
		tx = tx.Where("channel_id = ?", params.ChannelId)
	}
	if params.SourceIp != "" {
		tx = tx.Where("source_ip = ?", params.SourceIp)
	}
	tx = applyLogMetricFilters(tx, params)
	tx = applyLogEnumFilters(tx, params, true)

	// Apply ordering
	if params.Order != "" {
		orderFields := strings.Split(params.Order, ",")
		for _, field := range orderFields {
			field = strings.TrimSpace(field)
			desc := strings.HasPrefix(field, "-")
			if desc {
				field = field[1:]
			}
			if allowedLogsOrderFields[field] {
				if desc {
					field = field + " DESC"
				}
				tx = tx.Order(field)
			}
		}
	} else {
		// Default ordering
		tx = tx.Order("id DESC")
	}

	err := tx.Find(&logs).Error
	return logs, err
}

func GetUserLogsList(userId int, params *LogsListParams) (*DataResult[Log], error) {
	var logs []*Log

	tx := DB.Where("user_id = ?", userId).Omit("cost_quota", "upstream_request_id")

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
	tx = applyLogMetricFilters(tx, params)
	tx = applyLogEnumFilters(tx, params, false)

	result, err := PaginateAndOrder[Log](tx, &params.PaginationParams, &logs, allowedLogsOrderFields)
	if err != nil {
		return nil, err
	}

	for _, log := range *result.Data {
		if log.Type == LogTypeManage || log.Type == LogTypeSystem {
			log.SourceIp = ""
		}
	}

	if err := FillLogsHasDetail(*result.Data); err != nil {
		return nil, err
	}

	return result, nil
}

// GetAllUserLogsList returns all user logs matching the criteria without pagination (for export)
func GetAllUserLogsList(userId int, params *LogsListParams) ([]*Log, error) {
	var logs []*Log

	tx := DB.Where("user_id = ?", userId).Omit("cost_quota", "upstream_request_id")

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
	tx = applyLogMetricFilters(tx, params)
	tx = applyLogEnumFilters(tx, params, false)

	// Apply ordering
	if params.Order != "" {
		orderFields := strings.Split(params.Order, ",")
		for _, field := range orderFields {
			field = strings.TrimSpace(field)
			desc := strings.HasPrefix(field, "-")
			if desc {
				field = field[1:]
			}
			if allowedLogsOrderFields[field] {
				if desc {
					field = field + " DESC"
				}
				tx = tx.Order(field)
			}
		}
	} else {
		// Default ordering
		tx = tx.Order("id DESC")
	}

	err := tx.Find(&logs).Error
	if err != nil {
		return nil, err
	}

	// Apply the same filtering as in GetUserLogsList
	for _, log := range logs {
		if log.Type == LogTypeManage || log.Type == LogTypeSystem {
			log.SourceIp = ""
		}
	}

	return logs, nil
}

func SearchAllLogs(keyword string) (logs []*Log, err error) {
	err = DB.Where("type = ? or content LIKE ?", keyword, keyword+"%").Order("id desc").Limit(config.MaxRecentItems).Find(&logs).Error
	return logs, err
}

func SearchUserLogs(userId int, keyword string) (logs []*Log, err error) {
	err = DB.Where("user_id = ? and type = ?", userId, keyword).Order("id desc").Limit(config.MaxRecentItems).Omit("cost_quota", "upstream_request_id").Find(&logs).Error
	return logs, err
}

func SumUsedQuota(params *LogsListParams) (quota int) {
	tx := DB.Table("logs").Select(assembleSumSelectStr("quota"))
	if params.Username != "" {
		tx = tx.Where("username = ?", params.Username)
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
	if params.ModelName != "" {
		tx = tx.Where("model_name = ?", params.ModelName)
	}
	if params.ChannelId != 0 {
		tx = tx.Where("channel_id = ?", params.ChannelId)
	}
	if params.SourceIp != "" {
		tx = tx.Where("source_ip = ?", params.SourceIp)
	}
	// 与列表口径一致:总消费也施加 finish_reason / min_quota / min_tokens 过滤。
	tx = applyLogMetricFilters(tx, params)
	tx = applyLogEnumFilters(tx, params, true)
	// 「总消费」按定义只统计 LogTypeConsume，调用方传入的 params.LogType 在此被有意忽略。
	// 即便 Tab 切到「全部」（log_type=0）也只汇总消费类型，避免把充值/管理/系统日志的 quota 混进总消费。
	tx.Where("type = ?", LogTypeConsume).Scan(&quota)
	return quota
}

// LogRequestHistogramBucket 请求量直方图的单个时间桶:Bucket 为按方言格式化的桶标签(小时/天),
// Count 为该桶内的请求条数,Quota 为该桶消费额度合计,Tokens 为该桶输入+输出 token 合计。
// W9-E 日志页直方图用;LOGX-3 增补 Quota/Tokens 供指标切换,count 语义保持不变。
type LogRequestHistogramBucket struct {
	Bucket string `json:"bucket" gorm:"column:bucket"`
	Count  int64  `json:"count" gorm:"column:count"`
	Quota  int64  `json:"quota" gorm:"column:quota"`
	Tokens int64  `json:"tokens" gorm:"column:tokens"`
}

// queryLogRequestHistogram 在已施加过滤的 tx 上按 created_at 分桶聚合:单条聚合 SQL、无 N+1。
// 每桶同时返回 count(*)、SUM(quota)、SUM(prompt_tokens+completion_tokens),SUM 表达式三方言通用。
// bucketType 取 "hour" 或 "day",与 getTimestampGroupsSelect 的粒度一致。仅返回有数据的桶。
func queryLogRequestHistogram(tx *gorm.DB, bucketType string) ([]*LogRequestHistogramBucket, error) {
	buckets := make([]*LogRequestHistogramBucket, 0)
	groupSelect := getTimestampGroupsSelect("created_at", bucketType, "bucket")
	err := tx.Select(groupSelect + ", count(*) as count, COALESCE(SUM(quota), 0) as quota, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tokens").Group("bucket").Order("bucket ASC").Scan(&buckets).Error
	return buckets, err
}

// GetLogsRequestHistogram 站点管理员上下文的请求量直方图,过滤口径与 GetLogsList 完全一致。
func GetLogsRequestHistogram(params *LogsListParams, bucketType string) ([]*LogRequestHistogramBucket, error) {
	tx := DB.Table("logs")
	if params.LogType != LogTypeUnknown {
		tx = tx.Where("type = ?", params.LogType)
	}
	if params.ModelName != "" {
		tx = tx.Where("model_name = ?", params.ModelName)
	}
	if params.Username != "" {
		tx = tx.Where("username = ?", params.Username)
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
	if params.ChannelId != 0 {
		tx = tx.Where("channel_id = ?", params.ChannelId)
	}
	if params.SourceIp != "" {
		tx = tx.Where("source_ip = ?", params.SourceIp)
	}
	tx = applyLogMetricFilters(tx, params)
	tx = applyLogEnumFilters(tx, params, true)
	return queryLogRequestHistogram(tx, bucketType)
}

// GetUserLogsRequestHistogram 个人上下文的请求量直方图,过滤口径与 GetUserLogsList 完全一致
// (锁定 user_id,忽略 admin 专属维度 channel_id/source_ip/username)。
func GetUserLogsRequestHistogram(userId int, params *LogsListParams, bucketType string) ([]*LogRequestHistogramBucket, error) {
	tx := DB.Table("logs").Where("user_id = ?", userId)
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
	tx = applyLogMetricFilters(tx, params)
	tx = applyLogEnumFilters(tx, params, false)
	return queryLogRequestHistogram(tx, bucketType)
}

func DeleteOldLog(targetTimestamp int64) (int64, error) {
	result := DB.Where("type = ? AND created_at < ?", LogTypeConsume, targetTimestamp).Delete(&Log{})
	return result.RowsAffected, result.Error
}

// DeleteOldLogBatch 分批删除指定时间之前的消费日志，返回本批删除行数。
// 先查一批 ID 再按 ID 删，避免超大事务锁表。
func DeleteOldLogBatch(targetTimestamp int64, batchSize int) (int64, error) {
	var ids []int
	err := DB.Model(&Log{}).Select("id").
		Where("type = ? AND created_at < ?", LogTypeConsume, targetTimestamp).
		Limit(batchSize).Pluck("id", &ids).Error
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	result := DB.Where("id IN ?", ids).Delete(&Log{})
	if result.Error != nil {
		return result.RowsAffected, result.Error
	}
	// 顺带级联清理关联的 log_details，避免 LogAutoDeleteDays < LogIORetentionDays 时
	// 孤儿明细要等 TTL cron 才被清掉；失败仅记日志（孤儿由 CleanupOldLogDetails 兜底自愈）。
	if err := DB.Where("log_id IN ?", ids).Delete(&LogDetail{}).Error; err != nil {
		logger.SysError("failed to clean up log_details of deleted logs: " + err.Error())
	}
	return result.RowsAffected, nil
}

type LogStatistic struct {
	Date             string `gorm:"column:date"`
	RequestCount     int64  `gorm:"column:request_count"`
	Quota            int64  `gorm:"column:quota"`
	CostQuota        int64  `gorm:"column:cost_quota"`
	PromptTokens     int64  `gorm:"column:prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens"`
	RequestTime      int64  `gorm:"column:request_time"`
}

type LogStatisticGroupModel struct {
	LogStatistic
	ModelName string `gorm:"column:model_name"`
}

type LogStatisticGroupChannel struct {
	LogStatistic
	Channel string `gorm:"column:channel"`
}

type RpmTpmStatistics struct {
	RPM int64   `json:"rpm"`
	TPM int64   `json:"tpm"`
	CPM float64 `json:"cpm"`
	PPM float64 `json:"ppm"` // Profit Per Minute (美元)：每分钟利润 = (收入 - 成本) / QuotaPerUnit
}

// GetRpmTpmStatistics 统计最近60秒的实时流量（滑动窗口，仅消费日志）。
// userId 为 0 时统计全站，否则仅统计该用户。
func GetRpmTpmStatistics(userId int) (*RpmTpmStatistics, error) {
	var result struct {
		RPM        int64 `gorm:"column:rpm"`
		TPM        int64 `gorm:"column:tpm"`
		TotalQuota int64 `gorm:"column:total_quota"`
		CostQuota  int64 `gorm:"column:cost_quota"`
	}

	// 获取最近60秒的统计数据
	now := time.Now().Unix()
	startTime := now - 60

	query := DB.Table("logs").
		Select("COUNT(*) as rpm, COALESCE(SUM(prompt_tokens + completion_tokens), 0) as tpm, COALESCE(SUM(quota), 0) as total_quota, COALESCE(SUM(cost_quota), 0) as cost_quota").
		Where("type = ? AND created_at >= ?", LogTypeConsume, startTime)
	if userId != 0 {
		query = query.Where("user_id = ?", userId)
	}

	err := query.Scan(&result).Error

	if err != nil {
		return nil, err
	}

	// 计算每分钟消费金额 (美元)
	// total_quota 是系统内部的配额单位，需要转换为美元
	cpm := float64(result.TotalQuota) / float64(config.QuotaPerUnit)
	// 每分钟利润 = (收入 - 成本) / QuotaPerUnit
	ppm := float64(result.TotalQuota-result.CostQuota) / float64(config.QuotaPerUnit)

	return &RpmTpmStatistics{
		RPM: result.RPM,
		TPM: result.TPM,
		CPM: cpm,
		PPM: ppm,
	}, nil
}
