package settings

import (
	"bytes"
	"encoding/json"
)

// GeneratedPath is where the console's copy of the catalog lives, from the repository root.
func GeneratedPath() string { return "web/src/generated/settings-catalog.json" }

// GeneratedJSON is the console's copy of the catalog.
func GeneratedJSON() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(map[string]any{
		"_comment": "Generated from common/settings by `go generate ./common/settings`. Do not edit.",
		"settings": All(),
	}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
