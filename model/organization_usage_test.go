package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/datatypes"
)

// seedOrgUsageLogs 造一个影子账户与两名成员的消费日志,外加一条无关用户日志
func seedOrgUsageLogs(t *testing.T) (shadowUserId int) {
	t.Helper()
	shadow, err := CreateShadowUserForOrg(DB, "usage-test")
	if err != nil {
		t.Fatalf("创建影子账户失败: %v", err)
	}
	memberLogs := []struct {
		memberId   int
		tokenName  string
		modelName  string
		createdAt  int64
		quota      int
		prompt     int
		cached     int
		groupRatio float64 // 0 表示无折扣(不写 metadata.group_ratio)
	}{
		{101, "tok-a", "gpt-4o", 1700000000, 100, 10, 4, 0},
		{101, "tok-a", "gpt-4o-mini", 1700000100, 50, 5, 0, 0},
		{102, "tok-b", "gpt-4o", 1700086400, 200, 20, 8, 0.5},
	}
	for _, ml := range memberLogs {
		meta := map[string]any{"org_member_id": ml.memberId}
		if ml.cached > 0 {
			meta["cached_read_tokens"] = ml.cached
		}
		if ml.groupRatio > 0 {
			meta["group_ratio"] = ml.groupRatio
		}
		log := &Log{
			UserId:       shadow.Id,
			Username:     shadow.Username,
			Type:         LogTypeConsume,
			TokenName:    ml.tokenName,
			ModelName:    ml.modelName,
			CreatedAt:    ml.createdAt,
			Quota:        ml.quota,
			PromptTokens: ml.prompt,
			Metadata:     datatypes.NewJSONType(meta),
		}
		if err := DB.Create(log).Error; err != nil {
			t.Fatalf("插入组织日志失败: %v", err)
		}
	}
	// 影子账户充值日志(无 org_member_id)与无关用户消费日志,均不应进入消费聚合
	if err := DB.Create(&Log{UserId: shadow.Id, Type: LogTypeTopup, CreatedAt: 1700000000, Quota: 1000}).Error; err != nil {
		t.Fatalf("插入充值日志失败: %v", err)
	}
	if err := DB.Create(&Log{UserId: 999, Type: LogTypeConsume, CreatedAt: 1700000000, Quota: 777, ModelName: "gpt-4o"}).Error; err != nil {
		t.Fatalf("插入无关日志失败: %v", err)
	}
	return shadow.Id
}

