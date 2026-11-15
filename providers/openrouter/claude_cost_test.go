package openrouter

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

// claudeCostUpstream answers a Claude format request with the given message id and usage, and
// counts the generation lookups that follow.
func claudeCostUpstream(t *testing.T, messageID, usage string, lookups *atomic.Int32) *OpenRouterProvider {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/generation" {
			lookups.Add(1)
			_, _ = w.Write([]byte(`{"data":{"id":"` + r.URL.Query().Get("id") + `","provider_name":"Anthropic","total_cost":0.0123}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"` + messageID + `","type":"message","role":"assistant","model":"anthropic/claude-sonnet-4-5",` +
			`"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":` + usage + `}`))
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL + "/api"
	channel := &model.Channel{Type: config.ChannelTypeOpenRouter, Key: "sk-or-test", BaseURL: &baseURL}
	p := OpenRouterProviderFactory{}.Create(channel).(*OpenRouterProvider)

	body := []byte(claudeCodeStyleBody)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(config.GinRequestBodyKey, body)
	c.Set(logger.RequestIdKey, "req-claude-cost")
	p.SetContext(c)
	p.SetUsage(&types.Usage{})
	return p
}

func claudeCostRequest(t *testing.T) *claude.ClaudeRequest {
	t.Helper()
	return &claude.ClaudeRequest{Model: "claude-sonnet-4-5", MaxTokens: 1024}
}

// A cost in the usage of a Claude format response reaches the usage the request is billed and
// logged from.
func TestOpenRouterClaudeUsageCost(t *testing.T) {
	var lookups atomic.Int32
	p := claudeCostUpstream(t, "msg_1", `{"input_tokens":10,"output_tokens":2,"cost":0.0042}`, &lookups)
	if _, err := p.CreateClaudeChat(claudeCostRequest(t)); err != nil {
		t.Fatalf("CreateClaudeChat: %v", err.Message)
	}
	if got := p.GetUsage().UpstreamCost; got != 0.0042 {
		t.Errorf("upstream cost = %v, want 0.0042", got)
	}
	// "msg_1" is not a generation id, so nothing is looked up.
	time.Sleep(50 * time.Millisecond)
	if lookups.Load() != 0 {
		t.Errorf("generation lookups = %d, want none for a msg_ id", lookups.Load())
	}
}

// Without a cost in the response, the generation is looked up by the message id and its
// total_cost becomes the log's upstream cost.
func TestOpenRouterClaudeCostFromGeneration(t *testing.T) {
	setupBalanceTestDB(t)
	if err := model.DB.AutoMigrate(&model.Log{}); err != nil {
		t.Fatal(err)
	}
	oldSQLite := common.UsingSQLite
	common.UsingSQLite = true
	t.Cleanup(func() { common.UsingSQLite = oldSQLite })
	oldFetch, oldUpdate := genFetchDelays, genUpdateDelays
	genFetchDelays, genUpdateDelays = []time.Duration{0}, []time.Duration{0, 10 * time.Millisecond}
	t.Cleanup(func() { genFetchDelays, genUpdateDelays = oldFetch, oldUpdate })

	log := &model.Log{Type: model.LogTypeConsume, ChannelId: 1, ExpectedCost: 0.01,
		Metadata: datatypes.NewJSONType(map[string]any{"request_id": "req-claude-cost"})}
	if err := model.DB.Create(log).Error; err != nil {
		t.Fatal(err)
	}

	var lookups atomic.Int32
	p := claudeCostUpstream(t, "gen-abc123", `{"input_tokens":10,"output_tokens":2}`, &lookups)
	if _, err := p.CreateClaudeChat(claudeCostRequest(t)); err != nil {
		t.Fatalf("CreateClaudeChat: %v", err.Message)
	}
	if got := p.GetUsage().UpstreamCost; got != 0 {
		t.Errorf("the response carried no cost, got %v", got)
	}

	var stored model.Log
	for range 100 {
		model.DB.First(&stored, log.Id)
		if stored.UpstreamCost > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if stored.UpstreamCost != 0.0123 || stored.ExpectedCost != 0.01 {
		t.Fatalf("log costs = %v / %v, want 0.0123 / 0.01", stored.UpstreamCost, stored.ExpectedCost)
	}
	if stored.Metadata.Data()["or_provider"] != "Anthropic" {
		t.Errorf("metadata = %v", stored.Metadata.Data())
	}
	if lookups.Load() != 1 {
		t.Errorf("generation lookups = %d, want 1", lookups.Load())
	}
}
