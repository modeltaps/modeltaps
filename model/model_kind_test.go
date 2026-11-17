package model

import "testing"

func TestModelKindFromName(t *testing.T) {
	cases := map[string]string{
		"openai/gpt-5":                          "",
		"anthropic/claude-opus-5.5":             "",
		"openai/gpt-realtime-2":                 "",
		"openai/tts-1-hd":                       ModelKindSpeech,
		"mistral/voxtral-mini-tts-3":            ModelKindSpeech,
		"elevenlabs/eleven-v3":                  ModelKindSpeech,
		"microsoft/mai-voice-2":                 ModelKindSpeech,
		"fish-audio/s2-pro":                     ModelKindSpeech,
		"fish-audio/transcribe-1":               ModelKindTranscription,
		"openai/whisper-1":                      ModelKindTranscription,
		"elevenlabs/scribe-v2":                  ModelKindTranscription,
		"spacexai/grok-stt":                     ModelKindTranscription,
		"openai/gpt-4o-mini-transcribe":         ModelKindTranscription,
		"elevenlabs/music-v2":                   ModelKindAudio,
		"elevenlabs/sound-effects-v2":           ModelKindAudio,
		"openai/text-embedding-3-small":         ModelKindEmbedding,
		"cohere/embed-v4.0":                     ModelKindEmbedding,
		"cohere/rerank-v3.5":                    ModelKindRerank,
		"google/gemini-decision-1":              ModelKindDecision,
		"openai/gpt-6-luna-decisions":           ModelKindDecision,
		"parallel/parallel-decisions-fast":      ModelKindDecision,
		"perplexity/pplx-decider-v1.3":          ModelKindDecision,
		"decision-1":                            ModelKindDecision,
		"google/veo-3.1-fast-generate-001":      ModelKindVideo,
		"klingai/kling-v3.0-t2v":                ModelKindVideo,
		"spacexai/grok-imagine-video":           ModelKindVideo,
		"alibaba/wan-v2.6-i2v":                  ModelKindVideo,
		"spacexai/grok-imagine-image":           ModelKindImage,
		"bfl/flux-2-pro":                        ModelKindImage,
		"openai/gpt-image-2":                    ModelKindImage,
		"google/gemini-3.1-flash-image":         ModelKindImage,
		"bytedance/seedream-4.5":                ModelKindImage,
		"inference-net/schematron-v2-small":     "",
		"mistral/codestral":                     "",
		"tencent/hy-mt2-lite":                   "",
		"stealth/glyph-cluster":                 "",
		"thinkingmachines/inkling-2-small":      "",
		"openai/gpt-oss-safeguard-20b":          "",
		"deepseek/deepseek-v4-flash-vision-exp": "",
	}
	for name, want := range cases {
		if got := ModelKindFromName(name); got != want {
			t.Errorf("ModelKindFromName(%q) = %q, want %q", name, got, want)
		}
	}
}

// The upstream's own type wins, then the catalog's non-text output, then the name.
func TestResolveModelKind(t *testing.T) {
	if got := ResolveModelKind(ModelKindEmbedding, []string{"image"}, "x-image"); got != ModelKindEmbedding {
		t.Errorf("upstream kind should win, got %q", got)
	}
	if got := ResolveModelKind("", []string{"text", "image"}, "gemini-flash"); got != ModelKindImage {
		t.Errorf("catalog image output should win over the name, got %q", got)
	}
	if got := ResolveModelKind("", []string{"text"}, "openai/tts-1"); got != ModelKindSpeech {
		t.Errorf("text-only output leaves it to the name, got %q", got)
	}
	if got := ModelKindFromUpstreamType("evaluation"); got != ModelKindDecision {
		t.Errorf("Vercel's evaluation type is a decision model, got %q", got)
	}
	if got := ModelKindFromUpstreamType("language"); got != "" {
		t.Errorf("a language model has no kind, got %q", got)
	}
	if got := ModelKindFromOutputModalities([]string{"speech"}); got != ModelKindSpeech {
		t.Errorf("speech output, got %q", got)
	}
}

// Endpoints without chat name the kind; otherwise output modalities and the name decide.
func TestCatalogModelKind(t *testing.T) {
	cases := []struct {
		name, endpoints, output, want string
	}{
		{"text-embedding-3-small", `["embeddings"]`, "", ModelKindEmbedding},
		{"gpt-realtime", `["realtime"]`, `["text","audio"]`, ModelKindRealtime},
		{"gpt-image-1", `["images"]`, `["image"]`, ModelKindImage},
		{"liquid-d1", `["decisions"]`, `["text"]`, ModelKindDecision},
		{"gpt-5.5", `["chat","responses"]`, `["text"]`, ""},
		{"gemini-3-flash-image", `["chat","responses","images"]`, `["text","image"]`, ModelKindImage},
		{"openai/tts-1", "", `["text"]`, ModelKindSpeech},
		{"claude-opus-5-5", "", "", ""},
	}
	for _, c := range cases {
		if got := CatalogModelKind(c.name, c.endpoints, c.output); got != c.want {
			t.Errorf("CatalogModelKind(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
