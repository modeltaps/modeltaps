package vercel

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const sampleModels = `{"object":"list","data":[
 {"id":"anthropic/claude-sonnet-4","owned_by":"anthropic","name":"Claude Sonnet 4","description":"d","context_window":200000,"max_tokens":64000,"type":"language","tags":["tool-use","reasoning","vision"],
  "pricing":{"input":"0.000003","output":"0.000015","input_cache_read":"0.0000003","input_cache_write":"0.00000375"}},
 {"id":"openai/text-embedding-3-small","owned_by":"openai","name":"Embedding","type":"embedding","pricing":{"input":"0.00000002"}},
 {"id":"openai/gpt-image-1","owned_by":"openai","name":"GPT Image","type":"image","pricing":{"image":"0.04"}},
 {"id":"","name":"broken"}
]}`

func TestDefaultURLs(t *testing.T) {
	cfg := getConfig()
	if got := cfg.BaseURL + cfg.ChatCompletions; got != "https://ai-gateway.vercel.sh/v1/chat/completions" {
		t.Errorf("chat URL = %q", got)
	}
	if got := cfg.BaseURL + cfg.ModelList; got != "https://ai-gateway.vercel.sh/v1/models" {
		t.Errorf("models URL = %q", got)
	}
}

func TestBuildPriceSync(t *testing.T) {
	var resp modelListResponse
	if err := json.Unmarshal([]byte(sampleModels), &resp); err != nil {
		t.Fatal(err)
	}
	prices, unconvertible := buildPriceSync(&resp)
	if len(prices) != 2 {
		t.Fatalf("prices = %d, want 2", len(prices))
	}
	sonnet := prices[0]
	if sonnet.Model != "anthropic/claude-sonnet-4" || sonnet.ChannelType != config.ChannelTypeVercelAIGateway {
		t.Errorf("price[0] = %+v", sonnet)
	}
	// $3/M 输入 = 1.5 倍率，$15/M 输出 = 7.5 倍率（1 倍率 = $2/M）。
	if sonnet.Input != 1.5 || sonnet.Output != 7.5 {
		t.Errorf("sonnet ratios = %v/%v, want 1.5/7.5", sonnet.Input, sonnet.Output)
	}
	if sonnet.ExtraRatios == nil || sonnet.ExtraRatios.Data()[config.UsageExtraCachedRead] != 0.1 {
		t.Errorf("sonnet cache read ratio = %v, want 0.1", sonnet.ExtraRatios)
	}
	if prices[1].Model != "openai/text-embedding-3-small" || prices[1].Output != 0 {
		t.Errorf("price[1] = %+v", prices[1])
	}
	if len(unconvertible) != 1 || unconvertible[0].Model != "openai/gpt-image-1" || unconvertible[0].Reason != model.UnconvertiblePerRequest {
		t.Errorf("unconvertible = %+v", unconvertible)
	}
}

func TestBuildModelInfos(t *testing.T) {
	var resp modelListResponse
	if err := json.Unmarshal([]byte(sampleModels), &resp); err != nil {
		t.Fatal(err)
	}
	infos := buildModelInfos(&resp)
	if len(infos) != 3 {
		t.Fatalf("infos = %d, want 3", len(infos))
	}
	sonnet := infos[0]
	if sonnet.ContextLength != 200000 || sonnet.MaxTokens != 64000 {
		t.Errorf("sonnet context/max = %d/%d", sonnet.ContextLength, sonnet.MaxTokens)
	}
	if sonnet.InputModalities != `["text","image"]` || sonnet.OutputModalities != `["text"]` {
		t.Errorf("sonnet modalities = %s -> %s", sonnet.InputModalities, sonnet.OutputModalities)
	}
	if sonnet.Capabilities != `["tool_call","reasoning"]` {
		t.Errorf("sonnet capabilities = %s", sonnet.Capabilities)
	}
	if infos[1].Capabilities != "" {
		t.Errorf("embedding without tags should have unknown capabilities, got %q", infos[1].Capabilities)
	}
	if infos[2].OutputModalities != `["image"]` {
		t.Errorf("image model output = %s", infos[2].OutputModalities)
	}
}

func TestBalance(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.Channel{}); err != nil {
		t.Fatal(err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}

	var gotPath, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"balance":"95.50","total_used":"4.50"}`))
	}))
	defer upstream.Close()

	baseURL := upstream.URL
	channel := &model.Channel{Type: config.ChannelTypeVercelAIGateway, Key: "vck_test", BaseURL: &baseURL}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	p := VercelProviderFactory{}.Create(channel).(*VercelProvider)

	balance, err := p.Balance()
	if err != nil {
		t.Fatalf("Balance() error = %v", err)
	}
	if gotPath != "/v1/credits" || gotAuth != "Bearer vck_test" {
		t.Errorf("request = %s %s", gotPath, gotAuth)
	}
	if balance != 95.5 {
		t.Errorf("balance = %v, want 95.5", balance)
	}
	var stored model.Channel
	if err := model.DB.First(&stored, channel.Id).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Balance != 95.5 {
		t.Errorf("stored balance = %v", stored.Balance)
	}
}

// open-weights 标签 → 开放权重；released 缺失时退回 created。
func TestBuildModelInfosLifecycle(t *testing.T) {
	raw := `{"data":[
		{"id":"alibaba/qwen-3-14b","type":"language","tags":["reasoning","open-weights"],"released":1745798400,"created":1755815280},
		{"id":"anthropic/claude-opus-5.5","type":"language","tags":["reasoning"],"created":1790000000}
	]}`
	var resp modelListResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	infos := buildModelInfos(&resp)
	if len(infos) != 2 {
		t.Fatalf("infos = %d, want 2", len(infos))
	}
	if infos[0].OpenWeights == nil || !*infos[0].OpenWeights || infos[0].ReleasedAt != 1745798400 {
		t.Errorf("open-weights model = %+v", infos[0])
	}
	if infos[1].OpenWeights == nil || *infos[1].OpenWeights || infos[1].ReleasedAt != 1790000000 {
		t.Errorf("closed model = %+v", infos[1])
	}
}
