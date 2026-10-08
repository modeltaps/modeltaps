package controller

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"github.com/spf13/viper"
)

func setupModelTypePricing(t *testing.T, modes map[string]string) {
	t.Helper()
	instance := &model.Pricing{Prices: make(map[string]*model.Price), Match: make([]string, 0)}
	for name, mode := range modes {
		instance.Prices[name] = &model.Price{
			Model:     name,
			ModelInfo: &model.ModelInfoResponse{Model: name, Mode: mode},
		}
	}
	old := model.PricingInstance
	model.PricingInstance = instance
	t.Cleanup(func() { model.PricingInstance = old })
}

func setModelInfoModeSwitch(t *testing.T, enabled bool) {
	t.Helper()
	old := viper.GetBool("routing.model_info_mode")
	viper.Set("routing.model_info_mode", enabled)
	t.Cleanup(func() { viper.Set("routing.model_info_mode", old) })
}

// 开关关闭：测速分类与改动前一致，库里的 mode 被忽略。
func TestGetModelTypeSwitchOff(t *testing.T) {
	setModelInfoModeSwitch(t, false)
	setupModelTypePricing(t, map[string]string{
		"gemini-2.5-flash-image": model.ModelModeChat,
		"flux-pro":               model.ModelModeChat,
		"gpt-4o":                 model.ModelModeImage,
	})

	cases := []struct{ modelName, want string }{
		{"tts-1", "noSupport"},
		{"text-embedding-3-small", "embeddings"},
		{"gemini-2.5-flash-image", "chat"},
		{"gemini-3.1-flash-image-preview", "chat"},
		{"imagen-4.0", "image"},
		{"dall-e-3", "image"},
		{"flux-pro", "image"},
		{"o1-preview", "response"},
		{"gpt-4o", "chat"},
	}
	for _, c := range cases {
		if got := getModelType(c.modelName); got != c.want {
			t.Errorf("getModelType(%q) = %q, want %q", c.modelName, got, c.want)
		}
	}
}

// 开关打开：库里的 mode 决定分类，显式 chat 可否决名单与 imageRegex 关键词。
func TestGetModelTypeSwitchOn(t *testing.T) {
	setModelInfoModeSwitch(t, true)
	setupModelTypePricing(t, map[string]string{
		"gemini-2.5-flash-image": model.ModelModeChat,
		"flux-pro":               model.ModelModeChat,
		"custom-drawer":          model.ModelModeChatImage,
		"custom-predict":         model.ModelModeImage,
		"gpt-4o":                 "",
	})

	cases := []struct{ modelName, want string }{
		{"gemini-2.5-flash-image", "chat"},
		{"flux-pro", "chat"},
		{"custom-drawer", "chat"},
		{"custom-predict", "image"},
		{"gpt-4o", "chat"},
		{"gemini-3.1-flash-image-preview", "chat"},
		{"dall-e-3", "image"},
	}
	for _, c := range cases {
		if got := getModelType(c.modelName); got != c.want {
			t.Errorf("getModelType(%q) = %q, want %q", c.modelName, got, c.want)
		}
	}
}
