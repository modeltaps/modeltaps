package controller

import (
	"errors"
	"reflect"
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// 渠道 models 全部存在于上游列表时，结果应为 ok、无缺失。
func TestComputeModelDrift_AllPresent(t *testing.T) {
	got := computeModelDrift("gpt-4o,claude-3.7-sonnet", "", []string{"gpt-4o", "claude-3.7-sonnet", "gpt-4o-mini"}, nil)
	if !got.OK {
		t.Fatalf("expected ok=true, got %#v", got)
	}
	if len(got.MissingModels) != 0 {
		t.Fatalf("expected no missing, got %v", got.MissingModels)
	}
	if got.CheckedAt == 0 {
		t.Fatalf("expected checked_at to be set")
	}
}

// 部分模型上游已下架时，缺失列表应精确列出渠道侧名称，且 ok=false。
func TestComputeModelDrift_PartialMissing(t *testing.T) {
	got := computeModelDrift(" gpt-4o , claude-3.5-haiku ,", "", []string{"gpt-4o"}, nil)
	if got.OK {
		t.Fatalf("expected ok=false, got %#v", got)
	}
	want := []string{"claude-3.5-haiku"}
	if !reflect.DeepEqual(got.MissingModels, want) {
		t.Fatalf("missing mismatch: want %v got %v", want, got.MissingModels)
	}
}

// 渠道 models 是对外别名时，应按 model_mapping 解析到的上游真实名对比，避免误报。
func TestComputeModelDrift_ModelMapping(t *testing.T) {
	mapping := `{"haiku":"anthropic/claude-3.5-haiku","sonnet":"anthropic/claude-3.7-sonnet"}`
	// 上游仍有 sonnet 的真实名，但 haiku 的真实名已下架 -> 只报 haiku（渠道侧别名）。
	got := computeModelDrift("haiku,sonnet", mapping, []string{"anthropic/claude-3.7-sonnet"}, nil)
	if got.OK {
		t.Fatalf("expected ok=false, got %#v", got)
	}
	want := []string{"haiku"}
	if !reflect.DeepEqual(got.MissingModels, want) {
		t.Fatalf("missing mismatch: want %v got %v", want, got.MissingModels)
	}
}

// 上游拉取失败时不得写入结果（返回 shouldPersist=false），避免把网络/鉴权错误误报为漂移。
func TestEvaluateDrift_FetchErrorSkips(t *testing.T) {
	result, ok := evaluateDrift("gpt-4o", "", nil, errors.New("upstream unavailable"), nil)
	if ok {
		t.Fatalf("expected shouldPersist=false on fetch error")
	}
	if result != nil {
		t.Fatalf("expected nil result on fetch error, got %#v", result)
	}
}

// provider 不支持模型列表能力时同样跳过，不写入误报。
func TestEvaluateDrift_NotSupportedSkips(t *testing.T) {
	result, ok := evaluateDrift("gpt-4o", "", nil, ErrModelListNotSupported, nil)
	if ok || result != nil {
		t.Fatalf("expected skip on ErrModelListNotSupported, got ok=%v result=%#v", ok, result)
	}
}

// 拉取成功时应写入结果。
func TestEvaluateDrift_SuccessPersists(t *testing.T) {
	result, ok := evaluateDrift("gpt-4o", "", []string{"gpt-4o"}, nil, nil)
	if !ok {
		t.Fatalf("expected shouldPersist=true on success")
	}
	if result == nil || !result.OK {
		t.Fatalf("expected ok result, got %#v", result)
	}
}

// 首次检测（无基线快照）：只落上游快照建立基线，不产生任何「新增」，OK 仍按缺失判定。
func TestComputeModelDrift_FirstRunOnlySnapshots(t *testing.T) {
	upstream := []string{"gpt-4o", "gpt-4o-mini", "o3"}
	got := computeModelDrift("gpt-4o", "", upstream, nil)
	if len(got.NewModels) != 0 {
		t.Fatalf("expected no new models on first run, got %v", got.NewModels)
	}
	if !reflect.DeepEqual(got.UpstreamSnapshot, upstream) {
		t.Fatalf("snapshot mismatch: want %v got %v", upstream, got.UpstreamSnapshot)
	}
	if !got.OK {
		t.Fatalf("expected ok=true, got %#v", got)
	}
}

// 第二次检测上游多出模型时：仅未配置的新增进入 NewModels，OK 不受新增影响。
func TestComputeModelDrift_NewModelsAgainstSnapshot(t *testing.T) {
	prev := &model.ModelDriftResult{UpstreamSnapshot: []string{"gpt-4o", "gpt-4o-mini"}}
	got := computeModelDrift("gpt-4o", "", []string{"gpt-4o", "gpt-4o-mini", "o3", "o3-mini"}, prev)
	want := []string{"o3", "o3-mini"}
	if !reflect.DeepEqual(got.NewModels, want) {
		t.Fatalf("new models mismatch: want %v got %v", want, got.NewModels)
	}
	if !got.OK {
		t.Fatalf("upstream additions must not mark drift: %#v", got)
	}
	if len(got.MissingModels) != 0 {
		t.Fatalf("expected no missing, got %v", got.MissingModels)
	}
}

// 管理员把新增模型加入渠道 models（或经 model_mapping 映射）后，下次检测应自动从 NewModels 移除。
func TestComputeModelDrift_NewModelsDropWhenConfigured(t *testing.T) {
	prev := &model.ModelDriftResult{
		NewModels:        []string{"o3", "o3-mini"},
		UpstreamSnapshot: []string{"gpt-4o", "o3", "o3-mini"},
	}
	got := computeModelDrift("gpt-4o,o3-alias", `{"o3-alias":"o3"}`, []string{"gpt-4o", "o3", "o3-mini"}, prev)
	want := []string{"o3-mini"}
	if !reflect.DeepEqual(got.NewModels, want) {
		t.Fatalf("new models mismatch: want %v got %v", want, got.NewModels)
	}
}

// 管理员忽略后（NewModels 已清空）快照仍在：同一批模型不会再次被报为新增。
func TestComputeModelDrift_DismissedNotReported(t *testing.T) {
	upstream := []string{"gpt-4o", "o3", "o3-mini"}
	prev := &model.ModelDriftResult{NewModels: nil, UpstreamSnapshot: upstream}
	got := computeModelDrift("gpt-4o", "", upstream, prev)
	if len(got.NewModels) != 0 {
		t.Fatalf("expected dismissed models to stay dismissed, got %v", got.NewModels)
	}
	if !reflect.DeepEqual(got.UpstreamSnapshot, upstream) {
		t.Fatalf("snapshot mismatch: want %v got %v", upstream, got.UpstreamSnapshot)
	}
}

// 上游拉取失败时不写回，既有 NewModels 与快照保持不变（沿用 evaluateDrift 语义）。
func TestEvaluateDrift_FetchErrorKeepsNewModels(t *testing.T) {
	prev := &model.ModelDriftResult{
		NewModels:        []string{"o3"},
		UpstreamSnapshot: []string{"gpt-4o", "o3"},
	}
	result, ok := evaluateDrift("gpt-4o", "", nil, errors.New("upstream unavailable"), prev)
	if ok || result != nil {
		t.Fatalf("expected skip on fetch error, got ok=%v result=%#v", ok, result)
	}
	if !reflect.DeepEqual(prev.NewModels, []string{"o3"}) {
		t.Fatalf("prev result must not be mutated, got %v", prev.NewModels)
	}
}

// 上游已不再返回的模型不应继续留在 NewModels（避免陈旧标记）。
func TestComputeModelDrift_StaleNewModelsPruned(t *testing.T) {
	prev := &model.ModelDriftResult{
		NewModels:        []string{"o3", "o3-mini"},
		UpstreamSnapshot: []string{"gpt-4o", "o3", "o3-mini"},
	}
	got := computeModelDrift("gpt-4o", "", []string{"gpt-4o", "o3"}, prev)
	want := []string{"o3"}
	if !reflect.DeepEqual(got.NewModels, want) {
		t.Fatalf("new models mismatch: want %v got %v", want, got.NewModels)
	}
}
