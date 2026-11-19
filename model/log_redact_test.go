package model

import (
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func seedRedactLog(t *testing.T, userId int) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&Log{}, &LogDetail{}, &Channel{}); err != nil {
		t.Fatal(err)
	}
	oldSQLite, oldDB := common.UsingSQLite, DB
	common.UsingSQLite, DB = true, testDB
	t.Cleanup(func() { DB, common.UsingSQLite = oldDB, oldSQLite })
	log := &Log{
		UserId: userId, Type: LogTypeConsume, ModelName: "gpt-5.4-mini", Quota: 100, CostQuota: 80,
		ChannelId: 4, UpstreamRequestId: "req-1", UpstreamCost: 0.01, ExpectedCost: 0.02, CreatedAt: 1000,
		Metadata: datatypes.NewJSONType(map[string]any{
			"group_ratio":             1,
			"finish_reason":           "stop",
			"supply_id":               9,
			"upstream_model":          "openai/gpt-5.4-mini",
			"response_model":          "gpt-5.4-nano",
			"response_model_mismatch": true,
			"session_affinity":        "hit",
			"upstream_host":           "deepinfra",
			"retry_attempts":          []any{map[string]any{"channel_id": 3, "status": 429, "cooldown": true}},
		}),
	}
	if err := DB.Create(log).Error; err != nil {
		t.Fatal(err)
	}
}

func assertRedacted(t *testing.T, where string, l *Log) {
	t.Helper()
	if l.ChannelId != 0 || l.CostQuota != 0 || l.UpstreamCost != 0 || l.ExpectedCost != 0 || l.UpstreamRequestId != "" {
		t.Errorf("%s: channel or cost fields kept: %+v", where, l)
	}
	meta := l.Metadata.Data()
	for _, k := range adminOnlyLogMetaKeys {
		if _, ok := meta[k]; ok {
			t.Errorf("%s: metadata kept %s", where, k)
		}
	}
	if meta["finish_reason"] != "stop" {
		t.Errorf("%s: finish_reason dropped: %v", where, meta)
	}
	attempts, _ := meta["retry_attempts"].([]any)
	if len(attempts) != 1 {
		t.Fatalf("%s: retry_attempts = %v", where, meta["retry_attempts"])
	}
	a := attempts[0].(map[string]any)
	if _, ok := a["channel_id"]; ok || a["status"] != float64(429) || a["cooldown"] != true {
		t.Errorf("%s: attempt = %v, want status and cooldown without channel_id", where, a)
	}
}

// A user's own logs and export do not reveal channels, supplies, upstream names or costs.
func TestUserLogsAreRedacted(t *testing.T) {
	seedRedactLog(t, 7)
	params := &LogsListParams{PaginationParams: PaginationParams{Page: 1, Size: 10}}
	result, err := GetUserLogsList(7, params)
	if err != nil || len(*result.Data) != 1 {
		t.Fatalf("GetUserLogsList: %v %v", err, result)
	}
	assertRedacted(t, "list", (*result.Data)[0])
	all, err := GetAllUserLogsList(7, params, testExportPage)
	if err != nil || len(all) != 1 {
		t.Fatalf("GetAllUserLogsList: %v", err)
	}
	assertRedacted(t, "export", all[0])
}

// Organization logs (members and owners alike) are redacted the same way.
func TestOrgLogsAreRedacted(t *testing.T) {
	seedRedactLog(t, 500)
	result, err := GetOrgLogsList(500, &OrgLogsListParams{})
	if err != nil || len(*result.Data) != 1 {
		t.Fatalf("GetOrgLogsList: %v %v", err, result)
	}
	assertRedacted(t, "org list", (*result.Data)[0])
	all, err := GetAllOrgLogsList(500, &OrgLogsListParams{}, testExportPage)
	if err != nil || len(all) != 1 {
		t.Fatalf("GetAllOrgLogsList: %v", err)
	}
	assertRedacted(t, "org export", all[0])
}

// The site admin's log list keeps everything.
func TestAdminLogsKeepChannelFields(t *testing.T) {
	seedRedactLog(t, 7)
	result, err := GetLogsList(&LogsListParams{PaginationParams: PaginationParams{Page: 1, Size: 10}})
	if err != nil || len(*result.Data) != 1 {
		t.Fatalf("GetLogsList: %v", err)
	}
	l := (*result.Data)[0]
	if l.ChannelId != 4 || l.CostQuota != 80 || l.Metadata.Data()["supply_id"] == nil {
		t.Errorf("admin view lost fields: %+v", l)
	}
}
