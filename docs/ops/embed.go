// Package opsdocs embeds the English operator docs, for the admin MCP tool that searches them
// (settings_docs). The other languages stay files only.
package opsdocs

import "embed"

//go:embed en
var English embed.FS
