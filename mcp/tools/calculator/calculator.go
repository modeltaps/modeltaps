// Package calculator 提供了一个基本的计算器工具
// Package calculator provides a basic calculator tool
package calculator

import (
	"context"
	"fmt"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

const NAME = "calculator"

// Calculator 实现了计算器工具
type Calculator struct{}
type calculatorQueryParam struct {
	Operation string  `json:"operation" description:"Operation to perform (add, subtract, multiply, divide)" required:"true" enum:"add,subtract,multiply,divide"`
	X         float64 `json:"x" description:"First number" required:"true"`
	Y         float64 `json:"y" description:"Second number" required:"true"`
}

// GetTool 返回计算器工具的定义
func (c *Calculator) GetTool() *protocol.Tool {
	// 创建一个新的计算器工具
	// Create a new calculator tool
	calculatorTool, _ := protocol.NewTool(
		NAME,
		"Basic arithmetic calculator: add, subtract, multiply and divide",
		calculatorQueryParam{},
	)

	return calculatorTool
}

// HandleRequest 处理计算器工具的请求
func (c *Calculator) HandleRequest(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	t := calculatorQueryParam{}

	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &t); err != nil {
		return nil, err
	}

	// 根据操作类型执行相应的计算
	// Perform calculation based on operation type
	var result float64
	switch t.Operation {
	case "add":
		result = t.X + t.Y
	case "subtract":
		result = t.X - t.Y
	case "multiply":
		result = t.X * t.Y
	case "divide":
		// 检查除数是否为零
		// Check if divisor is zero
		if t.Y == 0 {
			return nil, fmt.Errorf("Cannot divide by zero") // Cannot divide by zero
		}
		result = t.X / t.Y
	}

	// 返回计算结果
	// Return calculation result
	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%.2f", result),
			},
		},
	}, nil
}
