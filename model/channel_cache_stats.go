package model

import (
	"strings"

	"github.com/modeltaps/modeltaps/common"
)

// ChannelCacheStat summarizes how well one channel keeps the upstream prompt cache:
// cached_read_tokens / prompt_tokens is the cache hit rate, affinity_hits / request_count
// the share of requests that stayed on the channel their session was bound to.
type ChannelCacheStat struct {
	ChannelId        int   `json:"channel_id" gorm:"column:channel_id"`
	RequestCount     int64 `json:"request_count" gorm:"column:request_count"`
	PromptTokens     int64 `json:"prompt_tokens" gorm:"column:prompt_tokens"`
	CachedReadTokens int64 `json:"cached_read_tokens" gorm:"column:cached_read_tokens"`
	AffinityHits     int64 `json:"affinity_hits" gorm:"column:affinity_hits"`
	AffinityBinds    int64 `json:"affinity_binds" gorm:"column:affinity_binds"`
}

// sessionAffinityExpr returns the SQL expression for logs.metadata.session_affinity text.
func sessionAffinityExpr() string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata ->> 'session_affinity')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$.session_affinity')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.session_affinity'))"
	}
}

// GetChannelCacheStats aggregates consume logs per channel in [start, end] (unix seconds;
// 0 leaves that side open). channelId > 0 limits the result to one channel.
func GetChannelCacheStats(start, end int64, channelId int) ([]*ChannelCacheStat, error) {
	affinity := sessionAffinityExpr()
	selectStr := strings.Join([]string{
		"channel_id",
		"count(*) as request_count",
		"sum(prompt_tokens) as prompt_tokens",
		"sum(" + cachedReadTokensExpr() + ") as cached_read_tokens",
		"sum(CASE WHEN " + affinity + " = 'hit' THEN 1 ELSE 0 END) as affinity_hits",
		"sum(CASE WHEN " + affinity + " = 'bound' THEN 1 ELSE 0 END) as affinity_binds",
	}, ", ")

	tx := DB.Table("logs").Select(selectStr).Where("type = ?", LogTypeConsume)
	if start > 0 {
		tx = tx.Where("created_at >= ?", start)
	}
	if end > 0 {
		tx = tx.Where("created_at <= ?", end)
	}
	if channelId > 0 {
		tx = tx.Where("channel_id = ?", channelId)
	}

	stats := make([]*ChannelCacheStat, 0)
	err := tx.Group("channel_id").Order("channel_id").Scan(&stats).Error
	return stats, err
}
