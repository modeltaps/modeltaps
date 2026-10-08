package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"
)

// 有差异：硬编码名单判真（dall-e-3 / gpt-image-1 / gemini-3-pro-image-preview）但未标注 image，
// 以及 model_info 缺失（nil 模态）的情况，都应进入 MissingImage 并触发通知判定。
func TestModelInfoConsistencyMismatch(t *testing.T) {
	summary := computeModelInfoConsistency([]ModelModalities{
		{Model: "dall-e-3", OutputModalities: []string{"text"}},
		{Model: "gpt-image-1", OutputModalities: nil},
		{Model: "gemini-3-pro-image-preview", OutputModalities: []string{}},
		{Model: "gpt-4o", OutputModalities: []string{"text"}},
		{Model: "some-unknown-model", OutputModalities: []string{"Image"}},
	})

	if summary.Checked != 5 {
		t.Fatalf("Checked = %d, want 5", summary.Checked)
	}
	want := []string{"dall-e-3", "gpt-image-1", "gemini-3-pro-image-preview"}
	if len(summary.MissingImage) != len(want) {
		t.Fatalf("MissingImage = %v, want %v", summary.MissingImage, want)
	}
	for i, name := range want {
		if summary.MissingImage[i].Model != name {
			t.Errorf("MissingImage[%d] = %q, want %q", i, summary.MissingImage[i].Model, name)
		}
	}
	// 反向差异只记录，不参与是否通知的判断
	if len(summary.UnexpectedImage) != 1 || summary.UnexpectedImage[0].Model != "some-unknown-model" {
		t.Errorf("UnexpectedImage = %v, want [some-unknown-model]", summary.UnexpectedImage)
	}
}

// 无差异：名单判真的模型都标注了 image（大小写/空白不敏感），非生图模型也未标 image。
// 此时 MissingImage 为空，即便存在反向差异也不应触发通知。
func TestModelInfoConsistencyNoMismatch(t *testing.T) {
	summary := computeModelInfoConsistency([]ModelModalities{
		{Model: "dall-e-3", OutputModalities: []string{"image"}},
		{Model: "imagen-4.0-generate-001", OutputModalities: []string{" IMAGE "}},
		{Model: "gemini-2.5-flash-image-preview", OutputModalities: []string{"text", "image"}},
		{Model: "gpt-4o", OutputModalities: []string{"text"}},
	})

	if len(summary.MissingImage) != 0 {
		t.Fatalf("MissingImage = %v, want empty", summary.MissingImage)
	}
	if len(summary.UnexpectedImage) != 0 {
		t.Fatalf("UnexpectedImage = %v, want empty", summary.UnexpectedImage)
	}
	if len(summary.ModeConflicts) != 0 {
		t.Fatalf("ModeConflicts = %v, want empty", summary.ModeConflicts)
	}
}

// DB 显式 mode 与硬编码名单判定不一致时进入 ModeConflicts；一致的不收录。
func TestModelInfoConsistencyModeConflicts(t *testing.T) {
	summary := computeModelInfoConsistency([]ModelModalities{
		{Model: "dall-e-3", OutputModalities: []string{"image"}, DBMode: model.ModelModeChat},
		{Model: "gemini-2.5-flash-image-preview", OutputModalities: []string{"image"}, DBMode: model.ModelModeChatImage},
		{Model: "gpt-4o", OutputModalities: []string{"text"}, DBMode: model.ModelModeImage},
		{Model: "gpt-image-1", OutputModalities: []string{"image"}, DBMode: model.ModelModeImage},
	})

	want := []ModelModeConflict{
		{Model: "dall-e-3", DBMode: model.ModelModeChat, HardcodedMode: model.ModelModeImage},
		{Model: "gpt-4o", DBMode: model.ModelModeImage, HardcodedMode: model.ModelModeChat},
	}
	if len(summary.ModeConflicts) != len(want) {
		t.Fatalf("ModeConflicts = %v, want %v", summary.ModeConflicts, want)
	}
	for i, w := range want {
		if summary.ModeConflicts[i] != w {
			t.Errorf("ModeConflicts[%d] = %v, want %v", i, summary.ModeConflicts[i], w)
		}
	}
}

// DB mode 为空（未设置）时不收录，且不影响 MissingImage 判定。
func TestModelInfoConsistencyModeUnsetNotReported(t *testing.T) {
	summary := computeModelInfoConsistency([]ModelModalities{
		{Model: "dall-e-3", OutputModalities: []string{"text"}},
		{Model: "gpt-4o", OutputModalities: []string{"text"}},
	})

	if len(summary.ModeConflicts) != 0 {
		t.Fatalf("ModeConflicts = %v, want empty", summary.ModeConflicts)
	}
	if len(summary.MissingImage) != 1 || summary.MissingImage[0].Model != "dall-e-3" {
		t.Errorf("MissingImage = %v, want [dall-e-3]", summary.MissingImage)
	}
}
