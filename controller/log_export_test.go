package controller

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupLogExportEndpointDB(t *testing.T, userId, rows int, createdAt int64) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&model.LogDetail{}, &model.Log{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB, oldSQLite := model.DB, common.UsingSQLite
	model.DB, common.UsingSQLite = testDB, true
	t.Cleanup(func() { model.DB, common.UsingSQLite = oldDB, oldSQLite })

	logs := make([]*model.Log, 0, rows)
	for i := 0; i < rows; i++ {
		logs = append(logs, &model.Log{UserId: userId, Type: model.LogTypeConsume, ModelName: "m", Content: fmt.Sprintf("row %d", i), CreatedAt: createdAt})
	}
	if rows > 0 {
		if err := testDB.Create(logs).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func setLogExportLimits(t *testing.T, maxRows, pageSize int) {
	t.Helper()
	oldRows, oldPage := logExportMaxRows, logExportPageSize
	logExportMaxRows, logExportPageSize = maxRows, pageSize
	t.Cleanup(func() { logExportMaxRows, logExportPageSize = oldRows, oldPage })
}

// callUserLogExport runs ExportUserLogsList for user 7 with the given query string.
func callUserLogExport(query string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/log/self/export?"+query, nil)
	c.Set("id", 7)
	ExportUserLogsList(c)
	return w
}

func exportErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("expected a JSON error, got %q: %s", ct, w.Body.String())
	}
	var body struct {
		Success bool   `json:"success"`
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Success || body.Message == "" {
		t.Fatalf("expected a failure with a message, got %+v", body)
	}
	return body.Code
}

func exportRange(start time.Time, days float64) string {
	end := start.Add(time.Duration(days * float64(24*time.Hour)))
	return fmt.Sprintf("start_timestamp=%d&end_timestamp=%d", start.Unix(), end.Unix())
}

func TestValidateLogExportRange(t *testing.T) {
	now := time.Now().Unix()
	day := int64(24 * 3600)
	cases := []struct {
		name       string
		start, end int64
		wantCode   string
		wantOK     bool
	}{
		{"no range", 0, 0, common.APICodeLogExportRangeRequired, false},
		{"start only", now - day, 0, common.APICodeLogExportRangeRequired, false},
		{"end only", 0, now, common.APICodeLogExportRangeRequired, false},
		{"31 days", now - 31*day, now, "", true},
		{"31 days and a second", now - 31*day - 1, now, common.APICodeLogExportRangeTooLong, false},
		{"one year", now - 366*day, now, common.APICodeLogExportRangeTooLong, false},
		{"end before start", now, now - day, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLogExportRange(tc.start, tc.end)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if tc.wantCode != "" {
				apiErr, ok := err.(*common.APIError)
				if !ok || apiErr.Code != tc.wantCode {
					t.Fatalf("error = %v, want code %s", err, tc.wantCode)
				}
			}
		})
	}
}

// An export without a time range, or with one longer than the limit, is refused before any
// row is read (SEC-70).
func TestUserLogExportRequiresBoundedRange(t *testing.T) {
	now := time.Now()
	setupLogExportEndpointDB(t, 7, 3, now.Add(-time.Hour).Unix())

	if code := exportErrorCode(t, callUserLogExport("")); code != common.APICodeLogExportRangeRequired {
		t.Fatalf("no range: code = %q", code)
	}
	if code := exportErrorCode(t, callUserLogExport(exportRange(now.Add(-90*24*time.Hour), 91))); code != common.APICodeLogExportRangeTooLong {
		t.Fatalf("90 days: code = %q", code)
	}
}

// A range with more rows than the limit is refused with a code the console translates.
func TestUserLogExportRefusesTooManyRows(t *testing.T) {
	now := time.Now()
	setupLogExportEndpointDB(t, 7, 6, now.Add(-time.Hour).Unix())
	setLogExportLimits(t, 5, 2)

	w := callUserLogExport(exportRange(now.Add(-24*time.Hour), 1))
	if code := exportErrorCode(t, w); code != common.APICodeLogExportTooManyRows {
		t.Fatalf("code = %q", code)
	}
}

// Within the limits the CSV holds every matching row once, read over several pages.
func TestUserLogExportStreamsAllPages(t *testing.T) {
	now := time.Now()
	setupLogExportEndpointDB(t, 7, 7, now.Add(-time.Hour).Unix())
	setLogExportLimits(t, 7, 3)

	w := callUserLogExport(exportRange(now.Add(-24*time.Hour), 1) + "&order=-created_at")
	if ct := w.Header().Get("Content-Type"); ct != "text/csv" {
		t.Fatalf("Content-Type = %q: %s", ct, w.Body.String())
	}
	records, err := csv.NewReader(strings.NewReader(w.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse csv: %v", err)
	}
	if len(records) != 1+7 {
		t.Fatalf("csv has %d records, want header + 7", len(records))
	}
	seen := map[string]bool{}
	for _, r := range records[1:] {
		details := r[len(r)-1]
		if seen[details] {
			t.Fatalf("row %q exported twice", details)
		}
		seen[details] = true
	}
}

// A user runs one export at a time; the slot is free again once it finishes.
func TestLogExportOneAtATimePerUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	newCtx := func() (*gin.Context, *httptest.ResponseRecorder) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		return c, w
	}

	c1, _ := newCtx()
	release := acquireLogExport(c1, 41)
	if release == nil {
		t.Fatal("first export must start")
	}
	c2, w2 := newCtx()
	if acquireLogExport(c2, 41) != nil {
		t.Fatal("second export of the same user must be refused")
	}
	if code := exportErrorCode(t, w2); code != common.APICodeLogExportRunning {
		t.Fatalf("code = %q", code)
	}
	c3, _ := newCtx()
	other := acquireLogExport(c3, 42)
	if other == nil {
		t.Fatal("another user's export must start")
	}
	other()
	release()
	c4, _ := newCtx()
	again := acquireLogExport(c4, 41)
	if again == nil {
		t.Fatal("export must start again after the first one finished")
	}
	again()
}
