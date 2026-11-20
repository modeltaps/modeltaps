package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// A change applied from a settings assistant plan goes through the settings page's own save and is
// written to the admin log with the value it replaced; a change from the settings page is not.
func TestUpdateOptionLogsSettingsAssistantChanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Option{}, &model.Log{}, &model.User{}); err != nil {
		t.Fatal(err)
	}
	prevDB, prevRetry := model.DB, config.RetryTimes
	model.DB = db
	t.Cleanup(func() { model.DB, config.RetryTimes = prevDB, prevRetry })
	model.RegisterOptions()
	config.RetryTimes = 2

	put := func(value string, assistant bool) {
		t.Helper()
		r := gin.New()
		r.PUT("/api/option/", func(c *gin.Context) { c.Set("id", 1); c.Next() }, UpdateOption)
		req := httptest.NewRequest(http.MethodPut, "/api/option/", bytes.NewBufferString(`{"key":"RetryTimes","value":"`+value+`"}`))
		if assistant {
			req.Header.Set(SettingsAssistantHeader, "1")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if !strings.Contains(w.Body.String(), `"success":true`) {
			t.Fatalf("update failed: %s", w.Body.String())
		}
	}
	put("3", false)
	put("4", true)

	var logs []model.Log
	db.Where("type = ?", model.LogTypeManage).Find(&logs)
	if len(logs) != 1 || logs[0].UserId != 1 || logs[0].Content != `Settings assistant: changed RetryTimes from "3" to "4"` {
		t.Fatalf("admin log = %+v", logs)
	}
	if config.RetryTimes != 4 {
		t.Fatalf("RetryTimes = %d, want 4", config.RetryTimes)
	}
}
