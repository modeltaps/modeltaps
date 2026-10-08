package relay

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/providers/gemini"
	"github.com/modeltaps/modeltaps/providers/minimax"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/providers/openrouter"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupProtocolModelGroup 构造一个含 openai/gemini/anthropic 三类渠道的分组，返回还原函数。
func setupProtocolModelGroup(t *testing.T) func() {
	t.Helper()
	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels

	weight := uint(1)
	openaiChannel := &model.Channel{Id: 1, Type: config.ChannelTypeOpenAI, Weight: &weight, Status: config.ChannelStatusEnabled}
	geminiChannel := &model.Channel{Id: 2, Type: config.ChannelTypeGemini, Weight: &weight, Status: config.ChannelStatusEnabled}
	anthropicChannel := &model.Channel{Id: 3, Type: config.ChannelTypeAnthropic, Weight: &weight, Status: config.ChannelStatusEnabled}
	openrouterChannel := &model.Channel{Id: 4, Type: config.ChannelTypeOpenRouter, Weight: &weight, Status: config.ChannelStatusEnabled}

	model.ChannelGroup.Rule = map[string]map[string][][]int{
		"g": {
			"openai-only":      {{1}},
			"gemini-model":     {{2}},
			"claude-model":     {{3}},
			"openrouter-model": {{4}},
		},
		"openai-grp": {"openai-only": {{1}}},
	}
	model.ChannelGroup.Channels = map[int]*model.ChannelChoice{
		1: {Channel: openaiChannel},
		2: {Channel: geminiChannel},
		3: {Channel: anthropicChannel},
		4: {Channel: openrouterChannel},
	}
	return func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
	}
}

// TestFilterModelsByChannelType 覆盖列表端点使用的按协议渠道类型过滤逻辑。
func TestFilterModelsByChannelType(t *testing.T) {
	defer setupProtocolModelGroup(t)()

	all := []string{"claude-model", "gemini-model", "openai-only", "openrouter-model"}

	geminiModels := filterModelsByChannelType("g", all, AllowGeminiChannelType)
	if len(geminiModels) != 1 || geminiModels[0] != "gemini-model" {
		t.Fatalf("gemini filter: got %v want [gemini-model]", geminiModels)
	}

	// Claude 协议渠道类型现已包含 Gemini、OpenAI、OpenRouter，故四类模型均可路由。
	claudeModels := filterModelsByChannelType("g", all, AllowChannelType)
	if len(claudeModels) != 4 {
		t.Fatalf("claude filter: got %v want 4 entries (claude/gemini/openai/openrouter)", claudeModels)
	}
	gotClaude := make(map[string]bool, len(claudeModels))
	for _, m := range claudeModels {
		gotClaude[m] = true
	}
	for _, want := range []string{"claude-model", "gemini-model", "openai-only", "openrouter-model"} {
		if !gotClaude[want] {
			t.Fatalf("claude filter should include %q, got %v", want, claudeModels)
		}
	}

	// 分组内无对应协议渠道 → 空列表（非 nil）。
	empty := filterModelsByChannelType("openai-grp", []string{"openai-only"}, AllowGeminiChannelType)
	if empty == nil || len(empty) != 0 {
		t.Fatalf("expected non-nil empty slice, got %v", empty)
	}
}

func TestListGeminiModelsByToken_ProtocolFiltered(t *testing.T) {
	defer setupProtocolModelGroup(t)()

	// 有 gemini 渠道的分组：只返回 gemini-model。
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("token_group", "g")
	ListGeminiModelsByToken(c)

	var resp gemini.ModelListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, rec.Body.String())
	}
	if len(resp.Models) != 1 || resp.Models[0].Name != "models/gemini-model" {
		t.Fatalf("gemini endpoint: got %+v want single models/gemini-model", resp.Models)
	}

	// 无 gemini 渠道的分组：空列表。
	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Set("token_group", "openai-grp")
	ListGeminiModelsByToken(c2)

	var resp2 gemini.ModelListResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal empty: %v (body=%s)", err, rec2.Body.String())
	}
	if len(resp2.Models) != 0 {
		t.Fatalf("gemini endpoint (no gemini channel): got %+v want empty", resp2.Models)
	}
}

func TestListClaudeModelsByToken_ProtocolFiltered(t *testing.T) {
	defer setupProtocolModelGroup(t)()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("token_group", "g")
	ListClaudeModelsByToken(c)

	var resp claude.ModelListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, rec.Body.String())
	}
	// OpenAI/OpenRouter 渠道模型现应出现在 Claude 标签页列表中，共 4 个。
	if len(resp.Data) != 4 {
		t.Fatalf("claude endpoint: got %+v want 4 entries", resp.Data)
	}
	gotClaude := make(map[string]bool, len(resp.Data))
	for _, m := range resp.Data {
		gotClaude[m.ID] = true
	}
	for _, want := range []string{"claude-model", "gemini-model", "openai-only", "openrouter-model"} {
		if !gotClaude[want] {
			t.Fatalf("claude endpoint should include %q, got %+v", want, resp.Data)
		}
	}

	// 仅含 OpenAI 渠道的分组：OpenAI 现为 Claude 可路由类型，故应返回该 openai-only 模型。
	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Set("token_group", "openai-grp")
	ListClaudeModelsByToken(c2)

	var resp2 claude.ModelListResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal empty: %v (body=%s)", err, rec2.Body.String())
	}
	if len(resp2.Data) != 1 || resp2.Data[0].ID != "openai-only" {
		t.Fatalf("claude endpoint (openai-only group): got %+v want single openai-only", resp2.Data)
	}
}

