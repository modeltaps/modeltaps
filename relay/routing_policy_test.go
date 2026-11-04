package relay

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestFirstUserMessageText 四种请求体形状都取到第一条用户消息的文本，忽略图片 / 工具块，剥系统提醒，按字符截断
func TestFirstUserMessageText(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"chat string content", `{"model":"x","messages":[{"role":"system","content":"sys"},{"role":"user","content":"fix the bug"},{"role":"user","content":"later"}]}`, "fix the bug"},
		{"chat text parts", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:..."}},{"type":"text","text":"what is"},{"type":"text","text":"this"}]}]}`, "what is\nthis"},
		{"claude skips tool_result only message", `{"messages":[{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":"ok"}]},{"role":"assistant","content":"a"},{"role":"user","content":[{"type":"text","text":"now refactor"}]}]}`, "now refactor"},
		{"responses string input", `{"model":"x","input":"summarize this"}`, "summarize this"},
		{"responses message items", `{"input":[{"role":"developer","content":"d"},{"role":"user","content":[{"type":"input_image","image_url":"u"},{"type":"input_text","text":"add a flag"}]}]}`, "add a flag"},
		{"gemini contents", `{"contents":[{"role":"user","parts":[{"inline_data":{"data":"x"}},{"text":"translate"}]}]}`, "translate"},
		{"gemini without role", `{"contents":[{"parts":[{"text":"hello"}]}]}`, "hello"},
		{"completions prompt", `{"model":"x","prompt":"once upon"}`, "once upon"},
		{"system reminder stripped", `{"messages":[{"role":"user","content":"<system-reminder>\nlong template\n</system-reminder>\nrename variable <system-reminder>x</system-reminder>"}]}`, "rename variable"},
		{"no user text", `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"u"}}]}]}`, ""},
		{"invalid json", `{not json`, ""},
		{"empty", ``, ""},
	}
	for _, tc := range cases {
		if got := firstUserMessageText([]byte(tc.body), 0); got != tc.want {
			t.Fatalf("%s: 期望 %q，实际 %q", tc.name, tc.want, got)
		}
	}

	long := `{"messages":[{"role":"user","content":"` + strings.Repeat("字", 50) + `"}]}`
	if got := firstUserMessageText([]byte(long), 10); len([]rune(got)) != 10 {
		t.Fatalf("截断应按字符数，实际 %d 个字符", len([]rune(got)))
	}
}

// TestParseDecisionAnswer 接受裸 JSON、带围栏 / 前后缀的 JSON、字符串与百分数形式的置信度；拒绝无标签
func TestParseDecisionAnswer(t *testing.T) {
	good := map[string]struct {
		label string
		conf  float64
	}{
		`{"label":"small","confidence":0.8}`:                                {"small", 0.8},
		"Sure.\n```json\n{\"label\": \"large\", \"confidence\": 0.55}\n```": {"large", 0.55},
		`{"label":"medium","confidence":"0.7"}`:                             {"medium", 0.7},
		`{"label":"medium","confidence":70}`:                                {"medium", 0.7},
		`{"label":"medium","confidence":"65%"}`:                             {"medium", 0.65},
		`{"label":"medium"}`:                                                {"medium", 0},
	}
	for content, want := range good {
		got, err := parseDecisionAnswer(content)
		if err != nil {
			t.Fatalf("%q: 不应报错: %v", content, err)
		}
		if got.Label != want.label || got.Confidence != want.conf {
			t.Fatalf("%q: 期望 %s/%.2f，实际 %s/%.2f", content, want.label, want.conf, got.Label, got.Confidence)
		}
	}
	for _, content := range []string{"", "small", `{"confidence":0.9}`, `{"label":}`} {
		if _, err := parseDecisionAnswer(content); err == nil {
			t.Fatalf("%q: 应报错", content)
		}
	}
}

