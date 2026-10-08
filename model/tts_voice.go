package model

import (
	"regexp"
	"strings"

	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/types"
)

const (
	TTSVoiceGenderFemale  = "female"
	TTSVoiceGenderMale    = "male"
	TTSVoiceGenderNeutral = "neutral"
)

var (
	// Kokoro：首字母为语言、第二字母 f/m 为性别，如 zf_xiaoxiao。
	kokoroVoicePattern = regexp.MustCompile(`^([abefhijpz])([fm])_[a-z0-9]+$`)
	// Deepgram：aura-2-thalia-en / flux-alexis-en，末段为语言。
	deepgramVoicePattern = regexp.MustCompile(`^(?:aura(?:-[0-9]+)?|flux)-[a-z]+-([a-z]{2})$`)
	// 区域前缀：en-US-Harper:MAI-Voice-2 / zh-CN-XiaochenNeural。
	localeVoicePattern = regexp.MustCompile(`^([a-z]{2})-[A-Z]{2}-`)
	// Voxtral：en_paul_sad / gb_oliver_neutral。
	voxtralVoicePattern = regexp.MustCompile(`^([a-z]{2})_[a-z]+_[a-z]+$`)
	// MiniMax：English_radiant_girl / Chinese (Mandarin)_Reliable_Executive。
	minimaxVoicePattern = regexp.MustCompile(`^([A-Z][a-z]+)(?: \([^)]*\))?_`)

	voiceTokenPattern = regexp.MustCompile(`[A-Z]?[a-z]+|[A-Z]+`)
)

var kokoroVoiceLanguages = map[string]string{
	"a": "en", "b": "en", "e": "es", "f": "fr", "h": "hi", "i": "it", "j": "ja", "p": "pt", "z": "zh",
}

var voxtralVoiceLanguages = map[string]string{
	"en": "en", "gb": "en", "us": "en", "fr": "fr", "es": "es", "de": "de", "it": "it", "pt": "pt",
	"nl": "nl", "hi": "hi", "ar": "ar", "zh": "zh", "ja": "ja", "ko": "ko",
}

var minimaxVoiceLanguages = map[string]string{
	"english": "en", "chinese": "zh", "cantonese": "zh", "japanese": "ja", "korean": "ko",
	"spanish": "es", "french": "fr", "german": "de", "portuguese": "pt", "italian": "it",
	"russian": "ru", "arabic": "ar", "turkish": "tr", "dutch": "nl", "ukrainian": "uk",
	"vietnamese": "vi", "indonesian": "id", "thai": "th", "polish": "pl", "romanian": "ro",
	"greek": "el", "czech": "cs", "finnish": "fi", "hindi": "hi",
}

// 性别词按词尾匹配，女声词先判（woman 以 man 结尾）。
var (
	femaleVoiceWords = []string{"female", "woman", "girl", "lady", "queen"}
	maleVoiceWords   = []string{"male", "man", "boy", "bloke"}
)

// InferTTSVoice 从音色 id 推断语言与性别，推不出的字段留空串。
func InferTTSVoice(id string) types.TTSVoice {
	voice := types.TTSVoice{ID: id}
	if m := kokoroVoicePattern.FindStringSubmatch(id); m != nil {
		voice.Language = kokoroVoiceLanguages[m[1]]
		if m[2] == "f" {
			voice.Gender = TTSVoiceGenderFemale
		} else {
			voice.Gender = TTSVoiceGenderMale
		}
		return voice
	}
	switch {
	case deepgramVoicePattern.MatchString(id):
		voice.Language = deepgramVoicePattern.FindStringSubmatch(id)[1]
	case localeVoicePattern.MatchString(id):
		voice.Language = localeVoicePattern.FindStringSubmatch(id)[1]
	case voxtralVoicePattern.MatchString(id):
		voice.Language = voxtralVoiceLanguages[voxtralVoicePattern.FindStringSubmatch(id)[1]]
	case minimaxVoicePattern.MatchString(id):
		voice.Language = minimaxVoiceLanguages[strings.ToLower(minimaxVoicePattern.FindStringSubmatch(id)[1])]
	}
	voice.Gender = inferTTSVoiceGender(id)
	return voice
}

func inferTTSVoiceGender(id string) string {
	for _, token := range voiceTokenPattern.FindAllString(id, -1) {
		token = strings.ToLower(token)
		for _, word := range femaleVoiceWords {
			if strings.HasSuffix(token, word) {
				return TTSVoiceGenderFemale
			}
		}
		for _, word := range maleVoiceWords {
			if strings.HasSuffix(token, word) {
				return TTSVoiceGenderMale
			}
		}
	}
	return ""
}

// TTSVoicesFromIDs 逐个推断音色 id 的语言与性别，跳过空 id。
func TTSVoicesFromIDs(ids []string) []types.TTSVoice {
	voices := make([]types.TTSVoice, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		voices = append(voices, InferTTSVoice(id))
	}
	return voices
}

// ModelSupportedVoices 返回同步写入的音色 id；别名随主名，未同步或无目录行返回空切片。
func ModelSupportedVoices(name string) []string {
	canonical, _ := ResolveCanonicalModel(name)
	info := lookupModelInfo(canonical)
	if info == nil || info.SupportedVoices == "" {
		return []string{}
	}
	voices, _ := utils.UnmarshalString[[]string](info.SupportedVoices)
	if voices == nil {
		return []string{}
	}
	return voices
}