// TestOpenAICompatProvidersImplementChatInterface 锁定"类型断言坑"：
// sendCustomChannelWithClaudeFormat 通过 base.ChatInterface（而非具体 *openai.OpenAIProvider）
// 调用转换链。OpenRouterProvider 是内嵌 OpenAIProvider 的独立结构体，直接断言具体类型会失败，
// 此测试保证两类 provider 都满足该接口，转换链不会因断言失败而 panic / 报 channel_error。
func TestOpenAICompatProvidersImplementChatInterface(t *testing.T) {
	var openaiProvider interface{} = &openai.OpenAIProvider{}
	if _, ok := openaiProvider.(providersBase.ChatInterface); !ok {
		t.Fatal("OpenAIProvider must implement base.ChatInterface for Claude conversion chain")
	}

	var openrouterProvider interface{} = &openrouter.OpenRouterProvider{}
	if _, ok := openrouterProvider.(providersBase.ChatInterface); !ok {
		t.Fatal("OpenRouterProvider must implement base.ChatInterface for Claude conversion chain")
	}
}

// TestFetchChannelByModel_NativeFirst 覆盖 Claude 协议"原生优先"渠道选择：
// 同组同模型既有原生 Anthropic 又有 OpenRouter 时应选原生；仅有 OpenRouter 时落到转换渠道。
func TestFetchChannelByModel_NativeFirst(t *testing.T) {
	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels
	defer func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
	}()

	weight := uint(1)
	anthropicCh := &model.Channel{Id: 10, Type: config.ChannelTypeAnthropic, Weight: &weight, Status: config.ChannelStatusEnabled}
	openrouterCh := &model.Channel{Id: 11, Type: config.ChannelTypeOpenRouter, Weight: &weight, Status: config.ChannelStatusEnabled}

	model.ChannelGroup.Rule = map[string]map[string][][]int{
		"g": {
			// 同一优先级同时含原生 Anthropic 与 OpenRouter
			"mixed": {{10, 11}},
			// 仅 OpenRouter
			"openrouter-only": {{11}},
		},
	}
	model.ChannelGroup.Channels = map[int]*model.ChannelChoice{
		10: {Channel: anthropicCh},
		11: {Channel: openrouterCh},
	}

	newCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("token_group", "g")
		c.Set("allow_channel_type", AllowChannelType)
		c.Set("prefer_channel_type", nativeClaudeChannelType)
		return c
	}

	// 混合分组：应优先原生 Anthropic（渠道 10）
	ch, err := fetchChannelByModel(newCtx(), "mixed")
	if err != nil {
		t.Fatalf("mixed: unexpected err %v", err)
	}
	if ch.Id != 10 {
		t.Fatalf("mixed: got channel %d want 10 (native anthropic)", ch.Id)
	}

	// 仅 OpenRouter：回退到转换渠道（渠道 11）
	ch2, err := fetchChannelByModel(newCtx(), "openrouter-only")
	if err != nil {
		t.Fatalf("openrouter-only: unexpected err %v", err)
	}
	if ch2.Id != 11 {
		t.Fatalf("openrouter-only: got channel %d want 11 (openrouter)", ch2.Id)
	}
}

// TestListClaudeModelsByToken_NewlyAllowedChannels 锁定白名单扩容后的列表口径：
// 新纳入的对话渠道（Groq/Deepseek/Bedrock/Codex 等）模型应出现在 /claude/v1/models，
// 纯图像/音频/任务型渠道（Midjourney/Suno/Flux/Kling）模型必须仍被过滤掉。
func TestListClaudeModelsByToken_NewlyAllowedChannels(t *testing.T) {
	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels
	defer func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
	}()

	weight := uint(1)
	// 每个模型一条渠道，模型名即「渠道语义」，便于断言到具体归属。
	chatChannels := map[string]int{
		"groq-model":       config.ChannelTypeGroq,
		"deepseek-model":   config.ChannelTypeDeepseek,
		"bedrock-model":    config.ChannelTypeBedrock,
		"codex-model":      config.ChannelTypeCodex,
		"geminicli-model":  config.ChannelTypeGeminiCli,
		"claudecode-model": config.ChannelTypeClaudeCode,
	}
	nonChatChannels := map[string]int{
		"midjourney-model": config.ChannelTypeMidjourney,
		"suno-model":       config.ChannelTypeSuno,
		"flux-model":       config.ChannelTypeFlux,
		"kling-model":      config.ChannelTypeKling,
	}

	rule := map[string][][]int{}
	channels := map[int]*model.ChannelChoice{}
	id := 100
	for modelName, channelType := range chatChannels {
		channels[id] = &model.ChannelChoice{Channel: &model.Channel{
			Id: id, Type: channelType, Weight: &weight, Status: config.ChannelStatusEnabled}}
		rule[modelName] = [][]int{{id}}
		id++
	}
	for modelName, channelType := range nonChatChannels {
		channels[id] = &model.ChannelChoice{Channel: &model.Channel{
			Id: id, Type: channelType, Weight: &weight, Status: config.ChannelStatusEnabled}}
		rule[modelName] = [][]int{{id}}
		id++
	}
	model.ChannelGroup.Rule = map[string]map[string][][]int{"g": rule}
	model.ChannelGroup.Channels = channels

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("token_group", "g")
	ListClaudeModelsByToken(c)

	var resp claude.ModelListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, rec.Body.String())
	}
	got := make(map[string]bool, len(resp.Data))
	for _, m := range resp.Data {
		got[m.ID] = true
	}

	for modelName := range chatChannels {
		if !got[modelName] {
			t.Fatalf("新纳入的对话渠道模型 %q 应出现在 Claude 列表, got %+v", modelName, resp.Data)
		}
	}
	for modelName := range nonChatChannels {
		if got[modelName] {
			t.Fatalf("非对话渠道模型 %q 不应出现在 Claude 列表, got %+v", modelName, resp.Data)
		}
	}
	if len(resp.Data) != len(chatChannels) {
		t.Fatalf("Claude 列表条目数不符：期望 %d，实际 %d (%+v)", len(chatChannels), len(resp.Data), resp.Data)
	}
}

