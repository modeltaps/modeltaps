package e2e

// 自动路由全链路：请求 model=策略名 → 决策上游分类 → 目标上游收到目标模型名；
// 会话固定、决策失败兜底、Claude 入口与流式各验一遍。决策调用的计费在 ROUTE-3，这里只断言目标请求计费。

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

const (
	routingPolicyName   = "auto/coder"
	routingDecisionName = "decider"
	routingCheapModel   = "cheap-model"
	routingBigModel     = "big-model"
)

// decisionJSON 决策上游的应答：模型只回一段 JSON。
func decisionJSON(label string, confidence float64) map[string]any {
	return ChatCompletionJSON(routingDecisionName, `{"label":"`+label+`","confidence":`+ftoa(confidence)+`}`, 20, 8)
}

func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// newRoutingHarness 两个假上游：决策渠道只提供 decider，目标渠道提供 cheap / big 两个模型；
// 再建一条策略 small→cheap、large→big、兜底 big。
func newRoutingHarness(t *testing.T, decisionURL, targetURL string, channelType int) *Harness {
	t.Helper()
	h := NewHarness(t, HarnessOptions{
		Model: routingCheapModel,
		Channels: []ChannelSpec{
			{Name: "fake-decision", BaseURL: decisionURL, Models: routingDecisionName, Type: channelType},
			{Name: "fake-target", BaseURL: targetURL, Models: routingCheapModel + "," + routingBigModel, Type: channelType},
		},
		Prices: []PriceSpec{
			{Model: routingDecisionName, Input: 1, Output: 1},
			{Model: routingCheapModel, Input: 1, Output: 1},
			{Model: routingBigModel, Input: 10, Output: 10},
		},
	})
	policy := &model.RoutingPolicy{
		Name: routingPolicyName, Enabled: true, DecisionModel: routingDecisionName,
		Question: "How big is the change?",
		Routes: []model.RoutingRoute{
			{Label: "small", Description: "additive change", TargetModel: routingCheapModel},
			{Label: "large", Description: "cross-module change", TargetModel: routingBigModel},
		},
		FallbackModel: routingBigModel, Threshold: 0.5, PinTTLSeconds: 3600,
	}
	if err := policy.Insert(); err != nil {
		t.Fatalf("建策略失败: %v", err)
	}
	if err := model.ReloadRoutingPolicyCache(); err != nil {
		t.Fatalf("加载策略索引失败: %v", err)
	}
	prevRedis := config.RedisEnabled
	config.RedisEnabled = false
	// 固定记录按「令牌 + 策略 + 会话」哈希，各用例的 seed 令牌 id 与提示前缀都一样，先清掉上个用例的。
	model.ClearStickyMemory()
	t.Cleanup(func() {
		config.RedisEnabled = prevRedis
		model.ResetRoutingPolicyCache()
		model.ClearStickyMemory()
	})
	return h
}

func chatBodyFor(modelName, text string) string {
	return `{"model":"` + modelName + `","messages":[{"role":"user","content":"` + text + `"}]}`
}

// 决策过阈值 → 目标上游收到 cheap-model，决策上游收到的是策略的问题与标签而不是用户原始 body；计费按目标模型价。
func TestAutoRoutingChatRoutesToClassifiedTarget(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingCheapModel, "done", 100, 50)})

	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)
	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}

	decisionReq := decision.LastRequest(t).JSONBody(t)
	if decisionReq["model"] != routingDecisionName {
		t.Fatalf("决策上游应收到 decider，实际 %v", decisionReq["model"])
	}
	msgs := decisionReq["messages"].([]any)
	userMsg := msgs[len(msgs)-1].(map[string]any)["content"].(string)
	if !strings.Contains(userMsg, "How big is the change?") || !strings.Contains(userMsg, "- small: additive change") || !strings.Contains(userMsg, "add a log line") {
		t.Fatalf("决策提示内容不对:\n%s", userMsg)
	}
	if decisionReq["stream"] == true {
		t.Fatalf("决策调用不应流式")
	}

	targetReq := target.LastRequest(t).JSONBody(t)
	if targetReq["model"] != routingCheapModel {
		t.Fatalf("目标上游应收到 cheap-model，实际 %v", targetReq["model"])
	}
	if len(target.Requests()) != 1 || len(decision.Requests()) != 1 {
		t.Fatalf("两个上游各应收到一次请求，实际 target=%d decision=%d", len(target.Requests()), len(decision.Requests()))
	}

	// 计费：目标模型 1/1 倍率，150 token → 150；决策调用本任务不计费。
	if spent := quotaBefore - h.UserQuota(t); spent != 150 {
		t.Fatalf("应按目标模型扣 150，实际扣 %d", spent)
	}
	logs := consumeLogs(t)
	if len(logs) != 1 || logs[0].ModelName != routingCheapModel {
		t.Fatalf("应有一条 cheap-model 的消费日志，实际 %+v", logs)
	}
}

