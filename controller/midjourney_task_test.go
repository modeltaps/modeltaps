package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	provider "github.com/modeltaps/modeltaps/providers/midjourney"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const mjTestUserQuota = 1000

func setupMjTaskTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "mj-task.db")
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.Midjourney{}, &model.User{}, &model.Log{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldBatch := model.DB, config.BatchUpdateEnabled
	model.DB, config.BatchUpdateEnabled = testDB, false
	t.Cleanup(func() { model.DB, config.BatchUpdateEnabled = oldDB, oldBatch })
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	if err := testDB.Create(&model.User{Id: 1, Username: "mj-user", Quota: mjTestUserQuota}).Error; err != nil {
		t.Fatal(err)
	}
}

// runMjPoll answers the poller's list-by-condition request with items and runs one poll round.
func runMjPoll(t *testing.T, task *model.Midjourney, item provider.MidjourneyDto) *model.Midjourney {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]provider.MidjourneyDto{item})
	}))
	defer upstream.Close()

	if err := model.DB.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	base := upstream.URL
	channel := &model.Channel{Id: 1, BaseURL: &base, Key: "secret"}
	if err := MjTaskHandler(channel, []string{task.MjId}, map[string]*model.Midjourney{task.MjId: task}); err != nil {
		t.Fatalf("poll: %v", err)
	}
	var stored model.Midjourney
	if err := model.DB.First(&stored, task.Id).Error; err != nil {
		t.Fatal(err)
	}
	return &stored
}

func mjUserQuota(t *testing.T) int {
	t.Helper()
	var user model.User
	if err := model.DB.First(&user, 1).Error; err != nil {
		t.Fatal(err)
	}
	return user.Quota
}

func overAnHourAgo() int64 {
	return time.Now().Add(-61 * time.Minute).UnixMilli()
}

// SEC-76: a task the upstream finished just past the one hour mark is a success, not a refund.
func TestMjTaskLateSuccessIsNotTimedOut(t *testing.T) {
	setupMjTaskTestDB(t)
	submit := overAnHourAgo()
	task := &model.Midjourney{UserId: 1, Code: 1, MjId: "late", Action: "IMAGINE", Progress: "50%",
		Status: "IN_PROGRESS", SubmitTime: submit, ChannelId: 1, Quota: 300}
	stored := runMjPoll(t, task, provider.MidjourneyDto{MjId: "late", Status: "SUCCESS", Progress: "100%",
		SubmitTime: submit, ImageUrl: "https://cdn.example.com/late.png"})

	if stored.Status != "SUCCESS" || stored.FailReason != "" {
		t.Fatalf("late success must stay a success, got status=%q reason=%q", stored.Status, stored.FailReason)
	}
	if stored.ImageUrl != "https://cdn.example.com/late.png" {
		t.Fatalf("image url = %q, want the upstream result", stored.ImageUrl)
	}
	if got := mjUserQuota(t); got != mjTestUserQuota {
		t.Fatalf("user quota = %d, want %d (no refund for a success)", got, mjTestUserQuota)
	}
}

// SEC-76: a timed out task is refunded what it was charged and keeps no fetchable image.
func TestMjTaskTimeoutRefundsChargeAndDropsImage(t *testing.T) {
	setupMjTaskTestDB(t)
	submit := overAnHourAgo()
	task := &model.Midjourney{UserId: 1, Code: 1, MjId: "slow", Action: "IMAGINE", Progress: "90%",
		Status: "IN_PROGRESS", SubmitTime: submit, ChannelId: 1, Quota: 300}
	stored := runMjPoll(t, task, provider.MidjourneyDto{MjId: "slow", Status: "IN_PROGRESS", Progress: "95%",
		SubmitTime: submit, ImageUrl: "https://cdn.example.com/preview.png"})

	if stored.Status != "FAILURE" || stored.Progress != "100%" {
		t.Fatalf("timed out task: status=%q progress=%q, want FAILURE at 100%%", stored.Status, stored.Progress)
	}
	if stored.ImageUrl != "" {
		t.Fatalf("timed out task keeps image url %q", stored.ImageUrl)
	}
	if got := mjUserQuota(t); got != mjTestUserQuota+300 {
		t.Fatalf("user quota = %d, want %d (refund of the 300 charged)", got, mjTestUserQuota+300)
	}
}

// SEC-63: an INPAINT task left in MODAL state was charged nothing, so its timeout refunds nothing.
func TestMjTaskTimeoutOfUnchargedModalRefundsNothing(t *testing.T) {
	setupMjTaskTestDB(t)
	submit := overAnHourAgo()
	task := &model.Midjourney{UserId: 1, Code: 21, MjId: "modal", Action: "INPAINT", Progress: "0%",
		Status: "MODAL", SubmitTime: submit, ChannelId: 1, Quota: 0}
	stored := runMjPoll(t, task, provider.MidjourneyDto{MjId: "modal", Status: "MODAL", Progress: "0%", SubmitTime: submit})

	if stored.Status != "FAILURE" {
		t.Fatalf("status = %q, want FAILURE", stored.Status)
	}
	if got := mjUserQuota(t); got != mjTestUserQuota {
		t.Fatalf("user quota = %d, want %d (nothing was charged)", got, mjTestUserQuota)
	}
}

func TestMjTaskTimedOut(t *testing.T) {
	now := time.Now().UnixMilli()
	old := &model.Midjourney{Progress: "10%", SubmitTime: now - mjTaskTimeoutMs - 1}
	fresh := &model.Midjourney{Progress: "10%", SubmitTime: now - 1000}
	cases := []struct {
		name string
		task *model.Midjourney
		item provider.MidjourneyDto
		want bool
	}{
		{"old and still running", old, provider.MidjourneyDto{Status: "IN_PROGRESS", Progress: "40%"}, true},
		{"old modal", old, provider.MidjourneyDto{Status: "MODAL", Progress: "0%"}, true},
		{"fresh", fresh, provider.MidjourneyDto{Status: "IN_PROGRESS", Progress: "40%"}, false},
		{"upstream success", old, provider.MidjourneyDto{Status: "SUCCESS", Progress: "100%"}, false},
		{"upstream success without progress", old, provider.MidjourneyDto{Status: "SUCCESS"}, false},
		{"upstream at 100%", old, provider.MidjourneyDto{Status: "IN_PROGRESS", Progress: "100%"}, false},
		{"upstream failure", old, provider.MidjourneyDto{Status: "FAILURE", Progress: "40%"}, false},
	}
	for _, tc := range cases {
		if got := mjTaskTimedOut(tc.task, tc.item, now); got != tc.want {
			t.Errorf("%s: timed out = %v, want %v", tc.name, got, tc.want)
		}
	}
}