// TestFetchChannelByModel_PreferNativeOverNewlyAllowed 覆盖扩容后的原生优先：
// 新纳入的转换型渠道（Groq）不得把原生 Claude 渠道挤掉；仅无原生时才落到 Groq。
func TestFetchChannelByModel_PreferNativeOverNewlyAllowed(t *testing.T) {
	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels
	defer func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
	}()

	weight := uint(1)
	anthropicCh := &model.Channel{Id: 20, Type: config.ChannelTypeAnthropic, Weight: &weight, Status: config.ChannelStatusEnabled}
	groqCh := &model.Channel{Id: 21, Type: config.ChannelTypeGroq, Weight: &weight, Status: config.ChannelStatusEnabled}

	model.ChannelGroup.Rule = map[string]map[string][][]int{
		"g": {
			// Groq 排在原生之前，证明优先级来自 prefer_channel_type 而非排列顺序
			"mixed":     {{21, 20}},
			"groq-only": {{21}},
		},
	}
	model.ChannelGroup.Channels = map[int]*model.ChannelChoice{
		20: {Channel: anthropicCh},
		21: {Channel: groqCh},
	}

	newCtx := func() *gin.Context {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("token_group", "g")
		c.Set("allow_channel_type", AllowChannelType)
		c.Set("prefer_channel_type", nativeClaudeChannelType)
		return c
	}

	ch, err := fetchChannelByModel(newCtx(), "mixed")
	if err != nil {
		t.Fatalf("mixed: unexpected err %v", err)
	}
	if ch.Id != 20 {
		t.Fatalf("mixed: got channel %d want 20 (native anthropic)", ch.Id)
	}

	ch2, err := fetchChannelByModel(newCtx(), "groq-only")
	if err != nil {
		t.Fatalf("groq-only: unexpected err %v", err)
	}
	if ch2.Id != 21 {
		t.Fatalf("groq-only: got channel %d want 21 (groq conversion channel)", ch2.Id)
	}
}

// TestGetOpenAIModelWithName_Architecture 锁定 /v1/models 的 architecture 口径：
// 有 model_info 时输出输入 / 输出模态（空模态序列化为 []），无 model_info 时不出现该键。
func TestGetOpenAIModelWithName_Architecture(t *testing.T) {
	prevPricing := model.PricingInstance
	prevOwnedBys := model.ModelOwnedBysInstance
	defer func() {
		model.PricingInstance = prevPricing
		model.ModelOwnedBysInstance = prevOwnedBys
	}()

	model.ModelOwnedBysInstance = &model.ModelOwnedBys{
		ModelOwnedBy: map[int]*model.ModelOwnedBy{
			config.ChannelTypeOpenAI: {Id: config.ChannelTypeOpenAI, Name: "OpenAI"},
		},
	}
	model.PricingInstance = &model.Pricing{Match: []string{}}
	model.PricingInstance.Prices = map[string]*model.Price{
		"with-info": {
			Model:       "with-info",
			ChannelType: config.ChannelTypeOpenAI,
			ModelInfo: &model.ModelInfoResponse{
				Model:            "with-info",
				InputModalities:  []string{"text", "image"},
				OutputModalities: []string{"text"},
			},
		},
		"empty-modalities": {
			Model:       "empty-modalities",
			ChannelType: config.ChannelTypeOpenAI,
			ModelInfo:   &model.ModelInfoResponse{Model: "empty-modalities"},
		},
		"without-info": {
			Model:       "without-info",
			ChannelType: config.ChannelTypeOpenAI,
		},
	}

	marshal := func(modelName string) string {
		b, err := json.Marshal(getOpenAIModelWithName(modelName))
		if err != nil {
			t.Fatalf("marshal %s: %v", modelName, err)
		}
		return string(b)
	}

	got := marshal("with-info")
	if !strings.Contains(got, `"architecture":{"input_modalities":["text","image"],"output_modalities":["text"]}`) {
		t.Fatalf("with-info: architecture 未按预期序列化, got %s", got)
	}

	// model_info 存在但模态为空：应为 [] 而非 null
	gotEmpty := marshal("empty-modalities")
	if !strings.Contains(gotEmpty, `"architecture":{"input_modalities":[],"output_modalities":[]}`) {
		t.Fatalf("empty-modalities: 空模态应序列化为 [], got %s", gotEmpty)
	}

	// 无 model_info：JSON 中不应出现 architecture 键
	gotNone := marshal("without-info")
	if strings.Contains(gotNone, "architecture") {
		t.Fatalf("without-info: 不应出现 architecture 键, got %s", gotNone)
	}
}

// TestGetOpenAIModelWithName_Capabilities 锁定 /v1/models 的 capabilities 口径：
// 有能力时按词表顺序输出数组，能力为空或无 model_info 时不出现该键；architecture 输出不受影响。
func TestGetOpenAIModelWithName_Capabilities(t *testing.T) {
	prevPricing := model.PricingInstance
	prevOwnedBys := model.ModelOwnedBysInstance
	defer func() {
		model.PricingInstance = prevPricing
		model.ModelOwnedBysInstance = prevOwnedBys
	}()

	model.ModelOwnedBysInstance = &model.ModelOwnedBys{
		ModelOwnedBy: map[int]*model.ModelOwnedBy{
			config.ChannelTypeOpenAI: {Id: config.ChannelTypeOpenAI, Name: "OpenAI"},
		},
	}
	model.PricingInstance = &model.Pricing{Match: []string{}}
	model.PricingInstance.Prices = map[string]*model.Price{
		"with-caps": {
			Model:       "with-caps",
			ChannelType: config.ChannelTypeOpenAI,
			ModelInfo: &model.ModelInfoResponse{
				Model:            "with-caps",
				InputModalities:  []string{"text"},
				OutputModalities: []string{"text"},
				Capabilities:     []string{"tool_call", "reasoning"},
			},
		},
		"empty-caps": {
			Model:       "empty-caps",
			ChannelType: config.ChannelTypeOpenAI,
			ModelInfo: &model.ModelInfoResponse{
				Model:        "empty-caps",
				Capabilities: []string{},
			},
		},
		"without-info": {
			Model:       "without-info",
			ChannelType: config.ChannelTypeOpenAI,
		},
	}

	marshal := func(modelName string) string {
		b, err := json.Marshal(getOpenAIModelWithName(modelName))
		if err != nil {
			t.Fatalf("marshal %s: %v", modelName, err)
		}
		return string(b)
	}

	got := marshal("with-caps")
	if !strings.Contains(got, `"capabilities":["tool_call","reasoning"]`) {
		t.Fatalf("with-caps: capabilities 未按预期序列化, got %s", got)
	}
	// architecture 口径不受能力字段影响
	if !strings.Contains(got, `"architecture":{"input_modalities":["text"],"output_modalities":["text"]}`) {
		t.Fatalf("with-caps: architecture 输出应保持不变, got %s", got)
	}

	// model_info 存在但能力为空切片：不应出现 capabilities 键，architecture 仍为空模态
	gotEmpty := marshal("empty-caps")
	if strings.Contains(gotEmpty, "capabilities") {
		t.Fatalf("empty-caps: 不应出现 capabilities 键, got %s", gotEmpty)
	}
	if !strings.Contains(gotEmpty, `"architecture":{"input_modalities":[],"output_modalities":[]}`) {
		t.Fatalf("empty-caps: architecture 输出应保持不变, got %s", gotEmpty)
	}

	// 无 model_info：既无 architecture 也无 capabilities
	gotNone := marshal("without-info")
	if strings.Contains(gotNone, "capabilities") {
		t.Fatalf("without-info: 不应出现 capabilities 键, got %s", gotNone)
	}
	if strings.Contains(gotNone, "architecture") {
		t.Fatalf("without-info: 不应出现 architecture 键, got %s", gotNone)
	}
}

