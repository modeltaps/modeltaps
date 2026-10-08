package openai

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/types"
	"net/http"
)

// speechVoiceNames 是 OpenAI TTS 官方当前的内置音色。
var speechVoiceNames = []string{
	"alloy", "ash", "ballad", "coral", "echo", "fable", "nova",
	"onyx", "sage", "shimmer", "verse", "marin", "cedar",
}

// defaultSpeechVoiceNames 是最早的六个 OpenAI 音色，各家兼容实现普遍支持。
var defaultSpeechVoiceNames = []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}

// BuiltinSpeechVoices 返回 OpenAI 官方内置音色；音色多语种、官方未标注性别，语言与性别留空。
func BuiltinSpeechVoices() []types.TTSVoice {
	return speechVoicesFromNames(speechVoiceNames)
}

// DefaultSpeechVoices 返回六个经典 OpenAI 音色，供没有专属音色表的模型兜底。
func DefaultSpeechVoices() []types.TTSVoice {
	return speechVoicesFromNames(defaultSpeechVoiceNames)
}

func speechVoicesFromNames(names []string) []types.TTSVoice {
	voices := make([]types.TTSVoice, 0, len(names))
	for _, name := range names {
		voices = append(voices, types.TTSVoice{ID: name})
	}
	return voices
}

func (p *OpenAIProvider) CreateSpeech(request *types.SpeechAudioRequest) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	req, errWithCode := p.GetRequestTextBody(config.RelayModeAudioSpeech, request.Model, request)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer req.Body.Close()

	// 发送请求
	var resp *http.Response
	resp, errWithCode = p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}

	if resp.Header.Get("Content-Type") == "application/json" {
		return nil, requester.HandleErrorResp(resp, p.Requester.ErrorHandler, p.Requester.IsOpenAI)
	}

	p.Usage.TotalTokens = p.Usage.PromptTokens

	return resp, nil
}
