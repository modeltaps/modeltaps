package model

import "testing"

func TestParseModelFamily(t *testing.T) {
	cases := []struct {
		id   string
		want ModelFamily
	}{
		{"anthropic/claude-opus-5.5", ModelFamily{Family: "claude-opus", Version: "5.5"}},
		{"claude-opus-5-5", ModelFamily{Family: "claude-opus", Version: "5.5"}},
		{"claude-opus-4-1-20250805", ModelFamily{Family: "claude-opus", Version: "4.1", Snapshot: "20250805"}},
		{"claude-3-5-sonnet-20241022", ModelFamily{Family: "claude-sonnet", Version: "3.5", Snapshot: "20241022"}},
		{"claude-haiku-4-5-20251001", ModelFamily{Family: "claude-haiku", Version: "4.5", Snapshot: "20251001"}},
		{"anthropic/claude-opus-5.5:batch", ModelFamily{Family: "claude-opus", Version: "5.5", Variant: "batch"}},
		{"openai/gpt-5.1-codex-max", ModelFamily{Family: "gpt-codex-max", Version: "5.1"}},
		{"gpt-4o", ModelFamily{Family: "gpt-o", Version: "4"}},
		{"gpt-4o-mini-2024-07-18", ModelFamily{Family: "gpt-o-mini", Version: "4", Snapshot: "2024-07-18"}},
		{"gpt-3.5-turbo-0613", ModelFamily{Family: "gpt-turbo", Version: "3.5", Snapshot: "0613"}},
		{"o3-mini", ModelFamily{Family: "o-mini", Version: "3"}},
		{"google/gemini-3.1-pro-preview", ModelFamily{Family: "gemini-pro", Version: "3.1", Preview: true}},
		{"gemini-2.5-pro", ModelFamily{Family: "gemini-pro", Version: "2.5"}},
		{"gemini-4-argon", ModelFamily{Family: "gemini-argon", Version: "4"}},
		{"google/gemma-4-26b-a4b-it:free", ModelFamily{Family: "gemma-26b-a4b-it", Version: "4", Variant: "free"}},
		{"qwen/qwen3.5-122b-a10b", ModelFamily{Family: "qwen-122b-a10b", Version: "3.5"}},
		{"qwen3.5-plus-02-15", ModelFamily{Family: "qwen-plus", Version: "3.5", Snapshot: "02-15"}},
		{"moonshotai/kimi-k2.5", ModelFamily{Family: "kimi-k", Version: "2.5"}},
		{"kimi-k2-0905", ModelFamily{Family: "kimi-k", Version: "2", Snapshot: "0905"}},
		{"deepseek-v4.1-flash", ModelFamily{Family: "deepseek-v-flash", Version: "4.1"}},
		{"deepseek-r1-0528", ModelFamily{Family: "deepseek-r", Version: "1", Snapshot: "0528"}},
		{"z-ai/glm-4.5v", ModelFamily{Family: "glm-v", Version: "4.5"}},
		{"glm-4.5-air", ModelFamily{Family: "glm-air", Version: "4.5"}},
		{"x-ai/grok-4.20", ModelFamily{Family: "grok", Version: "4.20"}},
		{"mistral-medium-3-5", ModelFamily{Family: "mistral-medium", Version: "3.5"}},
		{"mistralai/mistral-large-2512", ModelFamily{Family: "mistral-large", Snapshot: "2512"}},
		{"meta-llama/llama-3.1-8b-instruct", ModelFamily{Family: "llama-8b-instruct", Version: "3.1"}},
		{"mixtral-8x22b-instruct", ModelFamily{Family: "mixtral-8x22b-instruct"}},
		{"gemini-nano-banana-2.1", ModelFamily{Family: "gemini-nano-banana", Version: "2.1"}},
		{"minimax/minimax-m3.1-flash:free", ModelFamily{Family: "minimax-m-flash", Version: "3.1", Variant: "free"}},
		{"gpt-4o-latest", ModelFamily{Family: "gpt-o", Version: "4"}},
		{"~anthropic/claude-opus-latest", ModelFamily{Family: "claude-opus", Alias: true}},
		{"  Claude-Opus-5-5 ", ModelFamily{Family: "claude-opus", Version: "5.5"}},
	}
	for _, tc := range cases {
		if got := ParseModelFamily(tc.id); got != tc.want {
			t.Errorf("ParseModelFamily(%q) = %+v, want %+v", tc.id, got, tc.want)
		}
	}
}

func TestCompareModelVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"5.5", "5", 1},
		{"5", "4.8", 1},
		{"4.20", "4.3", -1},
		{"4.5", "4.5", 0},
		{"4.10", "4.1", 0},
		{"10", "9.9", 1},
		{"", "1", -1},
		{"3.5.1", "3.5", 1},
	}
	for _, tc := range cases {
		if got := CompareModelVersions(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareModelVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestParseCatalogDate(t *testing.T) {
	cases := map[string]int64{
		"2026-04-14":   1776124800,
		"2026-04":      1775001600,
		" 2026-11-11 ": 1794355200,
		"":             0,
		"soon":         0,
	}
	for in, want := range cases {
		if got := ParseCatalogDate(in); got != want {
			t.Errorf("ParseCatalogDate(%q) = %d, want %d", in, got, want)
		}
	}
}