// setupVendorOwnedBys 用内存 sqlite 装配 model_owned_by / model_info，返回还原函数。
func setupVendorOwnedBys(t *testing.T, infos []*model.ModelInfo) func() {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "vendor_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&model.ModelOwnedBy{}, &model.ModelInfo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	owners := []*model.ModelOwnedBy{
		{Id: config.ChannelTypeOpenAI, Name: "OpenAI", Slug: "openai"},
		{Id: config.ChannelTypeAnthropic, Name: "Anthropic", Slug: "anthropic"},
		{Id: config.ChannelTypeGemini, Name: "Google Gemini", Slug: "google", Icon: "brand:gemini"},
	}
	if err := testDB.Create(&owners).Error; err != nil {
		t.Fatalf("seed owners: %v", err)
	}
	for _, info := range infos {
		if err := testDB.Create(info).Error; err != nil {
			t.Fatalf("seed model info: %v", err)
		}
	}

	prevDB, prevInstance := model.DB, model.ModelOwnedBysInstance
	prevModelChannelType := model.ChannelGroup.ModelChannelType
	model.DB = testDB
	model.ChannelGroup.ModelChannelType = map[string]int{"gemini-model": config.ChannelTypeGemini}
	model.ModelOwnedBysInstance = &model.ModelOwnedBys{}
	if err := model.ModelOwnedBysInstance.Load(); err != nil {
		t.Fatalf("load owners: %v", err)
	}
	return func() {
		model.DB, model.ModelOwnedBysInstance = prevDB, prevInstance
		model.ChannelGroup.ModelChannelType = prevModelChannelType
	}
}

// setupCatalogModels 装配「可见主名 + 别名 + 隐藏模型 + 无目录行模型」的目录与可路由分组，返回还原函数。
func setupCatalogModels(t *testing.T) func() {
	t.Helper()
	restoreVendors := setupVendorOwnedBys(t, []*model.ModelInfo{
		{Model: "gemini-2.5-pro", VendorID: config.ChannelTypeGemini, Endpoints: `["chat","responses"]`},
		{Model: "google/gemini-2.5-pro", AliasOf: "gemini-2.5-pro"},
		{Model: "draft-model", VendorID: config.ChannelTypeOpenAI, Hidden: true},
		// 可见但厂商推不出：列表里有它，检索也必须能拿到条目。
		{Model: "orphan-model"},
	})

	prevModelGroup := model.ChannelGroup.ModelGroup
	prevPublicGroup := model.GlobalUserGroupRatio.PublicGroup
	prevPricing := model.PricingInstance

	// rowless-model 只在渠道里出现、目录里没有行：有渠道即可见。
	model.ChannelGroup.ModelGroup = map[string]map[string]bool{
		"gemini-2.5-pro":        {"default": true},
		"google/gemini-2.5-pro": {"default": true},
		"draft-model":           {"default": true},
		"orphan-model":          {"default": true},
		"rowless-model":         {"default": true},
	}
	model.GlobalUserGroupRatio.PublicGroup = []string{"default"}
	model.PricingInstance = &model.Pricing{Match: []string{}}
	model.PricingInstance.Prices = map[string]*model.Price{}

	return func() {
		model.ChannelGroup.ModelGroup = prevModelGroup
		model.GlobalUserGroupRatio.PublicGroup = prevPublicGroup
		model.PricingInstance = prevPricing
		restoreVendors()
	}
}

// setEnforceHidden 临时设置 catalog.enforce_hidden 并在用例结束后还原。
func setEnforceHidden(t *testing.T, enabled bool) {
	t.Helper()
	old := viper.GetBool("catalog.enforce_hidden")
	viper.Set("catalog.enforce_hidden", enabled)
	t.Cleanup(func() { viper.Set("catalog.enforce_hidden", old) })
}

