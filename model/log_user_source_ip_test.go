package model

import (
	"sort"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupUserSourceIpDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&Log{}, &LogDetail{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldUsingSQLite, oldDB := common.UsingSQLite, DB
	common.UsingSQLite = true
	DB = testDB
	t.Cleanup(func() { DB, common.UsingSQLite = oldDB, oldUsingSQLite })

	seed := []*Log{
		{UserId: 7, Type: LogTypeConsume, Content: "c1", Quota: 10, SourceIp: "1.1.1.1", CreatedAt: 1000},
		{UserId: 7, Type: LogTypeConsume, Content: "c2", Quota: 20, SourceIp: "2.2.2.2", CreatedAt: 1000},
		{UserId: 7, Type: LogTypeConsume, Content: "c3", Quota: 40, SourceIp: "", CreatedAt: 1000},
		{UserId: 7, Type: LogTypeTopup, Content: "t1", SourceIp: "1.1.1.1", CreatedAt: 1000},
		// An administrator acted on the account from 9.9.9.9.
		{UserId: 7, Type: LogTypeManage, Content: "m1", SourceIp: "9.9.9.9", CreatedAt: 1000},
		{UserId: 7, Type: LogTypeSystem, Content: "s1", SourceIp: "1.1.1.1", CreatedAt: 1000},
		{UserId: 8, Type: LogTypeConsume, Content: "other", Quota: 5, SourceIp: "1.1.1.1", CreatedAt: 1000},
	}
	if err := DB.Create(seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func userLogContents(t *testing.T, params *LogsListParams) []string {
	t.Helper()
	params.PaginationParams = PaginationParams{Page: 1, Size: 100}
	result, err := GetUserLogsList(7, params)
	if err != nil {
		t.Fatalf("GetUserLogsList: %v", err)
	}
	all, err := GetAllUserLogsList(7, params)
	if err != nil {
		t.Fatalf("GetAllUserLogsList: %v", err)
	}
	var list []string
	for _, l := range *result.Data {
		list = append(list, l.Content)
	}
	var export []string
	for _, l := range all {
		export = append(export, l.Content)
	}
	sort.Strings(list)
	sort.Strings(export)
	if len(list) != len(export) {
		t.Fatalf("list %v and export %v must agree", list, export)
	}
	for i := range list {
		if list[i] != export[i] {
			t.Fatalf("list %v and export %v must agree", list, export)
		}
	}
	return list
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestUserLogsFilterBySourceIp(t *testing.T) {
	setupUserSourceIpDB(t)

	cases := []struct {
		name   string
		params LogsListParams
		want   []string
	}{
		{"no filter lists everything of the user", LogsListParams{}, []string{"c1", "c2", "c3", "m1", "s1", "t1"}},
		{"single value", LogsListParams{SourceIp: "1.1.1.1"}, []string{"c1", "t1"}},
		{"include list", LogsListParams{SourceIps: []string{"1.1.1.1", "2.2.2.2"}}, []string{"c1", "c2", "t1"}},
		{"exclude list keeps rows without a visible ip", LogsListParams{ExcludeSourceIps: []string{"1.1.1.1"}}, []string{"c2", "c3", "m1", "s1"}},
		{"include and exclude combine", LogsListParams{SourceIps: []string{"1.1.1.1", "2.2.2.2"}, ExcludeSourceIps: []string{"2.2.2.2"}}, []string{"c1", "t1"}},
		{"unknown ip matches nothing", LogsListParams{SourceIp: "3.3.3.3"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := tc.params
			if got := userLogContents(t, &params); !equalStrings(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Manage and system logs hold the administrator's IP, which the user side blanks. They must not
// be matchable either, or a user could test candidate IPs against the row count.
func TestUserLogsSourceIpDoesNotRevealAdminIp(t *testing.T) {
	setupUserSourceIpDB(t)

	if got := userLogContents(t, &LogsListParams{SourceIp: "9.9.9.9"}); len(got) != 0 {
		t.Fatalf("filtering by the administrator's IP must match nothing, got %v", got)
	}
	if got := userLogContents(t, &LogsListParams{SourceIps: []string{"9.9.9.9"}}); len(got) != 0 {
		t.Fatalf("include list with the administrator's IP must match nothing, got %v", got)
	}
	// The manage log with that IP is kept by an exclude filter, exactly as if it had no IP, so the
	// result is the same for the administrator's IP and for an IP nobody used.
	withAdmin := userLogContents(t, &LogsListParams{ExcludeSourceIps: []string{"9.9.9.9"}})
	withUnused := userLogContents(t, &LogsListParams{ExcludeSourceIps: []string{"8.8.8.8"}})
	if !equalStrings(withAdmin, withUnused) {
		t.Fatalf("exclude result must not depend on the administrator's IP: %v vs %v", withAdmin, withUnused)
	}
	// A system log written under an ordinary user IP is not matchable by it.
	if got := userLogContents(t, &LogsListParams{SourceIp: "1.1.1.1"}); equalStrings(got, []string{"c1", "s1", "t1"}) || !equalStrings(got, []string{"c1", "t1"}) {
		t.Fatalf("system logs must not match the IP filter, got %v", got)
	}
}

func TestUserLogsHistogramAndStatFilterBySourceIp(t *testing.T) {
	setupUserSourceIpDB(t)

	buckets, err := GetUserLogsRequestHistogram(7, &LogsListParams{SourceIps: []string{"1.1.1.1"}}, "day")
	if err != nil {
		t.Fatalf("histogram: %v", err)
	}
	var count int64
	for _, b := range buckets {
		count += b.Count
	}
	if count != 2 {
		t.Fatalf("histogram count for 1.1.1.1 = %d, want 2 (consume + top-up, no system log, no other user)", count)
	}

	// The total spend only counts consume logs and stays on the user's own rows.
	if got := SumUsedQuota(&LogsListParams{UserId: 7, SourceIps: []string{"1.1.1.1"}}); got != 10 {
		t.Fatalf("SumUsedQuota(1.1.1.1) = %d, want 10", got)
	}
	if got := SumUsedQuota(&LogsListParams{UserId: 7, SourceIp: "2.2.2.2"}); got != 20 {
		t.Fatalf("SumUsedQuota(single 2.2.2.2) = %d, want 20", got)
	}
	if got := SumUsedQuota(&LogsListParams{UserId: 7, ExcludeSourceIps: []string{"1.1.1.1"}}); got != 60 {
		t.Fatalf("SumUsedQuota(exclude 1.1.1.1) = %d, want 60", got)
	}
}
