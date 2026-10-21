package vercel

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
)

// getClaudeConfig 返回 Vercel AI Gateway 的 Anthropic Messages 入口配置。
// 网关把 cache_control 原样透传给 Anthropic / Vertex / Bedrock 的 Claude 模型，所以 Claude 格式请求
// 直接转发而不走 Claude→OpenAI 转换链（转换链会丢掉客户端的缓存断点）。
// 网关同时接受 x-api-key 与 Bearer，这里与 OpenRouter 保持一致用 Bearer。
func getClaudeConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         "https://ai-gateway.vercel.sh",
		ChatCompletions: "/v1/messages",
		ModelList:       "/v1/models",
	}
}

// claudeProvider 基于当前渠道委托出一个走 Anthropic 原生转发路径的 ClaudeProvider，
// 复用同一 BaseProvider（共享 Channel/Context/Usage），仅替换端点配置、鉴权方式与错误解析。
func (p *VercelProvider) claudeProvider() *claude.ClaudeProvider {
	provider := &claude.ClaudeProvider{BaseProvider: p.BaseProvider, BearerAuth: true}
	provider.Config = getClaudeConfig()
	provider.Requester = requester.NewHTTPRequester(p.Channel.GetProxy(), claude.RequestErrorHandle)
	return provider
}

func (p *VercelProvider) CreateClaudeChat(request *claude.ClaudeRequest) (*claude.ClaudeResponse, *types.OpenAIErrorWithStatusCode) {
	return p.claudeProvider().CreateClaudeChat(request)
}

func (p *VercelProvider) CreateClaudeChatStream(request *claude.ClaudeRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	return p.claudeProvider().CreateClaudeChatStream(request)
}
