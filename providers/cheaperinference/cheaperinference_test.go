package cheaperinference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
)

// Entries trimmed from GET /public/models as it answered on 2026-11-19.
const catalogJSON = `{"models":[
 {"id":"claude-opus-5.5","aliases":["anthropic/claude-opus-5-5"],"model_type":"text","provider_name":"Anthropic","context_length":1000000,"max_output_tokens":128000,"input_per_million":"2.714232","output_per_million":"13.571161","cache_read_per_million":"0.135711","cache_write_per_million":"3.392790","supports_vision":true,"supports_reasoning":true,"features":["tool-use","structured-output","explicit-caching"],"is_visible":true},
 {"id":"claude-opus-5.5-batch","aliases":["anthropic/claude-opus-5-5-batch"],"model_type":"text","provider_name":"anthropic","input_per_million":"1.700000","output_per_million":"8.500000","is_visible":true},
 {"id":"claude-haiku-5.5","aliases":["anthropic/claude-haiku-5-5"],"model_type":"text","provider_name":"Anthropic","input_per_million":"0.065000","output_per_million":"0.325000","cache_read_per_million":"0.006500","cache_write_per_million":"0.081250","input_token_price_threshold":100000,"input_per_million_above_threshold":"0.130000","output_per_million_above_threshold":"0.487500","is_visible":true},
 {"id":"deepseek-v4.1-flash","aliases":[],"model_type":"text","provider_name":"DeepSeek","input_per_million":"0.007000","output_per_million":"0.340000","cache_read_per_million":"0.003300","is_visible":true},
 {"id":"glyph-cluster","aliases":[],"model_type":"text","provider_name":"Stealth","input_per_million":"0.000000","output_per_million":"0.000000","is_free":true,"is_visible":true},
 {"id":"all-minilm-l12-v2","aliases":[],"model_type":"embedding","input_per_million":"0.005720","output_per_million":"0.000000","is_visible":true},
 {"id":"gemini-3.1-flash-image","aliases":["google/gemini-3.1-flash-image"],"model_type":"image","input_per_million":"0.425000","output_per_million":"2.550000","media_unit_price":"0.039000","media_unit":"image","is_visible":true},
 {"id":"gemini-3.5-transcribe","aliases":["google/gemini-3.5-transcribe"],"model_type":"transcription","input_per_million":"1.300000","output_per_million":"7.800000","is_visible":true}
]}`

func sampleModels(t *testing.T) []publicModel {
	t.Helper()
	var list publicModelList
	if err := json.Unmarshal([]byte(catalogJSON), &list); err != nil {
		t.Fatal(err)
	}
	return list.Models
}

func TestDefaultModelsUseVendorAliases(t *testing.T) {
	var got []string
	for _, m := range defaultModels(sampleModels(t), 0) {
		got = append(got, listedName(m))
	}
	// Batch variants and non-text kinds are left out; a model with an alias is listed under it,
	// whose bare name is the lab's own id.
	want := []string{"anthropic/claude-opus-5-5", "anthropic/claude-haiku-5-5", "deepseek-v4.1-flash", "glyph-cluster", "all-minilm-l12-v2"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("default models = %v, want %v", got, want)
	}
}

func TestPriceSync(t *testing.T) {
	prices, unconvertible := buildPriceSync(defaultModels(sampleModels(t), 0))
	byName := map[string]*model.Price{}
	for _, p := range prices {
		byName[p.Model] = p
	}
	opus := byName["anthropic/claude-opus-5-5"]
	if opus == nil || opus.ChannelType != config.ChannelTypeCheaperInference {
		t.Fatalf("prices = %+v", prices)
	}
	// Ratio 1 = $2 per million tokens.
	if !near(opus.Input, 2.714232/2) || !near(opus.Output, 13.571161/2) {
		t.Errorf("opus ratios = %v / %v", opus.Input, opus.Output)
	}
	if opus.LongContext != nil {
		t.Errorf("opus has no long context price, got %+v", opus.LongContext)
	}
	haiku := byName["anthropic/claude-haiku-5-5"]
	if haiku == nil || haiku.LongContext == nil {
		t.Fatalf("haiku = %+v", haiku)
	}
	if tier := haiku.LongContext.Data(); tier.Threshold != 100000 || tier.InputRatio != 2 || tier.OutputRatio != 1.5 {
		t.Errorf("haiku long context = %+v", tier)
	}
	if len(unconvertible) != 1 || unconvertible[0].Model != "glyph-cluster" {
		t.Errorf("unconvertible = %+v", unconvertible)
	}
}

func near(a, b float64) bool {
	d := a - b
	return d < 1e-6 && d > -1e-6
}

