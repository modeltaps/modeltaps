package relay

import (
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
)

// 自动路由只看会话的第一条用户消息。四种请求体形状按字段识别，与入口路径无关：
//   - messages[]  Chat Completions 与 Claude Messages（content 为字符串或 text 块数组）
//   - input       Responses（字符串，或 role=user 的消息项数组，content 为字符串或 input_text 块）
//   - contents[]  Gemini（role=user 或无 role，parts[].text）
//   - prompt      旧版 completions
// 图片、文件、工具调用与工具结果一律忽略；首条用户消息里没有任何文本时视为空输入。

// systemReminderPattern 匹配编码 Agent（如 Claude Code）注入到首条用户消息里的系统提醒块：
// 它们是模板而不是用户的话，留着会让所有请求都显得又长又复杂。
var systemReminderPattern = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)

// firstUserMessageText 从请求体里取第一条用户消息的纯文本，剥掉系统提醒块并按 maxChars 截断。
func firstUserMessageText(body []byte, maxChars int) string {
	text := extractFirstUserText(body)
	text = systemReminderPattern.ReplaceAllString(text, "")
	text = strings.TrimSpace(text)
	if maxChars > 0 {
		runes := []rune(text)
		if len(runes) > maxChars {
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
			return input.String()
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
		return prompt.String()
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
		text := contentText(content, textType)
		if text == "" {
			// 只含工具结果 / 图片的用户消息不算，继续往后找第一条有文字的。
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
		if len(parts) == 0 {
			return true
		}
		found = strings.Join(parts, "\n")
		return false
	})
	return found
}
