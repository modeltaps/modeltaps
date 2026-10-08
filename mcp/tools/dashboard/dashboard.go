package dashboard

import (
	"context"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"
	"errors"
	"fmt"
	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"time"
)

const NAME = "dashboard"

// Dashboar
// Dashboard
type Dashboard struct{}

type dashboardQueryParam struct {
	StartOfDay string `json:"startOfDay" description:"Start date, format 2025-01-01, defaults to empty string" default:"" required:"false"`
	EndOfDay   string `json:"endOfDay" description:"End date, format 2025-01-01, defaults to empty string" default:"" required:"false"`
}

// GetTool 返回模型查询工具的定义
func (c *Dashboard) GetTool() *protocol.Tool {
	dashboardTool, _ := protocol.NewTool(
		NAME,
		"Usage query: returns model usage for the given period. Accepts start and end dates; if omitted, returns the last seven days",
		dashboardQueryParam{},
	)
	return dashboardTool
}

func (c *Dashboard) HandleRequest(ctx context.Context, req *protocol.CallToolRequest) (*protocol.CallToolResult, error) {
	id := ctx.Value("id")
	if id == nil {
		logger.SysLog("User not found, failed to get id from ctx")
		return nil, errors.New("User not found")
	}
	userId, ok := id.(int)
	if !ok {
		logger.SysLog("User not found, invalid id type")
		return nil, errors.New("User not found")
	}
	query := dashboardQueryParam{}
	if err := protocol.VerifyAndUnmarshal(req.RawArguments, &query); err != nil {
		logger.SysLog(fmt.Sprintf("Error: %s", err.Error()))
		return nil, err
	}
	if query.StartOfDay == "" || query.EndOfDay == "" {
		query.StartOfDay = time.Now().AddDate(0, 0, -7).Format("2006-01-02")
		query.EndOfDay = time.Now().Format("2006-01-02")
	}

	dashboards, err := model.GetUserModelStatisticsByPeriod(userId, query.StartOfDay, query.EndOfDay)
	if err != nil {
		return nil, err
	}
	// 转成字符串
	result := fmt.Sprintf("%s-%s invoice\n", query.StartOfDay, query.EndOfDay)
	for _, m := range dashboards {
		quotaCost := float64(m.Quota) * 0.002 / 1000
		result += fmt.Sprintf("Date:%v RequestCount:%d RequestTime(ms):%d Quota($):%.6f ModelName:%s InputToken:%d OutputToken:%d \n", m.Date, m.RequestCount, m.RequestTime, quotaCost, m.ModelName, m.PromptTokens, m.CompletionTokens)
	}
	// 返回查询结果
	return &protocol.CallToolResult{
		Content: []protocol.Content{
			&protocol.TextContent{
				Type: "text",
				Text: fmt.Sprintf("%v", result),
			},
		},
	}, nil
}
