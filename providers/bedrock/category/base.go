package category

import (
	"github.com/modeltaps/modeltaps/common/model_utils"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"
	"errors"
	"net/http"
	"strings"
)

// 基础模型映射关系
var bedrockMap = map[string]string{
	//base model id
	"claude-3-7-sonnet-20250219": "anthropic.claude-3-7-sonnet-20250219-v1:0",
	"claude-3-5-sonnet-20240620": "anthropic.claude-3-5-sonnet-20240620-v1:0",
	"claude-3-5-sonnet-20241022": "anthropic.claude-3-5-sonnet-20241022-v2:0",
	"claude-3-opus-20240229":     "anthropic.claude-3-opus-20240229-v1:0",
	"claude-3-sonnet-20240229":   "anthropic.claude-3-sonnet-20240229-v1:0",
	"claude-3-haiku-20240307":    "anthropic.claude-3-haiku-20240307-v1:0",
	"claude-3-5-haiku-20241022":  "anthropic.claude-3-5-haiku-20241022-v1:0",
	"claude-2.1":                 "anthropic.claude-v2:1",
	"claude-2.0":                 "anthropic.claude-v2",
	"claude-instant-1.2":         "anthropic.claude-instant-v1",
	"claude-sonnet-4-20250514":   "anthropic.claude-sonnet-4-20250514-v1:0",
	"claude-opus-4-20250514":     "anthropic.claude-opus-4-20250514-v1:0",
	"claude-opus-4-1-20250805":   "anthropic.claude-opus-4-1-20250805-v1:0",
	"claude-sonnet-4-5-20250929": "anthropic.claude-sonnet-4-5-20250929-v1:0",
	"claude-sonnet-4-6":          "anthropic.claude-sonnet-4-6",
	"claude-haiku-4-5-20251001":  "anthropic.claude-haiku-4-5-20251001-v1:0",
	"claude-opus-4-5-20251101":   "anthropic.claude-opus-4-5-20251101-v1:0",
	"claude-opus-4-6":            "anthropic.claude-opus-4-6-v1",
	"claude-opus-4-7":            "anthropic.claude-opus-4-7",
	"claude-opus-4-8":            "anthropic.claude-opus-4-8",
	"claude-sonnet-5":            "anthropic.claude-sonnet-5",
	"claude-sonnet-5-5":          "anthropic.claude-sonnet-5-5",
	"claude-fable-5":             "anthropic.claude-fable-5",
	"claude-fable-5-1":           "anthropic.claude-fable-5-1",
	"claude-opus-5":              "anthropic.claude-opus-5",
	"claude-opus-5-5":            "anthropic.claude-opus-5-5",
}

// 用户显式书写的区域前缀（手动覆盖优先）
// au. and jp. are the AP geo profiles of the 4.5+ generation; the region table below cannot
// express them, so they are only reachable through an explicit prefix.
var regionPrefixes = []string{"global.", "us.", "eu.", "apac.", "au.", "jp."}

