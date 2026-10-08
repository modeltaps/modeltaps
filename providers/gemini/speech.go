package gemini

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/types"
)

// geminiDefaultVoice 是 voice 无法映射到任何 Gemini 预置音色时的兜底音色。
const geminiDefaultVoice = "Kore"

// geminiDefaultSampleRate 是 Gemini TTS 未在 mimeType 中声明 rate 时的采样率。
const geminiDefaultSampleRate = 24000

// geminiPrebuiltVoiceNames 是 Gemini 预置音色名（规范大小写）。
var geminiPrebuiltVoiceNames = []string{
	"Zephyr", "Puck", "Charon", "Kore", "Fenrir", "Leda", "Orus", "Aoede",
	"Callirrhoe", "Autonoe", "Enceladus", "Iapetus", "Umbriel", "Algieba",
	"Despina", "Erinome", "Algenib", "Rasalgethi", "Laomedeia", "Achernar",
	"Alnilam", "Schedar", "Gacrux", "Pulcherrima", "Achird", "Zubenelgenubi",
	"Vindemiatrix", "Sadachbia", "Sadaltager", "Sulafat",
}

// geminiPrebuiltVoices 是 Gemini 预置音色表（小写名 -> 规范名），
// 允许客户端直接用 Gemini 音色名调用。
var geminiPrebuiltVoices = buildPrebuiltVoiceIndex(geminiPrebuiltVoiceNames)

// BuiltinSpeechVoices 返回 Gemini 预置音色列表；音色多语种、官方未标注性别，语言与性别留空。
func BuiltinSpeechVoices() []types.TTSVoice {
	voices := make([]types.TTSVoice, 0, len(geminiPrebuiltVoiceNames))
	for _, name := range geminiPrebuiltVoiceNames {
		voices = append(voices, types.TTSVoice{ID: name})
	}
	return voices
}

// openaiVoiceToGemini 把 OpenAI 预置 voice 名映射到 Gemini 预置音色。
var openaiVoiceToGemini = map[string]string{
	"alloy":   "Kore",
	"echo":    "Puck",
	"fable":   "Aoede",
	"onyx":    "Charon",
	"nova":    "Leda",
	"shimmer": "Zephyr",
	"ash":     "Fenrir",
	"ballad":  "Orus",
	"coral":   "Autonoe",
	"sage":    "Umbriel",
	"verse":   "Iapetus",
}

func buildPrebuiltVoiceIndex(names []string) map[string]string {
	index := make(map[string]string, len(names))
	for _, name := range names {
		index[strings.ToLower(name)] = name
	}
	return index
}

// mapSpeechVoice 把 OpenAI speech 的 voice 参数映射为 Gemini 预置音色名，
// 未识别时回落到默认音色。
func mapSpeechVoice(voice string) string {
	key := strings.ToLower(strings.TrimSpace(voice))
	if name, ok := geminiPrebuiltVoices[key]; ok {
		return name
	}
	if name, ok := openaiVoiceToGemini[key]; ok {
		return name
	}
	return geminiDefaultVoice
}

func (p *GeminiProvider) CreateSpeech(request *types.SpeechAudioRequest) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	geminiRequest := &GeminiChatRequest{
		Model: request.Model,
		Contents: []GeminiChatContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: request.Input}},
			},
		},
		GenerationConfig: GeminiChatGenerationConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: &SpeechConfig{
				VoiceConfig: &VoiceConfig{
					PrebuiltVoiceConfig: &PrebuiltVoiceConfig{VoiceName: mapSpeechVoice(request.Voice)},
				},
			},
		},
	}

	// 不走 getChatRequest：TTS 请求不能带 tools（codeExecution 插件与 AUDIO modality 不兼容）。
	fullRequestURL := p.GetFullRequestURL("generateContent", geminiRequest.Model)
	req, errWithCode := p.NewRequestWithCustomParams(http.MethodPost, fullRequestURL, geminiRequest, p.GetRequestHeaders(), geminiRequest.Model)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	geminiResponse := &GeminiChatResponse{}
	if _, errWithCode = p.Requester.SendRequest(req, geminiResponse, false); errWithCode != nil {
		return nil, errWithCode
	}

	if geminiResponse.ErrorInfo != nil && geminiResponse.ErrorInfo.Message != "" {
		return nil, common.StringErrorWrapper(geminiResponse.ErrorInfo.Message, "gemini_error", http.StatusBadRequest)
	}

	inlineData := findSpeechInlineData(geminiResponse)
	if inlineData == nil {
		return nil, common.StringErrorWrapper("gemini response contains no audio data", "gemini_no_audio", http.StatusInternalServerError)
	}

	audio, err := base64.StdEncoding.DecodeString(inlineData.Data)
	if err != nil {
		return nil, common.ErrorWrapper(err, "decode_audio_failed", http.StatusInternalServerError)
	}

	body, contentType, errWithCode := buildSpeechBody(request.ResponseFormat, inlineData.MimeType, audio)
	if errWithCode != nil {
		return nil, errWithCode
	}

	p.Usage.TotalTokens = p.Usage.PromptTokens

	header := http.Header{}
	header.Set("Content-Type", contentType)
	header.Set("Content-Length", strconv.Itoa(len(body)))

	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}, nil
}

