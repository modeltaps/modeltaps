package model

import (
	"regexp"
	"strings"
)

// vendorNameRules guess the vendor from a bare model name when nothing better is known (no
// canonical id, no "vendor/" prefix): names platforms list without saying who made the model,
// such as Azure's "grok-4-20-reasoning" or a relay's "claude-fable-5-2". Patterns match the
// lower-case name after dropping any "vendor/" and region ("us.", "global.") prefixes.
var vendorNameRules = []struct {
	pattern *regexp.Regexp
	slug    string
}{
	{regexp.MustCompile(`^claude-`), "anthropic"},
	{regexp.MustCompile(`^(gpt-|chatgpt-|codex-|o\d(-|$)|dall-e|whisper|tts-|text-embedding-|sora-|omni-moderation)`), "openai"},
	{regexp.MustCompile(`^(gemini-|gemma-|imagen-|veo-|lyria-)`), "google"},
	{regexp.MustCompile(`^grok-`), "x-ai"},
	{regexp.MustCompile(`^deepseek-`), "deepseek"},
	{regexp.MustCompile(`^(qwen|qwq-|qvq-)`), "qwen"},
	{regexp.MustCompile(`^(kimi-|moonshot-)`), "moonshotai"},
	{regexp.MustCompile(`^(glm-|chatglm)`), "z-ai"},
	{regexp.MustCompile(`^(mistral-|ministral-|codestral-|pixtral-|magistral-|devstral-|voxtral-)`), "mistralai"},
	{regexp.MustCompile(`^(llama-|llama\d)`), "meta-llama"},
	{regexp.MustCompile(`^(command-|cohere-|embed-(english|multilingual)|rerank-)`), "cohere"},
	{regexp.MustCompile(`^(minimax-|abab)`), "minimax"},
	{regexp.MustCompile(`^hunyuan-`), "tencent"},
	{regexp.MustCompile(`^ernie-`), "baidu"},
	{regexp.MustCompile(`^yi-`), "01-ai"},
	{regexp.MustCompile(`^flux[-.]`), "black-forest-labs"},
	{regexp.MustCompile(`^(nemotron|nv-embed)`), "nvidia"},
	{regexp.MustCompile(`^(nova-|titan-)`), "amazon"},
	{regexp.MustCompile(`^phi-\d`), "microsoft"},
	{regexp.MustCompile(`^(doubao|seed-|seedream|seedance)`), "bytedance"},
	{regexp.MustCompile(`^mimo-`), "xiaomi"},
	{regexp.MustCompile(`^step-\d`), "stepfun"},
	{regexp.MustCompile(`^longcat-`), "meituan"},
	{regexp.MustCompile(`^sonar`), "perplexity"},
	{regexp.MustCompile(`^(eleven_|eleven-)`), "elevenlabs"},
	{regexp.MustCompile(`^voyage-`), "voyageai"},
	{regexp.MustCompile(`^(lfm-|lfm\d)`), "liquid"},
	{regexp.MustCompile(`^mercury`), "inception"},
	{regexp.MustCompile(`^(stable-diffusion|sdxl|sd3)`), "stability-ai"},
	{regexp.MustCompile(`^jina-`), "jina"},
	{regexp.MustCompile(`^recraft-`), "recraft"},
	{regexp.MustCompile(`^kling-`), "kling"},
	{regexp.MustCompile(`^ideogram`), "ideogram"},
}

var regionPrefix = regexp.MustCompile(`^(us|eu|ap|apac|global|jp|au|ca)\.`)

// VendorIDFromNameRules returns the vendor the name rules point at, 0 when none matches or the
// vendor has no row.
func VendorIDFromNameRules(modelName string) int {
	name := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(modelName, "~")))
	name = bareModelID(name)
	name = regionPrefix.ReplaceAllString(name, "")
	name = strings.TrimPrefix(name, "anthropic.")
	for _, rule := range vendorNameRules {
		if rule.pattern.MatchString(name) {
			return ModelOwnedBysInstance.GetIdBySlug(rule.slug)
		}
	}
	return 0
}
