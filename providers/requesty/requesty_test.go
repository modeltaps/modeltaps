package requesty

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const pinnedJSON = `{"data":[
 {"id":"anthropic/claude-opus-5-5","input_price":0.000005,"output_price":0.000025,"caching_price":0.00000625,"cached_price":0.0000005,"context_window":1000000,"max_output_tokens":128000,"supports_vision":true,"supports_tool_calling":true,"supports_reasoning":true,"model_lab":"anthropic","model_canonical_name":"claude-opus-5-5"},
 {"id":"vertex/claude-opus-5-5","input_price":0.000005,"output_price":0.000025,"model_lab":"anthropic","model_canonical_name":"claude-opus-5-5"},
 {"id":"vertex/claude-opus-5-5@eu","input_price":0.0000055,"output_price":0.0000275,"model_lab":"anthropic","model_canonical_name":"claude-opus-5-5"},
 {"id":"deepinfra/qwen3-32b","input_price":0.0000001,"output_price":0.0000003,"model_lab":"alibaba","model_canonical_name":"qwen3-32b"},
 {"id":"together/qwen3-32b","input_price":0.0000002,"output_price":0.0000006,"model_lab":"alibaba","model_canonical_name":"qwen3-32b"},
 {"id":"google/gemini-x:flex","input_price":0.0000005,"output_price":0.000002,"model_lab":"google","model_canonical_name":"gemini-x"},
 {"id":"deepinfra/glm-5.2","input_price":0.000001,"output_price":0.000003,"model_lab":"deepinfra","model_canonical_name":"glm-5.2"},
 {"id":"sail/gemma-4-31b-it","input_price":0.0000001,"output_price":0.0000002,"model_lab":"google","model_canonical_name":"gemma-4-31b"},
 {"id":"runware/some-image","input_price":0,"output_price":0,"supports_image_generation":true,"model_lab":"runware","model_canonical_name":"some-image"}
]}`

const managedJSON = `{"data":[
 {"id":"claude-opus-5-5","input_price":0.000005,"output_price":0.000025,"cached_price":0.0000005,"model_lab":"anthropic","model_canonical_name":"claude-opus-5-5"},
 {"id":"glm-5.2","input_price":0.000001,"output_price":0.000003,"model_lab":"zai","model_canonical_name":"glm-5.2"},
 {"id":"gemma-4-31b-it","input_price":0.0000001,"output_price":0.0000002,"model_lab":"google","model_canonical_name":"gemma-4-31b-it"},
 {"id":"claude-opus-5-5@eu","input_price":0.0000055,"output_price":0.0000275,"model_lab":"anthropic","model_canonical_name":"claude-opus-5-5"}
]}`

func sampleCatalog(t *testing.T) *catalog {
	t.Helper()
	var pinned, managed modelListResponse
	if err := json.Unmarshal([]byte(pinnedJSON), &pinned); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(managedJSON), &managed); err != nil {
		t.Fatal(err)
	}
	return &catalog{managed: managed.Data, pinned: pinned.Data}
}

func ids(models []routerModel) []string {
	out := make([]string, 0, len(models))
	for _, m := range models {
		out = append(out, m.Id)
	}
	return out
}

func TestDefaultModelsListsEachModelOnce(t *testing.T) {
	got := ids(defaultModels(sampleCatalog(t)))
	// The managed name wins over every pinned id of Claude; Qwen has no managed name, so one pinned
	// id is taken; region and tier variants are left out.
	// Qwen is not hosted by its lab here, so the smallest pinned id is taken.
	// Requesty files glm-5.2 under another lab on DeepInfra, and gemma under another canonical name
	// on Sail: both are the managed model on one host.
	want := []string{"claude-opus-5-5", "glm-5.2", "gemma-4-31b-it", "deepinfra/qwen3-32b", "runware/some-image"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("default models = %v, want %v", got, want)
	}
	if all := allModels(sampleCatalog(t)); len(all) != 13 {
		t.Errorf("all models = %d, want 13 (every id once)", len(all))
	}
}