// setupRoutingResolveTest 建一条策略，替换分类函数为可编程的假实现，返回还原函数。
func setupRoutingResolveTest(t *testing.T, policy *model.RoutingPolicy) (calls *int, setResult func(label string, conf float64, err error)) {
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
	if err := policy.Insert(); err != nil {
		t.Fatalf("建策略失败: %v", err)
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatalf("加载索引失败: %v", err)
	}

	count := 0
	var (
		resultLabel string
		resultConf  float64
		resultErr   error
	)
	prevClassify := classifyRoute
	classifyRoute = func(c *gin.Context, p *model.RoutingPolicy, text string) (*ClassifyResult, error) {
		count++
		if resultErr != nil {
			return nil, resultErr
		}
		return &ClassifyResult{Label: resultLabel, Confidence: resultConf}, nil
	}
	prevRedis := config.RedisEnabled
	config.RedisEnabled = false
	model.ClearStickyMemory()
	t.Cleanup(func() {
		model.ClearStickyMemory()
		classifyRoute = prevClassify
		config.RedisEnabled = prevRedis
		model.DB = prevDB
		model.ResetRoutingPolicyCache()
	})
	return &count, func(label string, conf float64, err error) { resultLabel, resultConf, resultErr = label, conf, err }
}

func testPolicy(name string, pinTTL int) *model.RoutingPolicy {
	return &model.RoutingPolicy{
		Name: name, Enabled: true, DecisionModel: "decider",
		Routes: []model.RoutingRoute{
			{Label: "small", TargetModel: "cheap-model"},
			{Label: "large", TargetModel: "big-model"},
		},
		FallbackModel: "fallback-model", Threshold: 0.5, PinTTLSeconds: pinTTL,
	}
}

// newRoutingRequest 构造带请求体与会话头的 gin context。
func newRoutingRequest(sessionID, body string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	if sessionID != "" {
		c.Request.Header.Set("session_id", sessionID)
	}
	c.Set("token_id", 7)
	c.Set(config.GinRequestBodyKey, []byte(body))
	return c
}

const routingBody = `{"model":"auto/t","messages":[{"role":"user","content":"add a log line"}]}`

func decisionOf(t *testing.T, c *gin.Context) *RoutingDecision {
	t.Helper()
	d, ok := utils.GetGinValue[*RoutingDecision](c, RoutingDecisionContextKey)
	if !ok || d == nil {
		t.Fatalf("context 里应有决策")
	}
	return d
}

// TestResolveRoutingPolicy_Outcomes 分类过阈值走目标；低于阈值 / 出错 / 答非所问 / 空输入走兜底，原因各异
func TestResolveRoutingPolicy_Outcomes(t *testing.T) {
	calls, setResult := setupRoutingResolveTest(t, testPolicy("auto/t", 0))

	setResult("small", 0.9, nil)
	c := newRoutingRequest("", routingBody)
	if got := resolveRoutingPolicy(c, "auto/t"); got != "cheap-model" {
		t.Fatalf("过阈值应走目标，实际 %s", got)
	}
	d := decisionOf(t, c)
	if d.Reason != RoutingReasonClassified || d.Label != "small" || d.Confidence != 0.9 {
		t.Fatalf("决策字段不对: %+v", d)
	}
	if c.GetString("routing_policy") != "auto/t" || c.GetString("requested_model") != "auto/t" {
		t.Fatalf("应记录策略名与 requested_model")
	}

	setResult("large", 0.3, nil)
	c = newRoutingRequest("", routingBody)
	if got := resolveRoutingPolicy(c, "auto/t"); got != "fallback-model" {
		t.Fatalf("低于阈值应走兜底，实际 %s", got)
	}
	if d := decisionOf(t, c); d.Reason != RoutingReasonBelowThreshold || d.Label != "large" {
		t.Fatalf("低于阈值应保留标签供日志: %+v", d)
	}

	setResult("", 0, errors.New("boom"))
	c = newRoutingRequest("", routingBody)
	if got := resolveRoutingPolicy(c, "auto/t"); got != "fallback-model" {
		t.Fatalf("出错应走兜底，实际 %s", got)
	}
	if d := decisionOf(t, c); d.Reason != RoutingReasonDecisionError {
		t.Fatalf("出错原因不对: %+v", d)
	}

	setResult("unknown", 0.99, nil)
	c = newRoutingRequest("", routingBody)
	if got := resolveRoutingPolicy(c, "auto/t"); got != "fallback-model" {
		t.Fatalf("答非所问应走兜底，实际 %s", got)
	}
	if d := decisionOf(t, c); d.Reason != RoutingReasonDecisionError {
		t.Fatalf("答非所问原因不对: %+v", d)
	}

	before := *calls
	setResult("small", 0.9, nil)
	c = newRoutingRequest("", `{"model":"auto/t","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"u"}}]}]}`)
	if got := resolveRoutingPolicy(c, "auto/t"); got != "fallback-model" {
		t.Fatalf("空输入应走兜底，实际 %s", got)
	}
	if d := decisionOf(t, c); d.Reason != RoutingReasonEmptyInput {
		t.Fatalf("空输入原因不对: %+v", d)
	}
	if *calls != before {
		t.Fatalf("空输入不应调用分类器")
	}

	// 不是策略 / 其它模型名原样返回，不写决策。
	c = newRoutingRequest("", routingBody)
	if got := resolveRoutingPolicy(c, "gpt-5"); got != "gpt-5" {
		t.Fatalf("普通模型应原样返回")
	}
	if _, ok := c.Get(RoutingDecisionContextKey); ok {
		t.Fatalf("普通模型不应写决策")
	}
}

