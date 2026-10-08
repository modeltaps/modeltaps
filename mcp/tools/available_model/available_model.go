package available_model

import (
	"context"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay"
	"errors"
	"fmt"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
)

const NAME = "available_model"

type AvailableModel struct{}

type modelQueryParam struct {
	GroupName string `json:"groupName" description:"Group name, empty by default" default:"" required:"false"`
}

// GetTool 返回模型查询工具的定义
func (c *AvailableModel) GetTool() *protocol.Tool {
	availableTool, _ := protocol.NewTool(
		NAME,
		"List available models with provider, name, input price and output price",
		modelQueryParam{},
	)
	return availableTool
}

func (c *AvailableModel) HandleRequest(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	id := ctx.Value("id")
	if id == nil {
		logger.SysLog("User not found, failed to get id from ctx")
		return nil, errors.New("User not found")
	}
	userId, ok := id.(int)
	user, err := model.GetUserById(userId, false)
	if err != nil {
		logger.SysLog("Failed to get user info")
		return nil, errors.New("User not found")
	}
	if !ok {
		logger.SysLog("User not found, invalid id type")
		return nil, errors.New("User not found")
	}
	query := modelQueryParam{}
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &query); err != nil {
		logger.SysLog(fmt.Sprintf("Error: %s", err.Error()))
		return nil, err
	}
	if query.GroupName == "" {
		query.GroupName = user.Group
	}
	models := relay.GetAvailableModels(query.GroupName)
	// 转成字符串
	modelsStr := fmt.Sprintf("Models in group [%s]\n", query.GroupName)
	for _, m := range models {
		modelsStr += fmt.Sprintf("Provider:%s Name:%s Input price:$%f/1K Output price:$%f/1K \n", m.OwnedBy, m.Price.Model, m.Price.Input*0.002, m.Price.Output*0.002)
	}
	// 返回查询结果
	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%s", modelsStr),
			},
		},
	}, nil
}
