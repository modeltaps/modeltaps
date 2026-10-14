package model

import "testing"

func TestShouldUseResponsesForModel(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := []struct {
		name    string
		setting *string
		model   string
		want    bool
	}{
		{"nil setting uses defaults", nil, "o3-pro", true},
		{"nil setting, chat model", nil, "gpt-4o", false},
		{"empty setting uses defaults", str("  "), "codex-mini-latest", true},
		{"added model", str("gpt-5-pro\nmy-model"), "gpt-5-pro", true},
		{"adding keeps defaults", str("gpt-5-pro"), "o3-pro", true},
		{"comma separated", str("a, gpt-5-pro"), "gpt-5-pro", true},
		{"json array", str(`["gpt-5-pro"]`), "gpt-5-pro", true},
		{"removed default", str("-o3-pro"), "o3-pro", false},
		{"removal keeps other defaults", str("-o3-pro"), "o1-pro", true},
		{"removal wins over addition", str("gpt-5-pro\n- gpt-5-pro"), "gpt-5-pro", false},
		{"blank model", str("gpt-5-pro"), " ", false},
		{"model name is trimmed", str("gpt-5-pro"), " gpt-5-pro ", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			channel := &Channel{Need2ResponseModels: tc.setting}
			if got := channel.ShouldUseResponsesForModel(tc.model); got != tc.want {
				t.Fatalf("ShouldUseResponsesForModel(%q) = %v, want %v", tc.model, got, tc.want)
			}
		})
	}

	var nilChannel *Channel
	if !nilChannel.ShouldUseResponsesForModel("o3-pro") {
		t.Fatal("a nil channel must fall back to the defaults")
	}
}
