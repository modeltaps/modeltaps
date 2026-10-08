package model

import (
	"testing"

	"github.com/spf13/viper"
)

// setupModeTestPricing 用给定的 model_info.mode 构造 PricingInstance；
// mode 为 "-" 表示该模型没有 model_info 行。
func setupModeTestPricing(t *testing.T, prices map[string]string) {
	t.Helper()
	instance := &Pricing{Prices: make(map[string]*Price), Match: make([]string, 0)}
	for name, mode := range prices {
		price := &Price{Model: name}
		if mode != "-" {
			price.ModelInfo = &ModelInfoResponse{Model: name, Mode: mode}
		}
		instance.Prices[name] = price
	}
	old := PricingInstance
	PricingInstance = instance
	t.Cleanup(func() { PricingInstance = old })
}

func setModelInfoModeSwitch(t *testing.T, enabled bool) {
	t.Helper()
	old := viper.GetBool("routing.model_info_mode")
	viper.Set("routing.model_info_mode", enabled)
	t.Cleanup(func() { viper.Set("routing.model_info_mode", old) })
}

// 开关关闭：只走硬编码名单，DB 里的 mode 一律忽略。
func TestResolveModelModeSwitchOff(t *testing.T) {
	setModelInfoModeSwitch(t, false)
	setupModeTestPricing(t, map[string]string{
		"dall-e-3":               ModelModeChat,
		"gemini-2.5-flash-image": ModelModeChat,
		"gpt-4o":                 ModelModeImage,
		"custom-drawer":          ModelModeChatImage,
		"no-row":                 "-",
	})

	cases := []struct {
		model string
		want  string
	}{
		{"dall-e-3", ModelModeImage},
		{"gpt-image-1", ModelModeImage},
		{"imagen-4.0", ModelModeImage},
		{"gemini-2.5-flash-image", ModelModeChatImage},
		{"gemini-3.1-flash-image-preview", ModelModeChatImage},
		{"gpt-4o", ModelModeChat},
		{"custom-drawer", ModelModeChat},
		{"no-row", ModelModeChat},
	}
	for _, c := range cases {
		if got := ResolveModelMode(c.model); got != c.want {
			t.Errorf("ResolveModelMode(%q) = %q, want %q", c.model, got, c.want)
		}
		if got := ModelModeFromInfo(c.model); got != "" {
			t.Errorf("ModelModeFromInfo(%q) = %q, want empty while switch is off", c.model, got)
		}
	}
}

// 开关打开：DB 非空以 DB 为准（含 chat 否决名单）；DB 为空或无行则回落名单。
func TestResolveModelModeSwitchOn(t *testing.T) {
	setModelInfoModeSwitch(t, true)
	setupModeTestPricing(t, map[string]string{
		"dall-e-3":               ModelModeChat,
		"gemini-2.5-flash-image": ModelModeChat,
		"custom-drawer":          ModelModeChatImage,
		"custom-predict":         ModelModeImage,
		"gpt-4o":                 "",
		"gpt-image-1":            "",
		"no-row":                 "-",
	})

	cases := []struct {
		model string
		want  string
	}{
		{"dall-e-3", ModelModeChat},
		{"gemini-2.5-flash-image", ModelModeChat},
		{"custom-drawer", ModelModeChatImage},
		{"custom-predict", ModelModeImage},
		{"gpt-4o", ModelModeChat},
		{"gpt-image-1", ModelModeImage},
		{"no-row", ModelModeChat},
		{"imagen-4.0", ModelModeImage},
		{"gemini-3.1-flash-image-preview", ModelModeChatImage},
	}
	for _, c := range cases {
		if got := ResolveModelMode(c.model); got != c.want {
			t.Errorf("ResolveModelMode(%q) = %q, want %q", c.model, got, c.want)
		}
	}

	if got := ModelModeFromInfo("dall-e-3"); got != ModelModeChat {
		t.Errorf("ModelModeFromInfo(dall-e-3) = %q, want %q", got, ModelModeChat)
	}
	for _, m := range []string{"gpt-4o", "no-row", "imagen-4.0"} {
		if got := ModelModeFromInfo(m); got != "" {
			t.Errorf("ModelModeFromInfo(%q) = %q, want empty", m, got)
		}
	}
}

func TestIsValidModelMode(t *testing.T) {
	for _, mode := range []string{"", ModelModeChat, ModelModeImage, ModelModeChatImage} {
		if !IsValidModelMode(mode) {
			t.Errorf("IsValidModelMode(%q) = false, want true", mode)
		}
	}
	for _, mode := range []string{"Chat", "video", "image_chat", "embedding"} {
		if IsValidModelMode(mode) {
			t.Errorf("IsValidModelMode(%q) = true, want false", mode)
		}
	}
}
