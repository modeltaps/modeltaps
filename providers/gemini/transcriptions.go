package gemini

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/types"
)

// geminiInlineAudioMaxSize 是走 inlineData 内联音频的原始文件大小上限。
// Gemini generateContent 单请求体上限 20MB，base64 会放大约 1.33 倍，
// 这里按原始字节 15MB 卡住，超出的走 File API 上传（暂未实现）。
const geminiInlineAudioMaxSize = 15 * 1024 * 1024

// geminiTranscriptionInstruction 约束模型只吐转写文本，避免掺入解释性内容。
const geminiTranscriptionInstruction = "Transcribe the audio verbatim. Output only the transcription text, without any commentary, headings, timestamps or speaker labels."

func (p *GeminiProvider) CreateTranscriptions(request *types.AudioRequest) (*types.AudioResponseWrapper, *types.OpenAIErrorWithStatusCode) {
	if request.File == nil {
		return nil, common.StringErrorWrapperLocal("audio file is required", "invalid_request", http.StatusBadRequest)
	}

	switch request.ResponseFormat {
	case "", "json", "verbose_json", "text":
	default:
		return nil, common.StringErrorWrapperLocal("response_format "+request.ResponseFormat+" is not supported by gemini", "unsupported_response_format", http.StatusBadRequest)
	}

	if request.File.Size > geminiInlineAudioMaxSize {
		return nil, common.StringErrorWrapperLocal("audio file too large for gemini inline request (max 15MB)", "file_too_large", http.StatusBadRequest)
	}

	audioData, errWithCode := readAudioBase64(request)
	if errWithCode != nil {
		return nil, errWithCode
	}

	geminiRequest := buildTranscriptionRequest(request, audioData)

	fullRequestURL := p.GetFullRequestURL("generateContent", request.Model)
	headers := p.GetRequestHeaders()
	// 入站是 multipart/form-data，CommonRequestHeaders 会照抄，这里必须改回 JSON
	headers["Content-Type"] = "application/json"

	req, err := p.Requester.NewRequest(http.MethodPost, fullRequestURL, p.Requester.WithBody(geminiRequest), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}
	defer req.Body.Close()

	geminiResponse := &GeminiChatResponse{}
	if _, errWithCode = p.Requester.SendRequest(req, geminiResponse, false); errWithCode != nil {
		return nil, errWithCode
	}

	text := extractTranscriptionText(geminiResponse)

	audioResponseWrapper := &types.AudioResponseWrapper{}
	if request.ResponseFormat == "text" {
		audioResponseWrapper.Headers = map[string]string{"Content-Type": "text/plain; charset=utf-8"}
		audioResponseWrapper.Body = []byte(text)
	} else {
		body, err := json.Marshal(&types.AudioResponse{Text: text})
		if err != nil {
			return nil, common.ErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
		}
		audioResponseWrapper.Headers = map[string]string{"Content-Type": "application/json"}
		audioResponseWrapper.Body = body
	}

	usage := p.GetUsage()
	if usage != nil {
		if geminiResponse.UsageMetadata != nil {
			usage.PromptTokens = geminiResponse.UsageMetadata.PromptTokenCount
			usage.CompletionTokens = geminiResponse.UsageMetadata.CandidatesTokenCount
		} else {
			usage.CompletionTokens = common.CountTokenText(text, request.Model)
		}
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}

	return audioResponseWrapper, nil
}

func readAudioBase64(request *types.AudioRequest) (string, *types.OpenAIErrorWithStatusCode) {
	file, err := request.File.Open()
	if err != nil {
		return "", common.ErrorWrapper(err, "open_audio_file_failed", http.StatusBadRequest)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, geminiInlineAudioMaxSize+1))
	if err != nil {
		return "", common.ErrorWrapper(err, "read_audio_file_failed", http.StatusBadRequest)
	}
	if len(data) > geminiInlineAudioMaxSize {
		return "", common.StringErrorWrapperLocal("audio file too large for gemini inline request (max 15MB)", "file_too_large", http.StatusBadRequest)
	}

	return base64.StdEncoding.EncodeToString(data), nil
}

func buildTranscriptionRequest(request *types.AudioRequest, audioBase64 string) *GeminiChatRequest {
	instruction := []string{geminiTranscriptionInstruction}
	if request.Language != "" {
		instruction = append(instruction, "The audio language is "+request.Language+"; transcribe in that language.")
	}
	if request.Prompt != "" {
		instruction = append(instruction, "Context hint: "+request.Prompt)
	}

	geminiRequest := &GeminiChatRequest{
		Model: request.Model,
		Contents: []GeminiChatContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: strings.Join(instruction, "\n")},
					{InlineData: &GeminiInlineData{
						MimeType: getAudioMimeType(request.File.Filename, request.File.Header.Get("Content-Type")),
						Data:     audioBase64,
					}},
				},
			},
		},
		SafetySettings: []GeminiChatSafetySettings{
			{Category: "HARM_CATEGORY_HARASSMENT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_HATE_SPEECH", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_SEXUALLY_EXPLICIT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_DANGEROUS_CONTENT", Threshold: "BLOCK_NONE"},
			{Category: "HARM_CATEGORY_CIVIC_INTEGRITY", Threshold: "BLOCK_NONE"},
		},
	}

	if request.Temperature != 0 {
		temperature := float64(request.Temperature)
		geminiRequest.GenerationConfig.Temperature = &temperature
	}

	return geminiRequest
}

// geminiAudioMimeTypes 是 Gemini 音频理解支持的 MIME 类型，按扩展名映射。
var geminiAudioMimeTypes = map[string]string{
	".wav":  "audio/wav",
	".mp3":  "audio/mp3",
	".aiff": "audio/aiff",
	".aif":  "audio/aiff",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".flac": "audio/flac",
	".m4a":  "audio/mp4",
	".mp4":  "audio/mp4",
	".mpeg": "audio/mpeg",
	".mpga": "audio/mpeg",
	".webm": "audio/webm",
}

// getAudioMimeType 以文件扩展名优先（客户端上传的 Content-Type 常为 application/octet-stream），
// 扩展名不认识时退回表单声明的类型，再兜底 audio/mp3。
func getAudioMimeType(filename, declaredType string) string {
	ext := strings.ToLower(path.Ext(filename))
	if mimeType, ok := geminiAudioMimeTypes[ext]; ok {
		return mimeType
	}

	declaredType = strings.TrimSpace(strings.Split(declaredType, ";")[0])
	if strings.HasPrefix(declaredType, "audio/") {
		return declaredType
	}

	return "audio/mp3"
}

// extractTranscriptionText 汇总候选内容里的文本 part，跳过思考内容。
func extractTranscriptionText(response *GeminiChatResponse) string {
	if response == nil || len(response.Candidates) == 0 {
		return ""
	}

	var texts []string
	for _, part := range response.Candidates[0].Content.Parts {
		if part.Thought || part.Text == "" {
			continue
		}
		texts = append(texts, part.Text)
	}

	return strings.TrimSpace(strings.Join(texts, "\n"))
}
