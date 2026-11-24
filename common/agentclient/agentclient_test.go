package agentclient

import (
	"net/http"
	"testing"
)

func TestFromUserAgent(t *testing.T) {
	cases := map[string]string{
		"claude-cli/2.1.300 (external, cli)":                    ClaudeCode,
		"claude-cli/2.1.300 (external, sdk-ts)":                 ClaudeCode,
		"codex_cli_rs/0.140.0 (Mac OS 26.0.0; arm64) iTerm.app": Codex,
		"codex_exec/0.140.0 (Ubuntu 24.4.0; x86_64)":            Codex,
		"opencode/1.2.3":                   OpenCode,
		"GeminiCLI/0.30.0 (darwin; arm64)": GeminiCLI,
		"  Claude-CLI/2.0.0":               ClaudeCode,
		"curl/8.7.1":                       "",
		"Mozilla/5.0 (claude-cli)":         "",
		"":                                 "",
	}
	for ua, want := range cases {
		if got := FromUserAgent(ua); got != want {
			t.Errorf("FromUserAgent(%q) = %q, want %q", ua, got, want)
		}
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		endpoint, userAgent, session, want string
	}{
		{"", "", "", ""},
		{"", "claude-cli/2.1.0", "", ClaudeCode},
		// A client endpoint names the client whatever the headers say.
		{ClaudeCode, "curl/8.7.1", "", ClaudeCode},
		{Codex, "opencode/1.0.0", "", Codex},
		// The generic endpoint is refined by headers that name the agent.
		{CodingAgent, "", "", CodingAgent},
		{CodingAgent, "curl/8.7.1", "", CodingAgent},
		{CodingAgent, "opencode/1.0.0", "", OpenCode},
		// Goose sends no User-Agent, only its session header.
		{CodingAgent, "", "20261124_2", Goose},
		{"", "", "20261124_2", Goose},
		// The session header alone does not override a User-Agent.
		{CodingAgent, "curl/8.7.1", "20261124_2", CodingAgent},
		{CodingAgent, "opencode/1.0.0", "x", OpenCode},
	}
	for _, tc := range cases {
		header := http.Header{}
		if tc.userAgent != "" {
			header.Set("User-Agent", tc.userAgent)
		}
		if tc.session != "" {
			header.Set(GooseSessionHeader, tc.session)
		}
		if got := Resolve(tc.endpoint, header); got != tc.want {
			t.Errorf("Resolve(%q, ua=%q, session=%q) = %q, want %q", tc.endpoint, tc.userAgent, tc.session, got, tc.want)
		}
	}
}

func TestClaudeCodeModelID(t *testing.T) {
	cases := []struct {
		name    string
		context int
		want    string
	}{
		{"claude-opus-4-8", 1_000_000, "claude-code/claude-opus-4-8[1m]"},
		{"gpt-5.5", 1_050_000, "claude-code/gpt-5.5[1m]"},
		{"claude-haiku-4-5", 200_000, "claude-code/claude-haiku-4-5"},
		{"deepseek/deepseek-v4", 0, "claude-code/deepseek/deepseek-v4"},
	}
	for _, tc := range cases {
		id := ClaudeCodeModelID(tc.name, tc.context)
		if id != tc.want {
			t.Errorf("ClaudeCodeModelID(%q, %d) = %q, want %q", tc.name, tc.context, id, tc.want)
		}
		if back := StripClaudeCodeModelID(id); back != tc.name {
			t.Errorf("StripClaudeCodeModelID(%q) = %q, want %q", id, back, tc.name)
		}
	}
}

func TestStripClaudeCodeModelIDLeavesOtherNames(t *testing.T) {
	for in, want := range map[string]string{
		"claude-sonnet-5-5":                 "claude-sonnet-5-5",
		"claude-opus-4-6[1m]":               "claude-opus-4-6",
		"claude-opus-4-6[1M]":               "claude-opus-4-6",
		"anthropic/claude-opus-5":           "anthropic/claude-opus-5",
		"claude-code/anthropic/claude-opus": "anthropic/claude-opus",
		" claude-code/gpt-5.5[1m] ":         "gpt-5.5",
	} {
		if got := StripClaudeCodeModelID(in); got != want {
			t.Errorf("StripClaudeCodeModelID(%q) = %q, want %q", in, got, want)
		}
	}
}
