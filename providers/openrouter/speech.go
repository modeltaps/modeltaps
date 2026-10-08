package openrouter

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/providers/gemini"
	"github.com/modeltaps/modeltaps/types"
)

// CreateSpeech 在 OpenAI 兼容转发之上补一层：OpenRouter 上的 Gemini TTS 只出裸 PCM，
// 请求 wav 时改向上游要 pcm，再补 WAV 头返回 audio/wav；其他模型与格式原样透传。
// 透传渠道下未指定格式也按 wav 处理，pcm 改写由 patchPassThroughBody 回写进原始请求体。
func (p *OpenRouterProvider) CreateSpeech(request *types.SpeechAudioRequest) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	format := strings.TrimSpace(request.ResponseFormat)
	wantWAV := strings.EqualFold(format, "wav") || (format == "" && p.Channel.PassThroughBody)
	if !isGeminiSpeechModel(request.Model) || !wantWAV {
		return p.OpenAIProvider.CreateSpeech(request)
	}

	upstreamRequest := *request
	upstreamRequest.ResponseFormat = "pcm"
	resp, errWithCode := p.OpenAIProvider.CreateSpeech(&upstreamRequest)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer resp.Body.Close()

	pcm, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.ErrorWrapper(err, "read_audio_failed", http.StatusInternalServerError)
	}
	body := gemini.PCMToWAV(pcm, resp.Header.Get("Content-Type"))

	header := http.Header{}
	header.Set("Content-Type", "audio/wav")
	header.Set("Content-Length", strconv.Itoa(len(body)))

	return &http.Response{
		StatusCode:    http.StatusOK,
		Header:        header,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
	}, nil
}

func isGeminiSpeechModel(modelName string) bool {
	return strings.HasPrefix(strings.ToLower(modelName), "google/gemini")
}