// /api/available_model 的目录口径：可路由且未隐藏的主名（含无目录行的模型），别名与隐藏模型不出现，
// 带 vendor / endpoints / aliases 字段；enforce 关闭时退回「所有可路由模型」。
func TestGetAvailableModels_CatalogConvergence(t *testing.T) {
	defer setupCatalogModels(t)()

	setEnforceHidden(t, true)
	got := getAvailableModels("")
	if len(got) != 3 {
		t.Fatalf("enforce 开启时应返回可见主名与无目录行模型, got %v", got)
	}
	for _, name := range []string{"orphan-model", "rowless-model"} {
		if _, ok := got[name]; !ok {
			t.Fatalf("应包含 %s, got %v", name, got)
		}
	}
	entry, ok := got["gemini-2.5-pro"]
	if !ok {
		t.Fatalf("应包含主名 gemini-2.5-pro, got %v", got)
	}
	if entry.OwnedBy != "Google Gemini" || entry.Vendor == nil ||
		entry.Vendor.Id != config.ChannelTypeGemini || entry.Vendor.Name != "Google Gemini" || entry.Vendor.Icon != "brand:gemini" {
		t.Fatalf("vendor 字段不符: owned_by=%q vendor=%+v", entry.OwnedBy, entry.Vendor)
	}
	// 前端按厂商归类认 slug：厂商展示名是 "Google Gemini"，slug 必须是 google。
	if entry.Vendor.Slug != "google" {
		t.Fatalf("vendor.slug = %q, want google", entry.Vendor.Slug)
	}
	if len(entry.Endpoints) != 2 || entry.Endpoints[0] != "chat" || entry.Endpoints[1] != "responses" {
		t.Fatalf("endpoints 字段不符: %v", entry.Endpoints)
	}
	if len(entry.Aliases) != 1 || entry.Aliases[0] != "google/gemini-2.5-pro" {
		t.Fatalf("aliases 字段不符: %v", entry.Aliases)
	}

	setEnforceHidden(t, false)
	all := getAvailableModels("")
	if len(all) != 5 {
		t.Fatalf("enforce 关闭时应退回所有可路由模型, got %v", all)
	}
}