// TestResolveRoutingPolicy_DisabledPolicyPassesThrough 停用的策略当普通模型名处理（之后自然 model_not_found）
func TestResolveRoutingPolicy_DisabledPolicyPassesThrough(t *testing.T) {
	p := testPolicy("auto/off", 0)
	p.Enabled = false
	calls, _ := setupRoutingResolveTest(t, p)
	c := newRoutingRequest("", routingBody)
	if got := resolveRoutingPolicy(c, "auto/off"); got != "auto/off" || *calls != 0 {
		t.Fatalf("停用策略应原样返回且不分类，实际 %s calls=%d", got, *calls)
	}
}

// TestResolveRoutingPolicy_CachedWithinRequest 同一请求（重试）再次解析直接复用，不二次分类
func TestResolveRoutingPolicy_CachedWithinRequest(t *testing.T) {
	calls, setResult := setupRoutingResolveTest(t, testPolicy("auto/t", 0))
	setResult("large", 0.9, nil)
	c := newRoutingRequest("", routingBody)
	first := resolveRoutingPolicy(c, "auto/t")
	// 重试时 relay 已释放请求体，决策必须来自 context 缓存。
	c.Set(config.GinRequestBodyKey, nil)
	setResult("small", 0.9, nil)
	second := resolveRoutingPolicy(c, "auto/t")
	if first != "big-model" || second != "big-model" {
		t.Fatalf("两次解析应一致为 big-model，实际 %s / %s", first, second)
	}
	if *calls != 1 {
		t.Fatalf("同一请求只应分类一次，实际 %d", *calls)
	}
}

