package relay

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
)

// 令牌开启模型限制但列表为空时请求一律被拒，/v1/models 也必须返回空，而不是全部模型。
func TestFilterModelsByTokenLimit_EnabledEmptyListReturnsNone(t *testing.T) {
	c := newTestContext()
	setting := &model.TokenSetting{}
	setting.Limits.LimitModelSetting.Enabled = true
	c.Set("token_setting", setting)

	all := []string{"gpt-4o", "claude-sonnet"}
	if got := filterModelsByTokenLimit(c, all); len(got) != 0 {
		t.Fatalf("开启限制且列表为空时应返回空列表, got %v", got)
	}
	if err := CheckLimitModel(c, "gpt-4o"); err == nil || !IsModelNotFound(err) {
		t.Fatalf("开启限制且列表为空时请求应按 model_not_found 拒绝（列表与请求口径一致）, got %v", err)
	}

	setting.Limits.LimitModelSetting.Enabled = false
	if got := filterModelsByTokenLimit(c, all); len(got) != 2 {
		t.Fatalf("未开启限制时应原样返回, got %v", got)
	}
}

func stubOrgGuardrail(t *testing.T, fn func(shadowUserId, memberUserId int) (*model.OrgMemberGuardrail, error)) {
	t.Helper()
	orig := cacheGetOrgMemberGuardrailFn
	cacheGetOrgMemberGuardrailFn = fn
	t.Cleanup(func() { cacheGetOrgMemberGuardrailFn = orig })
}

func orgTokenContext(shadowUserId, createdBy int) *gin.Context {
	c := newTestContext()
	c.Set("id", shadowUserId)
	c.Set("token_created_by", createdBy)
	return c
}

// 组织令牌的模型列表按成员生效白名单过滤，匹配规则与入场校验同源（不区分大小写、* 前缀）。
func TestFilterModelsByOrgGuardrail(t *testing.T) {
	var loaded []int
	stubOrgGuardrail(t, func(shadowUserId, memberUserId int) (*model.OrgMemberGuardrail, error) {
		loaded = append(loaded, shadowUserId, memberUserId)
		return &model.OrgMemberGuardrail{ModelWhitelist: []string{"GPT-4*", "claude-sonnet"}}, nil
	})

	all := []string{"gpt-4o", "gpt-4.1", "gpt-3.5", "claude-sonnet", "claude-opus"}

	got, err := filterModelsByOrgGuardrail(orgTokenContext(100, 5), all)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"gpt-4o", "gpt-4.1", "claude-sonnet"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if !reflect.DeepEqual(loaded, []int{100, 5}) {
		t.Fatalf("应按 (影子账户, 创建成员) 加载守护数据, got %v", loaded)
	}
	for _, m := range got {
		if !(&model.OrgMemberGuardrail{ModelWhitelist: []string{"GPT-4*", "claude-sonnet"}}).ModelAllowed(m) {
			t.Fatalf("列出的模型 %s 在入场校验时会被拒绝", m)
		}
	}

	// 个人令牌（created_by 为 0 或等于 user_id）不加载守护数据，原样返回
	loaded = nil
	for _, c := range []*gin.Context{orgTokenContext(7, 0), orgTokenContext(7, 7)} {
		got, err := filterModelsByOrgGuardrail(c, all)
		if err != nil || len(got) != len(all) {
			t.Fatalf("个人令牌应原样返回, got %v err %v", got, err)
		}
	}
	if loaded != nil {
		t.Fatal("个人令牌不应加载组织守护数据")
	}
}

// 守护数据加载失败时列表接口 fail-closed 返回 503，不列出全部模型。
func TestListModelsByToken_OrgGuardrailFailClosed(t *testing.T) {
	defer setupProtocolModelGroup(t)()
	stubOrgGuardrail(t, func(int, int) (*model.OrgMemberGuardrail, error) {
		return nil, errors.New("db down")
	})

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set("token_group", "g")
	c.Set("id", 100)
	c.Set("token_created_by", 5)
	ListModelsByToken(c)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 (body=%s)", rec.Code, rec.Body.String())
	}
}

// 三个列表接口都套用组织成员白名单。
func TestListEndpoints_ApplyOrgGuardrail(t *testing.T) {
	defer setupCatalogModels(t)()
	defer setupProtocolModelGroup(t)()
	setEnforceHidden(t, false)
	stubOrgGuardrail(t, func(int, int) (*model.OrgMemberGuardrail, error) {
		return &model.OrgMemberGuardrail{ModelWhitelist: []string{"openai-only", "gemini-model"}}, nil
	})

	call := func(handler gin.HandlerFunc) []byte {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		c.Set("token_group", "g")
		c.Set("id", 100)
		c.Set("token_created_by", 5)
		handler(c)
		if rec.Code != http.StatusOK {
			t.Fatalf("status = %d (body=%s)", rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}

	var openaiResp struct {
		Data []OpenAIModels `json:"data"`
	}
	if err := json.Unmarshal(call(ListModelsByToken), &openaiResp); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(openaiResp.Data))
	for _, m := range openaiResp.Data {
		ids = append(ids, m.Id)
	}
	sort.Strings(ids)
	if !reflect.DeepEqual(ids, []string{"gemini-model", "openai-only"}) {
		t.Fatalf("/v1/models got %v", ids)
	}

	var claudeResp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(call(ListClaudeModelsByToken), &claudeResp); err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"openai-only": true, "gemini-model": true}
	for _, m := range claudeResp.Data {
		if !allowed[m.ID] {
			t.Fatalf("Claude 列表出现白名单外的模型 %s, got %v", m.ID, claudeResp.Data)
		}
	}

	var geminiResp struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(call(ListGeminiModelsByToken), &geminiResp); err != nil {
		t.Fatal(err)
	}
	if len(geminiResp.Models) == 0 {
		t.Fatal("gemini-model 在白名单内，Gemini 列表不应为空")
	}
	for _, m := range geminiResp.Models {
		if !allowed[strings.TrimPrefix(m.Name, "models/")] {
			t.Fatalf("Gemini 列表出现白名单外的模型 %s, got %v", m.Name, geminiResp.Models)
		}
	}
}

// 令牌开启限制且列表为空时，/v1/models 返回空数组（不是 null），客户端按数组解析不会出错。
func TestListModelsByToken_EmptyListEncodesArray(t *testing.T) {
	defer setupProtocolModelGroup(t)()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	c.Set("token_group", "g")
	setting := &model.TokenSetting{}
	setting.Limits.LimitModelSetting.Enabled = true
	c.Set("token_setting", setting)
	ListModelsByToken(c)

	var resp map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if string(resp["data"]) != "[]" {
		t.Fatalf("data = %s, want []", resp["data"])
	}
}

// 令牌无权使用的模型按 model_not_found（404）拒绝，不坍缩成 503「上游繁忙」。
func TestCheckLimitModel_NotAllowedIsModelNotFound(t *testing.T) {
	c := newTestContext()
	setting := &model.TokenSetting{}
	setting.Limits.LimitModelSetting.Enabled = true
	setting.Limits.LimitModelSetting.Models = []string{"gpt-4o"}
	c.Set("token_setting", setting)

	err := CheckLimitModel(c, "claude-sonnet")
	if err == nil || !IsModelNotFound(err) {
		t.Fatalf("白名单外的模型应按 model_not_found 拒绝, got %v", err)
	}
	if err := CheckLimitModel(c, "gpt-4o"); err != nil {
		t.Fatalf("白名单内的模型应放行: %v", err)
	}
}
