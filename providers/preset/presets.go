// Package preset serves OpenAI compatible platforms that need nothing of their own: each is a
// channel type with a fixed API root, its models.dev provider id for the channel's price list and
// for the model list when the platform's own /models fails, and the endpoints it serves.
//
// Checked against each platform's docs on 2026-11-15: roots, cache reporting (all but Fireworks,
// NVIDIA, ModelScope, Chutes and Hugging Face put cache hits in prompt_tokens_details.cached_tokens
// or, for Together and StepFun's international site, at the top level of usage) and balance
// endpoints (none documented apart from Novita, so no preset reads a balance yet).
package preset

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

// Preset is one platform.
type Preset struct {
	Type int
	Name string
	// Root is the API root the OpenAI paths are appended to ("https://api.together.xyz/v1").
	Root string
	// ModelsDev is the platform's provider id on models.dev.
	ModelsDev string
	// Embeddings and Responses say whether requests go to the platform's /embeddings and
	// /responses. Only documented and checked endpoints are on: a Responses request to a platform
	// without the flag is converted to Chat Completions.
	Embeddings bool
	Responses  bool
	// NoStreamOptions is set for a platform that rejects stream_options in a streamed request;
	// the others are asked to include usage in the last chunk.
	NoStreamOptions bool
}

// Presets lists the platforms, by channel type.
var Presets = map[int]Preset{}

func init() {
	for _, p := range []Preset{
		{Type: config.ChannelTypeTogether, Name: "Together AI", Root: "https://api.together.ai/v1", ModelsDev: "togetherai", Embeddings: true},
		{Type: config.ChannelTypeFireworks, Name: "Fireworks AI", Root: "https://api.fireworks.ai/inference/v1", ModelsDev: "fireworks-ai", Embeddings: true},
		{Type: config.ChannelTypeDeepInfra, Name: "DeepInfra", Root: "https://api.deepinfra.com/v1/openai", ModelsDev: "deepinfra", Embeddings: true},
		{Type: config.ChannelTypeCerebras, Name: "Cerebras", Root: "https://api.cerebras.ai/v1", ModelsDev: "cerebras"},
		{Type: config.ChannelTypeNebius, Name: "Nebius Token Factory", Root: "https://api.tokenfactory.nebius.com/v1", ModelsDev: "nebius", Embeddings: true},
		{Type: config.ChannelTypeNovita, Name: "Novita AI", Root: "https://api.novita.ai/openai/v1", ModelsDev: "novita-ai"},
		{Type: config.ChannelTypeSambaNova, Name: "SambaNova", Root: "https://api.sambanova.ai/v1", ModelsDev: "sambanova"},
		{Type: config.ChannelTypeNvidiaNIM, Name: "NVIDIA NIM", Root: "https://integrate.api.nvidia.com/v1", ModelsDev: "nvidia", Embeddings: true},
		{Type: config.ChannelTypeModelScope, Name: "ModelScope", Root: "https://api-inference.modelscope.cn/v1", ModelsDev: "modelscope"},
		{Type: config.ChannelTypeQiniu, Name: "Qiniu", Root: "https://api.qnaigc.com/v1", ModelsDev: "qiniu-ai"},
		{Type: config.ChannelTypeHuggingFace, Name: "Hugging Face", Root: "https://router.huggingface.co/v1", ModelsDev: "huggingface"},
		{Type: config.ChannelTypeChutes, Name: "Chutes", Root: "https://llm.chutes.ai/v1", ModelsDev: "chutes"},
		{Type: config.ChannelTypeBaseten, Name: "Baseten", Root: "https://inference.baseten.co/v1", ModelsDev: "baseten"},
		{Type: config.ChannelTypeStepFun, Name: "StepFun", Root: "https://api.stepfun.com/v1", ModelsDev: "stepfun"},
		{Type: config.ChannelTypeXiaomiMiMo, Name: "Xiaomi MiMo", Root: "https://api.xiaomimimo.com/v1", ModelsDev: "xiaomi"},
		{Type: config.ChannelTypeLongCat, Name: "LongCat", Root: "https://api.longcat.chat/openai/v1", ModelsDev: "longcat"},
	} {
		Presets[p.Type] = p
		model.SetChannelTypeLibraryProviders(p.Type, p.ModelsDev)
	}
}
