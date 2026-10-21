package openrouter

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
)

// getClaudeConfig 返回 OpenRouter 的 Anthropic 兼容入口配置。
// 客户端以 Claude 格式（/v1/messages）请求时直接转发到这里，而不是走 Claude→OpenAI 转换链：
// 转换链会丢掉客户端打的 cache_control 断点（系统提示 / 工具定义 / 近期用户消息），
// Claude Code 这类自带缓存断点的工具经 OpenRouter 就会失去提示缓存，费用成倍上涨。
// 鉴权必须用 Bearer：OpenRouter 把 x-api-key 当成用户自带的 Anthropic 密钥。
func getClaudeConfig() base.ProviderConfig {
	return base.ProviderConfig{
		BaseURL:         "https://openrouter.ai/api",
		ChatCompletions: "/v1/messages",
		ModelList:       "/v1/models",
	}
}

// claudeProvider 基于当前渠道委托出一个走 Anthropic 原生转发路径的 ClaudeProvider，
// 复用同一 BaseProvider（共享 Channel/Context/Usage），仅替换端点配置、鉴权方式与错误解析。
func (p *OpenRouterProvider) claudeProvider() *claude.ClaudeProvider {
	provider := &claude.ClaudeProvider{BaseProvider: p.BaseProvider, BearerAuth: true}
	provider.Config = getClaudeConfig()
	provider.Requester = requester.NewHTTPRequester(p.Channel.GetProxy(), claude.RequestErrorHandle)
	return provider
}

func (p *OpenRouterProvider) CreateClaudeChat(request *claude.ClaudeRequest) (*claude.ClaudeResponse, *types.OpenAIErrorWithStatusCode) {
	return p.claudeProvider().CreateClaudeChat(request)
}

func (p *OpenRouterProvider) CreateClaudeChatStream(request *claude.ClaudeRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	return p.claudeProvider().CreateClaudeChatStream(request)
}