func TestGetOrgUsageStatistics(t *testing.T) {
	setupOrgTestDB(t)
	shadowUserId := seedOrgUsageLogs(t)
	params := &OrgLogsListParams{}

	t.Run("按成员聚合", func(t *testing.T) {
		stats, err := GetOrgUsageStatistics(shadowUserId, OrgUsageGroupMember, params)
		if err != nil {
			t.Fatalf("按成员聚合失败: %v", err)
		}
		if len(stats) != 2 {
			t.Fatalf("应有 2 个成员分组,实际 %d", len(stats))
		}
		// 默认按 quota DESC:成员 102 (200) 在前
		if stats[0].MemberId != 102 || stats[0].Quota != 200 {
			t.Fatalf("成员 102 聚合错误: %+v", stats[0])
		}
		if stats[1].MemberId != 101 || stats[1].Quota != 150 || stats[1].RequestCount != 2 {
			t.Fatalf("成员 101 聚合错误: %+v", stats[1])
		}
		// 命中率口径:cached_read_tokens 汇总 metadata,缺失键归零。
		// 成员 102: cached 8 / prompt 20;成员 101: cached 4(仅一条有) / prompt 15
		if stats[0].CachedReadTokens != 8 {
			t.Fatalf("成员 102 cached_read_tokens 应为 8,实际 %d", stats[0].CachedReadTokens)
		}
		if stats[1].CachedReadTokens != 4 {
			t.Fatalf("成员 101 cached_read_tokens 应为 4(缺 metadata 归零),实际 %d", stats[1].CachedReadTokens)
		}
		// 折扣节省额:成员 102 group_ratio=0.5,original=400,节省=200;成员 101 无折扣归零
		if stats[0].SavedQuota != 200 {
			t.Fatalf("成员 102 saved_quota 应为 200(200/0.5-200),实际 %v", stats[0].SavedQuota)
		}
		if stats[1].SavedQuota != 0 {
			t.Fatalf("成员 101 saved_quota 应为 0(无折扣),实际 %v", stats[1].SavedQuota)
		}
	})

	t.Run("按模型聚合", func(t *testing.T) {
		stats, err := GetOrgUsageStatistics(shadowUserId, OrgUsageGroupModel, params)
		if err != nil {
			t.Fatalf("按模型聚合失败: %v", err)
		}
		if len(stats) != 2 {
			t.Fatalf("应有 2 个模型分组,实际 %d", len(stats))
		}
		if stats[0].ModelName != "gpt-4o" || stats[0].Quota != 300 {
			t.Fatalf("gpt-4o 聚合错误(不应包含无关用户的 777): %+v", stats[0])
		}
	})

	t.Run("按令牌聚合并按成员过滤", func(t *testing.T) {
		p := &OrgLogsListParams{MemberId: 101}
		stats, err := GetOrgUsageStatistics(shadowUserId, OrgUsageGroupToken, p)
		if err != nil {
			t.Fatalf("按令牌聚合失败: %v", err)
		}
		if len(stats) != 1 || stats[0].TokenName != "tok-a" || stats[0].Quota != 150 {
			t.Fatalf("成员过滤后令牌聚合错误: %+v", stats)
		}
	})

	t.Run("按日期聚合", func(t *testing.T) {
		oldSQLite := common.UsingSQLite
		common.UsingSQLite = true
		t.Cleanup(func() { common.UsingSQLite = oldSQLite })
		stats, err := GetOrgUsageStatistics(shadowUserId, OrgUsageGroupDate, params)
		if err != nil {
			t.Fatalf("按日期聚合失败: %v", err)
		}
		if len(stats) != 2 {
			t.Fatalf("两天的日志应有 2 个日期分组,实际 %d", len(stats))
		}
	})

	t.Run("非法维度报错", func(t *testing.T) {
		if _, err := GetOrgUsageStatistics(shadowUserId, "channel", params); err == nil {
			t.Fatal("非法聚合维度应报错")
		}
	})
}

func TestOrgLogsListAndSum(t *testing.T) {
	setupOrgTestDB(t)
	shadowUserId := seedOrgUsageLogs(t)

	result, err := GetOrgLogsList(shadowUserId, &OrgLogsListParams{})
	if err != nil {
		t.Fatalf("组织日志列表失败: %v", err)
	}
	if result.TotalCount != 4 {
		t.Fatalf("组织日志应为 4 条(3 消费 + 1 充值),实际 %d", result.TotalCount)
	}

	filtered, err := GetOrgLogsList(shadowUserId, &OrgLogsListParams{MemberId: 102})
	if err != nil {
		t.Fatalf("按成员过滤日志失败: %v", err)
	}
	if filtered.TotalCount != 1 {
		t.Fatalf("成员 102 应只有 1 条日志,实际 %d", filtered.TotalCount)
	}

	if quota := SumOrgUsedQuota(shadowUserId, &OrgLogsListParams{}); quota != 350 {
		t.Fatalf("组织消费总额应为 350(不含充值与他人日志),实际 %d", quota)
	}
	if quota := SumOrgUsedQuota(shadowUserId, &OrgLogsListParams{MemberId: 101}); quota != 150 {
		t.Fatalf("成员 101 消费总额应为 150,实际 %d", quota)
	}
}