// buildSpeechBody 按 response_format 把 Gemini 返回的音频整理成可直接下发的字节流。
//
// Gemini TTS 返回裸 PCM（audio/L16;codec=pcm;rate=24000）。网关不内置编码器，
// 因此只支持零成本的两种输出：wav（补 RIFF 头）与 pcm（原样）。请求 mp3/opus/aac/flac
// 需要重编码，直接报错而不是静默降级到别的格式。response_format 为空时输出 wav，
// 因为裸 PCM 大多数客户端无法直接播放。
func buildSpeechBody(responseFormat, mimeType string, audio []byte) ([]byte, string, *types.OpenAIErrorWithStatusCode) {
	format := strings.ToLower(strings.TrimSpace(responseFormat))

	if !isPCMMimeType(mimeType) {
		// 上游已经给出容器格式，只能原样透传。
		if format != "" && !strings.HasSuffix(mimeType, "/"+format) {
			return nil, "", common.StringErrorWrapper(
				fmt.Sprintf("gemini returned %s, cannot convert to response_format %q", mimeType, responseFormat),
				"unsupported_response_format", http.StatusBadRequest)
		}
		return audio, mimeType, nil
	}

	switch format {
	case "", "wav":
		return wrapPCMToWAV(audio, parsePCMSampleRate(mimeType)), "audio/wav", nil
	case "pcm":
		return audio, "audio/pcm", nil
	default:
		return nil, "", common.StringErrorWrapper(
			fmt.Sprintf("response_format %q requires re-encoding which is not supported for gemini tts, use wav or pcm", responseFormat),
			"unsupported_response_format", http.StatusBadRequest)
	}
}

func isPCMMimeType(mimeType string) bool {
	lower := strings.ToLower(mimeType)
	return strings.HasPrefix(lower, "audio/l16") || strings.Contains(lower, "codec=pcm")
}

// parsePCMSampleRate 从 "audio/L16;codec=pcm;rate=24000" 里取采样率。
func parsePCMSampleRate(mimeType string) int {
	for _, param := range strings.Split(mimeType, ";") {
		param = strings.TrimSpace(param)
		if !strings.HasPrefix(strings.ToLower(param), "rate=") {
			continue
		}
		if rate, err := strconv.Atoi(strings.TrimSpace(param[len("rate="):])); err == nil && rate > 0 {
			return rate
		}
	}
	return geminiDefaultSampleRate
}

// PCMToWAV 按 mimeType 里声明的采样率（缺省 24000）给 Gemini TTS 的裸 PCM 补上 WAV 头，
// 供经其他渠道（如 OpenRouter）转发的 Gemini TTS 复用。
func PCMToWAV(pcm []byte, mimeType string) []byte {
	return wrapPCMToWAV(pcm, parsePCMSampleRate(mimeType))
}

// wrapPCMToWAV 给 16bit 单声道小端 PCM 补上 44 字节 RIFF/WAVE 头。
func wrapPCMToWAV(pcm []byte, sampleRate int) []byte {
	const (
		numChannels   = 1
		bitsPerSample = 16
	)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8

	buf := bytes.NewBuffer(make([]byte, 0, 44+len(pcm)))
	buf.WriteString("RIFF")
	binary.Write(buf, binary.LittleEndian, uint32(36+len(pcm)))
	buf.WriteString("WAVE")
	buf.WriteString("fmt ")
	binary.Write(buf, binary.LittleEndian, uint32(16))
	binary.Write(buf, binary.LittleEndian, uint16(1)) // PCM
	binary.Write(buf, binary.LittleEndian, uint16(numChannels))
	binary.Write(buf, binary.LittleEndian, uint32(sampleRate))
	binary.Write(buf, binary.LittleEndian, uint32(byteRate))
	binary.Write(buf, binary.LittleEndian, uint16(blockAlign))
	binary.Write(buf, binary.LittleEndian, uint16(bitsPerSample))
	buf.WriteString("data")
	binary.Write(buf, binary.LittleEndian, uint32(len(pcm)))
	buf.Write(pcm)

	return buf.Bytes()
}

// findSpeechInlineData 取候选里第一段音频 inlineData。
func findSpeechInlineData(response *GeminiChatResponse) *GeminiInlineData {
	for i := range response.Candidates {
		parts := response.Candidates[i].Content.Parts
		for j := range parts {
			inline := parts[j].InlineData
			if inline != nil && inline.Data != "" && strings.HasPrefix(inline.MimeType, "audio/") {
				return inline
			}
		}
	}
	return nil
}
