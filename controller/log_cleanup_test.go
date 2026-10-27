package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func callDeleteHistoryLogs(t *testing.T, query string) map[string]any {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/log/?"+query, nil)
	DeleteHistoryLogs(c)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return body
}

// The manual cleanup answers at once that it started in the background; a second click while it
// runs is told another cleanup is running, each with a code the console translates.
func TestDeleteHistoryLogs_StartsInBackgroundAndRejectsOverlap(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Log{}, &model.LogDetail{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Hold the background cleanup inside its first query until the test lets it go.
	release := make(chan struct{})
	if err := testDB.Callback().Query().Before("gorm:query").Register("test:block", func(*gorm.DB) { <-release }); err != nil {
		t.Fatalf("register callback: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })

	body := callDeleteHistoryLogs(t, "target_timestamp=1000")
	if body["success"] != true || body["code"] != common.APICodeLogCleanupStarted {
		t.Fatalf("first call = %v, want success with %s", body, common.APICodeLogCleanupStarted)
	}
	body = callDeleteHistoryLogs(t, "target_timestamp=1000")
	if body["success"] != false || body["code"] != common.APICodeLogCleanupRunning {
		t.Fatalf("second call = %v, want failure with %s", body, common.APICodeLogCleanupRunning)
	}

	close(release)
	deadline := time.Now().Add(2 * time.Second)
	for model.IsLogCleanupRunning() {
		if time.Now().After(deadline) {
			t.Fatal("background cleanup did not finish")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestDeleteHistoryLogs_RequiresTimestamp(t *testing.T) {
	body := callDeleteHistoryLogs(t, "")
	if body["success"] != false {
		t.Fatalf("call without target_timestamp = %v, want failure", body)
	}
	if model.IsLogCleanupRunning() {
		t.Fatal("no cleanup may start without a timestamp")
	}
}
