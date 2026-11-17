// Package agentclient names the coding-agent clients that call the relay and holds the
// per-client conventions in one place: which endpoint prefix belongs to which client, how a
// client is recognised from its User-Agent, and the model ID spelling each client's model
// picker understands.
//
// A client endpoint (/claude-code, /codex/v1, /coding-agent) identifies the caller, not the
// protocol: the sub-path still decides the protocol, and the request is served by the same
// handler as the bare path (see router.SetRelayRouter).
package agentclient

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// Client IDs as stored in logs.client. They are stable strings: the organization usage
// analytics filter and group by them.
const (
	ClaudeCode  = "claude-code"
	Codex       = "codex"
	OpenCode    = "opencode"
	GeminiCLI   = "gemini-cli"
	CodingAgent = "coding-agent"
)

// Known lists the client IDs the relay can record, in display order.
var Known = []string{ClaudeCode, Codex, OpenCode, GeminiCLI, CodingAgent}

// EndpointContextKey holds the client named by the endpoint the request came in on.
const EndpointContextKey = "agent_client_endpoint"

// userAgentRules maps a User-Agent prefix (lower case) to the client that sends it.
// Claude Code and the Claude Agent SDK send "claude-cli/<version> (external, ...)", Codex sends
// "<originator>/<version> ..." with originator codex_cli_rs (codex_exec, codex_vscode for the
// other front ends), OpenCode sends "opencode/<version>" and Gemini CLI "GeminiCLI/<version>".
var userAgentRules = []struct {
	prefix string
	client string
}{
	{"claude-cli/", ClaudeCode},
	{"claude-code/", ClaudeCode},
	{"codex_", Codex},
	{"codex-", Codex},
	{"opencode/", OpenCode},
	{"geminicli/", GeminiCLI},
	{"gemini-cli/", GeminiCLI},
}

// FromUserAgent returns the client a User-Agent belongs to, "" when it is not a known one.
func FromUserAgent(userAgent string) string {
	ua := strings.ToLower(strings.TrimSpace(userAgent))
	for _, rule := range userAgentRules {
		if strings.HasPrefix(ua, rule.prefix) {
			return rule.client
		}
	}
	return ""
}

// Resolve returns the client to record for a request: the endpoint's client when the endpoint
// names one (/claude-code, /codex), otherwise the User-Agent's. The generic /coding-agent
// endpoint only says "a coding agent", so a User-Agent that names the agent refines it.
func Resolve(endpointClient, userAgent string) string {
	switch endpointClient {
	case "":
		return FromUserAgent(userAgent)
	case CodingAgent:
		if client := FromUserAgent(userAgent); client != "" {
			return client
		}
	}
	return endpointClient
}

// FromContext is Resolve for a relay request.
func FromContext(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	return Resolve(c.GetString(EndpointContextKey), c.GetHeader("User-Agent"))
}

// Claude Code model IDs on the /claude-code endpoint.
//
// Claude Code's gateway model discovery keeps only the IDs that contain "claude" or "anthropic",
// so every model is listed under the claude-code/ prefix; a model that accepts a prompt of one
// million tokens or more also gets the [1m] suffix, which tells Claude Code to use that context
// window. Both are display IDs: they are removed before routing, so billing and logs use the
// real model name.
const (
	ClaudeCodeModelPrefix = "claude-code/"
	ClaudeCodeLongContext = "[1m]"
	// LongContextTokens is the context length from which a model is listed with the [1m] suffix.
	LongContextTokens = 1_000_000
)

// ClaudeCodeModelID returns the ID /claude-code/v1/models lists for a model.
func ClaudeCodeModelID(modelName string, contextLength int) string {
	id := ClaudeCodeModelPrefix + modelName
	if contextLength >= LongContextTokens {
		id += ClaudeCodeLongContext
	}
	return id
}

// StripClaudeCodeModelID turns a model ID Claude Code sends back into the real model name. IDs
// without the prefix or suffix (Claude Code's built-in models, a name typed in /model) are
// returned unchanged.
func StripClaudeCodeModelID(id string) string {
	name := strings.TrimSpace(id)
	name = strings.TrimPrefix(name, ClaudeCodeModelPrefix)
	if strings.HasSuffix(strings.ToLower(name), ClaudeCodeLongContext) {
		name = name[:len(name)-len(ClaudeCodeLongContext)]
	}
	return name
}
