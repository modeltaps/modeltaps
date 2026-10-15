package codex

import (
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"time"
)

// CreateChatCompletion answers a non-streamed chat request. The upstream only streams, so the
// request goes through CreateResponses, which collects the terminal event, and the generic
// Responses-to-chat conversion carries over tool_calls and finish_reason.
func (p *CodexProvider) CreateChatCompletion(request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {
	response, errWithCode := p.CreateResponses(request.ToResponsesRequest())
	if errWithCode != nil {
		return nil, errWithCode
	}

	chatResponse := response.ToChat()
	chatResponse.Model = p.GetResponseModelName(request.Model)
	if chatResponse.Created == 0 {
		chatResponse.Created = time.Now().Unix()
	}

	return chatResponse, nil
}

// CreateChatCompletionStream answers a streamed chat request through the ConvertChat branch of
// CreateResponsesStream, sharing the OpenAI Responses-to-chat stream conversion (text,
// reasoning, function calls and usage).
func (p *CodexProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	responsesRequest := request.ToResponsesRequest()
	responsesRequest.ConvertChat = true

	return p.CreateResponsesStream(responsesRequest)
}

// applyDefaultHeaders 应用 Codex 默认请求头（只补充缺失的头，不覆盖已有的）
// 注意：这个方法的优先级最低，不会覆盖客户端透传的头或 ModelHeaders
func (p *CodexProvider) applyDefaultHeaders(headers map[string]string) {
	// 设置 Host（必需，但不覆盖已有的）
	if _, exists := headers["Host"]; !exists {
		headers["Host"] = "chatgpt.com"
	}

	// 设置 User-Agent（如果没有从客户端透传或 ModelHeaders 设置）
	// A custom User-Agent only replaces the User-Agent itself; the headers below must still be
	// filled in, otherwise the upstream version gate rejects the request.
	if _, exists := headers["User-Agent"]; !exists {
		headers["User-Agent"] = p.defaultUserAgent()
		// 尝试从 Other 字段读取自定义 UA
		if p.Channel.Other != "" {
			var config map[string]string
			if err := json.Unmarshal([]byte(p.Channel.Other), &config); err == nil {
				if userAgent, exists := config["user_agent"]; exists && userAgent != "" {
					headers["User-Agent"] = userAgent
				}
			}
		}
	}

	// The upstream gates models by minimal_client_version; a missing or low version gets new models
	// such as gpt-6 rejected.
	if _, exists := headers["version"]; !exists {
		headers["version"] = p.ClientVersion
	}

	// 设置 originator（官方 Codex CLI 标识，缺失时可能被拒或降级）
	if _, exists := headers["originator"]; !exists {
		headers["originator"] = DefaultCodexOriginator
	}

	// 设置 Accept（如果没有设置）
	if _, exists := headers["Accept"]; !exists {
		headers["Accept"] = "application/json"
	}
}
