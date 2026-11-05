package relay

import (
	"regexp"
	"strings"

	"github.com/modeltaps/modeltaps/common/session"

	"github.com/tidwall/gjson"
)

// 自动路由只看会话的第一条用户消息。四种请求体形状按字段识别，与入口路径无关：
//   - messages[]  Chat Completions 与 Claude Messages（content 为字符串或 text 块数组）
//   - input       Responses（字符串，或 role=user 的消息项数组，content 为字符串或 input_text 块）
//   - contents[]  Gemini（role=user 或无 role，parts[].text）
//   - prompt      旧版 completions
// 图片、文件、工具调用与工具结果一律忽略；首条用户消息里没有任何文本时视为空输入。

// agentContextPattern matches the context blocks that coding agents inject as user content:
// Claude Code's <system-reminder> blocks, and Codex's <environment_context>, <user_instructions>
// and "# AGENTS.md instructions for <dir>" + <INSTRUCTIONS> blocks. They are templates rather than
// what the user typed; left in, every request looks long and complex. Codex even sends them as a
// user message of their own ahead of the real prompt, so a message holding nothing else is skipped.
var agentContextPattern = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>` +
	`|<environment_context>.*?</environment_context>` +
	`|<user_instructions>.*?</user_instructions>` +
	`|(?:# AGENTS\.md instructions for [^\n]*\s*)?<INSTRUCTIONS>.*?</INSTRUCTIONS>`)

// stripAgentContext removes agent-injected context blocks and surrounding whitespace.
func stripAgentContext(text string) string {
	return strings.TrimSpace(agentContextPattern.ReplaceAllString(text, ""))
}

// firstUserMessageText 从请求体里取第一条用户消息的纯文本，剥掉 Agent 注入的上下文块并按 maxChars 截断。
func firstUserMessageText(body []byte, maxChars int) string {
	return truncateRunes(extractFirstUserText(body), maxChars)
}

func truncateRunes(text string, maxChars int) string {
	if maxChars > 0 {
		if runes := []rune(text); len(runes) > maxChars {
			text = string(runes[:maxChars])
		}
	}
	return text
}

func extractFirstUserText(body []byte) string {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return ""
	}
	if messages := gjson.GetBytes(body, "messages"); messages.IsArray() {
		return firstUserInMessages(messages, "text")
	}
	if input := gjson.GetBytes(body, "input"); input.Exists() {
		if input.Type == gjson.String {
			return stripAgentContext(input.String())
		}
		if input.IsArray() {
			return firstUserInMessages(input, "input_text")
		}
		return ""
	}
	if contents := gjson.GetBytes(body, "contents"); contents.IsArray() {
		return firstUserInGeminiContents(contents)
	}
	if prompt := gjson.GetBytes(body, "prompt"); prompt.Type == gjson.String {
		return stripAgentContext(prompt.String())
	}
	return ""
}

// firstUserInMessages 在 role=user 的消息里找第一条含文本的；textType 是文本块的 type 值
// （Chat / Claude 为 text，Responses 为 input_text）。
func firstUserInMessages(messages gjson.Result, textType string) string {
	found := ""
	messages.ForEach(func(_, message gjson.Result) bool {
		if message.Get("role").String() != "user" {
			return true
		}
		content := message.Get("content")
		text := stripAgentContext(contentText(content, textType))
		if text == "" {
			// 只含工具结果 / 图片 / Agent 注入上下文的用户消息不算，继续往后找第一条有文字的。
			return true
		}
		found = text
		return false
	})
	return found
}

// contentText 把 content（字符串或块数组）里的文本块拼起来。
func contentText(content gjson.Result, textType string) string {
	if content.Type == gjson.String {
		return content.String()
	}
	if !content.IsArray() {
		return ""
	}
	parts := make([]string, 0, 2)
	content.ForEach(func(_, block gjson.Result) bool {
		blockType := block.Get("type").String()
		if blockType == textType || blockType == "text" {
			if text := block.Get("text").String(); text != "" {
				parts = append(parts, text)
			}
		}
		return true
	})
	return strings.Join(parts, "\n")
}

func firstUserInGeminiContents(contents gjson.Result) string {
	found := ""
	contents.ForEach(func(_, content gjson.Result) bool {
		if role := content.Get("role").String(); role != "" && role != "user" {
			return true
		}
		parts := make([]string, 0, 2)
		content.Get("parts").ForEach(func(_, part gjson.Result) bool {
			if text := part.Get("text").String(); text != "" {
				parts = append(parts, text)
			}
			return true
		})
		text := stripAgentContext(strings.Join(parts, "\n"))
		if text == "" {
			return true
		}
		found = text
		return false
	})
	return found
}

// isAgentAuxiliaryRequest reports whether the request is a Claude Code side request rather than
// a turn of the agent loop. Claude Code tags every request with the session in metadata.user_id;
// its agent turns always declare tools, while side requests (session title, summaries) carry
// none. Those side requests share the session and often arrive before the first turn, and
// their first user message is a Claude Code template, not what the user typed.
func isAgentAuxiliaryRequest(body []byte) bool {
	if len(body) == 0 || !gjson.ValidBytes(body) {
		return false
	}
	userID := gjson.GetBytes(body, "metadata.user_id")
	if !userID.Exists() || session.ExtractSessionIDFromMetadataValue(userID.Value()) == "" {
		return false
	}
	tools := gjson.GetBytes(body, "tools")
	return !tools.IsArray() || len(tools.Array()) == 0
}
