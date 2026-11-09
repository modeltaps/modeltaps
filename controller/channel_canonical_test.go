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

func setupChannelCanonicalDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	if err := testDB.AutoMigrate(&model.Channel{}, &model.ModelInfo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
	for _, info := range []*model.ModelInfo{
		{Model: "claude-opus-5"},
		{Model: "anthropic/claude-opus-5", AliasOf: "claude-opus-5"},
	} {
		if err := testDB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
}

type canonicalBody struct {
	Success       bool              `json:"success"`
	Message       string            `json:"message"`
	Canonicalized map[string]string `json:"canonicalized"`
}

func callCanonical(t *testing.T, handler gin.HandlerFunc, method string, body any) canonicalBody {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	raw, _ := json.Marshal(body)
	c.Request = httptest.NewRequest(method, "/api/channel/", bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	handler(c)
	var got canonicalBody
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", w.Body.String(), err)
	}
	if !got.Success {
		t.Fatalf("request failed: %+v", got)
	}
	return got
}

func storedChannel(t *testing.T, id int) *model.Channel {
	t.Helper()
	channel, err := model.GetChannelById(id)
	if err != nil {
		t.Fatalf("load channel %d: %v", id, err)
	}
	return channel
}

// A channel that lists a catalog alias serves neither name (relay rewrites the alias to the
// canonical name before choosing a channel), so saving rewrites it to the canonical name and maps
// it back to the upstream name.
func TestAddChannelCanonicalizesCatalogAliases(t *testing.T) {
	setupChannelCanonicalDB(t)
	got := callCanonical(t, AddChannel, http.MethodPost, map[string]any{
		"type": config.ChannelTypeOpenAI, "key": "sk-a", "name": "a", "group": "default",
		"models": "anthropic/claude-opus-5,anthropic/claude-made-up-9",
	})
	if got.Canonicalized["anthropic/claude-opus-5"] != "claude-opus-5" || len(got.Canonicalized) != 1 {
		t.Fatalf("canonicalized = %v", got.Canonicalized)
	}
	var channel model.Channel
	if err := model.DB.First(&channel).Error; err != nil {
		t.Fatal(err)
	}
	if channel.Models != "claude-opus-5,anthropic/claude-made-up-9" {
		t.Fatalf("models = %q", channel.Models)
	}
	if channel.GetModelMapping() != `{"claude-opus-5":"anthropic/claude-opus-5"}` {
		t.Fatalf("mapping = %q", channel.GetModelMapping())
	}
}

func TestUpdateChannelCanonicalizesCatalogAliases(t *testing.T) {
	setupChannelCanonicalDB(t)
	stored := `{"x":"y"}`
	if err := model.DB.Create(&model.Channel{
		Id: 7, Type: config.ChannelTypeOpenAI, Key: "sk-a", Name: "a", Group: "default",
		Models: "x", ModelMapping: &stored,
	}).Error; err != nil {
		t.Fatal(err)
	}

	t.Run("request without model_mapping keeps the stored mapping", func(t *testing.T) {
		got := callCanonical(t, UpdateChannel, http.MethodPut, map[string]any{
			"id": 7, "type": config.ChannelTypeOpenAI, "name": "a", "group": "default",
			"models": "x,anthropic/claude-opus-5",
		})
		if got.Canonicalized["anthropic/claude-opus-5"] != "claude-opus-5" {
			t.Fatalf("canonicalized = %v", got.Canonicalized)
		}
		channel := storedChannel(t, 7)
		if channel.Models != "x,claude-opus-5" {
			t.Fatalf("models = %q", channel.Models)
		}
		if channel.GetModelMapping() != `{"claude-opus-5":"anthropic/claude-opus-5","x":"y"}` {
			t.Fatalf("mapping = %q", channel.GetModelMapping())
		}
	})

	t.Run("canonical names are saved as sent", func(t *testing.T) {
		mapping := `{"claude-opus-5":"anthropic/claude-opus-5"}`
		got := callCanonical(t, UpdateChannel, http.MethodPut, map[string]any{
			"id": 7, "type": config.ChannelTypeOpenAI, "name": "a", "group": "default",
			"models": "claude-opus-5", "model_mapping": mapping,
		})
		if len(got.Canonicalized) != 0 {
			t.Fatalf("canonicalized = %v", got.Canonicalized)
		}
		channel := storedChannel(t, 7)
		if channel.Models != "claude-opus-5" || channel.GetModelMapping() != mapping {
			t.Fatalf("models = %q mapping = %q", channel.Models, channel.GetModelMapping())
		}
	})
}
