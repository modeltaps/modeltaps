package safty

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/safty/types"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
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

// extractText collects every human-text string in data and joins them with "\n".
//
// data may be a string, or any value that encodes to JSON (request structs, slices of message
// structs, maps, json.RawMessage); it is normalized through a JSON round trip and walked
// recursively. Strings directly in data, or in arrays directly in data, are text; inside an
// object only strings under textKeys (or in arrays under them) are. Objects are searched at
// every depth, so text nested in messages, content parts, tools and items is found wherever it
// sits. Data URLs are never collected.
func extractText(data interface{}) (string, error) {
	if data == nil {
		return "", nil
	}

	var normalized interface{}
	switch v := data.(type) {
	case string:
		normalized = v
	case []byte:
		normalized = string(v)
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		if err := json.Unmarshal(raw, &normalized); err != nil {
			return "", err
		}
	}

	texts := make([]string, 0)
	collectText(normalized, true, &texts)
	return strings.Join(texts, "\n"), nil
}

// collectText appends the text in v to out. isText says whether a string at this position is
// text: true at the top level and under textKeys, false elsewhere. Arrays keep the setting of
// their position; objects decide per key.
func collectText(v interface{}, isText bool, out *[]string) {
	switch t := v.(type) {
	case string:
		if isText && strings.TrimSpace(t) != "" && !strings.HasPrefix(t, "data:") {
			*out = append(*out, t)
		}
	case []interface{}:
		for _, item := range t {
			collectText(item, isText, out)
		}
	case map[string]interface{}:
		keys := make([]string, 0, len(t))
		for key := range t {
			if !binaryKeys[key] {
				keys = append(keys, key)
			}
		}
		// A fixed order keeps the checked text the same for the same request.
		sort.Strings(keys)
		for _, key := range keys {
			collectText(t[key], textKeys[key], out)
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
