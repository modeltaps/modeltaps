package e2e

// 自动路由的打分方式：logprobs 打分、auto 在上游不给 logprobs 时改用自报、logprobs 模式不支持时兜底。

import (
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

type letterLogprob struct {
	token string
	prob  float64
}

// letterJSON 决策上游按选项字母作答，第一个 token 带 top_logprobs。
func letterJSON(answer string, tops ...letterLogprob) map[string]any {
	resp := ChatCompletionJSON(routingDecisionName, answer, 30, 1)
	top := make([]any, 0, len(tops))
	for _, t := range tops {
		top = append(top, map[string]any{"token": t.token, "logprob": math.Log(t.prob)})
	}
	choice := resp["choices"].([]any)[0].(map[string]any)
	choice["logprobs"] = map[string]any{
		"content": []any{map[string]any{"token": answer, "logprob": math.Log(tops[0].prob), "top_logprobs": top}},
	}
	return resp
}

// logprobs 打分：一次决策调用，请求带 logprobs，概率按选项字母合并归一；日志与响应头记打分方式。
func TestAutoRoutingLogprobsScoring(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: letterJSON("A",
		letterLogprob{"A", 0.8}, letterLogprob{"B", 0.1}, letterLogprob{" A", 0.05}, letterLogprob{"Sure", 0.05})})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingCheapModel, "done", 100, 50)})
	h := newRoutingHarnessWithScoring(t, decision.Server.URL, target.Server.URL, 0, model.RoutingScoringAuto)
	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(decision.Requests()) != 1 {
		t.Fatalf("应只有一次决策调用，实际 %d", len(decision.Requests()))
	}
	req := decision.LastRequest(t).JSONBody(t)
	if req["logprobs"] != true || req["top_logprobs"] != float64(20) || req["max_tokens"] != float64(16) {
		t.Fatalf("决策请求应带 logprobs / top_logprobs=20 / max_tokens=16: %v", req)
	}
	msgs := req["messages"].([]any)
	prompt := msgs[len(msgs)-1].(map[string]any)["content"].(string)
	if !strings.Contains(prompt, "A. small: additive change") || !strings.Contains(prompt, "B. large: cross-module change") {
		t.Fatalf("决策提示应把标签编成选项字母:\n%s", prompt)
	}
	if target.LastRequest(t).JSONBody(t)["model"] != routingCheapModel {
		t.Fatalf("应路由到 cheap-model")
	}
	// A 与 " A" 合并 0.85，覆盖 0.95，归一后 0.8947。
	if got := w.Header().Get("X-Modeltaps-Route"); got != "policy=auto/coder; label=small; model=cheap-model; confidence=0.89; scoring=logprobs; reason=classified" {
		t.Fatalf("X-Modeltaps-Route 不对: %q", got)
	}
	meta := consumeLogsByModel(t)[routingCheapModel].Metadata.Data()
	if meta["routing_scoring"] != model.RoutingScoringLogprobs {
		t.Fatalf("日志应记 routing_scoring=logprobs: %v", meta)
	}
	if cov, _ := meta["routing_coverage"].(float64); cov != 0.95 {
		t.Fatalf("routing_coverage 期望 0.95，实际 %v", meta["routing_coverage"])
	}
	probs, _ := meta["routing_probabilities"].([]any)
	if len(probs) != 2 {
		t.Fatalf("应记两个标签的概率: %v", meta["routing_probabilities"])
	}
	first := probs[0].(map[string]any)
	if first["label"] != "small" || first["probability"] != 0.8947 {
		t.Fatalf("最高概率标签不对: %v", first)
	}
	// 决策调用按 decider 价 31 token。
	if spent := quotaBefore - h.UserQuota(t); spent != 150+31 {
		t.Fatalf("应扣 150 + 31，实际 %d", spent)
	}
}

// auto：上游不返回 logprobs → 同一请求改用自报再调一次（两次都计费），之后的新会话直接自报。
func TestAutoRoutingAutoScoringFallsBackToSelfReported(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingCheapModel, "done", 100, 50)})
	h := newRoutingHarnessWithScoring(t, decision.Server.URL, target.Server.URL, 0, model.RoutingScoringAuto)
	quotaBefore := h.UserQuota(t)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	reqs := decision.Requests()
	if len(reqs) != 2 {
		t.Fatalf("首个会话应先试 logprobs 再自报，共两次决策调用，实际 %d", len(reqs))
	}
	if reqs[0].JSONBody(t)["logprobs"] != true || reqs[1].JSONBody(t)["logprobs"] != nil {
		t.Fatalf("第一次应带 logprobs、第二次不带")
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "policy=auto/coder; label=small; model=cheap-model; confidence=0.90; scoring=self_reported; reason=classified" {
		t.Fatalf("X-Modeltaps-Route 不对: %q", got)
	}
	if spent := quotaBefore - h.UserQuota(t); spent != 150+28*2 {
		t.Fatalf("两次决策调用都应计费：150 + 28×2，实际 %d", spent)
	}

	// 新会话：已记住不支持，只调一次且不带 logprobs。
	w = h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "rename a helper"))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	reqs = decision.Requests()
	if len(reqs) != 3 || reqs[2].JSONBody(t)["logprobs"] != nil {
		t.Fatalf("新会话应直接自报一次，实际共 %d 次", len(reqs))
	}
}

// logprobs 模式：上游不给 logprobs 就兜底，不改用自报。
func TestAutoRoutingLogprobsOnlyFallsBackWithoutSupport(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: decisionJSON("small", 0.9)})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingBigModel, "done", 100, 50)})
	h := newRoutingHarnessWithScoring(t, decision.Server.URL, target.Server.URL, 0, model.RoutingScoringLogprobs)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if len(decision.Requests()) != 1 {
		t.Fatalf("logprobs 模式不应再自报，实际 %d 次", len(decision.Requests()))
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "policy=auto/coder; model=big-model; reason=decision_error" {
		t.Fatalf("X-Modeltaps-Route 不对: %q", got)
	}
}

// 选项字母的概率之和太低（模型在回别的）→ 决策失败兜底。
func TestAutoRoutingLogprobsLowCoverageFallsBack(t *testing.T) {
	decision := NewFakeProvider(t)
	decision.SetResponse(FakeResponse{JSON: letterJSON("Sure",
		letterLogprob{"Sure", 0.9}, letterLogprob{"A", 0.05}, letterLogprob{"B", 0.05})})
	target := NewFakeProvider(t)
	target.SetResponse(FakeResponse{JSON: ChatCompletionJSON(routingBigModel, "done", 100, 50)})
	h := newRoutingHarnessWithScoring(t, decision.Server.URL, target.Server.URL, 0, model.RoutingScoringAuto)

	w := h.Post("/v1/chat/completions", chatBodyFor(routingPolicyName, "add a log line"))
	if w.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Modeltaps-Route"); got != "policy=auto/coder; model=big-model; reason=decision_error" {
		t.Fatalf("X-Modeltaps-Route 不对: %q", got)
	}
	if len(decision.Requests()) != 1 {
		t.Fatalf("带了 logprobs 的答案不应再改用自报，实际 %d 次", len(decision.Requests()))
	}
}
