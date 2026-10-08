package minimax

import (
	"bytes"
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/types"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// openaiVoiceOrder 固定 OpenAI 预置 voice 名的遍历顺序，便于导出稳定的内置音色列表。
var openaiVoiceOrder = []string{"alloy", "echo", "fable", "onyx", "nova", "shimmer"}

// builtinVoiceGenders 标注默认映射里 MiniMax 音色的性别。
var builtinVoiceGenders = map[string]string{
	"female-chengshu":    "female",
	"male-qn-qingse":     "male",
	"male-qn-jingying":   "male",
	"presenter_male":     "male",
	"presenter_female":   "female",
	"audiobook_female_1": "female",
}

func defaultVoiceMap() map[string][]string {
	return map[string][]string{
		"alloy":   {"female-chengshu"},
		"echo":    {"male-qn-qingse"},
		"fable":   {"male-qn-jingying"},
		"onyx":    {"presenter_male"},
		"nova":    {"presenter_female"},
		"shimmer": {"audiobook_female_1"},
	}
}

// systemChineseVoices 是 MiniMax 官方系统音色列表里的中文（普通话 / 粤语）音色，性别按音色名称标注，
// 名称看不出性别的留空（来源：platform.minimaxi.com/docs/faq/system-voice-id，2026-09）。
var systemChineseVoices = []types.TTSVoice{
	{ID: "male-qn-qingse", Language: "zh", Gender: "male"},
	{ID: "male-qn-jingying", Language: "zh", Gender: "male"},
	{ID: "male-qn-badao", Language: "zh", Gender: "male"},
	{ID: "male-qn-daxuesheng", Language: "zh", Gender: "male"},
	{ID: "female-shaonv", Language: "zh", Gender: "female"},
	{ID: "female-yujie", Language: "zh", Gender: "female"},
	{ID: "female-chengshu", Language: "zh", Gender: "female"},
	{ID: "female-tianmei", Language: "zh", Gender: "female"},
	{ID: "male-qn-qingse-jingpin", Language: "zh", Gender: "male"},
	{ID: "male-qn-jingying-jingpin", Language: "zh", Gender: "male"},
	{ID: "male-qn-badao-jingpin", Language: "zh", Gender: "male"},
	{ID: "male-qn-daxuesheng-jingpin", Language: "zh", Gender: "male"},
	{ID: "female-shaonv-jingpin", Language: "zh", Gender: "female"},
	{ID: "female-yujie-jingpin", Language: "zh", Gender: "female"},
	{ID: "female-chengshu-jingpin", Language: "zh", Gender: "female"},
	{ID: "female-tianmei-jingpin", Language: "zh", Gender: "female"},
	{ID: "clever_boy", Language: "zh", Gender: "male"},
	{ID: "cute_boy", Language: "zh", Gender: "male"},
	{ID: "lovely_girl", Language: "zh", Gender: "female"},
	{ID: "cartoon_pig", Language: "zh"},
	{ID: "bingjiao_didi", Language: "zh", Gender: "male"},
	{ID: "junlang_nanyou", Language: "zh", Gender: "male"},
	{ID: "chunzhen_xuedi", Language: "zh", Gender: "male"},
	{ID: "lengdan_xiongzhang", Language: "zh", Gender: "male"},
	{ID: "badao_shaoye", Language: "zh", Gender: "male"},
	{ID: "tianxin_xiaoling", Language: "zh", Gender: "female"},
	{ID: "qiaopi_mengmei", Language: "zh", Gender: "female"},
	{ID: "wumei_yujie", Language: "zh", Gender: "female"},
	{ID: "diadia_xuemei", Language: "zh", Gender: "female"},
	{ID: "danya_xuejie", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Reliable_Executive", Language: "zh"},
	{ID: "Chinese (Mandarin)_News_Anchor", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Mature_Woman", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Unrestrained_Young_Man", Language: "zh", Gender: "male"},
	{ID: "Arrogant_Miss", Language: "zh", Gender: "female"},
	{ID: "Robot_Armor", Language: "zh"},
	{ID: "Chinese (Mandarin)_Kind-hearted_Antie", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_HK_Flight_Attendant", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Humorous_Elder", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Gentleman", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Warm_Bestie", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Male_Announcer", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Sweet_Lady", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Southern_Young_Man", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Wise_Women", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Gentle_Youth", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Warm_Girl", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Kind-hearted_Elder", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Cute_Spirit", Language: "zh"},
	{ID: "Chinese (Mandarin)_Radio_Host", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Lyrical_Voice", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Straightforward_Boy", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Sincere_Adult", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Gentle_Senior", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Stubborn_Friend", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Crisp_Girl", Language: "zh", Gender: "female"},
	{ID: "Chinese (Mandarin)_Pure-hearted_Boy", Language: "zh", Gender: "male"},
	{ID: "Chinese (Mandarin)_Soft_Girl", Language: "zh", Gender: "female"},
	{ID: "Cantonese_GentleLady", Language: "zh", Gender: "female"},
	{ID: "Cantonese_PlayfulMan", Language: "zh", Gender: "male"},
	{ID: "Cantonese_CuteGirl", Language: "zh", Gender: "female"},
	{ID: "Cantonese_KindWoman", Language: "zh", Gender: "female"},
}

// BuiltinSpeechVoices 返回内置 MiniMax 中文音色：先是默认映射里的音色（按 OpenAI voice 顺序），
// 再接官方系统中文音色，按 id 去重。
func BuiltinSpeechVoices() []types.TTSVoice {
	mapping := defaultVoiceMap()
	seen := make(map[string]bool, len(mapping)+len(systemChineseVoices))
	voices := make([]types.TTSVoice, 0, len(mapping)+len(systemChineseVoices))
	for _, key := range openaiVoiceOrder {
		id := mapping[key][0]
		if seen[id] {
			continue
		}
		seen[id] = true
		voices = append(voices, types.TTSVoice{ID: id, Language: "zh", Gender: builtinVoiceGenders[id]})
	}
	for _, voice := range systemChineseVoices {
		if seen[voice.ID] {
			continue
		}
		seen[voice.ID] = true
		voices = append(voices, voice)
	}
	return voices
}

func (p *MiniMaxProvider) GetVoiceMap() map[string][]string {
	defaultVoiceMapping := defaultVoiceMap()

	if p.Channel.Plugin == nil {
		return defaultVoiceMapping
	}

	customVoiceMapping, ok := p.Channel.Plugin.Data()["voice"]
	if !ok {
		return defaultVoiceMapping
	}

	for key, value := range customVoiceMapping {
		if _, exists := defaultVoiceMapping[key]; !exists {
			continue
		}
		customVoiceValue, isString := value.(string)
		if !isString || customVoiceValue == "" {
			continue
		}
		customizeVoice := strings.Split(customVoiceValue, "|")
		defaultVoiceMapping[key] = customizeVoice
	}

	return defaultVoiceMapping
}

func (p *MiniMaxProvider) getRequestBody(request *types.SpeechAudioRequest) *SpeechRequest {

	var voice, emotion string
	voiceMap := p.GetVoiceMap()
	if voiceMap[request.Voice] != nil {
		voice = voiceMap[request.Voice][0]
		if len(voiceMap[request.Voice]) > 1 {
			emotion = voiceMap[request.Voice][1]
		}
	} else {
		voice = request.Voice
	}

	speechRequest := &SpeechRequest{
		Model: request.Model,
		Text:  request.Input,
		VoiceSetting: VoiceSetting{
			VoiceID: voice,
			Emotion: emotion,
			Speed:   request.Speed,
		},
	}

	// mp3-1-32000-128000
	if request.ResponseFormat != "" {
		formats := strings.Split(request.ResponseFormat, "-")
		speechRequest.AudioSetting = &AudioSetting{
			Format: formats[0],
		}
		if len(formats) > 1 {
			speechRequest.AudioSetting.Channel = utils.String2Int64(formats[1])
		}
		if len(formats) > 2 {
			speechRequest.AudioSetting.SampleRate = utils.String2Int64(formats[2])
		}
		if len(formats) > 3 {
			speechRequest.AudioSetting.Bitrate = utils.String2Int64(formats[3])
		}
	}

	return speechRequest
}

func (p *MiniMaxProvider) CreateSpeech(request *types.SpeechAudioRequest) (*http.Response, *types.OpenAIErrorWithStatusCode) {
	url, errWithCode := p.GetSupportedAPIUri(config.RelayModeAudioSpeech)
	if errWithCode != nil {
		return nil, errWithCode
	}
	fullRequestURL := p.GetFullRequestURL(url, request.Model)
	headers := p.GetRequestHeaders()

	requestBody := p.getRequestBody(request)

	req, err := p.Requester.NewRequest(http.MethodPost, fullRequestURL, p.Requester.WithBody(requestBody), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}
	defer req.Body.Close()

	speechResponse := &SpeechResponse{}
	_, errWithCode = p.Requester.SendRequest(req, speechResponse, false)
	if errWithCode != nil {
		return nil, errWithCode
	}

	if speechResponse.BaseResp.StatusCode != 0 {
		return nil, common.ErrorWrapper(errors.New(speechResponse.BaseResp.StatusMsg), "speech_error", http.StatusInternalServerError)
	}

	audioBytes, err := hex.DecodeString(speechResponse.Data.Audio)
	if err != nil {
		return nil, common.ErrorWrapper(err, "decode_audio_data_failed", http.StatusInternalServerError)
	}

	body := io.NopCloser(bytes.NewReader(audioBytes))

	response := &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       body,
		Header:     make(http.Header),
	}

	response.Header.Set("Content-Type", "audio/"+speechResponse.ExtraInfo.AudioFormat) // 例如 "audio/wav"
	response.Header.Set("Content-Length", strconv.FormatInt(speechResponse.ExtraInfo.AudioSize, 10))

	p.Usage.PromptTokens = speechResponse.ExtraInfo.UsageCharacters
	p.Usage.TotalTokens = speechResponse.ExtraInfo.UsageCharacters

	return response, nil
}
