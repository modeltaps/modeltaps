package middleware

import (
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/agentclient"
	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// AgentClientEndpoint marks a request that came in on a client endpoint and rewrites its path
// to the bare path that serves the same protocol (for example /claude-code/v1/messages to
// /claude/v1/messages, /codex/v1/responses to /v1/responses). It runs first in the endpoint's
// chain, so the auth, routing, billing and error handling downstream, which choose by path,
// behave exactly as on the bare path. The access log keeps the path the client called.
func AgentClientEndpoint(client, endpointPrefix, barePrefix string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(agentclient.EndpointContextKey, client)
		if rest, ok := strings.CutPrefix(c.Request.URL.Path, endpointPrefix); ok {
			c.Request.URL.Path = barePrefix + rest
			if c.Request.URL.RawPath != "" {
				c.Request.URL.RawPath = barePrefix + strings.TrimPrefix(c.Request.URL.RawPath, endpointPrefix)
			}
		}
		c.Next()
	}
}

// ClaudeCodeModelID replaces the display model ID that /claude-code/v1/models lists
// (claude-code/<model>[1m]) with the real model name in the request body before the relay reads
// it, so the upstream, billing and logs all see the real model. It runs after authentication:
// the body is not read for a request that is refused anyway.
func ClaudeCodeModelID() gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := common.ReadBodyRaw(c)
		if err != nil || len(body) == 0 {
			c.Next()
			return
		}
		modelResult := gjson.GetBytes(body, "model")
		if modelResult.Type != gjson.String {
			c.Next()
			return
		}
		if name := agentclient.StripClaudeCodeModelID(modelResult.String()); name != modelResult.String() {
			if rewritten, err := sjson.SetBytes(body, "model", name); err == nil {
				c.Set(config.GinRequestBodyKey, rewritten)
			}
		}
		c.Next()
	}
}
