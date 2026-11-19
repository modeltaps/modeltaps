package venice

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

// A trimmed copy of `GET /models?type=all` (2026-11-19).
const modelsJSON = `{"data":[
 {"id":"claude-opus-5-5","type":"text","created":1790000000,"model_spec":{"pricing":{"input":{"usd":4,"diem":4},"cache_input":{"usd":0.2,"diem":0.2},"cache_write":{"usd":5,"diem":5},"output":{"usd":20,"diem":20}},"availableContextTokens":1000000,"maxCompletionTokens":128000,"capabilities":{"supportsFunctionCalling":true,"supportsReasoning":true,"supportsVision":true},"description":"Opus","offline":false}},
 {"id":"openai-gpt-55","type":"text","model_spec":{"pricing":{"input":{"usd":5},"cache_input":{"usd":0.5},"output":{"usd":30},"extended":{"context_token_threshold":272000,"input":{"usd":10},"output":{"usd":45},"cache_input":{"usd":1}}},"capabilities":{"supportsResponseSchema":true},"offline":false}},
 {"id":"e2ee-glm-5-3-p","type":"text","model_spec":{"pricing":{"input":{"usd":1},"output":{"usd":3}},"capabilities":{"supportsE2EE":true},"offline":false}},
 {"id":"old-model","type":"text","model_spec":{"pricing":{"input":{"usd":1},"output":{"usd":3}},"offline":true}},
 {"id":"text-embedding-bge-m3","type":"embedding","model_spec":{"pricing":{"input":{"usd":0.15},"output":{"usd":0.6}},"offline":false}},
 {"id":"venice-sd35","type":"image","model_spec":{"pricing":{"generation":{"usd":0.01},"upscale":{"2x":{"usd":0.02}}},"offline":false}},
 {"id":"tts-kokoro","type":"tts","model_spec":{"pricing":{"input":{"usd":3.5}},"offline":false}},
 {"id":"nvidia/parakeet-tdt-0.6b-v3","type":"asr","model_spec":{"pricing":{"per_audio_second":{"usd":0.0001}},"offline":false}},
 {"id":"some-video","type":"video","model_spec":{"pricing":{},"offline":false}}
]}`

