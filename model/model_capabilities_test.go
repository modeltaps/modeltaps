package model

import "testing"

// 能力词表：序列化按固定顺序、去重、丢弃词表外值，空集合写 "[]"。
func TestModelCapabilitiesJSON(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"empty", nil, `[]`},
		{"all", []string{ModelCapabilityToolCall, ModelCapabilityReasoning, ModelCapabilityStructuredOutput}, `["tool_call","reasoning","structured_output"]`},
		{"reorder", []string{ModelCapabilityStructuredOutput, ModelCapabilityToolCall}, `["tool_call","structured_output"]`},
		{"dedupe", []string{ModelCapabilityReasoning, ModelCapabilityReasoning}, `["reasoning"]`},
		{"unknown dropped", []string{"vision", ModelCapabilityToolCall}, `["tool_call"]`},
	}
	for _, c := range cases {
		if got := ModelCapabilitiesJSON(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

func TestIsValidModelCapabilities(t *testing.T) {
	valid := []string{"", `[]`, `["tool_call"]`, `["reasoning","structured_output"]`}
	for _, raw := range valid {
		if !IsValidModelCapabilities(raw) {
			t.Errorf("IsValidModelCapabilities(%q) = false, want true", raw)
		}
	}
	invalid := []string{`["vision"]`, `["tool_call","vision"]`, `not json`, `{"a":1}`, `"tool_call"`}
	for _, raw := range invalid {
		if IsValidModelCapabilities(raw) {
			t.Errorf("IsValidModelCapabilities(%q) = true, want false", raw)
		}
	}
}

// 空串解析为空切片而非 null。
func TestModelInfoToResponseCapabilities(t *testing.T) {
	if got := (&ModelInfo{}).ToResponse().Capabilities; got == nil || len(got) != 0 {
		t.Errorf("empty capabilities = %#v, want empty slice", got)
	}
	got := (&ModelInfo{Capabilities: `["tool_call","reasoning"]`}).ToResponse().Capabilities
	if len(got) != 2 || got[0] != ModelCapabilityToolCall || got[1] != ModelCapabilityReasoning {
		t.Errorf("capabilities = %#v", got)
	}
}
