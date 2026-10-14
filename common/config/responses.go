package config

import (
	"encoding/json"
	"strings"
)

// DefaultNeed2ResponseModels lists models that only work on the Responses API.
// Chat completions requests for them are sent upstream as /v1/responses and the
// result is converted back to chat (relay/chat.go compatibleSend). The channel
// test uses the same table so it exercises the path real traffic takes.
// A channel can extend or trim this list with its need2response_models setting.
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
// the Responses API when the channel has no setting of its own.
func NeedsResponsesAPI(modelName string) bool {
	_, ok := DefaultNeed2ResponseModels[modelName]
	return ok
}

// ParseNeed2ResponseModels splits a channel's need2response_models setting into
// trimmed, non-empty entries. Entries are separated by newlines or commas; a JSON
// array of strings is accepted as well.
func ParseNeed2ResponseModels(data string) []string {
	trimmed := strings.TrimSpace(data)
	if trimmed == "" {
		return nil
	}

	var parts []string
	if !strings.HasPrefix(trimmed, "[") || json.Unmarshal([]byte(trimmed), &parts) != nil {
		parts = strings.FieldsFunc(trimmed, func(r rune) bool {
			return r == '\n' || r == ','
		})
	}

	models := make([]string, 0, len(parts))
	for _, part := range parts {
		if name := strings.TrimSpace(part); name != "" {
			models = append(models, name)
		}
	}
	return models
}
