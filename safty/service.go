package safty

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/safty/types"

	"github.com/tidwall/gjson"
)

// RegisterTool 注册一个新的安全检查器
// 参数:
//   - name: 检查器名称
//   - tool: 检查器实例
func RegisterTool(name string, tool SaftyTool) {
	Tools[name] = tool
	logger.SysLog(fmt.Sprintf("Safety tool %s registered", name))
}

// GetTool 根据名称获取安全检查器
// 参数:
//   - name: 检查器名称
//
// 返回值:
//   - SaftyTool: 检查器实例
//   - error: 如果检查器不存在则返回错误
func getTool(name string) (SaftyTool, error) {
	tool, ok := Tools[name]
	if !ok {
		return nil, errors.New("safety tool not found")
	}
	return tool, nil
}

// textKeys are the JSON keys whose string values carry human-written text in the request
// formats the gateway accepts: message and content-part text (OpenAI, Claude, Gemini,
// Responses, Realtime), prompts, system prompts and instructions, rerank queries and documents,
// tool descriptions, and tool-call arguments and outputs the client sends back. Strings under
// any other key (model names, roles, ids, enum values) are not text and are not checked, so a
// keyword list cannot match them by accident.
var textKeys = map[string]bool{
	"text":            true,
	"content":         true,
	"input":           true,
	"prompt":          true,
	"negative_prompt": true,
	"negativePrompt":  true,
	"instructions":    true,
	"system":          true,
	"query":           true,
	"documents":       true,
	"description":     true,
	"arguments":       true,
	"output":          true,
	"transcript":      true,
	"suffix":          true,
}

// binaryKeys hold image, audio and file payloads (URLs, base64 blobs). Their subtrees are
// skipped whole.
var binaryKeys = map[string]bool{
	"image_url":   true,
	"url":         true,
	"data":        true,
	"b64_json":    true,
	"file_data":   true,
	"file":        true,
	"image":       true,
	"mask":        true,
	"audio":       true,
	"input_audio": true,
	"inline_data": true,
	"inlineData":  true,
}

// opaqueTextKeys hold tool calls, tool results and code that the client sends back to the
// model: Gemini function calls and responses (whose args and response objects use any keys the
// tool defines) and executable code with its result. Every string anywhere in their subtrees is
// text, whatever its key; binaryKeys subtrees inside them are still skipped.
var opaqueTextKeys = map[string]bool{
	"functionCall":          true,
	"function_call":         true,
	"functionResponse":      true,
	"function_response":     true,
	"executableCode":        true,
	"executable_code":       true,
	"codeExecutionResult":   true,
	"code_execution_result": true,
}

// textMode says which strings at a position of the JSON tree are text.
type textMode int

const (
	textNone textMode = iota // only strings under textKeys, at any depth below
	textHere                 // strings here and in arrays here; objects decide per key
	textAll                  // every string in the subtree
)

// extractText collects every human-text string in data and joins them with "\n".
//
// data may be a string, or any value that encodes to JSON (request structs, slices of message
// structs, maps, json.RawMessage); it is encoded to JSON and walked recursively. Strings
// directly in data, or in arrays directly in data, are text; inside an object only strings
// under textKeys (or in arrays under them) are, plus every string under opaqueTextKeys. Objects
// are searched at every depth, so text nested in messages, content parts, tools and items is
// found wherever it sits. Data URLs are never collected.
func extractText(data interface{}) (string, error) {
	if data == nil {
		return "", nil
	}

	var raw []byte
	switch v := data.(type) {
	case string:
		return collectString(v), nil
	case []byte:
		return collectString(string(v)), nil
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		raw = encoded
	}

	return strings.Join(collectJSONText(gjson.ParseBytes(raw)), "\n"), nil
}

func collectString(s string) string {
	texts := make([]string, 0, 1)
	appendText(s, &texts)
	return strings.Join(texts, "\n")
}

// ExtractJSONText returns the human-text strings of a raw JSON value, with the rules of
// extractText (strings at the top level and in top-level arrays are text). It reads the bytes
// with gjson without decoding them, so large inline media costs no allocation, and it visits
// every occurrence of a duplicated key: a decoder keeps only one of them and an upstream parser
// may keep the other, so each one must be checked.
func ExtractJSONText(raw string) []string {
	return collectJSONText(gjson.Parse(raw))
}

func collectJSONText(v gjson.Result) []string {
	texts := make([]string, 0)
	collectText(v, textHere, &texts)
	return texts
}

func appendText(s string, out *[]string) {
	if strings.TrimSpace(s) != "" && !strings.HasPrefix(s, "data:") {
		*out = append(*out, s)
	}
}