func sample(t *testing.T) []veniceModel {
	t.Helper()
	var resp modelListResponse
	if err := json.Unmarshal([]byte(modelsJSON), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.Data
}

func TestDefaultModels(t *testing.T) {
	var got []string
	for _, m := range defaultModels(sample(t)) {
		got = append(got, m.Id)
	}
	// Text and embedding models that are online, without the end-to-end encrypted ones.
	if strings.Join(got, ",") != "claude-opus-5-5,openai-gpt-55,text-embedding-bge-m3" {
		t.Errorf("default models = %v", got)
	}
}

func TestPriceSync(t *testing.T) {
	prices, unconvertible := buildPriceSync(sample(t))
	byModel := map[string]*model.Price{}
	for _, p := range prices {
		byModel[p.Model] = p
		if p.ChannelType != config.ChannelTypeVenice {
			t.Errorf("%s channel type = %d", p.Model, p.ChannelType)
		}
	}
	if len(prices) != 4 || len(unconvertible) != 0 {
		t.Fatalf("prices = %d unconvertible = %v", len(prices), unconvertible)
	}
	opus := byModel["claude-opus-5-5"]
	want := model.PriceFromUSDPerToken("claude-opus-5-5", config.ChannelTypeVenice, 4e-6, 20e-6, 0.2e-6)
	if opus.Input != want.Input || opus.Output != want.Output || opus.LongContext != nil {
		t.Errorf("opus = %+v, want %+v", opus, want)
	}
	gpt := byModel["openai-gpt-55"]
	if gpt.LongContext == nil {
		t.Fatal("the extended price should become a long context tier")
	}
	if lc := gpt.LongContext.Data(); lc.Threshold != 272000 || lc.InputRatio != 2 || lc.OutputRatio != 1.5 {
		t.Errorf("long context = %+v", lc)
	}
	if asr := byModel["nvidia/parakeet-tdt-0.6b-v3"]; asr == nil || asr.Type != model.MinutesPriceType {
		t.Errorf("transcription should be priced by audio length, got %+v", asr)
	}
}

func TestChannelPrices(t *testing.T) {
	byModel := map[string]*model.ChannelPrice{}
	for _, p := range buildChannelPrices(sample(t)) {
		byModel[p.Model] = p
	}
	opus := byModel["claude-opus-5-5"]
	if opus == nil || opus.Input != 4 || opus.Output != 20 || opus.CacheRead != 0.2 || opus.CacheWrite != 5 {
		t.Errorf("opus = %+v", opus)
	}
	if img := byModel["venice-sd35"]; img == nil || img.Image != 0.01 || img.Kind != model.ModelKindImage {
		t.Errorf("image = %+v", img)
	}
	if tts := byModel["tts-kokoro"]; tts == nil || !tts.Other || tts.Kind != model.ModelKindSpeech {
		t.Errorf("speech = %+v", tts)
	}
	if asr := byModel["nvidia/parakeet-tdt-0.6b-v3"]; asr == nil || asr.AudioSecond != 0.0001 {
		t.Errorf("transcription = %+v", asr)
	}
	if _, ok := byModel["some-video"]; ok {
		t.Error("video models are priced per job and not relayed")
	}
}

func TestModelInfos(t *testing.T) {
	infos := buildModelInfos(defaultModels(sample(t)))
	opus := infos[0]
	if opus.ContextLength != 1000000 || opus.MaxTokens != 128000 || opus.Description != "Opus" {
		t.Errorf("opus = %+v", opus)
	}
	if opus.InputModalities != `["text","image"]` || opus.Capabilities != `["tool_call","reasoning"]` {
		t.Errorf("modalities %s capabilities %s", opus.InputModalities, opus.Capabilities)
	}
	if emb := infos[2]; emb.Endpoints != `["embeddings"]` {
		t.Errorf("embedding endpoints = %s", emb.Endpoints)
	}
}

func TestErrorParsing(t *testing.T) {
	if e := parseError(json.RawMessage(`"Authentication failed"`)); e == nil || e.Message != "Authentication failed" {
		t.Errorf("string error = %+v", e)
	}
	if e := parseError(json.RawMessage(`{"message":"bad model","type":"invalid_request_error"}`)); e == nil || e.Message != "bad model" {
		t.Errorf("object error = %+v", e)
	}
	if e := parseError(json.RawMessage(`""`)); e != nil {
		t.Errorf("empty error = %+v", e)
	}
}

func TestURLsAndKeys(t *testing.T) {
	p := VeniceProviderFactory{}.Create(&model.Channel{Key: "vn-key"}).(*VeniceProvider)
	if got := p.GetFullRequestURL(p.Config.ChatCompletions, ""); got != "https://api.venice.ai/api/v1/chat/completions" {
		t.Errorf("chat URL = %q", got)
	}
	if got := p.GetRequestHeaders()["Authorization"]; got != "Bearer vn-key" || p.manageKey != "vn-key" {
		t.Errorf("auth = %q manage = %q", got, p.manageKey)
	}
	admin := "vn-admin"
	q := VeniceProviderFactory{}.Create(&model.Channel{Key: "vn-key", ManageKey: &admin}).(*VeniceProvider)
	if q.manageKey != "vn-admin" {
		t.Errorf("manage key = %q", q.manageKey)
	}
}

func TestModelListIsReadWithoutKey(t *testing.T) {
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}
	var gotURL, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotURL, gotAuth = r.URL.String(), r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(modelsJSON))
	}))
	defer srv.Close()
	base := srv.URL + "/api/v1"
	p := VeniceProviderFactory{}.Create(&model.Channel{Key: "vn-key", BaseURL: &base}).(*VeniceProvider)
	list, err := p.GetModelList()
	if err != nil || len(list) != 3 {
		t.Fatalf("list = %v, %v", list, err)
	}
	if gotURL != "/api/v1/models?type=all" || gotAuth != "" {
		t.Errorf("url = %q auth = %q", gotURL, gotAuth)
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
	body := `{"canConsume":true,"consumptionCurrency":"DIEM","balances":{"diem":12.5,"usd":25,"bundledCredits":null,"earnedCredits":null},"diemEpochAllocation":100}`
	status := http.StatusOK
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	base := srv.URL + "/api/v1"
	channel := &model.Channel{Type: config.ChannelTypeVenice, Key: "vn-key", BaseURL: &base}
	if err := testDB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	p := VeniceProviderFactory{}.Create(channel).(*VeniceProvider)
	got, err := p.Balance()
	if err != nil || got != 37.5 {
		t.Fatalf("balance = %v, %v", got, err)
	}
	if gotPath != "/api/v1/billing/balance" || gotAuth != "Bearer vn-key" {
		t.Errorf("path = %q auth = %q", gotPath, gotAuth)
	}

	body = `{"canConsume":false,"balances":{"diem":null,"usd":-2,"bundledCredits":null,"earnedCredits":null}}`
	if got, err := p.Balance(); err != nil || got != 0 {
		t.Errorf("an account that cannot consume has 0, got %v, %v", got, err)
	}

	status, body = http.StatusUnauthorized, `{"error":"Authentication failed"}`
	if _, err := p.Balance(); err == nil || !strings.Contains(err.Error(), "Venice refused the key") {
		t.Errorf("err = %v", err)
	}
}
