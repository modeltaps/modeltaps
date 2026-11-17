package model

import (
	"sort"
	"time"

	"github.com/modeltaps/modeltaps/common"
)

// CostFloorStat sums up the requests of one model, channel and group that the cost floor charged
// at the channel cost, from the consume logs.
type CostFloorStat struct {
	Model       string `json:"model"`
	ChannelId   int    `json:"channel_id"`
	ChannelName string `json:"channel_name"`
	Group       string `json:"group" gorm:"column:group_name"`
	Requests    int64  `json:"requests"`
	// Charged is what those requests were charged; Quoted what the quote would have been. Both
	// in quota units.
	Charged int64 `json:"charged"`
	Quoted  int64 `json:"quoted"`
}

// CostFloorStats is the cost floor's effect over the last Days days.
type CostFloorStats struct {
	Days     int              `json:"days"`
	Requests int64            `json:"requests"`
	Charged  int64            `json:"charged"`
	Quoted   int64            `json:"quoted"`
	Rows     []*CostFloorStat `json:"rows"`
}

// billingMetaExpr extracts a key of metadata.billing as text, by database dialect.
func billingMetaExpr(key string) string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata -> 'billing' ->> '" + key + "')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$.billing." + key + "')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.billing." + key + "'))"
	}
}

func groupNameExpr() string {
	switch {
	case common.UsingPostgreSQL:
		return "(metadata ->> 'group_name')"
	case common.UsingSQLite:
		return "JSON_EXTRACT(metadata, '$.group_name')"
	default:
		return "JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.group_name'))"
	}
}

// GetCostFloorStats reads the consume logs of the last days (1 to 90) charged at the channel
// cost, grouped by model, channel and group, the largest surcharge first.
func GetCostFloorStats(days int) (*CostFloorStats, error) {
	if days < 1 {
		days = 7
	}
	if days > 90 {
		days = 90
	}
	since := time.Now().AddDate(0, 0, -days).Unix()
	quoted := "CAST(" + billingMetaExpr("quote_quota") + " AS DECIMAL(20,0))"
	if common.UsingPostgreSQL {
		quoted = "CAST(" + billingMetaExpr("quote_quota") + " AS NUMERIC)"
	}
	var rows []*CostFloorStat
	err := DB.Table("logs").
		Select("model_name AS model, channel_id, "+groupNameExpr()+" AS group_name"+
			", COUNT(*) AS requests, SUM(quota) AS charged, SUM("+quoted+") AS quoted").
		Where("type = ? AND created_at >= ? AND "+billingMetaExpr("basis")+" = ?", LogTypeConsume, since, "channel_cost").
		Group("model_name, channel_id, " + groupNameExpr()).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := &CostFloorStats{Days: days, Rows: rows}
	names := map[int]string{}
	for _, r := range rows {
		out.Requests += r.Requests
		out.Charged += r.Charged
		out.Quoted += r.Quoted
		if _, ok := names[r.ChannelId]; !ok {
			if ch := ChannelGroup.GetChannel(r.ChannelId); ch != nil {
				names[r.ChannelId] = ch.Name
			} else {
				names[r.ChannelId] = ""
			}
		}
		r.ChannelName = names[r.ChannelId]
	}
	sort.Slice(rows, func(i, j int) bool {
		di, dj := rows[i].Charged-rows[i].Quoted, rows[j].Charged-rows[j].Quoted
		if di != dj {
			return di > dj
		}
		return rows[i].Model < rows[j].Model
	})
	return out, nil
}
