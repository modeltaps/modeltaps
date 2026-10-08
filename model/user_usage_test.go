package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/datatypes"
)

// seedUserUsageLogs 造一个用户的消费日志,外加充值日志与无关用户日志。
// cached 为 metadata.cached_read_tokens(0 表示不写 metadata,覆盖缺失键归零口径)。
func seedUserUsageLogs(t *testing.T) (userId int) {
	t.Helper()
	user := &User{Username: "usage-user", Password: "x", AccessToken: "at-usage", AffCode: "aff-usage"}
	if err := DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	consumeLogs := []struct {
		tokenName  string
		modelName  string
		createdAt  int64
		quota      int
		prompt     int
		cached     int
		groupRatio float64 // 0 表示无折扣(不写 metadata.group_ratio)
	}{
		{"tok-a", "gpt-4o", 1700000000, 100, 10, 4, 0},
		{"tok-a", "gpt-4o-mini", 1700000100, 50, 5, 0, 0},
		{"tok-b", "gpt-4o", 1700086400, 200, 20, 8, 0.5},
	}
	for _, cl := range consumeLogs {
		log := &Log{
			UserId:       user.Id,
			Username:     user.Username,
			Type:         LogTypeConsume,
			TokenName:    cl.tokenName,
			ModelName:    cl.modelName,
			CreatedAt:    cl.createdAt,
			Quota:        cl.quota,
			PromptTokens: cl.prompt,
		}
		meta := map[string]any{}
		if cl.cached > 0 {
			meta["cached_read_tokens"] = cl.cached
		}
		if cl.groupRatio > 0 {
			meta["group_ratio"] = cl.groupRatio
		}
		if len(meta) > 0 {
			log.Metadata = datatypes.NewJSONType(meta)
		}
		if err := DB.Create(log).Error; err != nil {
			t.Fatalf("插入消费日志失败: %v", err)
		}
	}
	// 本人充值日志与无关用户消费日志,均不应进入消费聚合
	if err := DB.Create(&Log{UserId: user.Id, Type: LogTypeTopup, CreatedAt: 1700000000, Quota: 1000}).Error; err != nil {
		t.Fatalf("插入充值日志失败: %v", err)
	}
	if err := DB.Create(&Log{UserId: 999, Type: LogTypeConsume, CreatedAt: 1700000000, Quota: 777, ModelName: "gpt-4o", TokenName: "tok-a"}).Error; err != nil {
		t.Fatalf("插入无关日志失败: %v", err)
	}
	return user.Id
}

func TestGetUserUsageStatistics(t *testing.T) {
	setupOrgTestDB(t)
	userId := seedUserUsageLogs(t)
	params := &LogsListParams{}

	t.Run("按模型聚合", func(t *testing.T) {
		stats, err := GetUserUsageStatistics(userId, UserUsageGroupModel, params)
		if err != nil {
			t.Fatalf("按模型聚合失败: %v", err)
		}
		if len(stats) != 2 {
			t.Fatalf("应有 2 个模型分组,实际 %d", len(stats))
		}
		// 默认按 quota DESC:gpt-4o (300) 在前,且不应包含无关用户的 777 与充值日志
		if stats[0].ModelName != "gpt-4o" || stats[0].Quota != 300 || stats[0].RequestCount != 2 {
			t.Fatalf("gpt-4o 聚合错误: %+v", stats[0])
		}
		if stats[1].ModelName != "gpt-4o-mini" || stats[1].Quota != 50 {
			t.Fatalf("gpt-4o-mini 聚合错误: %+v", stats[1])
		}
	})

	t.Run("按令牌聚合", func(t *testing.T) {
		stats, err := GetUserUsageStatistics(userId, UserUsageGroupToken, params)
		if err != nil {
			t.Fatalf("按令牌聚合失败: %v", err)
		}
		if len(stats) != 2 {
			t.Fatalf("应有 2 个令牌分组,实际 %d", len(stats))
		}
		if stats[0].TokenName != "tok-b" || stats[0].Quota != 200 {
			t.Fatalf("tok-b 聚合错误: %+v", stats[0])
		}
		if stats[1].TokenName != "tok-a" || stats[1].Quota != 150 || stats[1].RequestCount != 2 || stats[1].PromptTokens != 15 {
			t.Fatalf("tok-a 聚合错误: %+v", stats[1])
		}
		// 命中率口径:cached_read_tokens 汇总 metadata,缺失键归零。
		// tok-b: cached 8 / prompt 20;tok-a: cached 4(仅一条有 metadata) / prompt 15
		if stats[0].CachedReadTokens != 8 {
			t.Fatalf("tok-b cached_read_tokens 应为 8,实际 %d", stats[0].CachedReadTokens)
		}
		if stats[1].CachedReadTokens != 4 {
			t.Fatalf("tok-a cached_read_tokens 应为 4(缺 metadata 归零),实际 %d", stats[1].CachedReadTokens)
		}
		// 折扣节省额:tok-b group_ratio=0.5,original=400,节省=200;tok-a 无折扣归零
		if stats[0].SavedQuota != 200 {
			t.Fatalf("tok-b saved_quota 应为 200(200/0.5-200),实际 %v", stats[0].SavedQuota)
		}
		if stats[1].SavedQuota != 0 {
			t.Fatalf("tok-a saved_quota 应为 0(无折扣),实际 %v", stats[1].SavedQuota)
		}
	})

	t.Run("按模型过滤后令牌聚合", func(t *testing.T) {
		p := &LogsListParams{ModelName: "gpt-4o"}
		stats, err := GetUserUsageStatistics(userId, UserUsageGroupToken, p)
		if err != nil {
			t.Fatalf("按令牌聚合失败: %v", err)
		}
		if len(stats) != 2 || stats[0].TokenName != "tok-b" || stats[0].Quota != 200 || stats[1].Quota != 100 {
			t.Fatalf("模型过滤后令牌聚合错误: %+v", stats)
		}
	})

	t.Run("按日期聚合", func(t *testing.T) {
		oldSQLite := common.UsingSQLite
		common.UsingSQLite = true
		t.Cleanup(func() { common.UsingSQLite = oldSQLite })
		stats, err := GetUserUsageStatistics(userId, UserUsageGroupDate, params)
		if err != nil {
			t.Fatalf("按日期聚合失败: %v", err)
		}
		if len(stats) != 2 {
			t.Fatalf("两天的日志应有 2 个日期分组,实际 %d", len(stats))
		}
		// date ASC:第一天 150,第二天 200
		if stats[0].Quota != 150 || stats[1].Quota != 200 {
			t.Fatalf("日期聚合排序或数值错误: %+v, %+v", stats[0], stats[1])
		}
	})

	t.Run("时间范围过滤", func(t *testing.T) {
		p := &LogsListParams{StartTimestamp: 1700000000, EndTimestamp: 1700000200}
		stats, err := GetUserUsageStatistics(userId, UserUsageGroupModel, p)
		if err != nil {
			t.Fatalf("时间范围过滤聚合失败: %v", err)
		}
		if len(stats) != 2 || stats[0].Quota != 100 || stats[1].Quota != 50 {
			t.Fatalf("时间范围过滤聚合错误: %+v", stats)
		}
	})

	t.Run("非法维度报错", func(t *testing.T) {
		if _, err := GetUserUsageStatistics(userId, "member", params); err == nil {
			t.Fatal("非法聚合维度应报错")
		}
	})
}
