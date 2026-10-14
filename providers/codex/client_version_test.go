package codex

import (
	"testing"

	"github.com/modeltaps/modeltaps/model"

	"gorm.io/datatypes"
)

func codexPlugin(v any) *datatypes.JSONType[model.PluginType] {
	data := datatypes.NewJSONType(model.PluginType{"codex": map[string]interface{}{"client_version": v}})
	return &data
}

func TestClientVersionPluginOverride(t *testing.T) {
	cases := []struct {
		name   string
		plugin *datatypes.JSONType[model.PluginType]
		want   string
	}{
		{"no plugin", nil, DefaultCodexVersion},
		{"empty value", codexPlugin("   "), DefaultCodexVersion},
		{"non-string value", codexPlugin(0.16), DefaultCodexVersion},
		{"override", codexPlugin(" 0.160.0 "), "0.160.0"},
		{"pre-release override", codexPlugin("0.158.0-alpha.2"), "0.158.0-alpha.2"},
		{"missing patch", codexPlugin("0.160"), DefaultCodexVersion},
		{"leading v", codexPlugin("v0.160.0"), DefaultCodexVersion},
		{"header injection", codexPlugin("0.160.0\r\nX-Evil: 1"), DefaultCodexVersion},
		{"embedded space", codexPlugin("0.160.0 beta"), DefaultCodexVersion},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// An empty key must not skip the plugin settings.
			provider := &CodexProvider{}
			provider.Channel = &model.Channel{Plugin: c.plugin}
			parseCodexConfig(provider)
			if provider.ClientVersion != c.want {
				t.Fatalf("ClientVersion = %q, want %q", provider.ClientVersion, c.want)
			}

			headers := map[string]string{}
			provider.applyDefaultHeaders(headers)
			if headers["version"] != c.want {
				t.Fatalf("version header = %q, want %q", headers["version"], c.want)
			}
			wantUA := "codex_cli_rs/" + c.want + " " + codexUserAgentPlatform
			if headers["User-Agent"] != wantUA {
				t.Fatalf("User-Agent = %q, want %q", headers["User-Agent"], wantUA)
			}
		})
	}
}

func TestDefaultUserAgentMatchesDefaultVersion(t *testing.T) {
	provider := &CodexProvider{}
	provider.Channel = &model.Channel{}
	parseCodexConfig(provider)
	if got := provider.defaultUserAgent(); got != DefaultCodexUserAgent {
		t.Fatalf("defaultUserAgent() = %q, want %q", got, DefaultCodexUserAgent)
	}
}

func TestApplyDefaultHeadersCustomUserAgent(t *testing.T) {
	const customUA = "my-agent/1.0"
	other := `{"user_agent":"` + customUA + `"}`

	t.Run("default version", func(t *testing.T) {
		provider := &CodexProvider{}
		provider.Channel = &model.Channel{Other: other}
		parseCodexConfig(provider)

		headers := map[string]string{}
		provider.applyDefaultHeaders(headers)
		assertHeaders(t, headers, map[string]string{
			"User-Agent": customUA,
			"version":    DefaultCodexVersion,
			"originator": DefaultCodexOriginator,
			"Host":       "chatgpt.com",
			"Accept":     "application/json",
		})
	})

	t.Run("plugin version", func(t *testing.T) {
		provider := &CodexProvider{}
		provider.Channel = &model.Channel{Other: other, Plugin: codexPlugin("0.160.0")}
		parseCodexConfig(provider)

		headers := map[string]string{}
		provider.applyDefaultHeaders(headers)
		assertHeaders(t, headers, map[string]string{
			"User-Agent": customUA,
			"version":    "0.160.0",
			"originator": DefaultCodexOriginator,
		})
	})

	t.Run("existing headers are kept", func(t *testing.T) {
		provider := &CodexProvider{}
		provider.Channel = &model.Channel{Other: other}
		parseCodexConfig(provider)

		headers := map[string]string{"User-Agent": "client/2.0", "version": "0.150.0", "originator": "client"}
		provider.applyDefaultHeaders(headers)
		assertHeaders(t, headers, map[string]string{
			"User-Agent": "client/2.0",
			"version":    "0.150.0",
			"originator": "client",
		})
	})
}

func assertHeaders(t *testing.T, got, want map[string]string) {
	t.Helper()
	for k, v := range want {
		if got[k] != v {
			t.Errorf("header %s = %q, want %q", k, got[k], v)
		}
	}
}
