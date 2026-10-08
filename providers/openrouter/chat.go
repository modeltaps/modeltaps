package openrouter

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/image"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
)

// requestID 取当前请求 ID(与响应头 X-Modeltaps-Request-Id 一致),用于把异步拉取的
// generation 统计回填到对应消费日志行。取不到时返回空串(enrich 侧会静默跳过)。
func (p *OpenRouterProvider) requestID() string {
	if p.Context == nil {
		return ""
	}
	return p.Context.GetString(logger.RequestIdKey)
}

// triggerGenerationEnrich 用响应里的 generation id 异步拉取 OpenRouter /api/v1/generation 统计
// 并回填日志 metadata(W9-M6)。失败静默降级,不影响主链路/计费/日志写入。
func (p *OpenRouterProvider) triggerGenerationEnrich(genID string) {
	enrichLogWithGeneration(p.GetBaseURL(), firstChannelKey(p.Channel.Key), p.Channel.GetProxy(), genID, p.requestID())
}

func (p *OpenRouterProvider) CreateChatCompletion(request *types.ChatCompletionRequest) (openaiResponse *types.ChatCompletionResponse, errWithCode *types.OpenAIErrorWithStatusCode) {
	orRequest := &ChatCompletionRequest{
		ChatCompletionRequest: *request,
	}

	modelProvider := strings.Split(request.Model, "/")[0]

	p.ConvertFromChatOpenai(orRequest, modelProvider)

	req, errWithCode := p.GetRequestTextBody(config.RelayModeChatCompletions, request.Model, orRequest)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	response := &openai.OpenAIProviderChatResponse{}
	// 发送请求
	_, errWithCode = p.Requester.SendRequest(req, response, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	// 检测是否错误
	openaiErr := openai.ErrorHandle(&response.OpenAIErrorResponse)
	if openaiErr != nil {
		errWithCode = &types.OpenAIErrorWithStatusCode{
			OpenAIError: *openaiErr,
			StatusCode:  http.StatusBadRequest,
		}
		return nil, errWithCode
	}

	if response.Usage == nil || response.Usage.CompletionTokens == 0 {
		response.Usage = &types.Usage{
			PromptTokens:     p.Usage.PromptTokens,
			CompletionTokens: 0,
			TotalTokens:      0,
		}
		// 那么需要计算
		response.Usage.CompletionTokens = common.CountTokenText(response.GetContent(), request.Model)
		response.Usage.TotalTokens = response.Usage.PromptTokens + response.Usage.CompletionTokens
	}

	*p.Usage = *response.Usage

	// Anthropic 缓存读取计费修正：把 cached_tokens 归入 cached_read_tokens 桶(0.1x)。
	// 复用 openai 共享门控（按模型名判定），anthropic/* 模型行为不变。
	openai.RemapAnthropicCacheUsage(request.Model, p.Usage)

	for index, choices := range response.Choices {
		if choices.Message.ReasoningContent == "" && choices.Message.Reasoning != "" {
			response.Choices[index].Message.ReasoningContent = choices.Message.Reasoning
			response.Choices[index].Message.Reasoning = ""
		}
	}

	// 非流式:响应体自带 generation id,异步拉取 provider 侧真实统计回填日志(W9-M6)。
	p.triggerGenerationEnrich(response.ID)

	return &response.ChatCompletionResponse, nil
}

func (p *OpenRouterProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	orRequest := &ChatCompletionRequest{
		ChatCompletionRequest: *request,
	}

	modelProvider := strings.Split(request.Model, "/")[0]
	p.ConvertFromChatOpenai(orRequest, modelProvider)

	streamOptions := orRequest.StreamOptions
	// 如果支持流式返回Usage 则需要更改配置：
	orRequest.StreamOptions = &types.StreamOptions{
		IncludeUsage: true,
	}

	req, errWithCode := p.GetRequestTextBody(config.RelayModeChatCompletions, orRequest.Model, orRequest)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// 恢复原来的配置
	orRequest.StreamOptions = streamOptions

	// 发送请求
	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	chatHandler := openai.OpenAIStreamHandler{
		Usage:      p.Usage,
		ModelName:  request.Model,
		EscapeJSON: p.StreamEscapeJSON,
		Context:    p.Context,

		ReasoningHandler: p.ReasoningHandler,
	}

	// Anthropic 缓存读取计费修正：在 usage 落账前把 cached_tokens 归入 cached_read_tokens 桶(0.1x)。
	// 复用 openai 共享门控（按模型名判定），非 Anthropic 模型为空操作。
	chatHandler.UsageHandler = func(usage *types.Usage) bool {
		openai.RemapAnthropicCacheUsage(request.Model, usage)
		return false
	}

	// 流式:generation id 藏在流分片里,拿到首个 id 时(仅一次)触发异步拉取 provider 侧统计回填(W9-M6)。
	// 计费/请求所需字段在此(handler 栈上)先读成值,闭包只持有值,不在流 goroutine 里访问 gin.Context。
	var genOnce sync.Once
	reqID := p.requestID()
	genBaseURL := p.GetBaseURL()
	genAPIKey := firstChannelKey(p.Channel.Key)
	genProxy := p.Channel.GetProxy()
	chatHandler.GenerationIDHandler = func(id string) {
		genOnce.Do(func() {
			enrichLogWithGeneration(genBaseURL, genAPIKey, genProxy, id, reqID)
		})
	}

	return requester.RequestStream(p.Requester, resp, chatHandler.HandlerChatStream)
}

func (p *OpenRouterProvider) ConvertFromChatOpenai(request *ChatCompletionRequest, modelProvider string) {
	if p.Channel.Plugin != nil {
		plugin := p.Channel.Plugin.Data()
		if pOther, ok := plugin["other"]; ok {
			if provider, ok := pOther["provider"].(string); ok && provider != "" {
				var orProvider map[string]orProvider
				err := json.Unmarshal([]byte(provider), &orProvider)
				if err == nil {
					if _, ok := orProvider[modelProvider]; ok {
						request.Provider = orProvider[modelProvider]
					}
				}
			}
		}
	}

	for indexM, message := range request.Messages {
		openaiContent := message.ParseContent()
		needConvert := false
		for indexP, part := range openaiContent {
			if part.Type == types.ContentTypeImageURL {
				mimeType, data, err := image.GetImageFromUrl(part.ImageURL.URL)
				if err != nil {
					continue
				}

				if mimeType == "application/pdf" {
					openaiContent[indexP] = types.ChatMessagePart{
						Type: "file",
						File: &types.ChatMessageFile{
							FileData: "data:application/pdf;base64," + data,
						},
					}
					needConvert = true
				}
			}
		}
		if needConvert {
			request.Messages[indexM].Content = openaiContent
		}
	}

	// 自动注入 prompt caching cache_control（模型命中 + 开关命中时；复用 openai 共享逻辑）。
	openai.InjectAnthropicPromptCaching(p.Channel, &request.ChatCompletionRequest)
}