func TestChannelPricesCoverIdsAndAliases(t *testing.T) {
	prices := buildChannelPrices(sampleModels(t))
	byName := map[string]*model.ChannelPrice{}
	for _, p := range prices {
		byName[p.Model] = p
	}
	for _, name := range []string{"claude-opus-5.5", "anthropic/claude-opus-5-5", "anthropic/claude-opus-5-5-batch"} {
		if byName[name] == nil {
			t.Errorf("no channel price for %s", name)
		}
	}
	if p := byName["claude-opus-5.5"]; p.Input != 2.714232 || p.CacheRead != 0.135711 || p.CacheWrite != 3.39279 {
		t.Errorf("opus channel price = %+v", p)
	}
	if p := byName["google/gemini-3.1-flash-image"]; p == nil || p.Image != 0.039 {
		t.Errorf("image price = %+v", p)
	}
	if p := byName["gemini-3.5-transcribe"]; p == nil || !p.Other {
		t.Errorf("transcription price = %+v", p)
	}
	if byName["glyph-cluster"] != nil {
		t.Errorf("a free model has no channel price")
	}
}

func channelWith(plugin model.PluginType) *model.Channel {
	channel := &model.Channel{Type: config.ChannelTypeCheaperInference, Key: "ci_live_test"}
	if plugin != nil {
		p := datatypes.NewJSONType(plugin)
		channel.Plugin = &p
	}
	return channel
}

func testContext(body string) *gin.Context {
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	ctx.Request.Header.Set("session_id", "conv-1")
	ctx.Set("token_id", 7)
	ctx.Set("original_model", "anthropic/claude-opus-5-5")
	return ctx
}

func TestOpenAIRequestsAskForCacheBreakpoints(t *testing.T) {
	p := CheaperInferenceProviderFactory{}.Create(channelWith(nil)).(*CheaperInferenceProvider)
	p.SetContext(testContext(`{}`))
	headers := p.GetRequestHeaders()
	if headers[headerPromptCache] != "on" {
		t.Errorf("x-ci-prompt-cache = %q, want on", headers[headerPromptCache])
	}
	session := headers[headerPromptCacheSession]
	if session == "" || strings.Contains(session, "conv-1") {
		t.Errorf("cache session = %q, want a hash of the session", session)
	}
	if headers["Authorization"] != "Bearer ci_live_test" {
		t.Errorf("authorization = %q", headers["Authorization"])
	}

	// Messages requests carry their own breakpoints: only the session goes up.
	claudeHeaders := p.claudeProvider().GetRequestHeaders()
	if _, ok := claudeHeaders[headerPromptCache]; ok {
		t.Errorf("messages request sent x-ci-prompt-cache = %q", claudeHeaders[headerPromptCache])
	}
	if claudeHeaders[headerPromptCacheSession] != session {
		t.Errorf("messages cache session = %q, want %q", claudeHeaders[headerPromptCacheSession], session)
	}
}

func TestPromptCacheModeFromPlugin(t *testing.T) {
	for mode, want := range map[string]string{"off": "off", "passthrough": "passthrough", "on": "on", "bogus": "on"} {
		p := CheaperInferenceProviderFactory{}.Create(channelWith(model.PluginType{"cheaperinference": {"prompt_cache": mode}})).(*CheaperInferenceProvider)
		p.SetContext(testContext(`{}`))
		if got := p.GetRequestHeaders()[headerPromptCache]; got != want {
			t.Errorf("plugin %q: header = %q, want %q", mode, got, want)
		}
	}
	// The channel's own headers win.
	channel := channelWith(nil)
	custom := `{"x-ci-prompt-cache":"off"}`
	channel.ModelHeaders = &custom
	p := CheaperInferenceProviderFactory{}.Create(channel).(*CheaperInferenceProvider)
	p.SetContext(testContext(`{}`))
	if got := p.GetRequestHeaders()[headerPromptCache]; got != "off" {
		t.Errorf("custom header lost: %q", got)
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
			_, _ = w.Write([]byte(`{"object":"account.balance","currency":"USD","balance_usd":30.5,"available_usd":28.25,"reserved_usd":2.25}`))
		} else {
			_, _ = w.Write([]byte(`{"error":{"message":"missing scope","type":"permission_denied_error","code":"permission_denied"}}`))
		}
	}))
	defer upstream.Close()

	channel := channelWith(nil)
	channel.Id = 1
	baseURL := upstream.URL
	channel.BaseURL = &baseURL
	manage := "ci_live_manage"
	channel.ManageKey = &manage
	if err := testDB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	p := CheaperInferenceProviderFactory{}.Create(channel).(*CheaperInferenceProvider)
	balance, err := p.Balance()
	if err != nil || balance != 28.25 {
		t.Fatalf("balance = %v, %v; want the available 28.25", balance, err)
	}
	if gotPath != "/v1/account/balance" || gotAuth != "Bearer ci_live_manage" {
		t.Errorf("request = %s with %q", gotPath, gotAuth)
	}

	status = http.StatusForbidden
	if _, err := p.Balance(); err == nil || !strings.Contains(err.Error(), "account:read") {
		t.Errorf("forbidden error = %v", err)
	}
}
