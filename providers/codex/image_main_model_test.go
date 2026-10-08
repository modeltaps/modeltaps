package codex

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"gorm.io/datatypes"
)

func TestImagesMainModelPluginOverride(t *testing.T) {
	plugin := func(v any) *datatypes.JSONType[model.PluginType] {
		data := datatypes.NewJSONType(model.PluginType{"codex": map[string]interface{}{"images_main_model": v}})
		return &data
	}
	cases := []struct {
		name   string
		plugin *datatypes.JSONType[model.PluginType]
		want   string
	}{
		{"no plugin", nil, imagesResponsesMainModel},
		{"empty value", plugin("   "), imagesResponsesMainModel},
		{"non-string value", plugin(42), imagesResponsesMainModel},
		{"override", plugin(" gpt-5.6-terra "), "gpt-5.6-terra"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// An empty key must not skip the plugin settings.
			provider := &CodexProvider{}
			provider.Channel = &model.Channel{Plugin: c.plugin}
			parseCodexConfig(provider)
			if provider.ImagesMainModel != c.want {
				t.Fatalf("ImagesMainModel = %q, want %q", provider.ImagesMainModel, c.want)
			}

			body := buildImagesRequestBody(imageToolActionGenerate, "a cat", imageGenerateTool{}, nil, "", provider.ImagesMainModel)
			if body.Model != c.want {
				t.Fatalf("request model = %q, want %q", body.Model, c.want)
			}
		})
	}
}
