package model

import (
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"gorm.io/datatypes"
)

func TestGetCostFloorStats(t *testing.T) {
	useSupplyTestDB(t)
	oldSQLite := common.UsingSQLite
	common.UsingSQLite = true
	t.Cleanup(func() { common.UsingSQLite = oldSQLite })
	if err := DB.AutoMigrate(&Log{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	add := func(model string, channel int, group, basis string, quota, quote int, at int64) {
		meta := map[string]any{"group_name": group}
		if basis != "" {
			meta[LogMetaBilling] = map[string]any{"basis": basis, "quote_quota": quote, "charged_quota": quota}
		}
		log := &Log{Type: LogTypeConsume, ModelName: model, ChannelId: channel, Quota: quota, CreatedAt: at, Metadata: datatypes.NewJSONType(meta)}
		if err := DB.Create(log).Error; err != nil {
			t.Fatal(err)
		}
	}
	add("gpt-5.5", 4, "default", "channel_cost", 1200, 1000, now)
	add("gpt-5.5", 4, "default", "channel_cost", 600, 500, now)
	add("gpt-5.5", 4, "vip", "channel_cost", 300, 290, now)
	add("gpt-5.5", 4, "default", "quote", 1000, 1000, now)
	add("gpt-5.5", 4, "default", "", 1000, 0, now)
	add("gpt-5.5", 4, "default", "channel_cost", 999, 1, now-30*86400)

	stats, err := GetCostFloorStats(7)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Requests != 3 || stats.Charged != 2100 || stats.Quoted != 1790 || len(stats.Rows) != 2 {
		t.Fatalf("stats = %+v", stats)
	}
	top := stats.Rows[0]
	if top.Group != "default" || top.Requests != 2 || top.Charged-top.Quoted != 300 || top.ChannelId != 4 {
		t.Errorf("top row = %+v", top)
	}
}
