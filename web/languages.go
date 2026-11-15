// Package web holds what the server reads from the frontend sources.
package web

import _ "embed"

// LanguagesJSON is the list of the console's languages (src/i18n/languages.json), shared with the
// frontend so a language is added in one place. common/lang reads it.
//
//go:embed src/i18n/languages.json
var LanguagesJSON []byte
