package relay

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/providers/gemini"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupRoutingPolicies 在内存库里建两条启用的策略：一条兜底是 Claude 渠道的模型，
// 一条兜底是 Gemini 渠道的模型，并加载索引。返回还原函数。
func setupRoutingPolicies(t *testing.T) func() {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.RoutingPolicy{}, &model.ModelInfo{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	prevDB := model.DB
	model.DB = testDB
	// /v1/models 逐模型取价，测试进程里没有价格实例，给一个空的。
	prevPricing := model.PricingInstance
	model.PricingInstance = &model.Pricing{Prices: map[string]*model.Price{}, Match: []string{}}

	routes := []model.RoutingRoute{
		{Label: "small", TargetModel: "openai-only"},
		{Label: "large", TargetModel: "claude-model"},
	}
	for _, p := range []*model.RoutingPolicy{
		{Name: "auto/claude", Enabled: true, DecisionModel: "openai-only", Routes: routes, FallbackModel: "claude-model", Threshold: 0.5},
		{Name: "auto/gemini", Enabled: true, DecisionModel: "openai-only", Routes: routes, FallbackModel: "gemini-model", Threshold: 0.5},
		{Name: "auto/off", Enabled: false, DecisionModel: "openai-only", Routes: routes, FallbackModel: "claude-model", Threshold: 0.5},
	} {
		if err := p.Insert(); err != nil {
			t.Fatalf("建策略 %s 失败: %v", p.Name, err)
		}
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatalf("加载策略索引失败: %v", err)
	}
	return func() {
		model.DB = prevDB
		model.PricingInstance = prevPricing
		model.ResetRoutingPolicyCache()
	}
}

// TestListModelsByToken_IncludesRoutingPolicies /v1/models 并入兜底可路由的启用策略
func TestListModelsByToken_IncludesRoutingPolicies(t *testing.T) {
	defer setupProtocolModelGroup(t)()
	defer setupRoutingPolicies(t)()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("token_group", "g")
	ListModelsByToken(c)

	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, rec.Body.String())
	}
	ids := make(map[string]bool, len(resp.Data))
	for _, m := range resp.Data {
		ids[m.ID] = true
	}
	if !ids["auto/claude"] || !ids["auto/gemini"] {
		t.Fatalf("列表应含两条启用策略，实际 %v", ids)
	}
	if ids["auto/off"] {
		t.Fatalf("停用策略不应出现在列表里")
	}

	// 分组里没有兜底模型的渠道：策略不并入。
	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Set("token_group", "openai-grp")
	ListModelsByToken(c2)
	if err := json.Unmarshal(rec2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, m := range resp.Data {
		if m.ID == "auto/claude" || m.ID == "auto/gemini" {
			t.Fatalf("兜底不可路由的分组不应列出策略 %s", m.ID)
		}
	}
}

// TestProtocolLists_RoutingPoliciesFollowFallback Gemini / Claude 列表按兜底模型的渠道类型决定是否列出策略
func TestProtocolLists_RoutingPoliciesFollowFallback(t *testing.T) {
	defer setupProtocolModelGroup(t)()
	defer setupRoutingPolicies(t)()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Set("token_group", "g")
	ListGeminiModelsByToken(c)
	var geminiResp gemini.ModelListResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &geminiResp); err != nil {
		t.Fatalf("unmarshal gemini: %v", err)
	}
	names := make(map[string]bool)
	for _, m := range geminiResp.Models {
		names[m.Name] = true
	}
	if !names["models/auto/gemini"] || names["models/auto/claude"] {
		t.Fatalf("Gemini 列表应只含兜底为 gemini 渠道的策略，实际 %v", names)
	}

	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	c2.Set("token_group", "g")
	ListClaudeModelsByToken(c2)
	var claudeResp claude.ModelListResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &claudeResp); err != nil {
		t.Fatalf("unmarshal claude: %v", err)
	}
	ids := make(map[string]bool)
	for _, m := range claudeResp.Data {
		ids[m.ID] = true
	}
	// Claude 协议入口接受 Gemini / OpenAI 渠道转换，两条策略的兜底都可达。
	if !ids["auto/claude"] || !ids["auto/gemini"] || ids["auto/off"] {
		t.Fatalf("Claude 列表应含两条启用策略，实际 %v", ids)
	}
}

// A signed-in user's available models include their personal policies as policy/<name>, listed for
// their group when the fallback routes there; platform policies are not repeated.
func TestAppendOwnRoutingPolicyAvailableModels(t *testing.T) {
	defer setupProtocolModelGroup(t)()
	defer setupRoutingPolicies(t)()
	prev := config.UserRoutingPolicyEnabled
	config.UserRoutingPolicyEnabled = true
	defer func() { config.UserRoutingPolicyEnabled = prev }()
	routes := []model.RoutingRoute{{Label: "small", TargetModel: "openai-only"}, {Label: "large", TargetModel: "claude-model"}}
	for _, p := range []*model.RoutingPolicy{
		{UserId: 7, Name: "mine", Enabled: true, DecisionModel: "openai-only", Routes: routes, FallbackModel: "claude-model", Threshold: 0.5},
		{UserId: 8, Name: "theirs", Enabled: true, DecisionModel: "openai-only", Routes: routes, FallbackModel: "claude-model", Threshold: 0.5},
	} {
		if err := model.DB.Create(p).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatal(err)
	}

	listed := map[string]*AvailableModelResponse{}
	appendOwnRoutingPolicyAvailableModels(listed, model.PersonalRoutingScope(7), "g")
	if len(listed) != 1 || listed["policy/mine"] == nil {
		t.Fatalf("want only policy/mine, got %v", listed)
	}
	if got := listed["policy/mine"]; got.Kind != model.ModelInfoKindRoutingPolicy || got.Routing == nil || got.Groups[0] != "g" {
		t.Fatalf("policy/mine should be a routing policy entry for group g: %+v", got)
	}

	// The fallback does not route in the group: not listed.
	listed = map[string]*AvailableModelResponse{}
	appendOwnRoutingPolicyAvailableModels(listed, model.PersonalRoutingScope(7), "openai-grp")
	if len(listed) != 0 {
		t.Fatalf("an unroutable fallback hides the policy, got %v", listed)
	}
}
