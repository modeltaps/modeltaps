package model

import (
	"sync"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
)

// Supply health for cost routing: a supply must have served well lately to be put ahead for its
// cost (supply_cost_tier.go). Health is the success rate of the channel and upstream model over the
// current and the previous hour, from the channel metrics (hourly rows plus this node's counters
// not flushed yet). A supply with fewer than supplyHealthMinRequests attempts in that window counts
// as serving well: it has not been seen failing, and holding it back would keep it from ever
// getting the requests that show how it serves.

// supplyHealthMinRequests is the number of attempts below which a success rate is not judged.
const supplyHealthMinRequests = 20

// supplyHealthTTL is how long the success rates read from the metrics are reused.
const supplyHealthTTL = time.Minute

type supplyHealthKey struct {
	channelId int
	model     string
}

type supplyHealthCounts struct {
	requests, successes int64
}

var supplyHealth struct {
	sync.Mutex
	at     time.Time
	counts map[supplyHealthKey]supplyHealthCounts
}

// loadSupplyHealth reads the attempts and successes of every channel and model since the start of
// the previous hour.
func loadSupplyHealth(now time.Time) map[supplyHealthKey]supplyHealthCounts {
	out := map[supplyHealthKey]supplyHealthCounts{}
	since := metricBucket(now) - 3600
	if DB != nil {
		var rows []struct {
			ChannelId int
			ModelName string
			Requests  int64
			Successes int64
		}
		err := DB.Model(&ChannelMetric{}).
			Select("channel_id, model_name, SUM(request_count) AS requests, SUM(success_count) AS successes").
			Where("bucket_ts >= ?", since).Group("channel_id, model_name").Scan(&rows).Error
		if err == nil {
			for _, r := range rows {
				out[supplyHealthKey{r.ChannelId, r.ModelName}] = supplyHealthCounts{r.Requests, r.Successes}
			}
		}
	}
	channelMetricMu.Lock()
	for k, m := range channelMetricPending {
		if k.bucket < since {
			continue
		}
		key := supplyHealthKey{k.channelId, k.model}
		c := out[key]
		c.requests += m.RequestCount
		c.successes += m.SuccessCount
		out[key] = c
	}
	channelMetricMu.Unlock()
	return out
}

// supplySuccessRate is the recent success rate of the channel serving the upstream model, ok false
// when it had too few attempts to judge.
func supplySuccessRate(channelId int, upstream string) (rate float64, ok bool) {
	now := time.Now()
	supplyHealth.Lock()
	if supplyHealth.counts == nil || now.Sub(supplyHealth.at) >= supplyHealthTTL {
		supplyHealth.counts, supplyHealth.at = loadSupplyHealth(now), now
	}
	c := supplyHealth.counts[supplyHealthKey{channelId, upstream}]
	supplyHealth.Unlock()
	if c.requests < supplyHealthMinRequests {
		return 0, false
	}
	return float64(c.successes) / float64(c.requests), true
}

// supplyServesWell reports whether the supply may be put ahead for its cost: its recent success
// rate reaches SupplyCostMinSuccessPercent, or it is not judged yet, or the check is off.
func supplyServesWell(channelId int, upstream string) bool {
	if config.SupplyCostMinSuccessPercent <= 0 {
		return true
	}
	rate, ok := supplySuccessRate(channelId, upstream)
	return !ok || rate*100 >= config.SupplyCostMinSuccessPercent
}

// resetSupplyHealth drops the cached success rates (tests).
func resetSupplyHealth() {
	supplyHealth.Lock()
	supplyHealth.counts = nil
	supplyHealth.Unlock()
}
