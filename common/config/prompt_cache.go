package config

// Prompt cache modes of a request (PROTO-9). The mode is resolved once per request, before the
// body is parsed, from the X-Modeltaps-Prompt-Cache header, then the API key's setting; "auto"
// leaves the decision to the channel's prompt_caching plugin and the site-wide switch.
//   - auto: follow the channel and the site-wide switch.
//   - on:   add cache breakpoints to Claude requests that carry none, whatever the channel says.
//   - off:  remove every cache_control marker from the request and add none. It cannot turn off
//     a provider's automatic prefix cache (OpenAI, DeepSeek, Gemini implicit caching).
const (
	PromptCacheModeAuto = "auto"
	PromptCacheModeOn   = "on"
	PromptCacheModeOff  = "off"

	// PromptCacheHeader lets one request override its key's mode; it is never sent upstream.
	PromptCacheHeader = "X-Modeltaps-Prompt-Cache"

	// GinPromptCacheModeKey holds the resolved mode; GinPromptCacheSourceKey where it came from
	// ("header", "key", or "" for the default); GinPromptCacheStrippedKey is true when "off"
	// removed markers the client sent.
	GinPromptCacheModeKey     = "prompt_cache_mode"
	GinPromptCacheSourceKey   = "prompt_cache_source"
	GinPromptCacheStrippedKey = "prompt_cache_stripped"
)

// NormalizePromptCacheMode returns the mode a stored or requested value names, and false for a
// value that names none. The empty string is "auto".
func NormalizePromptCacheMode(v string) (string, bool) {
	switch v {
	case "", PromptCacheModeAuto:
		return PromptCacheModeAuto, true
	case PromptCacheModeOn, PromptCacheModeOff:
		return v, true
	}
	return "", false
}
