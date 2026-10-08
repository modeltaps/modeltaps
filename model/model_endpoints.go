package model

import (
	"encoding/json"
	"strings"

	"github.com/modeltaps/modeltaps/types"
)

// 接口能力词表：声明模型可以被哪些接口调用，空串（未设置）表示未知。
const (
	ModelEndpointChat               = "chat"
	ModelEndpointResponses          = "responses"
	ModelEndpointImages             = "images"
	ModelEndpointAudioSpeech        = "audio.speech"
	ModelEndpointAudioTranscription = "audio.transcription"
	ModelEndpointEmbeddings         = "embeddings"
	ModelEndpointRerank             = "rerank"
	// ModelEndpointRealtime 对应 /v1/realtime 的 WebSocket 入口：它与 chat 是两套连接语义，
	// 既推不出也不影响任务类型（mode），只声明「该模型可经实时接口调用」。
	ModelEndpointRealtime = "realtime"
)

// modelEndpointOrder 决定序列化顺序，便于比对与测试。
var modelEndpointOrder = []string{
	ModelEndpointChat,
	ModelEndpointResponses,
	ModelEndpointRealtime,
	ModelEndpointImages,
	ModelEndpointAudioSpeech,
	ModelEndpointAudioTranscription,
	ModelEndpointEmbeddings,
	ModelEndpointRerank,
}

// isKnownModelEndpoint 判断单个取值是否属于词表。
func isKnownModelEndpoint(endpoint string) bool {
	for _, known := range modelEndpointOrder {
		if endpoint == known {
			return true
		}
	}
	return false
}

// UnknownModelEndpoints 返回原始数组里词表外的取值（去重、保持入参顺序）。
// 供种子等「写错要报错」的场景在归一化之前校验：ModelEndpointsJSON 会静默丢弃这些值。
func UnknownModelEndpoints(endpoints []string) []string {
	seen := make(map[string]bool, len(endpoints))
	unknown := make([]string, 0)
	for _, endpoint := range endpoints {
		if isKnownModelEndpoint(endpoint) || seen[endpoint] {
			continue
		}
		seen[endpoint] = true
		unknown = append(unknown, endpoint)
	}
	return unknown
}

// ModelEndpointsJSON 按词表固定顺序序列化接口能力；词表外的值丢弃，
// 一项都没有时返回空串（未设置），与能力字段的 "[]"（明确为空）语义不同。
func ModelEndpointsJSON(endpoints []string) string {
	has := make(map[string]bool, len(endpoints))
	for _, e := range endpoints {
		has[e] = true
	}
	ordered := make([]string, 0, len(modelEndpointOrder))
	for _, e := range modelEndpointOrder {
		if has[e] {
			ordered = append(ordered, e)
		}
	}
	if len(ordered) == 0 {
		return ""
	}
	b, err := json.Marshal(ordered)
	if err != nil {
		return ""
	}
	return string(b)
}

// IsValidModelEndpoints 校验后台写入的接口能力字段：空串（未设置）视为合法，
// 其余必须是 JSON 数组且元素全部属于词表。
func IsValidModelEndpoints(raw string) bool {
	if raw == "" {
		return true
	}
	var endpoints []string
	if err := json.Unmarshal([]byte(raw), &endpoints); err != nil {
		return false
	}
	return len(UnknownModelEndpoints(endpoints)) == 0
}

// hasModality 大小写不敏感地判断模态数组是否含某模态。
func hasModality(modalities []string, want string) bool {
	for _, m := range modalities {
		if strings.EqualFold(strings.TrimSpace(m), want) {
			return true
		}
	}
	return false
}

// onlyModality 判断模态数组非空且每一项都是 want。
func onlyModality(modalities []string, want string) bool {
	if len(modalities) == 0 {
		return false
	}
	for _, m := range modalities {
		if !strings.EqualFold(strings.TrimSpace(m), want) {
			return false
		}
	}
	return true
}

// speechToTextNameHints 是转写模型名里常见的标记，用于区分「输入 text+audio、输出 text」的
// 转写模型与音频对话模型。
var speechToTextNameHints = []string{"transcribe", "whisper", "asr", "stt"}

