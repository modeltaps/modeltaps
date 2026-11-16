package types

import "mime/multipart"

type SpeechAudioRequest struct {
	Model          string  `json:"model" binding:"required"`
	Input          string  `json:"input" binding:"required"`
	Voice          string  `json:"voice" binding:"required"`
	ResponseFormat string  `json:"response_format,omitempty"`
	Speed          float64 `json:"speed,omitempty"`
}

// TTSVoice 描述一个文字转语音音色：language 为 ISO 639-1 小写两字母，
// gender 为 female / male / neutral，推不出时均为空串。
type TTSVoice struct {
	ID       string `json:"id"`
	Language string `json:"language"`
	Gender   string `json:"gender"`
}

// AudioResponseFormatDiarizedJSON is the response_format that asks for speaker-labelled
// segments. Modeltaps uses the OpenAI diarized_json shape as its outward standard for
// meeting transcription; adapters for other upstreams translate to and from it.
const AudioResponseFormatDiarizedJSON = "diarized_json"

// AudioRequest is the multipart body of /v1/audio/transcriptions and /v1/audio/translations.
// Array fields use the `name[]` keys the OpenAI SDKs send.
type AudioRequest struct {
	File           *multipart.FileHeader `form:"file" binding:"required"`
	Model          string                `form:"model" binding:"required"`
	Language       string                `form:"language"`
	Prompt         string                `form:"prompt"`
	ResponseFormat string                `form:"response_format"`
	Temperature    float32               `form:"temperature"`

	TimestampGranularities []string `form:"timestamp_granularities[]"`
	Include                []string `form:"include[]"`
	// ChunkingStrategy is "auto" or a JSON-encoded server_vad object; required upstream for
	// diarization of audio longer than 30 seconds.
	ChunkingStrategy string `form:"chunking_strategy"`
	// KnownSpeakerNames and KnownSpeakerReferences pair up by index: a name and a short
	// reference clip (data URL) of that speaker.
	KnownSpeakerNames      []string `form:"known_speaker_names[]"`
	KnownSpeakerReferences []string `form:"known_speaker_references[]"`

	// FormValues holds every non-file field of the incoming form, so a rebuilt upstream
	// form can carry fields this struct does not know about.
	FormValues map[string][]string `form:"-" json:"-"`
}

// WantsDiarization reports whether the caller asked for speaker-labelled output.
func (r *AudioRequest) WantsDiarization() bool {
	return r.ResponseFormat == AudioResponseFormatDiarizedJSON ||
		len(r.KnownSpeakerNames) > 0 || len(r.KnownSpeakerReferences) > 0
}

// AudioResponse covers the json, verbose_json and diarized_json transcription shapes.
type AudioResponse struct {
	Task     string           `json:"task,omitempty"`
	Language string           `json:"language,omitempty"`
	Duration float64          `json:"duration,omitempty"`
	Segments []AudioSegment   `json:"segments,omitempty"`
	Text     string           `json:"text"`
	Words    []AudioWordsList `json:"words,omitempty"`
	Usage    *AudioUsage      `json:"usage,omitempty"`
}

// AudioSegment is one transcript segment. Speaker is set only in diarized_json.
// ID is an integer in verbose_json and a string ("seg_0") in diarized_json.
type AudioSegment struct {
	ID      any     `json:"id,omitempty"`
	Type    string  `json:"type,omitempty"`
	Speaker string  `json:"speaker,omitempty"`
	Start   float64 `json:"start"`
	End     float64 `json:"end"`
	Text    string  `json:"text"`
}

type AudioWordsList struct {
	Word  string  `json:"word"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// AudioUsage is the upstream usage block: type "duration" carries Seconds, type "tokens"
// carries the token counts.
type AudioUsage struct {
	Type         string  `json:"type,omitempty"`
	Seconds      float64 `json:"seconds,omitempty"`
	InputTokens  int     `json:"input_tokens,omitempty"`
	OutputTokens int     `json:"output_tokens,omitempty"`
	TotalTokens  int     `json:"total_tokens,omitempty"`
}

type AudioResponseWrapper struct {
	Headers map[string]string
	Body    []byte
}
