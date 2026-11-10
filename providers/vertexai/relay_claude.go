package vertexai

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/providers/vertexai/category"
	"github.com/modeltaps/modeltaps/types"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// vertexUnsupportedBetaTokens lists the anthropic-beta tokens known to be rejected (HTTP 400) by
// the Anthropic endpoint of Vertex AI. Vertex accepts many betas (context-1m, context-management,
// interleaved-thinking, compaction, computer-use, ...), so this is a deny list rather than an
// allow list: only tokens that are specific to the direct Anthropic API or confirmed to be
// refused are dropped and everything else is forwarded. An anthropic-beta set explicitly in the
// channel's ModelHeaders / HeaderOverride takes precedence and is not filtered.
//
// Maintenance: when Vertex answers 400 for a new token (the error mentions anthropic-beta or an
// unsupported beta), add it here. Until then an administrator can pin anthropic-beta in the
// channel's ModelHeaders.
var vertexUnsupportedBetaTokens = map[string]struct{}{
	"oauth-2025-04-20":                {},
	"claude-code-20250219":            {},
	"advisor-tool-2026-03-01":         {},
	"prompt-caching-scope-2026-01-05": {},
	"redact-thinking-2026-02-12":      {},
	"thinking-token-count-2026-05-13": {},
}

// The relay layer routes Claude-protocol requests natively when the provider implements this.
var _ claude.ClaudeChatInterface = (*VertexAIProvider)(nil)

