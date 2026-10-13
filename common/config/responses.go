package config

// DefaultNeed2ResponseModels lists models that only work on the Responses API.
// Chat completions requests for them are sent upstream as /v1/responses and the
// result is converted back to chat (relay/chat.go compatibleSend). The channel
// test uses the same table so it exercises the path real traffic takes.
var DefaultNeed2ResponseModels = map[string]struct{}{
	"o3-pro-2025-06-10":                {},
	"o3-pro":                           {},
	"o1-pro-2025-03-19":                {},
	"o1-pro":                           {},
	"o3-deep-research-2025-06-26":      {},
	"o3-deep-research":                 {},
	"o4-mini-deep-research-2025-06-26": {},
	"o4-mini-deep-research":            {},
	"codex-mini-latest":                {},
}

// NeedsResponsesAPI reports whether chat requests for modelName must go through
// the Responses API.
func NeedsResponsesAPI(modelName string) bool {
	_, ok := DefaultNeed2ResponseModels[modelName]
	return ok
}
