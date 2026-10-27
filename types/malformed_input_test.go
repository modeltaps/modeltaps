package types

import (
	"encoding/json"
	"testing"
)

func decodeAny(t *testing.T, raw string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return v
}

func TestParseToolChoice(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		wantType string
		wantFunc string
		wantErr  bool
	}{
		{name: "nil defaults to auto", raw: "null", wantType: ToolChoiceTypeAuto},
		{name: "string choice", raw: `"required"`, wantType: ToolChoiceTypeRequired},
		{name: "named function", raw: `{"type":"function","function":{"name":"get_weather"}}`, wantType: ToolChoiceTypeFunction, wantFunc: "get_weather"},
		{name: "function without name", raw: `{"type":"function","function":{}}`, wantErr: true},
		{name: "function name not a string", raw: `{"type":"function","function":{"name":42}}`, wantErr: true},
		{name: "empty function name", raw: `{"type":"function","function":{"name":""}}`, wantErr: true},
		{name: "function type without function object", raw: `{"type":"function"}`, wantErr: true},
		{name: "unknown object falls back to auto", raw: `{"type":"auto"}`, wantType: ToolChoiceTypeAuto},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := ChatCompletionRequest{ToolChoice: decodeAny(t, tt.raw)}
			gotType, gotFunc, err := req.ParseToolChoice()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got type=%q func=%q", gotType, gotFunc)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotType != tt.wantType || gotFunc != tt.wantFunc {
				t.Fatalf("got (%q, %q), want (%q, %q)", gotType, gotFunc, tt.wantType, tt.wantFunc)
			}
		})
	}
}

func TestEmbeddingParseInput(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "string", raw: `"hello"`, want: []string{"hello"}},
		{name: "string array", raw: `["a","b"]`, want: []string{"a", "b"}},
		{name: "missing", raw: "null", wantErr: true},
		{name: "empty array", raw: `[]`, wantErr: true},
		{name: "token array", raw: `[1,2,3]`, wantErr: true},
		{name: "mixed array", raw: `["a",null]`, wantErr: true},
		{name: "object", raw: `{"text":"a"}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := EmbeddingRequest{Input: decodeAny(t, tt.raw)}
			got, err := req.ParseInput()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				if _, err := req.ParseInputString(); err == nil {
					t.Fatal("ParseInputString: expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
			first, err := req.ParseInputString()
			if err != nil || first != tt.want[0] {
				t.Fatalf("ParseInputString = %q, %v; want %q", first, err, tt.want[0])
			}
		})
	}
}
