package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// 显式指定的 model 优先级最高，即使渠道配置了 TestModel 与廉价模型也用它。
func TestSelectTestModel_ExplicitWins(t *testing.T) {
	ch := &model.Channel{TestModel: "gpt-3.5-turbo", Models: "gpt-4o-mini,gpt-4o"}
	got, err := selectTestModel(ch, "gpt-4o")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %q", got)
	}
}

// 未显式指定时，channel.TestModel 优先于清单/兜底。
func TestSelectTestModel_TestModelWins(t *testing.T) {
	ch := &model.Channel{TestModel: "my-custom-model", Models: "gpt-4o-mini,gpt-4o"}
	got, err := selectTestModel(ch, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "my-custom-model" {
		t.Fatalf("expected my-custom-model, got %q", got)
	}
}

// 清单精确名命中：应选中清单内优先级最高的同名模型，而非 models 存储顺序第一个。
func TestSelectTestModel_CheapExactMatch(t *testing.T) {
	ch := &model.Channel{Models: "gpt-4o,claude-3-opus,gpt-4o-mini"}
	got, err := selectTestModel(ch, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "gpt-4o-mini" {
		t.Fatalf("expected gpt-4o-mini, got %q", got)
	}
}

// 精确名未命中时，按模式匹配（claude haiku / gemini flash）选取。
func TestSelectTestModel_CheapPatternMatch(t *testing.T) {
	ch := &model.Channel{Models: "gpt-4o,claude-3-5-haiku-20241022"}
	got, err := selectTestModel(ch, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "claude-3-5-haiku-20241022" {
		t.Fatalf("expected claude-3-5-haiku-20241022, got %q", got)
	}
}

// 精确名优先于模式：同时存在时应选精确名。
func TestSelectTestModel_ExactBeatsPattern(t *testing.T) {
	ch := &model.Channel{Models: "claude-3-5-haiku-20241022,gpt-4o-mini"}
	got, err := selectTestModel(ch, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "gpt-4o-mini" {
		t.Fatalf("expected gpt-4o-mini, got %q", got)
	}
}

// 清单皆未命中时，兜底为第一个 getModelType()=="chat" 的模型，跳过 embeddings/image 等。
func TestSelectTestModel_FirstChatFallback(t *testing.T) {
	ch := &model.Channel{Models: "text-embedding-3-small,dall-e-3,gpt-4o"}
	got, err := selectTestModel(ch, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %q", got)
	}
}

// models 为空：返回明确中文错误。
func TestSelectTestModel_EmptyModelsError(t *testing.T) {
	ch := &model.Channel{Models: ""}
	_, err := selectTestModel(ch, "")
	if err == nil {
		t.Fatalf("expected error for empty models")
	}
	if err.Error() != "channel has no testable model configured" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

// 无可测 chat 模型（全为 embeddings/image/noSupport）：返回明确中文错误。
func TestSelectTestModel_NoTestableModelError(t *testing.T) {
	ch := &model.Channel{Models: "text-embedding-3-small,dall-e-3,whisper-1"}
	_, err := selectTestModel(ch, "")
	if err == nil {
		t.Fatalf("expected error when no testable chat model")
	}
	if err.Error() != "channel has no testable model configured" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

// 自动兜底跳过 response 类型（o1/o3 等），选中第一个 chat 模型。
func TestSelectTestModel_ExcludesResponseFallback(t *testing.T) {
	ch := &model.Channel{Models: "o1-mini,o3-mini,gpt-4o"}
	got, err := selectTestModel(ch, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "gpt-4o" {
		t.Fatalf("expected gpt-4o, got %q", got)
	}
}

// 全为 response 类型（无 chat 可测）：返回明确中文错误。
func TestSelectTestModel_OnlyResponseError(t *testing.T) {
	ch := &model.Channel{Models: "o1-mini,o3-mini"}
	_, err := selectTestModel(ch, "")
	if err == nil {
		t.Fatalf("expected error when only response-type models")
	}
	if err.Error() != "channel has no testable model configured" {
		t.Fatalf("unexpected error message: %q", err.Error())
	}
}

// TestChannel 响应 model 字段据此返回去掉 # 后缀后的实际模型名。
func TestBaseModelName(t *testing.T) {
	cases := map[string]string{
		"gpt-5#low": "gpt-5",
		"gpt-4o":    "gpt-4o",
		"o1#high":   "o1",
		"a#b#c":     "a",
	}
	for in, want := range cases {
		if got := baseModelName(in); got != want {
			t.Fatalf("baseModelName(%q) = %q, want %q", in, got, want)
		}
	}
}
