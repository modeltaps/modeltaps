package vercel

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Claude thinking through the gateway's chat completions. The newest Claude models (Opus 4.7 and
// later, Sonnet 5 and later, Haiku 5 and later, Fable, Mythos) return empty thinking text unless
// the request asks for a summary, and the gateway's chat API has no field for that: reasoning_effort
// and reasoning turn thinking on, but its text stays empty, so a client that asked for reasoning
// shows a long pause and then the answer. The gateway takes the Anthropic setting as
// providerOptions.anthropic.thinking (checked 2026-11-24 with anthropic/claude-opus-5.5:
// reasoning_effort alone streamed an empty `reasoning`; with {type: "adaptive", display:
// "summarized"} it streamed the summary, which the chat handler passes on as reasoning_content).
// Display changes visibility only; the thinking is billed the same.

const chatCompletionsPath = "/v1/chat/completions"

// thinkingDisplayHook adds providerOptions.anthropic.thinking = {type: "adaptive", display:
// "summarized"} to a chat completions request that asks for reasoning (reasoning_effort or a
// reasoning object) from one of those models, unless the client set its own Anthropic thinking.
func thinkingDisplayHook(req *http.Request) error {
	if req.Method != http.MethodPost || req.Body == nil || !strings.HasSuffix(req.URL.Path, chatCompletionsPath) {
		return nil
	}
	if ct := req.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "json") {
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, maxHostOrderedBody+1))
	_ = req.Body.Close()
	if err != nil {
		return err
	}
	if len(body) <= maxHostOrderedBody {
		body = withThinkingDisplay(body)
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
	return nil
}

// withThinkingDisplay returns the body with the Anthropic thinking summary requested, or the body
// unchanged when it does not apply.
func withThinkingDisplay(body []byte) []byte {
	if !gjson.ValidBytes(body) {
		return body
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		return body
	}
	name := root.Get("model").String()
	if !strings.HasPrefix(strings.ToLower(name), "anthropic/") || !claude.ThinkingHiddenByDefault(name) {
		return body
	}
	if !root.Get("reasoning_effort").Exists() && !root.Get("reasoning").IsObject() {
		return body
	}
	if root.Get("providerOptions.anthropic.thinking").Exists() {
		return body
	}
	out, err := sjson.SetBytes(body, "providerOptions.anthropic.thinking", map[string]string{"type": "adaptive", "display": "summarized"})
	if err != nil {
		return body
	}
	return out
}

// chainHooks runs request hooks in order, skipping nil ones and stopping at the first error.
func chainHooks(hooks ...func(*http.Request) error) func(*http.Request) error {
	return func(req *http.Request) error {
		for _, hook := range hooks {
			if hook == nil {
				continue
			}
			if err := hook(req); err != nil {
				return err
			}
		}
		return nil
	}
}