// seedOrgMetricLogs 造带 finish_reason/completion_tokens 的组织消费日志,专测 W8-E 三过滤。
// 返回影子账户 id;其中一条不含 finish_reason(模拟旧日志容错)。
func seedOrgMetricLogs(t *testing.T) (shadowUserId int) {
	t.Helper()
	shadow, err := CreateShadowUserForOrg(DB, "metric-test")
	if err != nil {
		t.Fatalf("创建影子账户失败: %v", err)
	}
	rows := []struct {
		memberId     int
		finishReason string
		quota        int
		prompt       int
		completion   int
	}{
		{201, "stop", 100, 50, 50},     // tokens=100
		{201, "length", 300, 200, 100}, // tokens=300
		{202, "error", 10, 5, 5},       // tokens=10
		{202, "", 500, 400, 100},       // 无 finish_reason(旧日志),tokens=500
	}
	for _, r := range rows {
		meta := map[string]any{"org_member_id": r.memberId}
		if r.finishReason != "" {
			meta["finish_reason"] = r.finishReason
		}
		log := &Log{
			UserId:           shadow.Id,
			Username:         shadow.Username,
			Type:             LogTypeConsume,
			CreatedAt:        1700000000,
			Quota:            r.quota,
			PromptTokens:     r.prompt,
			CompletionTokens: r.completion,
			Metadata:         datatypes.NewJSONType(meta),
		}
		if err := DB.Create(log).Error; err != nil {
			t.Fatalf("插入组织日志失败: %v", err)
		}
	}
	return shadow.Id
}

func TestOrgLogsMetricFilters(t *testing.T) {
	setupOrgTestDB(t)
	// finishReasonExpr 走 SQLite 方言分支
	oldSQLite := common.UsingSQLite
	common.UsingSQLite = true
	t.Cleanup(func() { common.UsingSQLite = oldSQLite })

	shadowUserId := seedOrgMetricLogs(t)

	t.Run("finish_reason 过滤", func(t *testing.T) {
		res, err := GetOrgLogsList(shadowUserId, &OrgLogsListParams{FinishReason: "stop"})
		if err != nil {
			t.Fatalf("finish_reason 列表失败: %v", err)
		}
		if res.TotalCount != 1 || res.Data == nil || (*res.Data)[0].Quota != 100 {
			t.Fatalf("finish_reason=stop 应只命中 quota=100 一条,实际 %+v", res)
		}
		res, err = GetOrgLogsList(shadowUserId, &OrgLogsListParams{FinishReason: "length"})
		if err != nil {
			t.Fatalf("finish_reason 列表失败: %v", err)
		}
		if res.TotalCount != 1 {
			t.Fatalf("finish_reason=length 应只命中 1 条,实际 %d", res.TotalCount)
		}
	})

	t.Run("min_quota 过滤", func(t *testing.T) {
		res, err := GetOrgLogsList(shadowUserId, &OrgLogsListParams{MinQuota: 300})
		if err != nil {
			t.Fatalf("min_quota 列表失败: %v", err)
		}
		if res.TotalCount != 2 {
			t.Fatalf("min_quota>=300 应命中 2 条(300/500),实际 %d", res.TotalCount)
		}
	})

	t.Run("min_tokens 过滤", func(t *testing.T) {
		res, err := GetOrgLogsList(shadowUserId, &OrgLogsListParams{MinTokens: 300})
		if err != nil {
			t.Fatalf("min_tokens 列表失败: %v", err)
		}
		if res.TotalCount != 2 {
			t.Fatalf("min_tokens>=300 应命中 2 条(300/500),实际 %d", res.TotalCount)
		}
	})

	t.Run("三过滤组合", func(t *testing.T) {
		res, err := GetOrgLogsList(shadowUserId, &OrgLogsListParams{FinishReason: "length", MinQuota: 300, MinTokens: 300})
		if err != nil {
			t.Fatalf("组合过滤失败: %v", err)
		}
		if res.TotalCount != 1 {
			t.Fatalf("组合过滤应只命中 length/300/300 一条,实际 %d", res.TotalCount)
		}
	})

	t.Run("导出路径同样施加", func(t *testing.T) {
		logs, err := GetAllOrgLogsList(shadowUserId, &OrgLogsListParams{MinQuota: 300})
		if err != nil {
			t.Fatalf("导出过滤失败: %v", err)
		}
		if len(logs) != 2 {
			t.Fatalf("导出 min_quota>=300 应命中 2 条,实际 %d", len(logs))
		}
	})

	t.Run("总消费施加下限", func(t *testing.T) {
		if quota := SumOrgUsedQuota(shadowUserId, &OrgLogsListParams{MinQuota: 300}); quota != 800 {
			t.Fatalf("min_quota>=300 总消费应为 800(300+500),实际 %d", quota)
		}
	})

	t.Run("空值不过滤", func(t *testing.T) {
		res, err := GetOrgLogsList(shadowUserId, &OrgLogsListParams{})
		if err != nil {
			t.Fatalf("无过滤列表失败: %v", err)
		}
		if res.TotalCount != 4 {
			t.Fatalf("无过滤应返回全部 4 条,实际 %d", res.TotalCount)
		}
	})
}

