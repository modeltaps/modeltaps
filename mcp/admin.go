package mcp

import (
	"fmt"

	"github.com/ThinkInAIXYZ/go-mcp/protocol"
	"github.com/ThinkInAIXYZ/go-mcp/server"
	"github.com/ThinkInAIXYZ/go-mcp/transport"
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
