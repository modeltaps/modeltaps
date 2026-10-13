package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupChannelAPICodeDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatalf("migrate channels: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
}

type apiCodeBody struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Code    string         `json:"code"`
	Params  map[string]any `json:"params"`
}

// callHandler runs handler with the given path params, query and JSON body and decodes the reply.
func callHandler(t *testing.T, handler gin.HandlerFunc, method string, params gin.Params, query string, body any) apiCodeBody {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	c.Request = httptest.NewRequest(method, "/api/channel?"+query, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	handler(c)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	var got apiCodeBody
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode body %q: %v", w.Body.String(), err)
	}
	return got
}

func expectCode(t *testing.T, got apiCodeBody, code string, message string) {
	t.Helper()
	if got.Success {
		t.Fatalf("success = true, want false: %+v", got)
	}
	if got.Code != code {
		t.Fatalf("code = %q, want %q: %+v", got.Code, code, got)
	}
	if got.Message != message {
		t.Fatalf("message = %q, want the English %q", got.Message, message)
	}
}

func TestTestChannelCodes(t *testing.T) {
	setupChannelAPICodeDB(t)
	setModelInfoModeSwitch(t, false)
	if err := model.DB.Create(&model.Channel{Id: 1, Type: config.ChannelTypeOpenAI, Key: "sk-test", Name: "empty"}).Error; err != nil {
		t.Fatalf("insert channel: %v", err)
	}
	if err := model.DB.Create(&model.Channel{Id: 2, Type: config.ChannelTypeOpenAI, Key: "sk-test", Name: "tts", Models: "tts-1"}).Error; err != nil {
		t.Fatalf("insert channel: %v", err)
	}

	t.Run("invalid id", func(t *testing.T) {
		got := callHandler(t, TestChannel, http.MethodGet, gin.Params{{Key: "id", Value: "abc"}}, "", nil)
		expectCode(t, got, "invalid_channel_id", "invalid channel id")
	})
	t.Run("unknown channel", func(t *testing.T) {
		got := callHandler(t, TestChannel, http.MethodGet, gin.Params{{Key: "id", Value: "99"}}, "", nil)
		expectCode(t, got, "channel_not_found", "channel not found")
	})
	t.Run("no test model", func(t *testing.T) {
		got := callHandler(t, TestChannel, http.MethodGet, gin.Params{{Key: "id", Value: "1"}}, "", nil)
		expectCode(t, got, "channel_no_test_model", "channel has no testable model configured")
	})
	t.Run("failure keeps the raw reason as a param", func(t *testing.T) {
		got := callHandler(t, TestChannel, http.MethodGet, gin.Params{{Key: "id", Value: "2"}}, "model=tts-1", nil)
		expectCode(t, got, "channel_test_failed", "speed test failed, reason: unsupported model type")
		if got.Params["reason"] != "unsupported model type" {
			t.Fatalf("params.reason = %v, want the untranslated reason", got.Params["reason"])
		}
	})
}

func TestChannelValidationCodes(t *testing.T) {
	setupChannelAPICodeDB(t)
	if err := model.DB.Create(&model.Channel{Id: 1, Type: config.ChannelTypeOpenAI, Key: "sk-test", Name: "a", Tag: "grp"}).Error; err != nil {
		t.Fatalf("insert channel: %v", err)
	}

	t.Run("tag type mismatch", func(t *testing.T) {
		got := callHandler(t, AddChannel, http.MethodPost, nil, "", map[string]any{
			"type": config.ChannelTypeAnthropic, "key": "sk-x", "name": "b", "tag": "grp",
		})
		if got.Code != "channel_tag_type_mismatch" || got.Params["tag"] != "grp" || got.Success {
			t.Fatalf("unexpected body: %+v", got)
		}
	})
	t.Run("batch ids required", func(t *testing.T) {
		got := callHandler(t, BatchDeleteChannel, http.MethodDelete, nil, "", map[string]any{"ids": []int{}, "value": "x"})
		expectCode(t, got, "channel_ids_required", "ids must not be empty")
	})
	t.Run("model list key required", func(t *testing.T) {
		got := callHandler(t, GetModelList, http.MethodPost, nil, "", map[string]any{"type": config.ChannelTypeOpenAI})
		expectCode(t, got, "channel_key_required", "key is required")
	})
	// These used to answer with the relay error shape, which the console could not read.
	t.Run("tag required", func(t *testing.T) {
		got := callHandler(t, GetChannelsTag, http.MethodGet, gin.Params{{Key: "tag", Value: ""}}, "", nil)
		expectCode(t, got, "channel_tag_required", "tag is required")
	})
	t.Run("tag invalid status", func(t *testing.T) {
		got := callHandler(t, ChangeChannelsTagStatus, http.MethodPut, gin.Params{{Key: "tag", Value: "grp"}, {Key: "status", Value: "pause"}}, "", nil)
		expectCode(t, got, "channel_tag_invalid_status", "invalid status")
	})
	t.Run("tag invalid setting type", func(t *testing.T) {
		got := callHandler(t, UpdateChannelsTagPriority, http.MethodPut, gin.Params{{Key: "tag", Value: "grp"}}, "", map[string]any{"type": "speed", "value": 1})
		expectCode(t, got, "channel_tag_invalid_setting_type", "invalid type")
	})
}
