package openai

import (
	"bytes"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

type OpenAIStreamHandler struct {
	Usage      *types.Usage
	ModelName  string
	isAzure    bool
	EscapeJSON bool
	Context    *gin.Context // 添加 Context 用于获取响应模型名称

	ReasoningHandler bool
	ExtraBilling     map[string]types.ExtraBilling `json:"-"`
	UsageHandler     UsageHandler

	// GenerationIDHandler 可选回调:拿到上游流分片里的响应 id 时调用(OpenRouter 用于异步拉取
	// generation 统计,W9-M6)。其它 provider 不设置即零开销;调用方自行保证幂等(只处理首个 id)。
	GenerationIDHandler func(id string) `json:"-"`
}

func (p *OpenAIProvider) CreateChatCompletion(request *types.ChatCompletionRequest) (openaiResponse *types.ChatCompletionResponse, errWithCode *types.OpenAIErrorWithStatusCode) {
	if p.RequestHandleBefore != nil {
		errWithCode = p.RequestHandleBefore(request)
		if errWithCode != nil {
			return nil, errWithCode
		}
	}
	otherProcessing(request, p.GetOtherArg())

	// 对于自定义渠道，过滤空content的消息以保持与其他渠道一致的行为
	if p.Channel.Type == config.ChannelTypeCustom {
		request.Messages = common.FilterEmptyContentMessages(request.Messages)
	}

	// Anthropic prompt caching 自动注入（模型命中 + 开关命中时；用户已带 cache_control 时跳过）。
	InjectAnthropicPromptCaching(p.Channel, request)

	req, errWithCode := p.GetRequestTextBody(config.RelayModeChatCompletions, request.Model, request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	response := &OpenAIProviderChatResponse{}
	// 开启渠道 PassThroughBody 且 relay 层已放行（入口协议 == chat、响应原样直返）时，
	// 用 outputResp=true 让 SendRequest 回填 resp.Body：既 unmarshal 一份供计费，又能拿到上游
	// 原始字节用于响应字节透传（保留未知字段 / 字段顺序）。responses/claude 等兼容路径不放行，
	// 避免把 chat 字节当目标协议返回。
	passThrough := p.Channel.PassThroughBody && p.Context != nil && p.Context.GetBool(config.GinRawPassThroughAllowedKey)
	// 发送请求
	resp, errWithCode := p.Requester.SendRequest(req, response, passThrough)
	if errWithCode != nil {
		return nil, errWithCode
	}
	if passThrough {
		defer resp.Body.Close()
	}

	// 透传上游响应头（限流指纹等）：与字节透传解耦，成功响应即捕获。
	p.storeOpenAIUpstreamHeaders(resp.Header)

	// 检测是否错误
	openaiErr := ErrorHandle(&response.OpenAIErrorResponse)
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
	} else if p.UsageHandler != nil {
		p.UsageHandler(response.Usage)
	}

	*p.Usage = *response.Usage

	// Anthropic 缓存读取计费修正：把 cached_tokens 归入 cached_read_tokens 桶(0.1x)。
	// 严格按模型名门控，非 Anthropic 模型的原生 cached_tokens 计费不受影响。
	RemapAnthropicCacheUsage(request.Model, p.Usage)

	p.Usage.ExtraBilling = getChatExtraBilling(request)

	// 暂存上游原始字节，由 relay 层字节透传，保留未知字段 / 字段顺序。
	// 有别名映射需改 model 时，在原始字节上就地 sjson 改写顶层 model（不改字段顺序 / 不丢未知字段）；
	// 无映射时 UnifyModelInJSONBytes 恒 no-op。下方结构体 response.Model 改写仅回退路径生效。
	if passThrough {
		if rawBytes, readErr := io.ReadAll(resp.Body); readErr == nil && len(rawBytes) > 0 {
			if patched, changed := base.UnifyModelInJSONBytes(p.Context, rawBytes, "model"); changed {
				rawBytes = patched
			}
			p.Context.Set(config.GinRawResponseBodyKey, rawBytes)
		}
	}

	// 修改响应中的模型名称为用户请求的原始模型名称
	responseModel := p.GetResponseModelName(request.Model)
	response.Model = responseModel

	return &response.ChatCompletionResponse, nil
}

func (p *OpenAIProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	if p.RequestHandleBefore != nil {
		errWithCode := p.RequestHandleBefore(request)
		if errWithCode != nil {
			return nil, errWithCode
		}
	}
	otherProcessing(request, p.GetOtherArg())

	// 对于自定义渠道，过滤空content的消息以保持与其他渠道一致的行为
	if p.Channel.Type == config.ChannelTypeCustom {
		request.Messages = common.FilterEmptyContentMessages(request.Messages)
	}

	// Anthropic prompt caching 自动注入（模型命中 + 开关命中时；用户已带 cache_control 时跳过）。
	InjectAnthropicPromptCaching(p.Channel, request)

	streamOptions := request.StreamOptions
	// 如果支持流式返回Usage 则需要更改配置：
	if p.SupportStreamOptions {
		request.StreamOptions = &types.StreamOptions{
			IncludeUsage: true,
		}
	} else {
		// 避免误传导致报错
		request.StreamOptions = nil
	}
	req, errWithCode := p.GetRequestTextBody(config.RelayModeChatCompletions, request.Model, request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// 恢复原来的配置
	request.StreamOptions = streamOptions

	// 发送请求
	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	// 透传上游响应头（限流指纹等）：与字节透传解耦，成功响应即捕获。
	p.storeOpenAIUpstreamHeaders(resp.Header)

	chatHandler := OpenAIStreamHandler{
		Usage:      p.Usage,
		ModelName:  request.Model,
		isAzure:    p.IsAzure,
		EscapeJSON: p.StreamEscapeJSON,
		Context:    p.Context, // 传递 Context

		ExtraBilling: getChatExtraBilling(request),
		// Anthropic 缓存读取计费修正：与既有 UsageHandler 组合（先跑原有逻辑再 remap），不覆盖。
		UsageHandler: composeAnthropicUsageHandler(p.UsageHandler, request.Model),
	}

	return requester.RequestStream(p.Requester, resp, chatHandler.HandlerChatStream)
}

// composeAnthropicUsageHandler 包装既有 UsageHandler：先执行原有 handler（若有），
// 再对 usage 做 Anthropic 缓存读取计费修正（内部按模型名门控，非 Anthropic 为空操作）。
func composeAnthropicUsageHandler(existing UsageHandler, modelName string) UsageHandler {
	return func(usage *types.Usage) bool {
		forced := false
		if existing != nil {
			forced = existing(usage)
		}
		RemapAnthropicCacheUsage(modelName, usage)
		return forced
	}
}

func (h *OpenAIStreamHandler) HandlerChatStream(rawLine *[]byte, dataChan chan string, errChan chan error) {
	// 如果rawLine 前缀不为data:，则直接返回
	if !strings.HasPrefix(string(*rawLine), "data:") {
		*rawLine = nil
		return
	}

	// 去除前缀
	*rawLine = (*rawLine)[5:]
	*rawLine = bytes.TrimSpace(*rawLine)

	// 如果等于 DONE 则结束
	if string(*rawLine) == "[DONE]" {
		errChan <- io.EOF
		*rawLine = requester.StreamClosed
		return
	}

	var openaiResponse OpenAIProviderChatStreamResponse
	err := json.Unmarshal(*rawLine, &openaiResponse)
	if err != nil {
		errChan <- common.ErrorToOpenAIError(err)
		return
	}

	aiError := ErrorHandle(&openaiResponse.OpenAIErrorResponse)
	if aiError != nil {
		errChan <- aiError
		return
	}

	// 透传上游响应 id 给可选回调(OpenRouter 用于异步拉取 generation 统计,W9-M6);幂等由回调保证。
	if h.GenerationIDHandler != nil && openaiResponse.ID != "" {
		h.GenerationIDHandler(openaiResponse.ID)
	}

	if openaiResponse.Usage != nil {
		if openaiResponse.Usage.CompletionTokens > 0 {
			if h.UsageHandler != nil && h.UsageHandler(openaiResponse.Usage) {
				h.EscapeJSON = true
			}
			// 整体覆盖 Usage 会重置已累积的 TextBuilder(尾部 usage 分片场景),
			// 先保存已累积文本,覆盖后再写回,避免响应明细被清空。
			// FinishReason 同理:finish 分片常先于尾部 usage 分片到达,覆盖时需保留。
			accumulated := h.Usage.TextBuilder.String()
			finishReason := h.Usage.FinishReason
			*h.Usage = *openaiResponse.Usage
			h.Usage.TextBuilder.WriteString(accumulated)
			h.Usage.FinishReason = finishReason

			if h.ExtraBilling != nil {
				h.Usage.ExtraBilling = h.ExtraBilling
			}
		}

		if len(openaiResponse.Choices) == 0 {
			*rawLine = nil
			return
		}
	} else {
		if len(openaiResponse.Choices) > 0 && openaiResponse.Choices[0].Usage != nil {
			if openaiResponse.Choices[0].Usage.CompletionTokens > 0 {
				if h.UsageHandler != nil && h.UsageHandler(openaiResponse.Choices[0].Usage) {
					h.EscapeJSON = true
				}
				accumulated := h.Usage.TextBuilder.String()
				finishReason := h.Usage.FinishReason
				*h.Usage = *openaiResponse.Choices[0].Usage
				h.Usage.TextBuilder.WriteString(accumulated)
				h.Usage.FinishReason = finishReason
				if h.ExtraBilling != nil {
					h.Usage.ExtraBilling = h.ExtraBilling
				}
			}
		} else {
			if h.Usage.TotalTokens == 0 {
				h.Usage.TotalTokens = h.Usage.PromptTokens
			}
		}
	}

	// 修改响应中的模型名称为用户请求的原始模型名称。
	// 两条出口各需一份：默认出口发原始字节 → 必须在 *rawLine 字节上就地改 model；
	// EscapeJSON 出口 marshal 结构体 → 需结构体 openaiResponse.Model 也改。二者缺一不可，非冗余。
	// UnifyModelInJSONBytes 无映射时恒 no-op（gjson 查 model 值 == 上游名才 sjson 改）。
	if h.Context != nil {
		if patched, changed := base.UnifyModelInJSONBytes(h.Context, *rawLine, "model"); changed {
			*rawLine = patched
		}
		responseModel := base.GetResponseModelNameFromContext(h.Context, openaiResponse.Model)
		openaiResponse.Model = responseModel
	}

	// 始终累积流式内容到 TextBuilder，用于流中断时的 token 计算备用
	// 即使上游返回了 Usage 信息，流中断时最终的 Usage 可能不完整
	responseText := openaiResponse.GetResponseText()
	if responseText != "" {
		h.Usage.TextBuilder.WriteString(responseText)
	}

	// 累积最后一个非空 finish_reason 原文(W8-B),供消费日志归一。此处已在 Usage 覆盖之后:
	// 本 chunk 若同时携带 usage 分片,覆盖已完成不会清空;跨 chunk 的清空由上面覆盖处兜底保留。
	if reason := openaiResponse.GetFinishReason(); reason != "" {
		h.Usage.FinishReason = reason
	}

	if h.ReasoningHandler && len(openaiResponse.Choices) > 0 {
		for index, choices := range openaiResponse.Choices {
			if choices.Delta.ReasoningContent == "" && choices.Delta.Reasoning != "" {
				openaiResponse.Choices[index].Delta.ReasoningContent = choices.Delta.Reasoning
				openaiResponse.Choices[index].Delta.Reasoning = ""
			}
		}

		h.EscapeJSON = true
	}

	if h.EscapeJSON {
		if data, err := json.Marshal(openaiResponse.ChatCompletionStreamResponse); err == nil {
			dataChan <- string(data)
			return
		}
	}
	dataChan <- string(*rawLine)
}

func otherProcessing(request *types.ChatCompletionRequest, otherArg string) {
	matched, _ := regexp.MatchString(`(?i)^o[1-9]`, request.Model)
	if matched || model_utils.HasPrefixCaseInsensitive(request.Model, "gpt-5") {
		if request.MaxTokens > 0 {
			request.MaxCompletionTokens = request.MaxTokens
			request.MaxTokens = 0
		}
		if request.Model != "gpt-5-chat-latest" {
			request.Temperature = nil
		}
		// 只有当 otherArg 不为空且没有已存在的 Reasoning 设置时，才使用 otherArg 设置 ReasoningEffort
		if otherArg != "" && request.Reasoning == nil {
			request.ReasoningEffort = &otherArg
		}
		// 如果有 Reasoning 设置，优先使用 Reasoning.Effort 设置 ReasoningEffort
		if request.Reasoning != nil && request.Reasoning.Effort != "" {
			request.ReasoningEffort = &request.Reasoning.Effort
		}
	}
}

func getChatExtraBilling(request *types.ChatCompletionRequest) map[string]types.ExtraBilling {
	if !strings.Contains(request.Model, "search-preview") {
		return nil
	}

	searchType := "medium"
	if request.WebSearchOptions != nil && request.WebSearchOptions.SearchContextSize != "" {
		searchType = request.WebSearchOptions.SearchContextSize
	}

	return map[string]types.ExtraBilling{
		types.APITollTypeWebSearchPreview: {
			Type:      searchType,
			CallCount: 1,
		},
	}
}
