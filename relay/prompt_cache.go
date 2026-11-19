package relay

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
)

// applyPromptCacheMode resolves the request's prompt cache mode (PROTO-9) before the body is
// parsed and records it for the providers and the request log. The X-Modeltaps-Prompt-Cache
// header wins over the API key's setting; an unknown header value is ignored. In "off" mode the
// cache_control markers are removed from the cached body here, once, so every entry format and
// every provider (native passthrough included) sees a request without them.
func applyPromptCacheMode(c *gin.Context) {
	mode, source := config.PromptCacheModeAuto, ""
	if setting, ok := utils.GetGinValue[*model.TokenSetting](c, "token_setting"); ok && setting != nil {
		if m, ok := config.NormalizePromptCacheMode(setting.PromptCache); ok && m != config.PromptCacheModeAuto {
			mode, source = m, "key"
		}
	}
	if h := strings.ToLower(strings.TrimSpace(c.GetHeader(config.PromptCacheHeader))); h != "" {
		if m, ok := config.NormalizePromptCacheMode(h); ok {
			mode, source = m, "header"
		}
	}
	c.Set(config.GinPromptCacheModeKey, mode)
	c.Set(config.GinPromptCacheSourceKey, source)

	if mode != config.PromptCacheModeOff || !strings.Contains(c.ContentType(), "json") {
		return
	}
	// The body may not have been read yet (the distributor reads it only for some requests): read
	// it through the shared cache so setRequest parses the stripped copy.
	body, err := common.ReadBodyRaw(c)
	if err != nil {
		return
	}
	if stripped, changed := StripCacheControl(body); changed {
		c.Set(config.GinRequestBodyKey, stripped)
		c.Set(config.GinPromptCacheStrippedKey, true)
	}
}

// cacheControlParents are the places a cache_control marker can sit in the request formats the
// gateway accepts: the Messages request itself (automatic caching), its tools, system blocks,
// messages, content blocks and the blocks nested in a tool_result; the chat completions messages
// and content parts (OpenRouter style); the Responses input items and their content.
var cacheControlParents = []string{
	"",
	"tools.#",
	"system.#",
	"messages.#",
	"messages.#.content.#",
	"messages.#.content.#.content.#",
	"input.#",
	"input.#.content.#",
}

// StripCacheControl removes every cache_control marker from a request body, leaving the other
// bytes as they are. It reports whether anything was removed.
func StripCacheControl(body []byte) ([]byte, bool) {
	if len(body) == 0 || !gjson.ValidBytes(body) || !strings.Contains(string(body), "cache_control") {
		return body, false
	}
	var paths []string
	for _, parent := range cacheControlParents {
		collectCacheControlPaths(gjson.ParseBytes(body), parent, "", &paths)
	}
	if len(paths) == 0 {
		return body, false
	}
	out := body
	for _, p := range paths {
		if next, err := sjson.DeleteBytes(out, p); err == nil {
			out = next
		}
	}
	return out, len(out) != len(body)
}

// collectCacheControlPaths walks pattern (dot segments, "#" for every array element) from value
// and appends the sjson path of each cache_control key found at its end.
func collectCacheControlPaths(value gjson.Result, pattern, prefix string, paths *[]string) {
	if pattern == "" {
		if value.IsObject() && value.Get("cache_control").Exists() {
			*paths = append(*paths, joinPath(prefix, "cache_control"))
		}
		return
	}
	segment, rest, _ := strings.Cut(pattern, ".")
	if segment != "#" {
		collectCacheControlPaths(value.Get(segment), rest, joinPath(prefix, segment), paths)
		return
	}
	if !value.IsArray() {
		return
	}
	for i, item := range value.Array() {
		collectCacheControlPaths(item, rest, joinPath(prefix, strconv.Itoa(i)), paths)
	}
}

func joinPath(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	return prefix + "." + segment
}
