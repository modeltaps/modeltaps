package model

import (
	"sort"
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"

	"gorm.io/gorm"
)

// ChannelMetric aggregates one channel and model over one hour: attempts, successes, total
// latency of successful requests, time to first token (streams) and output speed.
// Every node adds its own deltas, so the rows are sums across the cluster.
type ChannelMetric struct {
	Id           int    `json:"-"`
	BucketTs     int64  `json:"bucket_ts" gorm:"uniqueIndex:idx_channel_metric_key,priority:1;index"`
	ChannelId    int    `json:"channel_id" gorm:"uniqueIndex:idx_channel_metric_key,priority:2"`
	ModelName    string `json:"model_name" gorm:"type:varchar(191);uniqueIndex:idx_channel_metric_key,priority:3"`
	RequestCount int64  `json:"request_count" gorm:"default:0"`
	SuccessCount int64  `json:"success_count" gorm:"default:0"`
	LatencyMsSum int64  `json:"latency_ms_sum" gorm:"default:0"`
	TtftMsSum    int64  `json:"ttft_ms_sum" gorm:"default:0"`
	TtftCount    int64  `json:"ttft_count" gorm:"default:0"`
	OutputTokens int64  `json:"output_tokens" gorm:"default:0"`
	GenerationMs int64  `json:"generation_ms" gorm:"default:0"`
}

type channelMetricKey struct {
	bucket    int64
	channelId int
	model     string
}

var (
	channelMetricMu      sync.Mutex
	channelMetricPending = map[channelMetricKey]*ChannelMetric{}
)

func metricBucket(t time.Time) int64 {
	return t.Unix() / 3600 * 3600
}

func pendingMetric(channelId int, modelName string) *ChannelMetric {
	k := channelMetricKey{bucket: metricBucket(time.Now()), channelId: channelId, model: modelName}
	m := channelMetricPending[k]
	if m == nil {
		m = &ChannelMetric{BucketTs: k.bucket, ChannelId: channelId, ModelName: modelName}
		channelMetricPending[k] = m
	}
	return m
}

// RecordChannelSuccess counts a successful request. latencyMs is the whole request time,
// ttftMs the time to the first streamed token (0 when unknown or not streamed), and
// outputTokens the completion tokens.
func RecordChannelSuccess(channelId int, modelName string, latencyMs, ttftMs int64, outputTokens int) {
	if channelId <= 0 || !config.ChannelMetricsEnabled {
		return
	}
	channelMetricMu.Lock()
	defer channelMetricMu.Unlock()
	m := pendingMetric(channelId, modelName)
	m.RequestCount++
	m.SuccessCount++
	m.LatencyMsSum += latencyMs
	if ttftMs > 0 {
		m.TtftMsSum += ttftMs
		m.TtftCount++
	}
	if outputTokens > 0 && latencyMs > ttftMs {
		m.OutputTokens += int64(outputTokens)
		m.GenerationMs += latencyMs - ttftMs
	}
}

// RecordChannelFailure counts a failed upstream attempt.
func RecordChannelFailure(channelId int, modelName string) {
	if channelId <= 0 || !config.ChannelMetricsEnabled {
		return
	}
	channelMetricMu.Lock()
	defer channelMetricMu.Unlock()
	pendingMetric(channelId, modelName).RequestCount++
}

// FlushChannelMetrics adds the pending counters to the database rows.
func FlushChannelMetrics() {
	channelMetricMu.Lock()
	pending := channelMetricPending
	channelMetricPending = map[channelMetricKey]*ChannelMetric{}
	channelMetricMu.Unlock()
	for _, m := range pending {
		if err := addChannelMetric(m); err != nil {
			logger.SysError("flush channel metrics failed: " + err.Error())
		}
	}
}

func addChannelMetric(m *ChannelMetric) error {
	update := func() (int64, error) {
		res := DB.Model(&ChannelMetric{}).
			Where("bucket_ts = ? AND channel_id = ? AND model_name = ?", m.BucketTs, m.ChannelId, m.ModelName).
			Updates(map[string]any{
				"request_count":  gorm.Expr("request_count + ?", m.RequestCount),
				"success_count":  gorm.Expr("success_count + ?", m.SuccessCount),
				"latency_ms_sum": gorm.Expr("latency_ms_sum + ?", m.LatencyMsSum),
				"ttft_ms_sum":    gorm.Expr("ttft_ms_sum + ?", m.TtftMsSum),
				"ttft_count":     gorm.Expr("ttft_count + ?", m.TtftCount),
				"output_tokens":  gorm.Expr("output_tokens + ?", m.OutputTokens),
				"generation_ms":  gorm.Expr("generation_ms + ?", m.GenerationMs),
			})
		return res.RowsAffected, res.Error
	}
	n, err := update()
	if err != nil || n > 0 {
		return err
	}
	row := *m
	if err := DB.Create(&row).Error; err != nil {
		// another node inserted the row first: add to it instead
		_, err = update()
		return err
	}
	return nil
}

// StartChannelMetricsFlusher flushes the counters every minute.
func StartChannelMetricsFlusher() {
	go func() {
		for range time.Tick(time.Minute) {
			FlushChannelMetrics()
		}
	}()
}

// CleanupOldChannelMetrics deletes hourly rows older than ChannelMetricsRetentionDays.
func CleanupOldChannelMetrics() (int64, error) {
	days := config.ChannelMetricsRetentionDays
	if days <= 0 {
		return 0, nil
	}
	cutoff := time.Now().AddDate(0, 0, -days).Unix()
	res := DB.Where("bucket_ts < ?", cutoff).Delete(&ChannelMetric{})
	return res.RowsAffected, res.Error
}

// GetChannelMetrics returns hourly rows in [start, end] for one channel (or all when 0),
// optionally one model, sorted by time then channel and model.
func GetChannelMetrics(start, end int64, channelId int, modelName string) ([]*ChannelMetric, error) {
	tx := DB.Model(&ChannelMetric{}).Where("bucket_ts >= ? AND bucket_ts <= ?", metricBucket(time.Unix(start, 0)), end)
	if channelId > 0 {
		tx = tx.Where("channel_id = ?", channelId)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	rows := make([]*ChannelMetric, 0)
	if err := tx.Find(&rows).Error; err != nil {
		return nil, err
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].BucketTs != rows[j].BucketTs {
			return rows[i].BucketTs < rows[j].BucketTs
		}
		if rows[i].ChannelId != rows[j].ChannelId {
			return rows[i].ChannelId < rows[j].ChannelId
		}
		return rows[i].ModelName < rows[j].ModelName
	})
	return rows, nil
}