// 同一会话（session_id 头）后续请求命中固定记录：决策上游不再被调用，目标不变；换会话重新分类。
func TestAutoRoutingPinsSessionAcrossRequests(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingCheapModel, "done", 10, 5)})
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)

	post := func(session, text string) {
		t.Helper()
		w := h.PostWithHeaders("/v1/chat/completions", chatBodyFor(routingPolicyName, text), map[string]string{"session_id": session})
		if w.Code != http.StatusOK {
			t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
		}
	}

	post("s1", "add a log line")
	decision.SetResponse(FakeResponse{JSON: decisionJSON("large", 0.95)})
	post("s1", "rewrite everything")
	if len(decision.Requests()) != 1 {
		t.Fatalf("同会话第二次请求不应再分类，实际决策上游收到 %d 次", len(decision.Requests()))
	}
	reqs := target.Requests()
	if len(reqs) != 2 || reqs[1].JSONBody(t)["model"] != routingCheapModel {
		t.Fatalf("同会话应固定在 cheap-model，实际 %v", reqs)
	}

	post("s2", "rewrite everything")
	if len(decision.Requests()) != 2 {
		t.Fatalf("新会话应重新分类，实际决策上游收到 %d 次", len(decision.Requests()))
	}
	if last := target.LastRequest(t).JSONBody(t); last["model"] != routingBigModel {
		t.Fatalf("新会话应路由到 big-model，实际 %v", last["model"])
	}
}

// 决策上游报错 → 走兜底模型，请求不失败。
func TestAutoRoutingFallsBackWhenDecisionFails(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{Status: http.StatusInternalServerError, Raw: `{"error":{"message":"down"}}`})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingBigModel, "done", 10, 5)})
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("决策失败也应 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if last := target.LastRequest(t).JSONBody(t); last["model"] != routingBigModel {
		t.Fatalf("应走兜底 big-model，实际 %v", last["model"])
	}
}

// Claude 入口 + 流式：策略名经 Messages 协议同样解析到目标模型。
func TestAutoRoutingClaudeEntryStream(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.8)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{Stream: ChatStreamChunks(routingCheapModel, []string{"hel", "lo"}, 10, 5)})
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, config.ChannelTypeGroq)

	w := h.Post("/claude/v1/messages", `{"model":"`+routingPolicyName+`","max_tokens":64,"stream":true,"messages":[{"role":"user","content":"add a log line"}]}`)
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "message_start") {
		t.Fatalf("应是 Claude 流式应答，实际 body=%s", w.Body.String())
	}
	if last := target.LastRequest(t).JSONBody(t); last["model"] != routingCheapModel {
		t.Fatalf("目标上游应收到 cheap-model，实际 %v", last["model"])
	}
	if len(decision.Requests()) != 1 {
		t.Fatalf("决策上游应被调用一次，实际 %d", len(decision.Requests()))
	}
}

// Responses 入口：策略名经 /v1/responses 同样解析到目标模型，上游收到 Responses 形态的请求。
func TestAutoRoutingResponsesEntry(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("large", 0.7)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: responsesJSON(routingBigModel, "done")})
	h := newRoutingHarness(t, decision.Server.URL, target.Server.URL, 0)

	w := h.Post("/v1/responses", `{"model":"`+routingPolicyName+`","input":"rewrite the whole module"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	last := target.LastRequest(t)
	if last.Path != "/v1/responses" || last.JSONBody(t)["model"] != routingBigModel {
		t.Fatalf("目标上游应在 /v1/responses 收到 big-model，实际 path=%s body=%s", last.Path, string(last.Body))
	}
	if !strings.Contains(decision.LastRequest(t).JSONBody(t)["messages"].([]any)[1].(map[string]any)["content"].(string), "rewrite the whole module") {
		t.Fatalf("决策提示应含 Responses 的 input 文本")
	}
}