// isSpeechToText 判断 models.dev 通用模态下的转写（STT）模型。
func isSpeechToText(modelName string, input, output []string) bool {
	if !onlyModality(output, "text") || !hasModality(input, "audio") {
		return false
	}
	if onlyModality(input, "audio") {
		return true
	}
	if hasModality(input, "image") || hasModality(input, "video") {
		return false
	}
	name := strings.ToLower(modelName)
	for _, hint := range speechToTextNameHints {
		if strings.Contains(name, hint) {
			return true
		}
	}
	return false
}

// ModelEndpointsFromModalities 由模态推导接口能力（同步用）：
// 输入含 text → chat + responses；输出含 image 且模型属图像生成名单 → images；
// 对话模型即使输出含 audio 也不推 audio.speech（音频经 chat 输出，不是 TTS 接口）。
// 推不出任何一项时返回空串，留给管理员设置。
// OpenRouter 专用输出模态：speech（TTS）→ 仅 audio.speech；transcription（STT）→ 仅 audio.transcription，
// 这两类模型只能走音频专用接口，不推 chat / responses。
// models.dev 的通用模态：输出只有 audio（TTS）→ 仅 audio.speech；输入只有 audio、输出只有 text（STT），
// 或输入含 audio 且名称带转写标记（transcribe / whisper / asr / stt）→ 仅 audio.transcription。
func ModelEndpointsFromModalities(modelName string, input, output []string) string {
	if hasModality(output, "transcription") {
		return ModelEndpointsJSON([]string{ModelEndpointAudioTranscription})
	}
	if hasModality(output, "speech") {
		return ModelEndpointsJSON([]string{ModelEndpointAudioSpeech})
	}
	if onlyModality(output, "audio") {
		return ModelEndpointsJSON([]string{ModelEndpointAudioSpeech})
	}
	if isSpeechToText(modelName, input, output) {
		return ModelEndpointsJSON([]string{ModelEndpointAudioTranscription})
	}
	endpoints := make([]string, 0, 3)
	if hasModality(input, "text") {
		endpoints = append(endpoints, ModelEndpointChat, ModelEndpointResponses)
	}
	if hasModality(output, "image") && types.IsImageGenerationModel(modelName) {
		endpoints = append(endpoints, ModelEndpointImages)
	}
	return ModelEndpointsJSON(endpoints)
}

// legacyModelEndpointsFromModalities 复现旧版同步推导（输入含 text → chat + responses；
// 输出含 image 且属图像生成名单 → images；输出含 audio → audio.speech），
// 仅供存量修正迁移识别「仍是同步原值、未被管理员改过」的行。
func legacyModelEndpointsFromModalities(modelName string, input, output []string) string {
	endpoints := make([]string, 0, 4)
	if hasModality(input, "text") {
		endpoints = append(endpoints, ModelEndpointChat, ModelEndpointResponses)
	}
	if hasModality(output, "image") && types.IsImageGenerationModel(modelName) {
		endpoints = append(endpoints, ModelEndpointImages)
	}
	if hasModality(output, "audio") {
		endpoints = append(endpoints, ModelEndpointAudioSpeech)
	}
	return ModelEndpointsJSON(endpoints)
}

// ModelModeFromEndpoints 由接口能力（+ 输出模态）推导任务类型：
// 含 images → image；含 chat 且输出含 image → chat_image；含 chat → chat；
// 推不出时返回空串。endpoints / outputModalities 均为 JSON 数组字符串。
func ModelModeFromEndpoints(endpointsJSON, outputModalitiesJSON string) string {
	if endpointsJSON == "" {
		return ""
	}
	var endpoints []string
	if err := json.Unmarshal([]byte(endpointsJSON), &endpoints); err != nil {
		return ""
	}
	var output []string
	_ = json.Unmarshal([]byte(outputModalitiesJSON), &output)

	hasChat := false
	for _, e := range endpoints {
		switch e {
		case ModelEndpointImages:
			return ModelModeImage
		case ModelEndpointChat, ModelEndpointResponses:
			hasChat = true
		}
	}
	if !hasChat {
		return ""
	}
	if hasModality(output, "image") {
		return ModelModeChatImage
	}
	return ModelModeChat
}
