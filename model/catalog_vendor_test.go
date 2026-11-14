package model

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

func setupCatalogVendors(t *testing.T) {
	t.Helper()
	setupModelOwnedByTestDB(t)
	ModelOwnedBysInstance.SyncModelOwnedBy(GetDefaultModelOwnedBy())
}

// Hosting platforms (Vertex, Azure, Groq) serve other labs' models: the vendor comes from the
// canonical id or the id prefix, never from the platform.
func TestBuildCatalogModelInfosVendorIgnoresHostingPlatform(t *testing.T) {
	setupCatalogVendors(t)
	const fx = `{
      "anthropic":     {"models": {"claude-fable-5-2": {"id":"claude-fable-5-2","canonical_model_id":"anthropic/claude-fable-5-2"}}},
      "google-vertex": {"models": {
        "claude-fable-5-2@default": {"id":"claude-fable-5-2@default","canonical_model_id":"anthropic/claude-fable-5-2"},
        "xai/grok-4.7":             {"id":"xai/grok-4.7"},
        "zai-org/glm-5-maas":       {"id":"zai-org/glm-5-maas"},
        "gemini-3-pro":             {"id":"gemini-3-pro","canonical_model_id":"google/gemini-3-pro"},
        "mystery-model":            {"id":"mystery-model"}
      }},
      "azure": {"models": {
        "grok-4.6": {"id":"grok-4.6","canonical_model_id":"xai/grok-4.6"},
        "phi-4":    {"id":"phi-4"}
      }},
      "groq":   {"models": {"llama-3.3-70b-versatile": {"id":"llama-3.3-70b-versatile","canonical_model_id":"meta/llama-3.3-70b"}}},
      "openai": {"models": {"gpt-4o": {"id":"gpt-4o"}}}
    }`
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(fx), &data); err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, m := range BuildCatalogModelInfos(data) {
		got[m.Model] = m.VendorID
	}
	want := map[string]int{
		"claude-fable-5-2":         config.ChannelTypeAnthropic,
		"claude-fable-5-2@default": config.ChannelTypeAnthropic,
		"xai/grok-4.7":             config.ChannelTypeXAI,
		"zai-org/glm-5-maas":       config.ChannelTypeZhipu,
		"gemini-3-pro":             config.ChannelTypeGemini,
		"mystery-model":            0,
		"grok-4.6":                 config.ChannelTypeXAI,
		"phi-4":                    0,
		"llama-3.3-70b-versatile":  config.ChannelTypeLLAMA,
		"gpt-4o":                   config.ChannelTypeOpenAI,
	}
	for model, w := range want {
		if g, ok := got[model]; !ok || g != w {
			t.Errorf("%s vendor = %d (present %v), want %d", model, g, ok, w)
		}
	}
}

// The same id listed by the lab and by a platform resolves to the lab's entry, whatever the map
// order.
func TestBuildCatalogModelInfosPrefersLabEntry(t *testing.T) {
	setupCatalogVendors(t)
	const fx = `{
      "mistral":       {"models": {"mistral-medium-3": {"id":"mistral-medium-3","name":"From lab"}}},
      "google-vertex": {"models": {"mistral-medium-3": {"id":"mistral-medium-3","name":"From vertex"}}}
    }`
	data := map[string]catalogProvider{}
	if err := json.Unmarshal([]byte(fx), &data); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		infos := BuildCatalogModelInfos(data)
		if len(infos) != 1 {
			t.Fatalf("want one row, got %d", len(infos))
		}
		if infos[0].Name != "From lab" || infos[0].VendorID != config.ChannelTypeMistral {
			t.Fatalf("got %q vendor %d, want the lab entry", infos[0].Name, infos[0].VendorID)
		}
	}
}

func TestVendorIDFromModelNameLabPrefixes(t *testing.T) {
	setupCatalogVendors(t)
	cases := map[string]int{
		"xai/grok-4":            config.ChannelTypeXAI,
		"alibaba/qwen3-max":     config.ChannelTypeAli,
		"zhipuai/glm-5":         config.ChannelTypeZhipu,
		"meta/llama-4-scout":    config.ChannelTypeLLAMA,
		"mistral/codestral-2":   config.ChannelTypeMistral,
		"deepseek-ai/deepseek":  config.ChannelTypeDeepseek,
		"Anthropic/claude-x":    config.ChannelTypeAnthropic,
		"nobody/whatever-model": 0,
	}
	for name, want := range cases {
		if got := VendorIDFromModelName(name); got != want {
			t.Errorf("VendorIDFromModelName(%q) = %d, want %d", name, got, want)
		}
	}
}

// A re-sync from the same source clears a vendor it can no longer back; admin-edited rows keep
// theirs.
func TestUpsertModelInfosClearsStaleSyncedVendor(t *testing.T) {
	setupModelInfoTestDB(t)
	rows := []ModelInfo{
		{Model: "synced", VendorID: config.ChannelTypeGemini, Source: ModelInfoSourceModelsDev},
		{Model: "edited", VendorID: config.ChannelTypeGemini, Source: ModelInfoSourceManual},
		{Model: "other-source", VendorID: config.ChannelTypeGemini, Source: ModelInfoSourceOpenRouter},
	}
	for i := range rows {
		if err := DB.Create(&rows[i]).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := UpsertModelInfos([]*ModelInfo{{Model: "synced"}, {Model: "edited"}}, ModelInfoSourceModelsDev); err != nil {
		t.Fatal(err)
	}
	if err := UpsertModelInfos([]*ModelInfo{{Model: "other-source"}}, ModelInfoSourceModelsDev); err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"synced": 0, "edited": config.ChannelTypeGemini, "other-source": config.ChannelTypeGemini}
	for model, w := range want {
		var got ModelInfo
		if err := DB.Where("model = ?", model).First(&got).Error; err != nil {
			t.Fatal(err)
		}
		if got.VendorID != w {
			t.Errorf("%s vendor = %d, want %d", model, got.VendorID, w)
		}
	}
}