// TestResolveRoutingPolicy_PinsSession 同一会话的后续请求命中固定记录；不同会话各自分类；策略改掉目标后重分类
func TestResolveRoutingPolicy_PinsSession(t *testing.T) {
	calls, setResult := setupRoutingResolveTest(t, testPolicy("auto/pin", 3600))
	setResult("small", 0.9, nil)

	c1 := newRoutingRequest("sess-pin-1", routingBody)
	if got := resolveRoutingPolicy(c1, "auto/pin"); got != "cheap-model" {
		t.Fatalf("首次应分类到 cheap-model，实际 %s", got)
	}

	setResult("large", 0.9, nil)
	c2 := newRoutingRequest("sess-pin-1", `{"model":"auto/pin","messages":[{"role":"user","content":"add a log line"},{"role":"assistant","content":"ok"},{"role":"user","content":"now rewrite everything"}]}`)
	if got := resolveRoutingPolicy(c2, "auto/pin"); got != "cheap-model" {
		t.Fatalf("同会话后续轮次应固定在 cheap-model，实际 %s", got)
	}
	if d := decisionOf(t, c2); d.Reason != RoutingReasonPinned || d.Label != "small" {
		t.Fatalf("应标记为 pinned: %+v", d)
	}
	if *calls != 1 {
		t.Fatalf("固定命中不应再分类，实际 %d 次", *calls)
	}

	c3 := newRoutingRequest("sess-pin-2", routingBody)
	if got := resolveRoutingPolicy(c3, "auto/pin"); got != "big-model" || *calls != 2 {
		t.Fatalf("新会话应重新分类到 big-model，实际 %s calls=%d", got, *calls)
	}

	// 管理员把 small 的目标改了：旧固定记录失效，重新分类并按新目标固定。
	p := model.LookupRoutingPolicy("auto/pin")
	p.Routes[0].TargetModel = "cheaper-model"
	if err := p.Update(); err != nil {
		t.Fatalf("更新策略失败: %v", err)
	}
	setResult("small", 0.9, nil)
	c4 := newRoutingRequest("sess-pin-1", routingBody)
	if got := resolveRoutingPolicy(c4, "auto/pin"); got != "cheaper-model" || *calls != 3 {
		t.Fatalf("目标变更后应重分类到 cheaper-model，实际 %s calls=%d", got, *calls)
	}
	if d := decisionOf(t, c4); d.Reason != RoutingReasonClassified {
		t.Fatalf("重分类原因不对: %+v", d)
	}
}

// TestBuildDecisionMessages 提示里带问题、每个标签及描述、用户文本
func TestBuildDecisionMessages(t *testing.T) {
	p := testPolicy("auto/t", 0)
	p.Question = "How big is the change?"
	p.Routes[0].Description = "one-line additions"
	msgs := buildDecisionMessages(p, "add a flag")
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Fatalf("应是 system + user 两条消息: %+v", msgs)
	}
	user := msgs[1].StringContent()
	for _, want := range []string{"How big is the change?", "- small: one-line additions", "- large", "add a flag"} {
		if !strings.Contains(user, want) {
			t.Fatalf("用户消息缺少 %q:\n%s", want, user)
		}
	}
}

// TestTryRoutingPolicyHandler 「试一试」对草稿策略逐个决策模型跑分类，不落库、不写固定记录；空文本直接兜底
func TestTryRoutingPolicyHandler(t *testing.T) {
	calls, setResult := setupRoutingResolveTest(t, testPolicy("auto/saved", 3600))
	setResult("large", 0.8, nil)

	body := `{"policy":{"name":"","decision_model":"decider","routes":[{"label":"small","target_model":"cheap-model"},{"label":"large","target_model":"big-model"}],"fallback_model":"fallback-model","threshold":0.5,"pin_ttl_seconds":3600},"text":"<system-reminder>x</system-reminder>rewrite everything","decision_models":["d1","d2"]}`
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/api/routing_policy/try", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("id", 1)
	c.Set("group", "default")
	TryRoutingPolicyHandler(c)
	if rec.Code != 200 {
		t.Fatalf("期望 200，实际 %d body=%s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	for _, want := range []string{`"decision_model":"d1"`, `"decision_model":"d2"`, `"label":"large"`, `"target_model":"big-model"`, `"reason":"classified"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("响应缺少 %s: %s", want, out)
		}
	}
	if *calls != 2 {
		t.Fatalf("两个候选模型应各分类一次，实际 %d", *calls)
	}
	if model.LookupRoutingPolicy("draft") != nil {
		t.Fatalf("草稿不应落库")
	}

	// 校验不过（只有一条路由）→ 400 带 code。
	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest("POST", "/api/routing_policy/try", strings.NewReader(`{"policy":{"decision_model":"d","routes":[{"label":"a","target_model":"m"}],"fallback_model":"m"},"text":"hi"}`))
	c.Request.Header.Set("Content-Type", "application/json")
	TryRoutingPolicyHandler(c)
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "routing_policy_routes_count") {
		t.Fatalf("校验失败应 400 带 code，实际 %d %s", rec.Code, rec.Body.String())
	}
}
