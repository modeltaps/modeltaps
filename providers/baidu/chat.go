package baidu

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type baiduStreamHandler struct {
	Usage   *types.Usage
	Request *types.ChatCompletionRequest
	Context *gin.Context
}

func (p *BaiduProvider) CreateChatCompletion(request *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode) {

	if p.UseOpenaiAPI {
		if modelNameConvert, ok := modelNameMap[request.Model]; ok {
			request.Model = modelNameConvert
		}
		return p.OpenAIProvider.CreateChatCompletion(request)
	}

	// Optional: if the Baidu channel needs empty-content messages filtered out, uncomment the line below
	// request.Messages = common.FilterEmptyContentMessages(request.Messages)

	req, errWithCode := p.getBaiduChatRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	baiduResponse := &BaiduChatResponse{}
	// Send the request
	_, errWithCode = p.Requester.SendRequest(req, baiduResponse, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	return p.convertToChatOpenai(baiduResponse, request)
}

func (p *BaiduProvider) CreateChatCompletionStream(request *types.ChatCompletionRequest) (requester.StreamReaderInterface[string], *types.OpenAIErrorWithStatusCode) {

	if p.UseOpenaiAPI {
		if modelNameConvert, ok := modelNameMap[request.Model]; ok {
			request.Model = modelNameConvert
		}
		return p.OpenAIProvider.CreateChatCompletionStream(request)
	}

	// Optional: if the Baidu channel needs empty-content messages filtered out, uncomment the line below
	// request.Messages = common.FilterEmptyContentMessages(request.Messages)

	req, errWithCode := p.getBaiduChatRequest(request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// Send the request
	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	chatHandler := &baiduStreamHandler{
		Usage:   p.Usage,
		Request: request,
		Context: p.Context,
	}

	return requester.RequestStream[string](p.Requester, resp, chatHandler.handlerStream)
}

func (p *BaiduProvider) getBaiduChatRequest(request *types.ChatCompletionRequest) (*http.Request, *types.OpenAIErrorWithStatusCode) {
	url, errWithCode := p.GetSupportedAPIUri(config.RelayModeChatCompletions)
	if errWithCode != nil {
		return nil, errWithCode
	}
	// Get the request URL
	fullRequestURL := p.GetFullRequestURL(url, request.Model)
	if fullRequestURL == "" {
		return nil, common.ErrorWrapper(nil, "invalid_baidu_config", http.StatusInternalServerError)
	}

	// Get the request headers
	headers := p.GetRequestHeaders()
	if request.Stream {
		headers["Accept"] = "text/event-stream"
	}

	baiduRequest := convertFromChatOpenai(request)
	// Create the request
	req, err := p.Requester.NewRequest(http.MethodPost, fullRequestURL, p.Requester.WithBody(baiduRequest), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}

	return req, nil
}

func (p *BaiduProvider) convertToChatOpenai(response *BaiduChatResponse, request *types.ChatCompletionRequest) (openaiResponse *types.ChatCompletionResponse, errWithCode *types.OpenAIErrorWithStatusCode) {
	aiError := errorHandle(&response.BaiduError)
	if aiError != nil {
		errWithCode = &types.OpenAIErrorWithStatusCode{
			OpenAIError: *aiError,
			StatusCode:  http.StatusBadRequest,
		}
		return
	}

	choice := types.ChatCompletionChoice{
		Index: 0,
		Message: types.ChatCompletionMessage{
			Role: "assistant",
		},
		FinishReason: types.FinishReasonStop,
	}

	if response.FunctionCall != nil {
		if request.Tools != nil {
			choice.Message.ToolCalls = []*types.ChatCompletionToolCalls{
				{
					Id:       response.Id,
					Type:     "function",
					Function: response.FunctionCall,
				},
			}
			choice.FinishReason = types.FinishReasonToolCalls
		} else {
			choice.Message.FunctionCall = response.FunctionCall
			choice.FinishReason = types.FinishReasonFunctionCall
		}
	} else {
		choice.Message.Content = response.Result
	}

	openaiResponse = &types.ChatCompletionResponse{
		ID:      response.Id,
		Object:  "chat.completion",
		Model:   request.Model,
		Created: response.Created,
		Choices: []types.ChatCompletionChoice{choice},
		Usage:   response.Usage,
	}

	*p.Usage = *openaiResponse.Usage

	return
}

func convertFromChatOpenai(request *types.ChatCompletionRequest) *BaiduChatRequest {
	baiduChatRequest := &BaiduChatRequest{
		Messages:    make([]BaiduMessage, 0, len(request.Messages)),
		Temperature: request.Temperature,
		Stream:      request.Stream,
		TopP:        request.TopP,
		// PenaltyScore:    request.FrequencyPenalty,
		MaxOutputTokens: request.MaxTokens,
	}

	if request.FrequencyPenalty != nil {
		baiduChatRequest.PenaltyScore = utils.GetPointer(utils.NumClamp(*request.FrequencyPenalty, 1, 2))
	}

	if request.Stop != nil {
		if stop, ok := request.Stop.(string); ok {
			baiduChatRequest.Stop = []string{stop}
		} else if stop, ok := request.Stop.([]string); ok {
			baiduChatRequest.Stop = stop
		}
	}

	if request.ResponseFormat != nil {
		baiduChatRequest.ResponseFormat = request.ResponseFormat.Type

	}

	for _, message := range request.Messages {
		if message.Role == types.ChatMessageRoleSystem {
			baiduChatRequest.System = message.StringContent()
			continue
		} else if message.ToolCalls != nil {
			baiduChatRequest.Messages = append(baiduChatRequest.Messages, BaiduMessage{
				Role: types.ChatMessageRoleAssistant,
				FunctionCall: &types.ChatCompletionToolCallsFunction{
					Name:      *message.Name,
					Arguments: "{}",
				},
			})
		} else if message.Role == types.ChatMessageRoleFunction || message.Role == types.ChatMessageRoleTool {
			baiduChatRequest.Messages = append(baiduChatRequest.Messages, BaiduMessage{
				Role:    types.ChatMessageRoleUser,
				Content: "This is the content returned by the function call, please answer the previous question:\n" + message.StringContent(),
			})
		} else {
			baiduChatRequest.Messages = append(baiduChatRequest.Messages, BaiduMessage{
				Role:    message.Role,
				Content: message.StringContent(),
			})
		}
	}

	if request.Tools != nil {
		functions := make([]*types.ChatCompletionFunction, 0, len(request.Tools))
		for _, tool := range request.Tools {
			functions = append(functions, &tool.Function)
		}
		baiduChatRequest.Functions = functions
	} else if request.Functions != nil {
		baiduChatRequest.Functions = request.Functions
	}

	return baiduChatRequest
}

// Convert to an OpenAI chat stream response body
func (h *baiduStreamHandler) handlerStream(rawLine *[]byte, dataChan chan string, errChan chan error) {
	// If rawLine does not start with data:, return directly
	if !strings.HasPrefix(string(*rawLine), "data: ") {
		*rawLine = nil
		return
	}

	// Strip the prefix
	*rawLine = (*rawLine)[6:]

	var baiduResponse BaiduChatStreamResponse
	err := json.Unmarshal(*rawLine, &baiduResponse)
	if err != nil {
		errChan <- common.ErrorToOpenAIError(err)
		return
	}

	aiError := errorHandle(&baiduResponse.BaiduError)
	if aiError != nil {
		errChan <- aiError
		return
	}

	h.convertToOpenaiStream(&baiduResponse, dataChan)

	if baiduResponse.IsEnd {
		errChan <- io.EOF
		*rawLine = requester.StreamClosed
		return
	}
}

func (h *baiduStreamHandler) convertToOpenaiStream(baiduResponse *BaiduChatStreamResponse, dataChan chan string) {
	choice := types.ChatCompletionStreamChoice{
		Index: 0,
		Delta: types.ChatCompletionStreamChoiceDelta{
			Role: "assistant",
		},
	}

	if baiduResponse.FunctionCall != nil {
		if h.Request.Tools != nil {
			choice.Delta.ToolCalls = []*types.ChatCompletionToolCalls{
				{
					Id:       baiduResponse.Id,
					Type:     "function",
					Function: baiduResponse.FunctionCall,
				},
			}
			choice.FinishReason = types.FinishReasonToolCalls
		} else {
			choice.Delta.FunctionCall = baiduResponse.FunctionCall
			choice.FinishReason = types.FinishReasonFunctionCall
		}
	} else {
		choice.Delta.Content = baiduResponse.Result
		if baiduResponse.IsEnd {
			choice.FinishReason = types.FinishReasonStop
		}
	}

	chatCompletion := types.ChatCompletionStreamResponse{
		ID:      baiduResponse.Id,
		Object:  "chat.completion.chunk",
		Created: baiduResponse.Created,
		Model:   base.GetResponseModelNameFromContext(h.Context, h.Request.Model),
	}

	if baiduResponse.FunctionCall == nil {
		chatCompletion.Choices = []types.ChatCompletionStreamChoice{choice}
		responseBody, _ := json.Marshal(chatCompletion)
		dataChan <- string(responseBody)
	} else {
		choices := choice.ConvertOpenaiStream()
		for _, choice := range choices {
			chatCompletionCopy := chatCompletion
			chatCompletionCopy.Choices = []types.ChatCompletionStreamChoice{choice}
			responseBody, _ := json.Marshal(chatCompletionCopy)
			dataChan <- string(responseBody)
		}
	}

	h.Usage.TotalTokens = baiduResponse.Usage.TotalTokens
	h.Usage.PromptTokens = baiduResponse.Usage.PromptTokens
	h.Usage.CompletionTokens += baiduResponse.Usage.CompletionTokens
}
