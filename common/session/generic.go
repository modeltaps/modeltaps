package session

import (
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// sessionHeaders are request headers coding clients use to name their session
// (Codex CLI sends session_id; others use one of the variants).
var sessionHeaders = []string{"session_id", "session-id", "x-session-id", "x-claude-code-session-id"}

// GenerateGenericSessionHash identifies a client session for channel affinity on any
// channel type. The key is scoped to the API key and the model, because upstream prompt
// caches are per model and two users must never share a binding. It returns "" when the
// request carries nothing to identify the session.
func GenerateGenericSessionHash(c *gin.Context, modelName string) string {
	if c == nil {
		return ""
	}
	id := genericSessionID(c)
	if id == "" {
		return ""
	}
	return HashContent(strconv.Itoa(c.GetInt("token_id")) + "\x00" + modelName + "\x00" + id)
}

// genericSessionID picks the most specific session marker in the request: the Claude
// Code session in metadata.user_id, a session header, the OpenAI prompt_cache_key, and
// finally the prompt prefix (system prompt plus the first user turn), since requests that
// share a prefix share the upstream cache entry.
func genericSessionID(c *gin.Context) string {
	var body []byte
	if raw, ok := c.Get(config.GinRequestBodyKey); ok {
		body, _ = raw.([]byte)
	}

	if userID := gjson.GetBytes(body, "metadata.user_id"); userID.Exists() {
		if sid := ExtractSessionIDFromMetadataValue(userID.Value()); sid != "" {
			return "claude:" + sid
		}
	}
	if c.Request != nil {
		for _, header := range sessionHeaders {
			if v := strings.TrimSpace(c.GetHeader(header)); v != "" {
				return "header:" + v
			}
		}
	}
	if key := gjson.GetBytes(body, "prompt_cache_key").String(); key != "" {
		return "cache_key:" + key
	}
	if prefix := promptPrefix(body); prefix != "" {
		return "prefix:" + HashContent(prefix)
	}
	return ""
}

// promptPrefix concatenates the system prompt and the turns up to the first user turn
// for the Claude Messages, OpenAI Chat / Responses and Gemini request shapes.
func promptPrefix(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var b strings.Builder
	for _, path := range []string{"system", "instructions", "systemInstruction.parts", "system_instruction.parts"} {
		b.WriteString(textOf(gjson.GetBytes(body, path)))
	}

	turns := gjson.GetBytes(body, "messages")
	if !turns.Exists() {
		turns = gjson.GetBytes(body, "input")
	}
	if !turns.Exists() {
		turns = gjson.GetBytes(body, "contents")
	}
	if turns.Type == gjson.String {
		b.WriteString(turns.String())
		return b.String()
	}
	for _, turn := range turns.Array() {
		role := turn.Get("role").String()
		b.WriteString(role)
		b.WriteString(textOf(turn.Get("content")))
		b.WriteString(textOf(turn.Get("parts")))
		if role == "user" || role == "" {
			break
		}
	}
	return b.String()
}

// textOf flattens a string, a content block, or an array of blocks / parts to text.
func textOf(r gjson.Result) string {
	switch {
	case !r.Exists():
		return ""
	case r.Type == gjson.String:
		return r.String()
	case r.IsArray():
		var b strings.Builder
		for _, item := range r.Array() {
			b.WriteString(textOf(item))
		}
		return b.String()
	case r.IsObject():
		return r.Get("text").String()
	}
	return ""
}