// /v1/models 返回可路由且未隐藏的主名（含无目录行的模型）；别名与隐藏模型不出现。
func TestListModelsByToken_CatalogConvergence(t *testing.T) {
	defer setupCatalogModels(t)()

	prevRule := model.ChannelGroup.Rule
	defer func() { model.ChannelGroup.Rule = prevRule }()
	model.ChannelGroup.Rule = map[string]map[string][][]int{
		"default": {
			"gemini-2.5-pro":        {{2}},
			"google/gemini-2.5-pro": {{2}},
			"draft-model":           {{1}},
			"rowless-model":         {{1}},
		},
	}

	listIds := func() []string {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("token_group", "default")
		ListModelsByToken(c)

		var resp struct {
			Data []OpenAIModels `json:"data"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("unmarshal: %v (body=%s)", err, rec.Body.String())
		}
		ids := make([]string, 0, len(resp.Data))
		for _, m := range resp.Data {
			ids = append(ids, m.Id)
		}
		return ids
	}

	setEnforceHidden(t, true)
	ids := listIds()
	sort.Strings(ids)
	if len(ids) != 2 || ids[0] != "gemini-2.5-pro" || ids[1] != "rowless-model" {
		t.Fatalf("enforce 开启时 /v1/models 应只见未隐藏主名, got %v", ids)
	}

	setEnforceHidden(t, false)
	if all := listIds(); len(all) != 4 {
		t.Fatalf("enforce 关闭时 /v1/models 应退回全部可路由模型, got %v", all)
	}
}

// /v1/models/:model：别名返回主名条目，隐藏模型返回 model_not_found 错误体。
func TestRetrieveModel_AliasAndHidden(t *testing.T) {
	defer setupCatalogModels(t)()
	setEnforceHidden(t, true)

	retrieve := func(name string) (int, map[string]any) {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Params = gin.Params{{Key: "model", Value: name}}
		RetrieveModel(c)

		var body map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("unmarshal %s: %v (body=%s)", name, err, rec.Body.String())
		}
		return rec.Code, body
	}

	code, alias := retrieve("google/gemini-2.5-pro")
	if code != http.StatusOK || alias["id"] != "gemini-2.5-pro" {
		t.Fatalf("别名应返回主名条目, got %d %v", code, alias)
	}

	// 路由是 /*model 通配，param 带前导 "/"，RetrieveModel 必须去掉后再解析。
	if _, wildcard := retrieve("/google/gemini-2.5-pro"); wildcard["id"] != "gemini-2.5-pro" {
		t.Fatalf("通配 param 应去掉前导斜杠后解析, got %v", wildcard)
	}

	// 隐藏模型：OpenAI 口径的 404 + model_not_found 错误体。
	code, body := retrieve("draft-model")
	if code != http.StatusNotFound {
		t.Fatalf("隐藏模型应返回 404, got %d", code)
	}
	errBody, ok := body["error"].(map[string]any)
	if !ok || errBody["code"] != "model_not_found" || errBody["type"] != "invalid_request_error" {
		t.Fatalf("错误体应符合 OpenAI 格式, got %v", body)
	}

	// 既无渠道也无目录行的模型名：同样是 404，与 chat 入口一致。
	code, missing := retrieve("no-such-model")
	if code != http.StatusNotFound {
		t.Fatalf("不存在的模型应返回 404, got %d %v", code, missing)
	}

	// 检索与列表同源：可见但厂商推不出的模型在列表里，检索也必须 200；无目录行的可路由模型同理。
	if code, orphan := retrieve("orphan-model"); code != http.StatusOK || orphan["id"] != "orphan-model" {
		t.Fatalf("可见模型应可检索, got %d %v", code, orphan)
	}
	if !isCatalogVisible("orphan-model") {
		t.Fatal("可见模型应出现在列表口径里")
	}
	if code, rowless := retrieve("rowless-model"); code != http.StatusOK || rowless["id"] != "rowless-model" {
		t.Fatalf("无目录行的可路由模型应可检索, got %d %v", code, rowless)
	}

	// enforce 关闭（应急回滚）时目录不再约束，但不存在的模型仍是 404，主名仍 200。
	setEnforceHidden(t, false)
	if code, body := retrieve("no-such-model"); code != http.StatusNotFound {
		t.Fatalf("enforce 关闭时不存在的模型仍应 404, got %d %v", code, body)
	}
	if code, main := retrieve("gemini-2.5-pro"); code != http.StatusOK || main["id"] != "gemini-2.5-pro" {
		t.Fatalf("enforce 关闭时可见主名仍应 200, got %d %v", code, main)
	}
}

// relay 入口：别名改写为主名并保留原始请求名；隐藏模型按「模型不存在」处理；无目录行的模型放行；
// enforce 关闭时隐藏不再生效。
func TestResolveCatalogModel(t *testing.T) {
	defer setupCatalogModels(t)()

	newCtx := func() *gin.Context {
		c := newTestContext()
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		return c
	}

	setEnforceHidden(t, true)

	c := newCtx()
	got, err := resolveCatalogModel(c, "google/gemini-2.5-pro", false)
	if err != nil {
		t.Fatalf("别名请求不应报错: %v", err)
	}
	if got != "gemini-2.5-pro" {
		t.Fatalf("别名应改写为主名, got %q", got)
	}
	if requested := c.GetString("requested_model"); requested != "google/gemini-2.5-pro" {
		t.Fatalf("应保留原始请求名, got %q", requested)
	}

	if _, err := resolveCatalogModel(newCtx(), "draft-model", false); !IsModelNotFound(err) {
		t.Fatalf("隐藏模型应收敛为 model_not_found, got %v", err)
	}
	if got, err := resolveCatalogModel(newCtx(), "no-catalog-row", false); err != nil || got != "no-catalog-row" {
		t.Fatalf("无目录行的模型不受隐藏约束，应原样放行, got (%q,%v)", got, err)
	}
	// 任务型豁免：无目录行的伪模型名原样放行
	if got, err := resolveCatalogModel(newCtx(), "mj_imagine", true); err != nil || got != "mj_imagine" {
		t.Fatalf("任务型豁免应原样放行伪模型名, got (%q,%v)", got, err)
	}

	setEnforceHidden(t, false)
	if got, err := resolveCatalogModel(newCtx(), "draft-model", false); err != nil || got != "draft-model" {
		t.Fatalf("enforce 关闭时应放行隐藏模型, got (%q,%v)", got, err)
	}
	// 别名改写与 enforce 无关，回滚后仍按主名选路与计费
	if got, err := resolveCatalogModel(newCtx(), "google/gemini-2.5-pro", false); err != nil || got != "gemini-2.5-pro" {
		t.Fatalf("enforce 关闭时别名仍应改写为主名, got (%q,%v)", got, err)
	}
}

// GetProvider 对隐藏模型直接收敛为 model_not_found（上层据此返回 404），不进入渠道选择。
func TestGetProvider_HiddenModelNotFound(t *testing.T) {
	defer setupCatalogModels(t)()
	setEnforceHidden(t, true)

	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("token_group", "default")

	_, _, err := GetProvider(c, "draft-model")
	if !IsModelNotFound(err) {
		t.Fatalf("expected IsModelNotFound=true, got %v", err)
	}
}

// GetTaskProvider（MJ / Suno / Kling 入口）对无目录行的内部伪模型名不做目录校验，直接进入后续
// 流程；无目录行的模型不受隐藏约束，走标准入口 GetProvider 同样越过目录校验。
// 用令牌模型限制作为「已越过目录校验」的观测点：它紧跟在目录校验之后，命中它即说明未被目录拦下。
func TestGetTaskProvider_ExemptFromCatalogEnforce(t *testing.T) {
	defer setupCatalogModels(t)()
	setEnforceHidden(t, true)

	newCtx := func() *gin.Context {
		c := newTestContext()
		c.Request = httptest.NewRequest(http.MethodPost, "/suno/submit/music", nil)
		c.Set("token_group", "default")
		setting := &model.TokenSetting{}
		setting.Limits.LimitModelSetting.Enabled = true
		setting.Limits.LimitModelSetting.Models = []string{"gemini-2.5-pro"}
		c.Set("token_setting", setting)
		return c
	}

	_, _, err := GetTaskProvider(newCtx(), "chirp-v3-5")
	if IsModelNotFound(err) {
		t.Fatalf("任务型入口不应被目录校验拦下, got %v", err)
	}
	if err == nil || !strings.Contains(err.Error(), "not supported for current token") {
		t.Fatalf("任务型入口应越过目录校验到达令牌限制, got %v", err)
	}

	_, _, err = GetProvider(newCtx(), "chirp-v3-5")
	if err == nil || !strings.Contains(err.Error(), "not supported for current token") {
		t.Fatalf("标准入口对无目录行模型应越过目录校验到达令牌限制, got %v", err)
	}
}

// 任务型豁免只覆盖无目录行的伪模型名：目录里存在但被隐藏的模型（Suno 的 mv 由用户请求体给出）
// 经任务入口同样被隐藏校验拦下，不能成为绕过面。
func TestGetTaskProvider_EnforcesHiddenForCatalogRows(t *testing.T) {
	defer setupCatalogModels(t)()
	setEnforceHidden(t, true)

	// 令牌模型限制紧跟目录校验，命中它即说明已越过目录校验。
	newCtx := func() *gin.Context {
		c := newTestContext()
		c.Request = httptest.NewRequest(http.MethodPost, "/suno/submit/music", nil)
		c.Set("token_group", "default")
		setting := &model.TokenSetting{}
		setting.Limits.LimitModelSetting.Enabled = true
		setting.Limits.LimitModelSetting.Models = []string{"gemini-2.5-pro"}
		c.Set("token_setting", setting)
		return c
	}

	if _, _, err := GetTaskProvider(newCtx(), "draft-model"); !IsModelNotFound(err) {
		t.Fatalf("目录中被隐藏的模型经任务入口应 model_not_found, got %v", err)
	}

	setEnforceHidden(t, false)
	_, _, err := GetTaskProvider(newCtx(), "draft-model")
	if err == nil || !strings.Contains(err.Error(), "not supported for current token") {
		t.Fatalf("enforce 关闭后任务入口应越过目录校验到达令牌限制, got %v", err)
	}
}

// 令牌模型限制对别名与主名同等生效：列表过滤与 relay 前置校验两侧口径一致。
func TestTokenLimitAliasEquivalence(t *testing.T) {
	defer setupCatalogModels(t)()

	newCtxWithLimit := func(allowed ...string) *gin.Context {
		c := newTestContext()
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		setting := &model.TokenSetting{}
		setting.Limits.LimitModelSetting.Enabled = true
		setting.Limits.LimitModelSetting.Models = allowed
		c.Set("token_setting", setting)
		return c
	}

	all := []string{"gemini-2.5-pro", "google/gemini-2.5-pro", "draft-model"}

	// 白名单写主名：别名与主名都放行
	got := filterModelsByTokenLimit(newCtxWithLimit("gemini-2.5-pro"), all)
	if len(got) != 2 {
		t.Fatalf("白名单写主名时别名应同样放行, got %v", got)
	}
	// 白名单写别名：主名与别名都放行
	gotAlias := filterModelsByTokenLimit(newCtxWithLimit("google/gemini-2.5-pro"), all)
	if len(gotAlias) != 2 {
		t.Fatalf("白名单写别名时主名应同样放行, got %v", gotAlias)
	}

	if err := CheckLimitModel(newCtxWithLimit("google/gemini-2.5-pro"), "gemini-2.5-pro"); err != nil {
		t.Fatalf("白名单写别名时主名请求应放行: %v", err)
	}
	if err := CheckLimitModel(newCtxWithLimit("gemini-2.5-pro"), "draft-model"); err == nil {
		t.Fatal("不在白名单内的模型仍应被拒绝")
	}
}

// 厂商解析链：model_info.vendor_id → 别名主名 vendor → 价格表 channel_type → 代表渠道 → 未知。
// owned_by 取 vendor slug（稳定机器口径），厂商整体推不出时才是 UnknownOwnedBy。
func TestGetModelOwnedByFallbackChain(t *testing.T) {
	defer setupVendorOwnedBys(t, []*model.ModelInfo{
		{Model: "claude-model", VendorID: config.ChannelTypeAnthropic},
		{Model: "alias-model", AliasOf: "claude-model"},
	})()
	defer setupProtocolModelGroup(t)()

	cases := []struct {
		name        string
		modelName   string
		channelType int
		want        string
	}{
		{"vendor_id 优先于渠道类型", "claude-model", config.ChannelTypeOpenAI, "anthropic"},
		{"别名随主名", "alias-model", 0, "anthropic"},
		{"无目录行回退价格表 channel_type", "openai-only", config.ChannelTypeOpenAI, "openai"},
		{"价格表未配置回退代表渠道", "gemini-model", 0, "google"},
		{"都推不出为未知", "nobody-serves-this", 0, model.UnknownOwnedBy},
	}
	for _, c := range cases {
		if got := *getModelOwnedBy(c.modelName, c.channelType); got != c.want {
			t.Errorf("%s: getModelOwnedBy(%q,%d) = %q, want %q", c.name, c.modelName, c.channelType, got, c.want)
		}
	}
}

// /v1/models 条目的 created 取目录行创建时间、owned_by 取 vendor slug；
// 别名两项均随主名；无目录行时 created 回退历史固定值、owned_by 回退未知。
func TestGetOpenAIModelWithName_CreatedAndOwnedBy(t *testing.T) {
	const canonicalCreated int64 = 1700000000
	defer setupVendorOwnedBys(t, []*model.ModelInfo{
		{Model: "claude-model", VendorID: config.ChannelTypeAnthropic, CreatedAt: canonicalCreated},
		{Model: "alias-model", AliasOf: "claude-model", CreatedAt: canonicalCreated + 86400},
	})()

	prevPricing := model.PricingInstance
	defer func() { model.PricingInstance = prevPricing }()
	model.PricingInstance = &model.Pricing{Match: []string{}}
	model.PricingInstance.Prices = map[string]*model.Price{}

	canonical := getOpenAIModelWithName("claude-model")
	if canonical.Created != int(canonicalCreated) {
		t.Errorf("主名 created 应取目录行创建时间, got %d want %d", canonical.Created, canonicalCreated)
	}
	if canonical.OwnedBy == nil || *canonical.OwnedBy != "anthropic" {
		t.Errorf("主名 owned_by 应为 vendor slug, got %v", canonical.OwnedBy)
	}

	alias := getOpenAIModelWithName("alias-model")
	if alias.Created != int(canonicalCreated) {
		t.Errorf("别名 created 应与主名一致, got %d want %d", alias.Created, canonicalCreated)
	}
	if alias.OwnedBy == nil || *alias.OwnedBy != "anthropic" {
		t.Errorf("别名 owned_by 应与主名一致, got %v", alias.OwnedBy)
	}

	none := getOpenAIModelWithName("no-catalog-row")
	if none.Created != defaultModelCreated {
		t.Errorf("无目录行 created 应回退固定值, got %d", none.Created)
	}
	if none.OwnedBy == nil || *none.OwnedBy != model.UnknownOwnedBy {
		t.Errorf("无目录行 owned_by 应回退未知, got %v", none.OwnedBy)
	}
}

// tts_voices 三层来源：同步音色（按 id 推断语言 / 性别）> 渠道类型内置音色表 > 六个经典 OpenAI 音色；
// MiniMax 模型的同步音色再并上内置 MiniMax 表；非 TTS 模型不带该字段。
func TestModelTTSVoices(t *testing.T) {
	speech := `["audio.speech"]`
	defer setupVendorOwnedBys(t, []*model.ModelInfo{
		{Model: "hexgrad/kokoro-82m", Endpoints: speech, SupportedVoices: `["af_alloy","zf_xiaoxiao"]`},
		{Model: "gemini-tts", Endpoints: speech},
		{Model: "minimax-tts", Endpoints: speech},
		{Model: "minimax/speech-2.8-hd", Endpoints: speech, SupportedVoices: `["English_radiant_girl","female-shaonv"]`},
		{Model: "minimax-native-synced", Endpoints: speech, SupportedVoices: `["English_radiant_girl"]`},
		{Model: "azure-tts", Endpoints: speech},
		{Model: "gpt-4o-mini-tts", Endpoints: speech},
		{Model: "other-tts", Endpoints: speech},
		{Model: "chat-model", Endpoints: `["chat"]`},
	})()
	model.ChannelGroup.ModelChannelType = map[string]int{
		"hexgrad/kokoro-82m":    config.ChannelTypeOpenRouter,
		"gemini-tts":            config.ChannelTypeGemini,
		"minimax-tts":           config.ChannelTypeMiniMax,
		"minimax/speech-2.8-hd": config.ChannelTypeOpenRouter,
		"minimax-native-synced": config.ChannelTypeMiniMax,
		"azure-tts":             config.ChannelTypeAzureSpeech,
		"gpt-4o-mini-tts":       config.ChannelTypeOpenAI,
		"other-tts":             config.ChannelTypeOpenRouter,
		"chat-model":            config.ChannelTypeOpenAI,
	}

	voices := func(name string) []types.TTSVoice {
		return modelTTSVoices(name, model.ModelEndpoints(name))
	}

	kokoro := voices("hexgrad/kokoro-82m")
	if len(kokoro) != 2 || kokoro[1] != (types.TTSVoice{ID: "zf_xiaoxiao", Language: "zh", Gender: "female"}) {
		t.Fatalf("kokoro voices = %+v", kokoro)
	}
	if got := voices("gemini-tts"); len(got) != 30 || got[0].ID != "Zephyr" {
		t.Fatalf("gemini voices = %+v", got)
	}
	builtinMiniMax := len(minimax.BuiltinSpeechVoices())
	if got := voices("minimax-tts"); len(got) != builtinMiniMax ||
		got[0] != (types.TTSVoice{ID: "female-chengshu", Language: "zh", Gender: "female"}) {
		t.Fatalf("minimax voices = %+v", got)
	}
	// MiniMax 模型：同步列表在前，再并上内置表，按 id 去重（female-shaonv 两边都有）。
	for _, name := range []string{"minimax/speech-2.8-hd", "minimax-native-synced"} {
		got := voices(name)
		synced := len(model.ModelSupportedVoices(name))
		if got[0] != (types.TTSVoice{ID: "English_radiant_girl", Language: "en", Gender: "female"}) {
			t.Fatalf("%s: synced voice should come first, got %+v", name, got[0])
		}
		want := synced + builtinMiniMax
		if name == "minimax/speech-2.8-hd" {
			want--
		}
		if len(got) != want {
			t.Fatalf("%s: len = %d, want %d", name, len(got), want)
		}
		if got[synced] != (types.TTSVoice{ID: "female-chengshu", Language: "zh", Gender: "female"}) {
			t.Fatalf("%s: builtin voices should follow synced ones, got %+v", name, got[synced])
		}
	}
	if got := voices("hexgrad/kokoro-82m"); len(got) != 2 {
		t.Fatalf("non-MiniMax synced voices should not merge builtin table, got %d", len(got))
	}
	// Azure 默认映射里 alloy 与 fable 同为 YunxiNeural，去重后 5 个。
	if got := voices("azure-tts"); len(got) != 5 ||
		got[0] != (types.TTSVoice{ID: "zh-CN-YunxiNeural", Language: "zh", Gender: "male"}) {
		t.Fatalf("azure voices = %+v", got)
	}
	if got := voices("gpt-4o-mini-tts"); len(got) != 13 || got[12].ID != "cedar" {
		t.Fatalf("openai voices = %+v", got)
	}
	if got := voices("other-tts"); len(got) != 6 || got[0].ID != "alloy" || got[5].ID != "shimmer" {
		t.Fatalf("fallback voices = %+v", got)
	}
	if got := voices("chat-model"); got != nil {
		t.Fatalf("non-TTS model should have no voices, got %+v", got)
	}

	b, err := json.Marshal(&AvailableModelResponse{Endpoints: []string{"chat"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "tts_voices") {
		t.Fatalf("non-TTS entry should omit tts_voices: %s", b)
	}
}

// tts_formats：Gemini TTS（直连或经 OpenRouter）只 wav / pcm 且 wav 在前；OpenRouter 其余按模型收窄
// （整体 mp3 / pcm，MiniMax 只 mp3），其余渠道返回完整列表；非 TTS 模型不带该字段。
func TestModelTTSFormats(t *testing.T) {
	speech := `["audio.speech"]`
	defer setupVendorOwnedBys(t, []*model.ModelInfo{
		{Model: "hexgrad/kokoro-82m", Endpoints: speech},
		{Model: "google/gemini-3.1-flash-tts-preview", Endpoints: speech},
		{Model: "minimax/speech-2.8-turbo", Endpoints: speech},
		{Model: "gpt-4o-mini-tts", Endpoints: speech},
		{Model: "gemini-tts", Endpoints: speech},
		{Model: "unknown-tts", Endpoints: speech},
		{Model: "chat-model", Endpoints: `["chat"]`},
	})()
	model.ChannelGroup.ModelChannelType = map[string]int{
		"hexgrad/kokoro-82m":                  config.ChannelTypeOpenRouter,
		"google/gemini-3.1-flash-tts-preview": config.ChannelTypeOpenRouter,
		"minimax/speech-2.8-turbo":            config.ChannelTypeOpenRouter,
		"gpt-4o-mini-tts":                     config.ChannelTypeOpenAI,
		"gemini-tts":                          config.ChannelTypeGemini,
		"chat-model":                          config.ChannelTypeOpenRouter,
	}

	formats := func(name string) []string {
		return modelTTSFormats(name, model.ModelEndpoints(name))
	}
	all := []string{"mp3", "opus", "aac", "flac", "wav", "pcm"}

	cases := map[string][]string{
		"hexgrad/kokoro-82m":                  {"mp3", "pcm"},
		"google/gemini-3.1-flash-tts-preview": {"wav", "pcm"},
		"minimax/speech-2.8-turbo":            {"mp3"},
		"gpt-4o-mini-tts":                     all,
		"gemini-tts":                          {"wav", "pcm"},
		"unknown-tts":                         all,
	}
	for name, want := range cases {
		if got := formats(name); !reflect.DeepEqual(got, want) {
			t.Errorf("%s formats = %v, want %v", name, got, want)
		}
	}
	if got := formats("chat-model"); got != nil {
		t.Fatalf("non-TTS model should have no formats, got %v", got)
	}

	b, err := json.Marshal(&AvailableModelResponse{Endpoints: []string{"chat"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "tts_formats") {
		t.Fatalf("non-TTS entry should omit tts_formats: %s", b)
	}
}
