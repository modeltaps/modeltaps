package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestResolveTestChannelType(t *testing.T) {
	setModelInfoModeSwitch(t, false)
	cases := []struct{ endpoint, model, want string }{
		{"", "gpt-4o", "chat"},
		{"", "o3-pro", "response"},
		{"auto", "text-embedding-3-small", "embeddings"},
		{"response", "gpt-4o", "response"},
		{" chat ", "o3-pro", "chat"},
		{"embeddings", "my-embed", "embeddings"},
		{"image", "my-image", "image"},
		{"bogus", "gpt-4o", "chat"},
	}
	for _, tc := range cases {
		if got := resolveTestChannelType(tc.endpoint, tc.model); got != tc.want {
			t.Errorf("resolveTestChannelType(%q, %q) = %q, want %q", tc.endpoint, tc.model, got, tc.want)
		}
	}
}

// A manual test that fails with an error the disable rules match must report the
// failure but leave the channel enabled.
func TestManualChannelTestDoesNotDisableChannel(t *testing.T) {
	setModelInfoModeSwitch(t, false)
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()

	oldAuto := config.AutomaticDisableChannelEnabled
	config.AutomaticDisableChannelEnabled = true
	t.Cleanup(func() { config.AutomaticDisableChannelEnabled = oldAuto })

	testDB := setupChannelTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"Incorrect API key provided","type":"invalid_request_error","code":"invalid_api_key"}}`)
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL
	channel := &model.Channel{
		Type:      config.ChannelTypeOpenAI,
		Key:       "sk-bad",
		Name:      "bad-key",
		Status:    config.ChannelStatusEnabled,
		BaseURL:   &baseURL,
		Models:    "gpt-4o-mini",
		TestModel: "gpt-4o-mini",
	}
	if err := testDB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	// Make sure the error is one the disable rules act on, so the test can fail.
	openaiErr, _ := testChannel(channel, "gpt-4o-mini", "")
	if !ShouldDisableChannel(channel.Type, openaiErr) {
		t.Fatalf("expected the upstream error to match the disable rules, got %+v", openaiErr)
	}

	w := callTestChannel(channel.Id, "")

	if !strings.Contains(w.Body.String(), `"success":false`) {
		t.Fatalf("want a failed test result, got %s", w.Body.String())
	}
	var got model.Channel
	if err := testDB.First(&got, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	if got.Status != config.ChannelStatusEnabled {
		t.Fatalf("channel status = %d after a manual test, want enabled", got.Status)
	}
}

// Only a test of the channel's own test model records the channel response time;
// testing other models from the multi-model dialog leaves it alone.
func TestManualChannelTestRecordsResponseTimeForTestModelOnly(t *testing.T) {
	setModelInfoModeSwitch(t, false)
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()
	testDB := setupChannelTestDB(t)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"c1","object":"chat.completion","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL
	channel := &model.Channel{
		Type:      config.ChannelTypeOpenAI,
		Key:       "sk-ok",
		Name:      "ok",
		Status:    config.ChannelStatusEnabled,
		BaseURL:   &baseURL,
		Models:    "gpt-4o-mini,gpt-4o",
		TestModel: "gpt-4o-mini",
	}
	if err := testDB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}

	readResponseTime := func() int {
		var got model.Channel
		if err := testDB.First(&got, channel.Id).Error; err != nil {
			t.Fatal(err)
		}
		return got.ResponseTime
	}

	if w := callTestChannel(channel.Id, "gpt-4o"); !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("test of gpt-4o failed: %s", w.Body.String())
	}
	time.Sleep(50 * time.Millisecond) // UpdateResponseTime runs in a goroutine
	if rt := readResponseTime(); rt != 0 {
		t.Fatalf("response time = %d after testing a non-test model, want 0", rt)
	}

	if w := callTestChannel(channel.Id, "gpt-4o-mini"); !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("test of gpt-4o-mini failed: %s", w.Body.String())
	}
	deadline := time.Now().Add(2 * time.Second)
	for readResponseTime() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("response time not recorded for the channel test model")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func setupChannelTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatal(err)
	}
	// Each connection to :memory: is a separate database; UpdateResponseTime runs on
	// another goroutine, so keep a single connection.
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatal(err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
	return testDB
}

func callTestChannel(id int, testModel string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	target := "/api/channel/test/" + strconv.Itoa(id)
	if testModel != "" {
		target += "?model=" + testModel
	}
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
	TestChannel(c)
	return w
}
