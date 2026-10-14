package check_channel

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/providers/gemini"
	"github.com/modeltaps/modeltaps/providers/vertexai_express"
)

// usesClaudeConversion reports whether the check request goes through the gateway's
// OpenAI-to-Claude conversion (claude.ConvertFromChatOpenai). Like Anthropic's own OpenAI
// compatibility layer, that conversion drops response_format, and an unknown role cannot reach the
// upstream, so the results must be read accordingly and not blamed on the upstream.
//
// Only the channel type and the mapped upstream model matter: an OpenAI-compatible relay that sells
// Claude models forwards response_format as is, so support depends on the upstream there.
func usesClaudeConversion(channelType int, upstreamModel string) bool {
	switch channelType {
	case config.ChannelTypeAnthropic, config.ChannelTypeClaudeCode:
		return true
	case config.ChannelTypeBedrock:
		// Bedrock routes by model; the anthropic family goes through the Claude conversion.
		return model_utils.ContainsCaseInsensitive(upstreamModel, "anthropic") ||
			model_utils.HasPrefixCaseInsensitive(upstreamModel, "claude")
	case config.ChannelTypeVertexAI:
		return model_utils.HasPrefixCaseInsensitive(upstreamModel, "claude")
	}
	return false
}

// rewritesUnknownRole reports whether the check request goes through a conversion that turns an
// unknown role into a valid one. The Claude and Gemini conversions both map unknown roles in their
// default branch, so the deliberately invalid "user11" request never makes the upstream fail: the
// upstream only sees a valid request.
//
// Unlike usesClaudeConversion this also covers the Gemini family (native Gemini, Gemini CLI, the
// Gemini models of Vertex AI, Antigravity and Vertex AI Express). Those forward response_format,
// so the JSON check still applies to them.
//
// geminiOpenAIAPI is true when a Gemini channel uses its OpenAI-compatible endpoint (the
// use_openai_api plugin), which forwards the request without the role conversion.
func rewritesUnknownRole(channelType int, upstreamModel string, geminiOpenAIAPI bool) bool {
	if usesClaudeConversion(channelType, upstreamModel) {
		return true
	}
	switch channelType {
	case config.ChannelTypeGemini, config.ChannelTypeVertexAIExpress:
		return !geminiOpenAIAPI
	case config.ChannelTypeGeminiCli, config.ChannelTypeAntigravity:
		return true
	case config.ChannelTypeVertexAI:
		// Same split as providers/vertexai/category.GetCategory; Claude models are covered above.
		return model_utils.HasPrefixCaseInsensitive(upstreamModel, "gemini") ||
			model_utils.HasPrefixCaseInsensitive(upstreamModel, "imagen") ||
			model_utils.HasPrefixCaseInsensitive(upstreamModel, "lyria")
	}
	return false
}

// usesGeminiOpenAIAPI reads the decision the provider factory already made, so it covers the
// cases the plugin alone does not: a service account key forces use_openai_api off, and Vertex AI
// Express never uses it.
func usesGeminiOpenAIAPI(provider any) bool {
	switch p := provider.(type) {
	case *gemini.GeminiProvider:
		return p.UseOpenaiAPI
	case *vertexai_express.VertexAIExpressProvider:
		return p.UseOpenaiAPI
	}
	return false
}
