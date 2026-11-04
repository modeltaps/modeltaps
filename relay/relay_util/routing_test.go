package relay_util

import "testing"

// TestRoutingDecisionHeaderAndMeta 响应头与日志字段按原因取舍：分类类带置信度，固定 / 兜底不带；无标签时省略 label
func TestRoutingDecisionHeaderAndMeta(t *testing.T) {
	classified := &RoutingDecision{Policy: "auto/c", Label: "small", TargetModel: "cheap", Confidence: 0.876, Reason: RoutingReasonClassified, DecisionMs: 120}
	if got := classified.HeaderValue(); got != "policy=auto/c; label=small; model=cheap; confidence=0.88; reason=classified" {
		t.Fatalf("classified header: %q", got)
	}
	meta := map[string]any{}
	classified.LogMeta(meta)
	if meta["routing_policy"] != "auto/c" || meta["routing_label"] != "small" || meta["routing_confidence"] != 0.876 || meta["routing_decision_ms"] != int64(120) || meta["routing_target"] != "cheap" {
		t.Fatalf("classified meta: %v", meta)
	}

	pinned := &RoutingDecision{Policy: "auto/c", Label: "small", TargetModel: "cheap", Reason: RoutingReasonPinned}
	if got := pinned.HeaderValue(); got != "policy=auto/c; label=small; model=cheap; reason=pinned" {
		t.Fatalf("pinned header: %q", got)
	}
	meta = map[string]any{}
	pinned.LogMeta(meta)
	if _, ok := meta["routing_confidence"]; ok {
		t.Fatalf("pinned 不应带置信度: %v", meta)
	}
	if _, ok := meta["routing_decision_ms"]; ok {
		t.Fatalf("pinned 不应带决策耗时: %v", meta)
	}

	fallback := &RoutingDecision{Policy: "auto/c", TargetModel: "big", Reason: RoutingReasonDecisionError, DecisionMs: 3000}
	if got := fallback.HeaderValue(); got != "policy=auto/c; model=big; reason=decision_error" {
		t.Fatalf("fallback header: %q", got)
	}
	meta = map[string]any{}
	fallback.LogMeta(meta)
	if _, ok := meta["routing_label"]; ok {
		t.Fatalf("无标签不应写 routing_label: %v", meta)
	}
	if meta["routing_decision_ms"] != int64(3000) {
		t.Fatalf("超时也应记耗时: %v", meta)
	}

	below := &RoutingDecision{Policy: "auto/c", Label: "large", TargetModel: "big", Confidence: 0.3, Reason: RoutingReasonBelowThreshold}
	if got := below.HeaderValue(); got != "policy=auto/c; label=large; model=big; confidence=0.30; reason=below_threshold" {
		t.Fatalf("below threshold header: %q", got)
	}
}
