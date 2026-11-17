package model

import (
	"math"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// testExportPage reads a whole small test export in one page.
var testExportPage = LogExportPage{MaxId: math.MaxInt32, Limit: 1000}

func setupLogExportDB(t *testing.T) {
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
}

func seedExportLogs(t *testing.T, userId, n int, createdAt int64) {
	t.Helper()
	logs := make([]*Log, 0, n)
	for i := 0; i < n; i++ {
		logs = append(logs, &Log{UserId: userId, Type: LogTypeConsume, Content: "c", CreatedAt: createdAt})
	}
	if err := DB.Create(logs).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
}

// readExportPages reads an export the way the controller does, page by page.
func readExportPages(t *testing.T, stat LogExportStat, pageSize int, read func(LogExportPage) ([]*Log, error)) []int {
	t.Helper()
	var ids []int
	for offset := 0; ; offset += pageSize {
		logs, err := read(LogExportPage{MaxId: stat.MaxId, Offset: offset, Limit: pageSize})
		if err != nil {
			t.Fatalf("page at %d: %v", offset, err)
		}
		for _, l := range logs {
			ids = append(ids, l.Id)
		}
		if len(logs) < pageSize {
			return ids
		}
	}
}

func assertEachIdOnce(t *testing.T, ids []int, want int) {
	t.Helper()
	seen := map[int]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("id %d exported twice: %v", id, ids)
		}
		seen[id] = true
	}
	if len(ids) != want {
		t.Fatalf("exported %d rows, want %d", len(ids), want)
	}
}

// Pages of an export cover each matching row once, even when the requested order has ties
// (all rows share created_at) and when rows are written while the export runs.
func TestUserLogExportPagesCoverEachRowOnce(t *testing.T) {
	setupLogExportDB(t)
	seedExportLogs(t, 7, 23, 1000)
	seedExportLogs(t, 8, 5, 1000)
	params := &LogsListParams{StartTimestamp: 900, EndTimestamp: 1100}
	params.Order = "-created_at"

	stat, err := CountAllUserLogsList(7, params)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if stat.Count != 23 || stat.MaxId == 0 {
		t.Fatalf("stat = %+v, want 23 rows and a max id", stat)
	}

	// Rows written after the export started are left out instead of shifting the pages.
	seedExportLogs(t, 7, 4, 1000)

	ids := readExportPages(t, stat, 5, func(page LogExportPage) ([]*Log, error) {
		return GetAllUserLogsList(7, params, page)
	})
	assertEachIdOnce(t, ids, 23)
	for _, id := range ids {
		if id > stat.MaxId {
			t.Fatalf("id %d is newer than the export snapshot %d", id, stat.MaxId)
		}
	}
}

func TestAdminLogExportCountsAndPages(t *testing.T) {
	setupLogExportDB(t)
	seedExportLogs(t, 7, 12, 1000)
	seedExportLogs(t, 8, 3, 5000) // outside the range

	params := &LogsListParams{StartTimestamp: 900, EndTimestamp: 1100}
	stat, err := CountAllLogsList(params)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if stat.Count != 12 {
		t.Fatalf("count = %d, want 12", stat.Count)
	}
	ids := readExportPages(t, stat, 4, func(page LogExportPage) ([]*Log, error) {
		return GetAllLogsList(params, page)
	})
	assertEachIdOnce(t, ids, 12)
}

func TestOrgLogExportCountsAndPages(t *testing.T) {
	setupLogExportDB(t)
	seedExportLogs(t, 500, 9, 1000)
	seedExportLogs(t, 7, 6, 1000) // not the organization's shadow account

	params := &OrgLogsListParams{StartTimestamp: 900, EndTimestamp: 1100}
	stat, err := CountAllOrgLogsList(500, params)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if stat.Count != 9 {
		t.Fatalf("count = %d, want 9", stat.Count)
	}
	ids := readExportPages(t, stat, 2, func(page LogExportPage) ([]*Log, error) {
		return GetAllOrgLogsList(500, params, page)
	})
	assertEachIdOnce(t, ids, 9)
}

// Nothing matches: the count is zero and no page returns rows.
func TestLogExportEmpty(t *testing.T) {
	setupLogExportDB(t)
	params := &LogsListParams{StartTimestamp: 900, EndTimestamp: 1100}
	stat, err := CountAllUserLogsList(7, params)
	if err != nil || stat.Count != 0 || stat.MaxId != 0 {
		t.Fatalf("stat = %+v, err = %v", stat, err)
	}
	logs, err := GetAllUserLogsList(7, params, LogExportPage{MaxId: stat.MaxId, Limit: 10})
	if err != nil || len(logs) != 0 {
		t.Fatalf("logs = %v, err = %v", logs, err)
	}
}
