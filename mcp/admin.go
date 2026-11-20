package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/mcp/admintools"
)

// NewAdminMcpServer is the admin MCP endpoint (/mcp/admin): a server of its own, so the tools of
// the site's settings are never listed to ordinary users of /mcp. Its tools only read.
func NewAdminMcpServer() *Server {
	sseTransport, sseHandler, err := transport.NewSSEServerTransportAndHandler("/mcp/admin/message")
	if err != nil {
		logger.SysLog(fmt.Sprintf("new admin mcp sse transport: %v", err))
		return nil
	}
	streamableTransport, streamableHandler, err := transport.NewStreamableHTTPServerTransportAndHandler()
	if err != nil {
		logger.SysLog(fmt.Sprintf("new admin mcp streamable transport: %v", err))
		return nil
	}
	info := server.WithServerInfo(protocol.Implementation{Name: config.SystemName + "-ADMIN MCP SERVER", Version: config.Version})
	sseServer, err := server.NewServer(sseTransport, info)
	if err != nil {
		logger.SysLog(fmt.Sprintf("new admin mcp sse server: %v", err))
		return nil
	}
	streamableServer, err := server.NewServer(streamableTransport, info)
	if err != nil {
		logger.SysLog(fmt.Sprintf("new admin mcp streamable server: %v", err))
		return nil
	}
	s := &Server{sseServer, sseHandler, streamableServer, streamableHandler}
	for _, tool := range admintools.Tools {
		s.SSEServer.RegisterTool(tool.GetTool(), tool.HandleRequest)
		s.StreamableServer.RegisterTool(tool.GetTool(), tool.HandleRequest)
	}
	return s
}

// The settings assistant of the console (⌘K "Ask AI") runs its tool loop in the browser, calling
// a model through the relay with the admin's playground key. It reads the same read-only tools as
// the admin MCP endpoint through these two plain JSON routes, under the same root permission.

// adminToolFunction is a tool as the chat completions API lists it.
type adminToolFunction struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

// HandleAdminToolList returns the admin tools in the chat completions "tools" format.
func HandleAdminToolList(c *gin.Context) {
	out := make([]adminToolFunction, 0, len(admintools.Tools))
	for _, tool := range admintools.Tools {
		t := tool.GetTool()
		var f adminToolFunction
		f.Type = "function"
		f.Function.Name = t.Name
		f.Function.Description = t.Description
		f.Function.Parameters = t.InputSchema
		out = append(out, f)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": out})
}

type adminToolCall struct {
	Name string `json:"name"`
	// Arguments are the JSON arguments the model wrote: a JSON string (as chat completions sends
	// them) or an object.
	Arguments json.RawMessage `json:"arguments"`
}

// HandleAdminToolCall runs one admin tool and returns its text result.
func HandleAdminToolCall(c *gin.Context) {
	var call adminToolCall
	if err := c.ShouldBindJSON(&call); err != nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "invalid parameters", nil)
		return
	}
	var tool admintools.Tool
	for _, t := range admintools.Tools {
		if t.GetTool().Name == call.Name {
			tool = t
			break
		}
	}
	if tool == nil {
		common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "unknown tool "+call.Name, nil)
		return
	}
	args := []byte(strings.TrimSpace(string(call.Arguments)))
	if len(args) > 0 && args[0] == '"' {
		var text string
		if err := json.Unmarshal(args, &text); err != nil {
			common.APIRespondWithCode(c, http.StatusOK, common.APICodeInvalidParams, "invalid arguments", nil)
			return
		}
		args = []byte(text)
	}
	if len(args) == 0 || string(args) == "null" {
		args = []byte("{}")
	}
	result, err := tool.HandleRequest(c.Request.Context(), &protocol.CallToolRequest{Name: call.Name, RawArguments: args})
	if err != nil {
		// The model reads the error and can try again with other arguments.
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"text": "error: " + err.Error()}})
		return
	}
	var text strings.Builder
	for _, content := range result.Content {
		if tc, ok := content.(*protocol.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"text": text.String()}})
}