// collectText appends the text in v to out; mode says which strings at this position are text.
// Arrays keep the mode of their position; objects decide per key unless the mode is textAll.
func collectText(v gjson.Result, mode textMode, out *[]string) {
	switch {
	case v.Type == gjson.String:
		if mode != textNone {
			appendText(v.String(), out)
		}
	case v.IsArray():
		v.ForEach(func(_, item gjson.Result) bool {
			collectText(item, mode, out)
			return true
		})
	case v.IsObject():
		type member struct {
			key   string
			value gjson.Result
		}
		members := make([]member, 0, 8)
		v.ForEach(func(key, value gjson.Result) bool {
			if name := key.String(); !binaryKeys[name] {
				members = append(members, member{name, value})
			}
			return true
		})
		// A fixed order keeps the checked text the same for the same request; the stable sort
		// keeps duplicates of a key in document order.
		sort.SliceStable(members, func(i, j int) bool { return members[i].key < members[j].key })
		for _, m := range members {
			childMode := textNone
			switch {
			case mode == textAll || opaqueTextKeys[m.key]:
				childMode = textAll
			case textKeys[m.key]:
				childMode = textHere
			}
			collectText(m.value, childMode, out)
		}
	}
}

// GetAllSafeToolsName 获取所有可用的安全检查器的名称
func GetAllSafeToolsName() []string {
	var toolsName = make([]string, 0)
	for name, _ := range Tools {
		toolsName = append(toolsName, name)
	}
	return toolsName
}

// CheckContentByToolName 使用指定的检查器检查内容
// 参数:
//   - toolName: 要使用的检查器名称
//   - content: 要检查的内容，可以是任意类型
//
// 返回值:
//   - bool: 内容是否安全
//   - CheckResult: 检查结果，包括是否安全、风险级别、原因和详细信息
//   - error: 检查过程中发生的错误
func CheckContentByToolName(toolName string, content interface{}) (types.CheckResult, error) {
	if config.EnableSafe == false {
		result := types.CheckResult{
			IsSafe:    true,
			RiskLevel: 0,
			Reason:    "",
			Details:   make([]string, 0),
		}
		return result, nil
	}
	result := types.CheckResult{}
	result.RiskLevel = 0
	result.IsSafe = false
	result.Reason = "not safe"
	result.Details = make([]string, 0)
	tool, err := getTool(toolName)
	if err != nil {
		result.Reason = "tool not found"
		logger.SysLog(fmt.Sprintf("Safety tool %s not found", toolName))
		return result, err
	}

	// 将内容转换为字符串
	contentStr, err := extractText(content)
	if err != nil {
		result.Reason = "convert to string failed"
		logger.SysLog(fmt.Sprintf("Safety tool %s convert to string failed", toolName))
		return result, err
	}

	// 如果内容为空，直接返回安全
	if contentStr == "" {
		result.RiskLevel = 0
		result.IsSafe = true
		result.Reason = ""
		return result, nil
	}

	return tool.Check(contentStr)
}

// CheckContent 使用指定的检查器检查内容
// 参数:
//   - content: 要检查的内容，可以是任意类型
//
// 返回值:
//   - bool: 内容是否安全
//   - CheckResult: 检查结果，包括是否安全、风险级别、原因和详细信息
//   - error: 检查过程中发生的错误
func CheckContent(content interface{}) (types.CheckResult, error) {
	if config.EnableSafe == false {
		result := types.CheckResult{
			IsSafe:    true,
			RiskLevel: 0,
			Code:      types.SafeDefaultSuccessCode,
			Reason:    types.SafeDefaultSuccessMessage,
			Details:   make([]string, 0),
		}
		return result, nil
	}
	result := types.CheckResult{}
	result.RiskLevel = 1
	result.IsSafe = false
	result.Code = types.SafeDefaultSuccessCode
	result.Reason = types.SafeDefaultErrorMessage
	result.Details = make([]string, 0)
	tool, err := getTool(config.SafeToolName)
	if err != nil {
		result.RiskLevel = 1
		result.Code = types.SafeDefaultErrorCode
		result.Reason = types.SafeDefaultErrorMessage
		logger.SysLog(fmt.Sprintf("Safety tool %s not found", config.SafeToolName))
		return result, err
	}

	// 将内容转换为字符串
	contentStr, err := extractText(content)
	if err != nil {
		result.RiskLevel = 1
		result.Code = types.SafeDefaultErrorCode
		result.Reason = types.SafeDefaultErrorMessage
		logger.SysLog(fmt.Sprintf("Safety tool %s convert to string failed", config.SafeToolName))
		return result, err
	}

	// 如果内容为空，直接返回安全
	if contentStr == "" {
		result.RiskLevel = 0
		result.IsSafe = true
		result.Code = types.SafeDefaultSuccessCode
		result.Reason = types.SafeDefaultSuccessMessage
		return result, nil
	}

	return tool.Check(contentStr)
}