func (p *VertexAIProvider) CreateClaudeChat(request *claude.ClaudeRequest) (*claude.ClaudeResponse, *types.OpenAIErrorWithStatusCode) {
	req, errWithCode := p.getClaudeNativeRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	claudeResponse := &claude.ClaudeResponse{}
	// With fingerprint pass-through the raw bytes are written back to resp.Body: one copy is
	// unmarshalled for billing and the original bytes are forwarded to the client (field order
	// and unknown fields preserved).
	passThrough := config.FingerprintPassThroughEnabled && p.Context != nil
	resp, errWithCode := p.Requester.SendRequest(req, claudeResponse, passThrough)
	if errWithCode != nil {
		return nil, errWithCode
	}

	base.ObserveUpstreamModel(p.Context, claudeResponse.Model)

	usage := p.GetUsage()
	if isOk := claude.ClaudeUsageToOpenaiUsage(&claudeResponse.Usage, usage); !isOk {
		usage.CompletionTokens = claude.ClaudeOutputUsage(claudeResponse)
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	if passThrough && resp != nil {
		if rawBytes, readErr := io.ReadAll(resp.Body); readErr == nil && len(rawBytes) > 0 {
			// Vertex reports its own model ID (claude-sonnet-4-5@20250929). Put the requested
			// name back only when "unify request and response model" is on, like the direct
			// Anthropic channel.
			if patched, changed := base.UnifyModelInJSONBytes(p.Context, rawBytes, "model"); changed {
				rawBytes = patched
			}
			p.Context.Set(config.GinRawResponseBodyKey, rawBytes)
		}
		resp.Body.Close()
	}

	return claudeResponse, nil
}

func (p *VertexAIProvider) CreateClaudeChatStream(request *claude.ClaudeRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	req, errWithCode := p.getClaudeNativeRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// streamRawPredict returns standard SSE ("data: {...}"), so the direct Claude stream handler
	// is reused. Context lets the unify-model switch rewrite message_start's model.
	chatHandler := &claude.ClaudeRelayStreamHandler{
		Usage:     p.Usage,
		ModelName: request.Model,
		Prefix:    `data: {`,
		Context:   p.Context,
	}

	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	return requester.RequestNoTrimStream(p.Requester, resp, chatHandler.HandlerStream)
}

// getClaudeNativeRequest builds the native Messages request to the Anthropic endpoint of
// Vertex AI: the URL uses publishers/anthropic, the body is passed through as bytes when
// possible, anthropic_version goes into the body and the betas into a header.
func (p *VertexAIProvider) getClaudeNativeRequest(request *claude.ClaudeRequest) (*http.Request, *types.OpenAIErrorWithStatusCode) {
	if p.ProjectID == "" || p.Region == "" {
		return nil, common.StringErrorWrapperLocal("vertexAI region or project id is empty", "invalid_vertexai_config", http.StatusInternalServerError)
	}

	// The native path does not go through getChatRequest, so select the Claude category here
	// (as getGeminiRequest does for Gemini): model name normalization, the action, the
	// publisher and the error handler all come from the category.
	p.Category = category.CategoryMap["claude"]
	modelName := p.Category.GetModelName(request.Model)
	fullRequestURL := p.GetFullRequestURL(modelName, p.Category.GetOtherUrl(request.Stream))
	if fullRequestURL == "" {
		return nil, common.ErrorWrapperLocal(nil, "invalid_vertexai_config", http.StatusInternalServerError)
	}

	headers, err := p.getRequestHeadersInternal()
	if err != nil {
		return nil, p.handleTokenError(err)
	}
	if request.Stream {
		headers["Accept"] = "text/event-stream"
	}
	p.applyClaudeBetaHeader(headers)

	p.Requester.ErrorHandler = RequestErrorHandle(p.Category.ErrorHandler, p.ProjectID)

	// Byte pass-through keeps fields the struct does not cover (cache_control, output_config,
	// context_management, ...). NewRequestWithCustomParamsBytes still merges the channel's
	// custom parameters into the body.
	if patched, ok := p.patchClaudeRequestBody(request); ok {
		return p.NewRequestWithCustomParamsBytes(http.MethodPost, fullRequestURL, patched, headers, request.Model)
	}

	// Fallback: struct serialization (no raw bytes available, for example a channel test that
	// builds a synthetic request).
	copyRequest := *request
	copyRequest.Model = ""
	vertexRequest := &category.ClaudeRequest{
		ClaudeRequest:    &copyRequest,
		AnthropicVersion: category.AnthropicVersion,
	}
	return p.NewRequestWithCustomParams(http.MethodPost, fullRequestURL, vertexRequest, headers, request.Model)
}

// patchClaudeRequestBody takes the raw /v1/messages body cached by the relay layer and makes
// the minimal byte-level edits:
//   - remove model (Vertex takes it from the URL and rejects a body that carries it)
//   - inject anthropic_version
//   - write back max_tokens when applyClaudeThinkingConstraints raised it
//   - remove thinking when the constraints cleared it
//   - strip the premium service-tier fields (speed / inference_geo / service_tier) unless the
//     channel allows them, as the other native Claude channels do
//
// stream is kept: Vertex tells streaming apart by rawPredict / streamRawPredict and the body's
// stream flag agrees with that. It returns (bytes, true) when pass-through is usable and
// (nil, false) when the caller should fall back to struct serialization.
func (p *VertexAIProvider) patchClaudeRequestBody(request *claude.ClaudeRequest) ([]byte, bool) {
	out, ok := p.ReadNativeRawBody("messages")
	if !ok {
		return nil, false
	}

	var err error
	if gjson.GetBytes(out, "model").Exists() {
		if out, err = sjson.DeleteBytes(out, "model"); err != nil {
			return nil, false
		}
	}

	if out, err = sjson.SetBytes(out, "anthropic_version", category.AnthropicVersion); err != nil {
		return nil, false
	}

	if request.MaxTokens > 0 && gjson.GetBytes(out, "max_tokens").Int() != int64(request.MaxTokens) {
		if out, err = sjson.SetBytes(out, "max_tokens", request.MaxTokens); err != nil {
			return nil, false
		}
	}

	if request.Thinking == nil && gjson.GetBytes(out, "thinking").Exists() {
		if out, err = sjson.DeleteBytes(out, "thinking"); err != nil {
			return nil, false
		}
	}

	return p.StripPremiumTierFields(out, base.ClaudePremiumTierRules), true
}

// applyClaudeBetaHeader decides the anthropic-beta header sent to Vertex:
//   - when the channel's ModelHeaders / HeaderOverride set the header explicitly it is kept as is
//   - otherwise the value a HeaderOverride wildcard / regex rule may have copied from the
//     client is merged with the client's own anthropic-beta, tokens Vertex does not accept
//     are dropped, and the header is omitted when nothing is left
func (p *VertexAIProvider) applyClaudeBetaHeader(headers map[string]string) {
	if p.channelSetsHeaderExplicitly("anthropic-beta") {
		return
	}

	// Pass-through rules ("*" / "re:") may already have copied the client value into headers.
	// Take it out and filter it together with the rest so it cannot bypass the deny list.
	values := make([]string, 0, 2)
	for key, value := range headers {
		if strings.EqualFold(key, "anthropic-beta") {
			values = append(values, value)
			delete(headers, key)
		}
	}
	if p.Context != nil && p.Context.Request != nil {
		values = append(values, p.Context.Request.Header.Values("anthropic-beta")...)
	}

	if filtered := filterVertexBetaTokens(values); filtered != "" {
		headers["anthropic-beta"] = filtered
	}
}

// channelSetsHeaderExplicitly reports whether an administrator configured a value for name in
// the channel's ModelHeaders or HeaderOverride (fixed value, {api_key} or {client_header:X}).
// The "*" and "re:" / "regex:" pass-through rules of HeaderOverride do not count.
func (p *VertexAIProvider) channelSetsHeaderExplicitly(name string) bool {
	if p.Channel == nil {
		return false
	}
	for _, raw := range []*string{p.Channel.ModelHeaders, p.Channel.HeaderOverride} {
		if raw == nil || *raw == "" {
			continue
		}
		var configured map[string]string
		if err := json.Unmarshal([]byte(*raw), &configured); err != nil {
			continue
		}
		for key := range configured {
			if strings.EqualFold(strings.TrimSpace(key), name) {
				return true
			}
		}
	}
	return false
}

// filterVertexBetaTokens splits anthropic-beta header values on commas, drops denied tokens and
// duplicates, and joins the rest with commas. It returns "" when nothing is left.
func filterVertexBetaTokens(values []string) string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		for _, token := range strings.Split(value, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			if _, blocked := vertexUnsupportedBetaTokens[token]; blocked {
				continue
			}
			if _, dup := seen[token]; dup {
				continue
			}
			seen[token] = struct{}{}
			out = append(out, token)
		}
	}
	return strings.Join(out, ",")
}
