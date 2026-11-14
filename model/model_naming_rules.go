package model

import (
	"regexp"

	"github.com/modeltaps/modeltaps/common/config"
)

// ModifierKind says what a ":modifier" on an upstream model name means ("claude-fable-5.2:free").
type ModifierKind int

const (
	// ModifierUnknown: not declared anywhere; the name is offered for confirmation.
	ModifierUnknown ModifierKind = iota
	// ModifierSame: the same model, offered and selected by default.
	ModifierSame
	// ModifierSameOptional: the same model under different terms (a free tier with its own rate
	// limits and data policy); offered but not selected by default.
	ModifierSameOptional
	// ModifierVariant: a different model or behaviour (web search, extended thinking); not a match.
	ModifierVariant
)

// NamingRules is what a provider declares about how it spells model names, so upstream names
// can be matched to public names without models.dev knowing the spelling. Everything is
// optional; genericNamingRules applies to every channel on top of the provider's own rules.
type NamingRules struct {
	// Prefixes are stripped from the start of a name, longest first ("global.", "anthropic.").
	// The part up to the last "/" is always stripped.
	Prefixes []string
	// Suffixes are stripped from the end of a name ("@default", "-v1:0").
	Suffixes []*regexp.Regexp
	// Modifiers classify the part after ":".
	Modifiers map[string]ModifierKind
}

var genericNamingRules = NamingRules{
	// Region and namespace prefixes in the Bedrock style; harmless on names that do not use them.
	Prefixes: []string{
		"global.", "us.", "eu.", "apac.", "jp.", "au.", "ca.", "us-gov.",
		"anthropic.", "meta.", "amazon.", "mistral.", "cohere.", "ai21.", "deepseek.", "qwen.", "openai.", "writer.",
	},
	Suffixes: []*regexp.Regexp{
		regexp.MustCompile(`@[a-z0-9._-]+$`), // Vertex: "@default", "@20250219"
		// Bedrock: "-v1:0". The colon is required: "deepseek-v3" is a version, not a suffix.
		regexp.MustCompile(`-v\d+:\d+$`),
	},
	Modifiers: map[string]ModifierKind{
		"free":     ModifierSameOptional,
		"beta":     ModifierSame,
		"thinking": ModifierVariant,
		"online":   ModifierVariant,
		"extended": ModifierVariant,
		"exacto":   ModifierVariant,
	},
}

// providerNamingRules holds each provider's own declarations, by channel type.
var providerNamingRules = map[int]NamingRules{
	config.ChannelTypeOpenRouter: {
		// Routing shortcuts over the same model: ":nitro" (fastest hosts), ":floor" (cheapest).
		Modifiers: map[string]ModifierKind{"nitro": ModifierSameOptional, "floor": ModifierSameOptional},
	},
}

// namingRulesFor merges the generic rules with the channel type's own; the provider wins on a
// modifier both declare.
func namingRulesFor(channelType int) NamingRules {
	own, ok := providerNamingRules[channelType]
	if !ok {
		return genericNamingRules
	}
	merged := NamingRules{
		Prefixes:  append(append([]string{}, own.Prefixes...), genericNamingRules.Prefixes...),
		Suffixes:  append(append([]*regexp.Regexp{}, own.Suffixes...), genericNamingRules.Suffixes...),
		Modifiers: map[string]ModifierKind{},
	}
	for k, v := range genericNamingRules.Modifiers {
		merged.Modifiers[k] = v
	}
	for k, v := range own.Modifiers {
		merged.Modifiers[k] = v
	}
	return merged
}

// variantWords are name parts that make a different model when added to a name: "gpt-5-mini" is
// not "gpt-5", "claude-fable-5-2-thinking" is not "claude-fable-5-2".
var variantWords = map[string]bool{
	"mini": true, "nano": true, "micro": true, "small": true, "medium": true, "large": true,
	"pro": true, "max": true, "plus": true, "ultra": true, "lite": true, "air": true,
	"flash": true, "turbo": true, "fast": true, "instant": true, "high": true, "low": true,
	"thinking": true, "reasoning": true, "reasoner": true, "think": true, "nothink": true,
	"instruct": true, "chat": true, "base": true, "vision": true, "vl": true, "omni": true,
	"audio": true, "realtime": true, "search": true, "online": true, "codex": true, "coder": true,
	"code": true, "research": true, "deep": true, "distill": true, "image": true, "tts": true,
	"embedding": true, "preview": true, "exp": true, "experimental": true, "latest": true,
	"fp8": true, "fp16": true, "bf16": true, "int4": true, "int8": true, "awq": true, "gptq": true,
	"gguf": true, "mlx": true, "free": true,
}

var sizeWordPattern = regexp.MustCompile(`^(a?\d+(\.\d+)?[bmkt]|\d+x\d+[bmt])$`)

func isVariantWord(w string) bool {
	return variantWords[w] || sizeWordPattern.MatchString(w)
}
