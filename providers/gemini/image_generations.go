package gemini

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"
	"net/http"
	"strings"
)

// CreateImageGenerationsStream Imagen 只有 predict 非流式端点；嵌入的 OpenAIProvider 流式方法
// 会按 config.ImagesGenerations（哨兵值 "1"）拼出无效 URL，覆写返回哨兵让 relay 层降级合成 SSE。
// geminicli / antigravity / vertexai_express 经嵌入继承此覆写。
func (p *GeminiProvider) CreateImageGenerationsStream(request *types.ImageRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {
	return nil, base.ImageStreamNotSupportedError()
}

func (p *GeminiProvider) CreateImageGenerations(request *types.ImageRequest) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	// 原生生图模型（gemini-2.5-flash-image 等）只有 generateContent 端点，:predict 会 404，
	// 必须走 modality 出图路径；Imagen 仍走下面的 :predict。
	// antigravity / vertexai_express 经嵌入继承此分支。
	if model.ResolveModelMode(request.Model) == model.ModelModeChatImage {
		return p.createNativeImageGenerations(request)
	}

	// 创建动态参数map
	parameters := make(GeminiImageParametersDynamic)
	parameters["sampleCount"] = request.N

	// 设置默认的personGeneration
	parameters["personGeneration"] = "allow_adult"

	// 处理AspectRatio
	if request.AspectRatio != nil {
		parameters["aspectRatio"] = *request.AspectRatio
	} else {
		switch request.Size {
		case "1024x1792":
			parameters["aspectRatio"] = "9:16"
		case "1792x1024":
			parameters["aspectRatio"] = "16:9"
		default:
			parameters["aspectRatio"] = "1:1"
		}
	}

	// 透传所有额外参数
	if request.ExtraParams != nil {
		for key, value := range request.ExtraParams {
			parameters[key] = value
		}
	}

	geminiRequest := &GeminiImageRequest{
		Instances: []GeminiImageInstance{
			{
				Prompt: request.Prompt,
			},
		},
		Parameters: parameters,
	}

	fullRequestURL := p.GetFullRequestURL("predict", request.Model)
	headers := p.GetRequestHeaders()

	req, err := p.Requester.NewRequest(http.MethodPost, fullRequestURL, p.Requester.WithBody(geminiRequest), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}

	defer req.Body.Close()

	geminiImageResponse := &GeminiImageResponse{}
	_, errWithCode := p.Requester.SendRequest(req, geminiImageResponse, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	imageCount := len(geminiImageResponse.Predictions)

	// 如果imageCount为0，则返回错误
	if imageCount == 0 {
		return nil, common.StringErrorWrapper("no image generated", "no_image_generated", http.StatusInternalServerError)
	}

	openaiResponse := &types.ImageResponse{
		Created: utils.GetTimestamp(),
		Data:    make([]types.ImageResponseDataInner, 0, imageCount),
	}

	for _, prediction := range geminiImageResponse.Predictions {
		if prediction.BytesBase64Encoded == "" {
			continue
		}

		openaiResponse.Data = append(openaiResponse.Data, types.ImageResponseDataInner{
			B64JSON: prediction.BytesBase64Encoded,
		})
	}

	usage := p.GetUsage()
	// PromptTokens保持之前根据prompt内容计算的值
	// CompletionTokens根据生成的图像数量计算，避免空回复计费问题
	usage.CompletionTokens = imageCount * 258
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens

	return openaiResponse, nil
}

// createNativeImageGenerations 用 generateContent + responseModalities:[Text,Image] 出图，
// 从 candidates 的 inlineData 提取 base64 图片转成 OpenAI images 响应。
func (p *GeminiProvider) createNativeImageGenerations(request *types.ImageRequest) (*types.ImageResponse, *types.OpenAIErrorWithStatusCode) {
	geminiRequest := &GeminiChatRequest{
		Model: request.Model,
		Contents: []GeminiChatContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{
						Text: request.Prompt,
					},
				},
			},
		},
		GenerationConfig: GeminiChatGenerationConfig{
			ResponseModalities: []string{"Text", "Image"},
		},
	}

	// imageConfig 并非所有原生生图模型都支持，只在调用方显式指定比例/尺寸时下发，
	// 避免默认请求因不支持的字段被上游 400。
	if aspectRatio := nativeImageAspectRatio(request); aspectRatio != "" {
		geminiRequest.GenerationConfig.ImageConfig = &ImageConfig{AspectRatio: aspectRatio}
	}

	fullRequestURL := p.GetFullRequestURL("generateContent", request.Model)
	headers := p.GetRequestHeaders()

	req, errWithCode := p.NewRequestWithCustomParams(http.MethodPost, fullRequestURL, geminiRequest, headers, request.Model)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	geminiResponse := &GeminiChatResponse{}
	_, errWithCode = p.Requester.SendRequest(req, geminiResponse, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	openaiResponse := &types.ImageResponse{
		Created: utils.GetTimestamp(),
		Data:    make([]types.ImageResponseDataInner, 0),
	}

	for _, candidate := range geminiResponse.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData == nil || part.InlineData.Data == "" {
				continue
			}
			if !strings.HasPrefix(part.InlineData.MimeType, "image/") {
				continue
			}
			openaiResponse.Data = append(openaiResponse.Data, types.ImageResponseDataInner{
				B64JSON: part.InlineData.Data,
			})
		}
	}

	if len(openaiResponse.Data) == 0 {
		return nil, common.StringErrorWrapper("no image generated", "no_image_generated", http.StatusInternalServerError)
	}

	// 计费口径与 chat 协议下同一模型一致：优先用上游 usageMetadata（图片输出 token 落在
	// candidatesTokenCount），缺失时按 1290 token/图 兜底（见 ConvertOpenAIUsageWithFallback）。
	usage := p.GetUsage()
	promptTokens := usage.PromptTokens
	converted := ConvertOpenAIUsageWithFallback(geminiResponse.UsageMetadata, geminiResponse)
	if converted.PromptTokens == 0 {
		// 上游未回 prompt token（或被中转裁掉）时保留 relay 层按 prompt 文本算出的值
		converted.PromptTokens = promptTokens
		converted.TotalTokens = converted.PromptTokens + converted.CompletionTokens
	}
	*usage = converted

	return openaiResponse, nil
}

// nativeImageAspectRatio 返回调用方显式指定的图片比例，未指定时返回空串。
func nativeImageAspectRatio(request *types.ImageRequest) string {
	if request.AspectRatio != nil && *request.AspectRatio != "" {
		return *request.AspectRatio
	}

	switch request.Size {
	case "1024x1792":
		return "9:16"
	case "1792x1024":
		return "16:9"
	case "1024x1024":
		return "1:1"
	default:
		return ""
	}
}