func TestPrices(t *testing.T) {
	prices, unconvertible := buildPriceSync(defaultModels(sampleCatalog(t)))
	if len(prices) != 4 || prices[0].Model != "claude-opus-5-5" || prices[0].ChannelType != config.ChannelTypeRequesty {
		t.Fatalf("prices = %+v", prices)
	}
	// $5/M input = ratio 2.5, $25/M output = 12.5 (ratio 1 = $2/M); cache read 0.1 of input.
	if prices[0].Input != 2.5 || prices[0].Output != 12.5 {
		t.Errorf("claude ratios = %v/%v", prices[0].Input, prices[0].Output)
	}
	if prices[0].ExtraRatios == nil || prices[0].ExtraRatios.Data()[config.UsageExtraCachedRead] != 0.1 {
		t.Errorf("cache read ratio = %v", prices[0].ExtraRatios)
	}
	if len(unconvertible) != 1 || unconvertible[0].Model != "runware/some-image" {
		t.Errorf("unconvertible = %+v", unconvertible)
	}
	channelPrices := buildChannelPrices(allModels(sampleCatalog(t)))
	byModel := map[string]*model.ChannelPrice{}
	for _, p := range channelPrices {
		byModel[p.Model] = p
	}
	opus := byModel["anthropic/claude-opus-5-5"]
	if opus == nil || opus.Input != 5 || opus.Output != 25 || opus.CacheWrite != 6.25 || opus.CacheRead != 0.5 {
		t.Errorf("opus channel price = %+v", opus)
	}
	if byModel["vertex/claude-opus-5-5@eu"] == nil {
		t.Error("a regional id should still have a channel price")
	}
	if img := byModel["runware/some-image"]; img == nil || !img.Other {
		t.Errorf("an image model priced another way should read Other, got %+v", img)
	}
}

func TestModelInfos(t *testing.T) {
	pinned := buildModelInfos(sampleCatalog(t).pinned[:1])[0]
	if pinned.ContextLength != 1000000 || pinned.MaxTokens != 128000 {
		t.Errorf("context/max = %d/%d", pinned.ContextLength, pinned.MaxTokens)
	}
	if pinned.InputModalities != `["text","image"]` || pinned.Capabilities != `["tool_call","reasoning"]` {
		t.Errorf("modalities %s capabilities %s", pinned.InputModalities, pinned.Capabilities)
	}
}

func TestKeyWithManageKey(t *testing.T) {
	channel := &model.Channel{Type: config.ChannelTypeRequesty, Key: "rq-inference|rq-manage"}
	p := RequestyProviderFactory{}.Create(channel).(*RequestyProvider)
	if p.Channel.Key != "rq-inference" || p.manageKey != "rq-manage" {
		t.Errorf("key = %q manage = %q", p.Channel.Key, p.manageKey)
	}
	if channel.Key != "rq-inference|rq-manage" {
		t.Errorf("the shared channel must not change, got %q", channel.Key)
	}
	if got := p.GetRequestHeaders()["Authorization"]; got != "Bearer rq-inference" {
		t.Errorf("auth header = %q", got)
	}
	if got := p.claudeProvider().GetRequestHeaders()["x-api-key"]; got != "rq-inference" {
		t.Errorf("claude x-api-key = %q", got)
	}
	single := RequestyProviderFactory{}.Create(&model.Channel{Key: "rq-one"}).(*RequestyProvider)
	if single.manageKey != "rq-one" {
		t.Errorf("a single key is also the manage key, got %q", single.manageKey)
	}
}

func TestURLs(t *testing.T) {
	cfg := getConfig()
	if got := cfg.BaseURL + cfg.ChatCompletions; got != "https://router.requesty.ai/v1/chat/completions" {
		t.Errorf("chat URL = %q", got)
	}
	c := getClaudeConfig(cfg.BaseURL)
	if got := c.BaseURL + c.ChatCompletions; got != "https://router.requesty.ai/v1/messages" {
		t.Errorf("messages URL = %q", got)
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
	status := http.StatusOK
	var gotPath, gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(`{"name":"acme","balance":42.25}`))
		} else {
			_, _ = w.Write([]byte(`{"error":{"message":"forbidden"}}`))
		}
	}))
	defer upstream.Close()
	oldURL := manageBaseURL
	manageBaseURL = upstream.URL
	t.Cleanup(func() { manageBaseURL = oldURL })

	channel := &model.Channel{Type: config.ChannelTypeRequesty, Key: "rq-inference|rq-manage"}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	p := RequestyProviderFactory{}.Create(channel).(*RequestyProvider)
	balance, err := p.Balance()
	if err != nil || balance != 42.25 {
		t.Fatalf("Balance() = %v, %v", balance, err)
	}
	if gotPath != "/v1/manage/org" || gotAuth != "Bearer rq-manage" {
		t.Errorf("request = %s %s", gotPath, gotAuth)
	}
	var stored model.Channel
	if err := model.DB.First(&stored, channel.Id).Error; err != nil || stored.Balance != 42.25 {
		t.Errorf("stored balance = %v (%v)", stored.Balance, err)
	}
	var key model.Channel
	model.DB.Select("key").First(&key, channel.Id)
	if key.Key != "rq-inference|rq-manage" {
		t.Errorf("saving the balance must not rewrite the key, got %q", key.Key)
	}

	status = http.StatusForbidden
	if _, err := p.Balance(); err == nil {
		t.Error("a key without manage permission should give an error")
	}
}