// 各 bedrock 模型支持的跨区 inference profile：region 根（aws region 第一段，如
// us-east-1 -> us）映射到该模型在此 region 实际可用的 profile 前缀。未列出的模型/region
// 按裸 id 直连，不加前缀。
//
// AP has two generations: claude-3.x, sonnet-4, opus-4-1 and other older models have apac.
// profiles; the 4.5+ generation has no apac. and instead offers the narrower au.
// (ap-southeast-2/4/6) and jp. (ap-northeast-1/3), with only global. elsewhere in AP. This table
// is keyed by region root (ap) and cannot tell au./jp. apart, so AP maps to global. (available in
// every AP region, no regional surcharge); deployments that need au./jp. data residency write the
// prefix on the model name. Exception: opus-4-5 has no au./jp. at all, only us./eu./global.
//
// "*" is the wildcard fallback for any region root. Without it, roots such as ca/me/sa/il/af/mx
// find no mapping and send the bare model id, which AWS rejects with a 400 for models that do not
// allow on-demand calls by bare id (haiku-4-5, for example). Every model whose global. profile is
// available in all regions should therefore carry "*".
//
// "ca": AWS counts ca-central-1 / ca-west-1 as source regions of the US geo (the us. profile's
// destinations include Canada), so models with a us. geo profile there map "ca" to "us" to keep
// data residency instead of falling back to "*" -> global. Not every model supports geo in ca:
// sonnet-4-5, haiku-4-5 and opus-4-5 do not in ca-west-1, so they have no "ca" key and use "*".
var awsModelCanCrossRegionMap = map[string]map[string]string{
	"anthropic.claude-3-sonnet-20240229-v1:0": {"us": "us", "eu": "eu", "ap": "apac"},
	"anthropic.claude-3-opus-20240229-v1:0":   {"us": "us"},
	// 3-haiku: geo inference ids are us./eu. only (no apac., per the model card). AP regions
	// support it in-region, so there is no "ap" key and no "*"; it is called by bare id there.
	"anthropic.claude-3-haiku-20240307-v1:0":    {"us": "us", "eu": "eu"},
	"anthropic.claude-3-5-sonnet-20240620-v1:0": {"us": "us", "eu": "eu", "ap": "apac"},
	"anthropic.claude-3-5-sonnet-20241022-v2:0": {"us": "us", "ap": "apac"},
	"anthropic.claude-3-5-haiku-20241022-v1:0":  {"us": "us"},
	"anthropic.claude-3-7-sonnet-20250219-v1:0": {"us": "us", "eu": "eu", "ap": "apac"},
	"anthropic.claude-sonnet-4-20250514-v1:0":   {"us": "us", "eu": "eu", "ap": "apac"},
	"anthropic.claude-opus-4-20250514-v1:0":     {"us": "us"},
	"anthropic.claude-opus-4-1-20250805-v1:0":   {"us": "us"},
	// 4.5 generation onwards: global. is available in every commercial region, so all of them
	// carry the "*" fallback.
	"anthropic.claude-sonnet-4-5-20250929-v1:0": {"us": "us", "eu": "eu", "ap": "global", "*": "global"},
	"anthropic.claude-sonnet-4-6":               {"us": "us", "eu": "eu", "ap": "global", "*": "global"},
	"anthropic.claude-haiku-4-5-20251001-v1:0":  {"us": "us", "eu": "eu", "ap": "global", "*": "global"},
	"anthropic.claude-opus-4-5-20251101-v1:0":   {"us": "us", "eu": "eu", "ap": "global", "*": "global"},
	"anthropic.claude-opus-4-6-v1":              {"us": "us", "ca": "us", "eu": "eu", "ap": "global", "*": "global"},
	"anthropic.claude-opus-4-7":                 {"us": "us", "ca": "us", "eu": "eu", "ap": "global", "*": "global"},
	"anthropic.claude-opus-4-8":                 {"us": "us", "ca": "us", "eu": "eu", "ap": "global", "*": "global"},
	"anthropic.claude-opus-5":                   {"us": "us", "eu": "eu", "ap": "global", "*": "global"},
	// opus-5-5 has the widest geo coverage (us/eu/au/jp); AP still maps to global. as above.
	"anthropic.claude-opus-5-5": {"us": "us", "eu": "eu", "ap": "global", "*": "global"},
	// sonnet-5: no geo profile in the EU, global only (AWS model card 2026-06-30).
	"anthropic.claude-sonnet-5": {"us": "us", "eu": "global", "ap": "global", "*": "global"},
	// sonnet-5-5: no geo profile anywhere (geo inference id N/A), always global.
	"anthropic.claude-sonnet-5-5": {"*": "global"},
	"anthropic.claude-fable-5":    {"us": "us", "eu": "global", "ap": "global", "*": "global"},
	// fable-5-1: geo profile in the US only; EU and AP fall back to global.
	"anthropic.claude-fable-5-1": {"us": "us", "eu": "global", "ap": "global", "*": "global"},
}

var CategoryMap = map[string]Category{}

type Category struct {
	ModelName                 string
	ChatComplete              ChatCompletionConvert
	ResponseChatComplete      ChatCompletionResponse
	ResponseChatCompleteStrem ChatCompletionStreamResponse
}

func GetCategory(modelName, region string) (*Category, error) {
	modelName = GetModelName(modelName, region)
	// 获取provider
	provider := ""

	if model_utils.ContainsCaseInsensitive(modelName, "anthropic") {
		provider = "anthropic"
	}

	if category, exists := CategoryMap[provider]; exists {
		category.ModelName = modelName
		return &category, nil
	}

	return nil, errors.New("category_not_found")
}

func GetModelName(modelName, region string) string {
	// 提取用户显式书写的区域前缀
	regionPrefix := ""
	for _, prefix := range regionPrefixes {
		if strings.HasPrefix(modelName, prefix) {
			regionPrefix = prefix
			modelName = modelName[len(prefix):]
			break
		}
	}

	// 查找基础模型映射
	if mappedName, exists := bedrockMap[modelName]; exists {
		modelName = mappedName
	}

	// 用户未显式写前缀时，按 region 自动推断跨区前缀
	if regionPrefix == "" {
		regionPrefix = autoCrossRegionPrefix(modelName, region)
	}

	// 如果有区域前缀，添加回去
	if regionPrefix != "" {
		modelName = regionPrefix + modelName
	}

	return modelName
}

// 按 region 与模型跨区可用性返回需拼接的前缀（如 "us."），不可跨区时返回空串
func autoCrossRegionPrefix(awsModelID, region string) string {
	regionRoot := region
	if i := strings.Index(region, "-"); i > 0 {
		regionRoot = region[:i]
	}

	profileMap, ok := awsModelCanCrossRegionMap[awsModelID]
	if !ok {
		return ""
	}

	profilePrefix, ok := profileMap[regionRoot]
	if !ok {
		// "*" applies to any region root without its own entry.
		profilePrefix, ok = profileMap["*"]
		if !ok {
			return ""
		}
	}

	return profilePrefix + "."
}

type ChatCompletionConvert func(*types.ChatCompletionRequest) (any, *types.OpenAIErrorWithStatusCode)
type ChatCompletionResponse func(base.ProviderInterface, *http.Response, *types.ChatCompletionRequest) (*types.ChatCompletionResponse, *types.OpenAIErrorWithStatusCode)

type ChatCompletionStreamResponse func(base.ProviderInterface, *types.ChatCompletionRequest) requester.HandlerPrefix[string]