func TestUserStatisticsExcludeShadow(t *testing.T) {
	setupOrgTestDB(t)
	normal := &User{Username: "alice", Password: "x", Quota: 100, UsedQuota: 10, Type: config.UserTypeNormal, CreatedTime: 1700000000}
	if err := DB.Create(normal).Error; err != nil {
		t.Fatalf("创建普通用户失败: %v", err)
	}
	shadow, err := CreateShadowUserForOrg(DB, "stat-test")
	if err != nil {
		t.Fatalf("创建影子账户失败: %v", err)
	}
	if err := DB.Model(&User{}).Where("id = ?", shadow.Id).Updates(map[string]any{"quota": 9999, "used_quota": 8888, "created_time": 1700000000}).Error; err != nil {
		t.Fatalf("更新影子账户配额失败: %v", err)
	}

	stat, err := GetStatisticsUser()
	if err != nil {
		t.Fatalf("GetStatisticsUser 失败: %v", err)
	}
	if stat.TotalUser != 1 {
		t.Fatalf("用户总数应排除影子账户(期望 1),实际 %d", stat.TotalUser)
	}
	if stat.TotalQuota != 100 || stat.TotalUsedQuota != 10 {
		t.Fatalf("配额统计应排除影子账户,实际 quota=%d used=%d", stat.TotalQuota, stat.TotalUsedQuota)
	}

	oldSQLite := common.UsingSQLite
	common.UsingSQLite = true
	t.Cleanup(func() { common.UsingSQLite = oldSQLite })
	periods, err := GetUserStatisticsByPeriod(1699990000, 1700010000)
	if err != nil {
		t.Fatalf("GetUserStatisticsByPeriod 失败: %v", err)
	}
	var total int64
	for _, p := range periods {
		total += p.UserCount
	}
	if total != 1 {
		t.Fatalf("注册统计应排除影子账户(期望 1),实际 %d", total)
	}
}

func TestGetUsernamesByIds(t *testing.T) {
	setupOrgTestDB(t)
	u1 := &User{Username: "bob", Password: "x", AccessToken: "at-bob", AffCode: "aff-bob"}
	u2 := &User{Username: "carol", Password: "x", AccessToken: "at-carol", AffCode: "aff-carol"}
	if err := DB.Create(u1).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	if err := DB.Create(u2).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	m, err := GetUsernamesByIds([]int{u1.Id, u2.Id, 99999})
	if err != nil {
		t.Fatalf("GetUsernamesByIds 失败: %v", err)
	}
	if len(m) != 2 || m[u1.Id] != "bob" || m[u2.Id] != "carol" {
		t.Fatalf("用户名映射错误: %v", m)
	}
	empty, err := GetUsernamesByIds(nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("空 ID 列表应返回空映射: %v, %v", empty, err)
	}
}
