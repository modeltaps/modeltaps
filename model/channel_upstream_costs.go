package model

// ChannelUpstreamCost compares, for one model of one channel, what the upstream reported it
// charged with what the gateway expected it to charge, over the requests whose upstream
// reported a cost. Both sums are in USD.
type ChannelUpstreamCost struct {
	ChannelId    int     `json:"channel_id"`
	ModelName    string  `json:"model_name"`
	RequestCount int64   `json:"request_count"`
	UpstreamCost float64 `json:"upstream_cost"`
	ExpectedCost float64 `json:"expected_cost"`
}

// GetChannelUpstreamCosts sums the consume logs that carry an upstream cost per channel and
// model in [start, end] (unix seconds; 0 leaves that side open). channelId > 0 limits the
// result to one channel. Rows are ordered by upstream cost, largest first.
func GetChannelUpstreamCosts(start, end int64, channelId int) ([]*ChannelUpstreamCost, error) {
	tx := DB.Table("logs").
		Select("channel_id, model_name, count(*) as request_count, sum(upstream_cost) as upstream_cost, sum(expected_cost) as expected_cost").
		Where("type = ? AND upstream_cost > 0", LogTypeConsume)
	if start > 0 {
		tx = tx.Where("created_at >= ?", start)
	}
	if end > 0 {
		tx = tx.Where("created_at <= ?", end)
	}
	if channelId > 0 {
		tx = tx.Where("channel_id = ?", channelId)
	}
	rows := make([]*ChannelUpstreamCost, 0)
	err := tx.Group("channel_id, model_name").Order("upstream_cost DESC").Scan(&rows).Error
	return rows, err
}
