package model

import (
	"regexp"
	"slices"
	"strings"

	"github.com/modeltaps/modeltaps/common/utils"
)

// Kinds of model the channel panel tags next to a model's name. A chat (language) model has no
// kind: it is what most of a channel lists, so only the others are marked.
const (
	ModelKindImage         = "image"
	ModelKindVideo         = "video"
	ModelKindSpeech        = "speech"        // text to speech
	ModelKindTranscription = "transcription" // speech to text
	ModelKindEmbedding     = "embedding"
	ModelKindRerank        = "rerank"
	ModelKindAudio         = "audio" // other generated audio: music, sound effects
	// ModelKindDecision answers typed questions (yes/no, choice, score) with probabilities
	// instead of generating text; it is served only on /v1/decisions.
	ModelKindDecision = "decision"
)

// ModelKindFromUpstreamType reads the model type an upstream's own list states: Vercel AI
// Gateway's "type" (language / embedding / reranking / image / video / evaluation). "" for a
// language model or a type it does not know.
func ModelKindFromUpstreamType(t string) string {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "image":
		return ModelKindImage
	case "video":
		return ModelKindVideo
	case "embedding", "embeddings":
		return ModelKindEmbedding
	case "reranking", "rerank":
		return ModelKindRerank
	case "speech", "tts":
		return ModelKindSpeech
	case "transcription", "stt", "asr":
		return ModelKindTranscription
	case "evaluation", "decision":
		return ModelKindDecision
	}
	return ""
}

// ModelKindFromOutputModalities reads the kind from what a model outputs (OpenRouter's
// architecture.output_modalities, the catalog's output modalities). Text alone says nothing: a
// catalog often lists speech or embedding models as text-out, so the name rules still get a say.
func ModelKindFromOutputModalities(out []string) string {
	has := map[string]bool{}
	for _, m := range out {
		has[strings.ToLower(m)] = true
	}
	switch {
	case has["video"]:
		return ModelKindVideo
	case has["image"]:
		return ModelKindImage
	case has["transcription"]:
		return ModelKindTranscription
	case has["speech"]:
		return ModelKindSpeech
	case has["embeddings"], has["embedding"]:
		return ModelKindEmbedding
	case has["audio"]:
		return ModelKindAudio
	}
	return ""
}

// decisionModelName matches decision models named as such ("gemini-decision-1",
// "gpt-6-luna-decisions", "pplx-decider-v1.3"). Decision models named otherwise (Liquid d1,
// TypeSafe jev) are known only from an upstream's own type.
var decisionModelName = regexp.MustCompile(`(^|[/_-])(decisions?|decider)([_.-]|$)`)

// Name rules, tried in this order: the more specific kinds first, so "grok-imagine-video" is a
// video model and "text-embedding-3" an embedding model.
var modelKindNameRules = []struct {
	kind string
	re   *regexp.Regexp
}{
	{ModelKindDecision, decisionModelName},
	{ModelKindRerank, regexp.MustCompile(`rerank`)},
	{ModelKindEmbedding, regexp.MustCompile(`embed`)},
	{ModelKindTranscription, regexp.MustCompile(`transcri|whisper|scribe|(^|[/_-])stt([_.-]|$)|(^|[/_-])asr([_.-]|$)`)},
	{ModelKindAudio, regexp.MustCompile(`music|sound-effect|lyria`)},
	{ModelKindSpeech, regexp.MustCompile(`(^|[/_-])tts([_.-]|$)|speech|(^|/)eleven-|voice|^fish-audio/`)},
	{ModelKindVideo, regexp.MustCompile(`video|(^|[/_-])veo-|(^|[/_-])kling|seedance|(^|[/_-])wan-|(^|[/_-])sora|hailuo|[it2r]2v\b`)},
	{ModelKindImage, regexp.MustCompile(`image|imagen|flux|dall-e|seedream|recraft|ideogram|stable-diffusion|sdxl|nano-banana|midjourney`)},
}

// ModelKindFromName guesses the kind from the model id ("elevenlabs/eleven-v3", "openai/tts-1",
// "google/veo-3.1-fast-generate-001"); "" when nothing matches, as for a chat model.
func ModelKindFromName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if i := strings.LastIndex(n, "/"); i >= 0 {
		// Keep one vendor segment: some vendors only make one kind of model (fish-audio).
		if j := strings.LastIndex(n[:i], "/"); j >= 0 {
			n = n[j+1:]
		}
	}
	for _, rule := range modelKindNameRules {
		if rule.re.MatchString(n) {
			return rule.kind
		}
	}
	return ""
}

// ResolveModelKind picks a model's kind from the best source that names one: the upstream's own
// list (stored with the channel's prices), then the catalog's output modalities, then the name.
func ResolveModelKind(upstreamKind string, outputModalities []string, name string) string {
	if upstreamKind != "" {
		return upstreamKind
	}
	if kind := ModelKindFromOutputModalities(outputModalities); kind != "" {
		return kind
	}
	return ModelKindFromName(name)
}

// ModelKindRealtime marks a catalog model served only on the realtime (voice session) endpoint.
const ModelKindRealtime = "realtime"

// endpointOnlyKinds maps the endpoint of a model that has no chat endpoint to its kind.
var endpointOnlyKinds = []struct{ endpoint, kind string }{
	{ModelEndpointImages, ModelKindImage},
	{ModelEndpointAudioSpeech, ModelKindSpeech},
	{ModelEndpointAudioTranscription, ModelKindTranscription},
	{ModelEndpointEmbeddings, ModelKindEmbedding},
	{ModelEndpointRerank, ModelKindRerank},
	{ModelEndpointDecisions, ModelKindDecision},
	{ModelEndpointRealtime, ModelKindRealtime},
}

// CatalogModelKind is the kind the model catalog shows for a row; "" is a chat (language) model.
// Endpoints an admin set without chat or responses name the kind; otherwise the output
// modalities and then the name decide, as for a channel's models.
func CatalogModelKind(name, endpointsJSON, outputModalitiesJSON string) string {
	endpoints, _ := utils.UnmarshalString[[]string](endpointsJSON)
	if len(endpoints) > 0 && !slices.Contains(endpoints, ModelEndpointChat) && !slices.Contains(endpoints, ModelEndpointResponses) {
		for _, rule := range endpointOnlyKinds {
			if slices.Contains(endpoints, rule.endpoint) {
				return rule.kind
			}
		}
	}
	output, _ := utils.UnmarshalString[[]string](outputModalitiesJSON)
	return ResolveModelKind("", output, name)
}
