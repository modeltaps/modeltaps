package model

import (
	"time"
	"unicode/utf8"

	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const (
	// LogIODefaultMaxBodyKB 单条请求/响应明细最大留存 KB 的默认值(1MB)。
	LogIODefaultMaxBodyKB = 1024
	// LogIORetentionDays log_details 自动清理保留天数(默认 30 天)。
	LogIORetentionDays = 30
)

// LogIOMaxBodyBytes 返回单条请求/响应明细的最大留存字节数,超出截断并打标志位。
// 从配置 log_io_max_body_kb(KB 单位,默认 1024 即 1MB)读取,非法值(<=0)回退默认。
func LogIOMaxBodyBytes() int {
	kb := viper.GetInt("log_io_max_body_kb")
	if kb <= 0 {
		kb = LogIODefaultMaxBodyKB
	}
	return kb * 1024
}

// LogDetail 独立存放完整请求/响应明细,与 logs 一对一(log_id),不污染高频 logs 表。
// 仅在 LogIOWriteEnabled(token) 为真时写入;按 created_at 做 30 天 TTL 清理(见 CleanupOldLogDetails)。
// 仅留存请求/响应 body,绝不记录鉴权头/密钥;token 归属字段供 T50f 读取端鉴权(CanViewTokenLogIO)。
type LogDetail struct {
	Id                int    `json:"id"`
	LogId             int    `json:"log_id" gorm:"index"`
	TokenId           int    `json:"token_id" gorm:"index"`
	UserId            int    `json:"user_id" gorm:"index"`
	CreatedBy         int    `json:"created_by" gorm:"default:0"`
	RequestBody       string `json:"request_body"`
	ResponseBody      string `json:"response_body"`
	RequestTruncated  bool   `json:"request_truncated" gorm:"default:false"`
	ResponseTruncated bool   `json:"response_truncated" gorm:"default:false"`
	CreatedAt         int64  `json:"created_at" gorm:"bigint;index"`
}

// LogIODetail 是写日志链路在 logs 行落库后用于派生 log_details 的临时载体(不持久化)。
// 由 relay 写日志链路在 LogIOWriteEnabled 为真时填充,经 Log.AfterCreate 钩子落 log_details。
type LogIODetail struct {
	TokenId           int
	CreatedBy         int
	RequestBody       string
	ResponseBody      string
	RequestTruncated  bool
	ResponseTruncated bool
}

// TruncateLogIOBody 将 body 截断到 LogIOMaxBodyBytes(),返回截断后内容与是否被截断。
// 截断后修剪末尾不完整的 UTF-8 字节,避免 utf8mb4 列拒绝非法序列。
func TruncateLogIOBody(body []byte) (string, bool) {
	maxBytes := LogIOMaxBodyBytes()
	if len(body) <= maxBytes {
		return string(body), false
	}
	truncated := body[:maxBytes]
	for i := 0; i < utf8.UTFMax-1 && len(truncated) > 0 && !utf8.Valid(truncated); i++ {
		truncated = truncated[:len(truncated)-1]
	}
	return string(truncated), true
}

// GetLogDetailByLogId 按 log_id 读取唯一一条请求/响应明细(T50f 读取端);
// 无记录(opt-in 未开启或已被 TTL 清理)时返回 gorm.ErrRecordNotFound,由调用方据此渲染空态。
func GetLogDetailByLogId(logId int) (*LogDetail, error) {
	if logId == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	var detail LogDetail
	if err := DB.Where("log_id = ?", logId).First(&detail).Error; err != nil {
		return nil, err
	}
	return &detail, nil
}

// FillLogsHasDetail 对当页日志批量标注 HasDetail:是否在 log_details 表留存了明细。
// 对当页 log id 做单次 IN + DISTINCT 查询后内存合并,避免逐行 EXISTS 的 N+1 与全表 JOIN。
// 已过 TTL 清理的日志因 log_details 无行,HasDetail 自然为 false。
func FillLogsHasDetail(logs []*Log) error {
	if len(logs) == 0 {
		return nil
	}
	ids := make([]int, 0, len(logs))
	for _, log := range logs {
		if log.Id != 0 {
			ids = append(ids, log.Id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	var existing []int
	if err := DB.Model(&LogDetail{}).
		Where("log_id IN ?", ids).
		Distinct().
		Pluck("log_id", &existing).Error; err != nil {
		return err
	}
	has := make(map[int]struct{}, len(existing))
	for _, id := range existing {
		has[id] = struct{}{}
	}
	for _, log := range logs {
		if _, ok := has[log.Id]; ok {
			log.HasDetail = true
		}
	}
	return nil
}

// CleanupOldLogDetails 删除超过保留期(LogIORetentionDays)的 log_details,返回删除行数。
func CleanupOldLogDetails() (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -LogIORetentionDays).Unix()
	result := DB.Where("created_at < ?", cutoff).Delete(&LogDetail{})
	return result.RowsAffected, result.Error
}
