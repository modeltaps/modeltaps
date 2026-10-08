package controller

import (
	"reflect"
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// 同步后，渠道里配置了但没匹配到上游价格的模型应被列出，给用户可见提示
// （常见原因：模型 id 带版本/别名，与上游精确 id 不符）。
func TestFindUnpricedChannelModels(t *testing.T) {
	prices := []*model.Price{
		{Model: "openai/gpt-4o"},
		{Model: "anthropic/claude-3.7-sonnet"},
	}
	got := findUnpricedChannelModels("openai/gpt-4o, anthropic/claude-3.5-sonnet , google/gemini-x", prices)
	want := []string{"anthropic/claude-3.5-sonnet", "google/gemini-x"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	if r := findUnpricedChannelModels("", prices); len(r) != 0 {
		t.Errorf("empty csv should yield none, got %v", r)
	}
	if r := findUnpricedChannelModels("openai/gpt-4o", prices); len(r) != 0 {
		t.Errorf("all matched should yield none, got %v", r)
	}
}
