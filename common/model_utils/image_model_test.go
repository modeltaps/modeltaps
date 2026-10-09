package model_utils

import (
	"github.com/modeltaps/modeltaps/common/config"
	"testing"
)

func TestIsGeminiNativeImageModel(t *testing.T) {
	prev := config.ModelNameCaseInsensitiveEnabled
	config.ModelNameCaseInsensitiveEnabled = true
	t.Cleanup(func() { config.ModelNameCaseInsensitiveEnabled = prev })

	cases := map[string]bool{
		"gemini-2.5-flash-image":         true,
		"gemini-2.5-flash-image-preview": true,
		"gemini-3-pro-image-preview":     true,
		"gemini-3.1-flash-lite-image":    true,
		"gemini-nano-banana-2.1":         true,
		"GEMINI-NANO-BANANA":             true,
		"gemini-2.0-flash-exp":           true,
		"Gemini-2.0-Flash-Exp":           true,
		"gemini-2.0-flash-exp-thinking":  false,
		"gemini-2.5-flash":               false,
		"imagen-4.0-generate-001":        false,
		"":                               false,
	}
	for model, want := range cases {
		if got := IsGeminiNativeImageModel(model); got != want {
			t.Errorf("IsGeminiNativeImageModel(%q) = %v, want %v", model, got, want)
		}
	}

	config.ModelNameCaseInsensitiveEnabled = false
	if IsGeminiNativeImageModel("Gemini-2.0-Flash-Exp") {
		t.Error("exact match must be case-sensitive when case-insensitive matching is disabled")
	}
	if !IsGeminiNativeImageModel("gemini-2.0-flash-exp") {
		t.Error("exact match failed with case-insensitive matching disabled")
	}
}
