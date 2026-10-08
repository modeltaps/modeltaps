package relay

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/azureSpeech"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/providers/gemini"
	"github.com/modeltaps/modeltaps/providers/minimax"
	"github.com/modeltaps/modeltaps/providers/openai"
	"github.com/modeltaps/modeltaps/types"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/gin-gonic/gin"
)

// ModelArchitecture 描述模型的输入 / 输出模态，来源于 model_info。
type ModelArchitecture struct {
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

// https://platform.openai.com/docs/api-reference/models/list
type OpenAIModels struct {
	Id           string             `json:"id"`
	Object       string             `json:"object"`
	Created      int                `json:"created"`
	OwnedBy      *string            `json:"owned_by"`
	Architecture *ModelArchitecture `json:"architecture,omitempty"`
	Capabilities []string           `json:"capabilities,omitempty"`
}

// filterModelsByTokenLimit 根据令牌的模型限制过滤模型列表
func filterModelsByTokenLimit(c *gin.Context, models []string) []string {
	// 检查是否启用了模型限制
	tokenSetting, exists := c.Get("token_setting")
	if !exists {
		return models
	}

	setting, ok := tokenSetting.(*model.TokenSetting)
	if !ok || setting == nil {
		return models
	}

	if !setting.Limits.LimitModelSetting.Enabled {
		return models
	}

	if len(setting.Limits.LimitModelSetting.Models) == 0 {
		return models
	}

	// 如果启用了模型限制，只返回限制的模型列表
	limitedModels := setting.Limits.LimitModelSetting.Models
	// 创建一个map用于快速查找；别名与主名同等生效，故白名单项额外登记其主名。
	allowedModelsMap := make(map[string]bool, len(limitedModels))
	for _, m := range limitedModels {
		allowedModelsMap[m] = true
		if canonical, isAlias := model.ResolveCanonicalModel(m); isAlias {
			allowedModelsMap[canonical] = true
		}
	}

	// 过滤模型列表，只保留允许的模型（白名单写主名时别名亦放行，反之亦然）
	filteredModels := make([]string, 0, len(models))
	for _, modelName := range models {
		if allowedModelsMap[modelName] {
			filteredModels = append(filteredModels, modelName)
			continue
		}
		if canonical, isAlias := model.ResolveCanonicalModel(modelName); isAlias && allowedModelsMap[canonical] {
			filteredModels = append(filteredModels, modelName)
		}
	}

	return filteredModels
}

// filterModelsByCatalog 按目录规则收敛公开列表：别名行与被隐藏的模型不出现。
// catalog.enforce_hidden 关闭时原样返回（应急回滚到「所有可路由模型」）。
func filterModelsByCatalog(models []string) []string {
	if !config.CatalogEnforceHidden() {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, modelName := range models {
		if isCatalogVisible(modelName) {
			filtered = append(filtered, modelName)
		}
	}
	return filtered
}

// isCatalogVisible 判断模型能否出现在公开列表；口径与检索、公开元信息列表同源，
// 实现见 model.IsCatalogVisible。
func isCatalogVisible(modelName string) bool {
	return model.IsCatalogVisible(modelName)
}

// filterModelsByChannelType 只保留在指定分组下存在对应协议渠道类型的模型。
// 用于让 Gemini/Claude 列表端点只返回该协议实际可路由的模型（其余模型经该协议请求会 model_not_found）；
// 无可路由模型时返回空列表。
func filterModelsByChannelType(groupName string, models []string, channelTypes []int) []string {
	filtered := make([]string, 0, len(models))
	for _, modelName := range models {
		if model.ChannelGroup.ModelSupportsChannelTypes(groupName, modelName, channelTypes) {
			filtered = append(filtered, modelName)
		}
	}
	return filtered
}

func ListModelsByToken(c *gin.Context) {
	groupName := c.GetString("token_group")
	if groupName == "" {
		groupName = c.GetString("group")
	}

	if groupName == "" {
		common.AbortWithMessage(c, http.StatusServiceUnavailable, "Group not found")
		return
	}

	models, err := model.ChannelGroup.GetGroupModels(groupName)
	if err != nil {
		c.JSON(200, gin.H{
			"object": "list",
			"data":   []string{},
		})
		return
	}
	sort.Strings(models)

	// 根据令牌的模型限制过滤模型列表
	models = filterModelsByTokenLimit(c, models)

	// 按目录规则收敛：去掉别名与被隐藏的模型
	models = filterModelsByCatalog(models)

	var groupOpenAIModels []*OpenAIModels
	for _, modelName := range models {
		groupOpenAIModels = append(groupOpenAIModels, getOpenAIModelWithName(modelName))
	}

	// 根据 OwnedBy 排序
	sort.Slice(groupOpenAIModels, func(i, j int) bool {
		if groupOpenAIModels[i].OwnedBy == nil {
			return true
		}
		if groupOpenAIModels[j].OwnedBy == nil {
			return false
		}
		return *groupOpenAIModels[i].OwnedBy < *groupOpenAIModels[j].OwnedBy
	})

	c.JSON(200, gin.H{
		"object": "list",
		"data":   groupOpenAIModels,
	})
}

// https://generativelanguage.googleapis.com/v1beta/models?key=xxxxxxx
func ListGeminiModelsByToken(c *gin.Context) {
	groupName := c.GetString("token_group")
	if groupName == "" {
		groupName = c.GetString("group")
	}

	if groupName == "" {
		common.AbortWithMessage(c, http.StatusServiceUnavailable, "Group not found")
		return
	}

	models, err := model.ChannelGroup.GetGroupModels(groupName)
	if err != nil {
		c.JSON(200, gemini.ModelListResponse{
			Models: []gemini.ModelDetails{},
		})
		return
	}
	sort.Strings(models)

	// 根据令牌的模型限制过滤模型列表
	models = filterModelsByTokenLimit(c, models)

	// 只返回 Gemini 协议实际可路由的模型（存在对应渠道类型），避免前端选中无渠道的模型渲染出实跑 404 的命令
	models = filterModelsByChannelType(groupName, models, AllowGeminiChannelType)

	geminiModels := make([]gemini.ModelDetails, 0, len(models))
	for _, modelName := range models {
		geminiModels = append(geminiModels, gemini.ModelDetails{
			Name:        fmt.Sprintf("models/%s", modelName),
			DisplayName: cases.Title(language.Und).String(strings.ReplaceAll(modelName, "-", " ")),
			SupportedGenerationMethods: []string{
				"generateContent",
			},
		})
	}

	c.JSON(200, gemini.ModelListResponse{
		Models: geminiModels,
	})
}

func ListClaudeModelsByToken(c *gin.Context) {
	groupName := c.GetString("token_group")
	if groupName == "" {
		groupName = c.GetString("group")
	}

	if groupName == "" {
		common.AbortWithMessage(c, http.StatusServiceUnavailable, "Group not found")
		return
	}

	models, err := model.ChannelGroup.GetGroupModels(groupName)
	if err != nil {
		c.JSON(200, claude.ModelListResponse{
			Data: []claude.Model{},
		})
		return
	}
	sort.Strings(models)

	// 根据令牌的模型限制过滤模型列表
	models = filterModelsByTokenLimit(c, models)

	// 只返回 Claude 协议实际可路由的模型（存在对应渠道类型），避免前端选中无渠道的模型渲染出实跑 404 的命令
	models = filterModelsByChannelType(groupName, models, AllowChannelType)

	claudeModelsData := make([]claude.Model, 0, len(models))
	for _, modelName := range models {
		claudeModelsData = append(claudeModelsData, claude.Model{
			ID:   modelName,
			Type: "model",
		})
	}

	c.JSON(200, claude.ModelListResponse{
		Data: claudeModelsData,
	})
}

func ListModelsForAdmin(c *gin.Context) {
	prices := model.PricingInstance.GetAllPrices()
	var openAIModels []OpenAIModels
	for modelId, price := range prices {
		openAIModels = append(openAIModels, OpenAIModels{
			Id:      modelId,
			Object:  "model",
			Created: getModelCreated(modelId),
			OwnedBy: getModelOwnedBy(modelId, price.ChannelType),
		})
	}
	// 根据 OwnedBy 排序
	sort.Slice(openAIModels, func(i, j int) bool {
		if openAIModels[i].OwnedBy == nil {
			return true // 假设 nil 值小于任何非 nil 值
		}
		if openAIModels[j].OwnedBy == nil {
			return false // 假设任何非 nil 值大于 nil 值
		}
		return *openAIModels[i].OwnedBy < *openAIModels[j].OwnedBy
	})

	c.JSON(200, gin.H{
		"object": "list",
		"data":   openAIModels,
	})
}

func RetrieveModel(c *gin.Context) {
	// 路由用 /*model 通配以支持含斜杠的别名，取值带前导 "/"，需去掉后才是模型名。
	requestedModel := strings.TrimPrefix(c.Param("model"), "/")
	// 别名可查但不暴露：返回的条目 id 为主名。
	modelName, _ := model.ResolveCanonicalModel(requestedModel)
	openaiModel := getOpenAIModelWithName(modelName)
	if isModelRetrievable(modelName, openaiModel) {
		c.JSON(http.StatusOK, openaiModel)
		return
	}
	openAIError := types.OpenAIError{
		Message: fmt.Sprintf("The model '%s' does not exist", requestedModel),
		Type:    "invalid_request_error",
		Param:   "model",
		Code:    "model_not_found",
	}
	// OpenAI 对 model_not_found 返回 404，错误体同格式；过去这里用 200 发错误体，客户端难以区分。
	c.JSON(http.StatusNotFound, gin.H{
		"error": openAIError,
	})
}

// isModelRetrievable 判定 /v1/models/{id} 能否返回条目：隐藏约束开启时口径与列表同源——
// 有启用渠道可路由且 isCatalogVisible，不再附加「厂商可解析」这一额外条件 —— 否则可见但推不出
// 厂商的模型会出现在列表里却检索不到。enforce 关闭时目录不再约束，退回旧判据，免得任意模型名都有条目。
func isModelRetrievable(modelName string, openaiModel *OpenAIModels) bool {
	if !config.CatalogEnforceHidden() {
		return openaiModel.OwnedBy != nil && *openaiModel.OwnedBy != model.UnknownOwnedBy
	}
	if _, routable := model.ChannelGroup.GetModelsGroups()[modelName]; !routable {
		return false
	}
	return isCatalogVisible(modelName)
}

// ModelVendor 是公开接口暴露的厂商信息：用户面只见厂商，不见渠道。
// Slug 是稳定的机器口径（openai / anthropic / google …）：厂商展示名可以随时改写
// （"Google Gemini"、"Google PaLM"），前端按厂商归类必须认 slug，不能认 name。
type ModelVendor struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	Icon string `json:"icon"`
}

// defaultModelCreated 是目录里没有创建时间时沿用的历史固定值，保持既有响应形态不变。
const defaultModelCreated = 1677649963

// getModelCreated 取模型目录行的创建时间（unix 秒）；别名随主名，无目录行回退固定值。
func getModelCreated(modelName string) int {
	if created := model.ModelCreatedAt(modelName); created > 0 {
		return int(created)
	}
	return defaultModelCreated
}

// getModelOwnedBy 解析模型的 owned_by：优先 vendor slug（稳定机器口径，厂商展示名可改写），
// slug 缺失时回退厂商展示名，厂商整体推不出时沿用 UnknownOwnedBy。回退链见 resolveModelVendor。
func getModelOwnedBy(modelName string, channelType int) (ownedBy *string) {
	vendor := resolveModelVendor(modelName, channelType)
	owner := vendor.Slug
	if owner == "" {
		owner = vendor.Name
	}
	return &owner
}

// resolveModelVendor 解析模型厂商，逐级回退：
// model_info.vendor_id → 别名指向主名的 vendor → 价格表 channel_type → 代表渠道 channel_type → 未知。
// 未知厂商返回 id 0、名称 UnknownOwnedBy 与默认图标。
func resolveModelVendor(modelName string, channelType int) *ModelVendor {
	candidates := []int{model.ResolveVendorID(modelName), channelType, model.ChannelGroup.GetModelChannelType(modelName)}
	for _, vendorID := range candidates {
		if name := ownedByName(vendorID); name != "" {
			return &ModelVendor{
				Id:   vendorID,
				Name: name,
				Slug: model.ModelOwnedBysInstance.GetSlug(vendorID),
				Icon: model.ModelOwnedBysInstance.GetIcon(vendorID),
			}
		}
	}

	return &ModelVendor{Name: model.UnknownOwnedBy, Icon: model.DefaultModelIcon}
}

// ownedByName 取厂商名；0 / 未登记的厂商返回空串，交给下一级回退。
func ownedByName(vendorID int) string {
	if vendorID == 0 {
		return ""
	}
	name := model.ModelOwnedBysInstance.GetName(vendorID)
	if name == model.UnknownOwnedBy {
		return ""
	}
	return name
}

func getOpenAIModelWithName(modelName string) *OpenAIModels {
	price := model.PricingInstance.GetPrice(modelName)

	openaiModel := &OpenAIModels{
		Id:      modelName,
		Object:  "model",
		Created: getModelCreated(modelName),
		OwnedBy: getModelOwnedBy(modelName, price.ChannelType),
	}

	// 有 model_info 才暴露 architecture；模态为空时用空切片，避免序列化成 null
	if price.ModelInfo != nil {
		openaiModel.Architecture = &ModelArchitecture{
			InputModalities:  price.ModelInfo.InputModalities,
			OutputModalities: price.ModelInfo.OutputModalities,
		}
		if openaiModel.Architecture.InputModalities == nil {
			openaiModel.Architecture.InputModalities = []string{}
		}
		if openaiModel.Architecture.OutputModalities == nil {
			openaiModel.Architecture.OutputModalities = []string{}
		}
	}

	// 能力仅在明确有值时暴露；"未知"（空串）与"明确无"（[]）都经 omitempty 省略该键
	if price.ModelInfo != nil && len(price.ModelInfo.Capabilities) > 0 {
		openaiModel.Capabilities = price.ModelInfo.Capabilities
	}

	return openaiModel
}

func GetModelOwnedBy(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    model.ModelOwnedBysInstance.GetAll(),
	})
}

type ModelPrice struct {
	Type   string  `json:"type"`
	Input  float64 `json:"input"`
	Output float64 `json:"output"`
}

// AvailableModelResponse 是 /api/available_model 的每项结构。
// owned_by 为厂商名（与 vendor.name 相同，保留给 ModelPrice 页兼容）；
// price.channel_type 保留仅供管理端排障，前端不得据此判断可用性——可用性以本接口是否返回该模型为准。
type AvailableModelResponse struct {
	Groups     []string         `json:"groups"`
	OwnedBy    string           `json:"owned_by"`
	Vendor     *ModelVendor     `json:"vendor"`
	Endpoints  []string         `json:"endpoints"`
	Aliases    []string         `json:"aliases"`
	Price      *model.Price     `json:"price"`
	TTSVoices  []types.TTSVoice `json:"tts_voices,omitempty"`
	TTSFormats []string         `json:"tts_formats,omitempty"`
}

// ttsFormatsAll 是 OpenAI 原生 /v1/audio/speech 接受的全部 response_format。
var ttsFormatsAll = []string{"mp3", "opus", "aac", "flac", "wav", "pcm"}

// OpenRouter 的 speech 接口只接受 mp3 / pcm，个别上游再收窄一档（2026-09 实测）。
var (
	ttsFormatsOpenRouter        = []string{"mp3", "pcm"}
	ttsFormatsOpenRouterMiniMax = []string{"mp3"}
)

// Gemini TTS（直连或经 OpenRouter）上游只出裸 PCM，网关补 WAV 头后只支持 wav / pcm；
// wav 在前，前端取首项作默认。
var ttsFormatsGemini = []string{"wav", "pcm"}

// modelTTSFormats 返回带 audio.speech 能力的模型可用输出格式：按代表渠道类型 + 模型名查内置表，
// 查不到时返回完整列表（不收窄，避免误伤）；非 TTS 模型返回 nil。
func modelTTSFormats(modelName string, endpoints []string) []string {
	if !utils.Contains(model.ModelEndpointAudioSpeech, endpoints) {
		return nil
	}
	switch model.ChannelGroup.GetModelChannelType(modelName) {
	case config.ChannelTypeGemini:
		return ttsFormatsGemini
	case config.ChannelTypeOpenRouter:
		name := strings.ToLower(modelName)
		switch {
		case strings.HasPrefix(name, "google/gemini"):
			return ttsFormatsGemini
		case strings.HasPrefix(name, "minimax/"):
			return ttsFormatsOpenRouterMiniMax
		}
		return ttsFormatsOpenRouter
	}
	return ttsFormatsAll
}

// modelTTSVoices 返回带 audio.speech 能力的模型可用音色：同步写入的音色优先（按 id 推断语言 / 性别），
// 否则按代表渠道类型取内置音色表，都没有时回落到六个经典 OpenAI 音色；非 TTS 模型返回 nil。
// MiniMax 模型（原生渠道或 OpenRouter 的 minimax/*）的同步列表只有英文音色，再并上内置 MiniMax 表。
func modelTTSVoices(modelName string, endpoints []string) []types.TTSVoice {
	if !utils.Contains(model.ModelEndpointAudioSpeech, endpoints) {
		return nil
	}
	channelType := model.ChannelGroup.GetModelChannelType(modelName)
	if ids := model.ModelSupportedVoices(modelName); len(ids) > 0 {
		voices := model.TTSVoicesFromIDs(ids)
		if isMiniMaxSpeechModel(modelName, channelType) {
			voices = mergeTTSVoices(voices, minimax.BuiltinSpeechVoices())
		}
		return voices
	}
	switch channelType {
	case config.ChannelTypeGemini:
		return gemini.BuiltinSpeechVoices()
	case config.ChannelTypeMiniMax:
		return minimax.BuiltinSpeechVoices()
	case config.ChannelTypeAzureSpeech:
		return azureSpeech.BuiltinSpeechVoices()
	case config.ChannelTypeOpenAI, config.ChannelTypeAzure:
		return openai.BuiltinSpeechVoices()
	}
	return openai.DefaultSpeechVoices()
}

func isMiniMaxSpeechModel(modelName string, channelType int) bool {
	if channelType == config.ChannelTypeMiniMax {
		return true
	}
	return channelType == config.ChannelTypeOpenRouter && strings.HasPrefix(strings.ToLower(modelName), "minimax/")
}

// mergeTTSVoices 按 id 去重合并两组音色，primary 的顺序与字段在前。
func mergeTTSVoices(primary, extra []types.TTSVoice) []types.TTSVoice {
	seen := make(map[string]bool, len(primary)+len(extra))
	merged := make([]types.TTSVoice, 0, len(primary)+len(extra))
	for _, list := range [][]types.TTSVoice{primary, extra} {
		for _, voice := range list {
			if seen[voice.ID] {
				continue
			}
			seen[voice.ID] = true
			merged = append(merged, voice)
		}
	}
	return merged
}

func AvailableModel(c *gin.Context) {
	groupName := c.GetString("group")

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    getAvailableModels(groupName),
	})
}

func GetAvailableModels(groupName string) map[string]*AvailableModelResponse {
	return getAvailableModels(groupName)
}

func getAvailableModels(groupName string) map[string]*AvailableModelResponse {
	publicModels := model.ChannelGroup.GetModelsGroups()
	publicGroups := model.GlobalUserGroupRatio.GetPublicGroupList()
	if groupName != "" && !utils.Contains(groupName, publicGroups) {
		publicGroups = append(publicGroups, groupName)
	}

	availableModels := make(map[string]*AvailableModelResponse, len(publicModels))

	for modelName, group := range publicModels {
		// 目录收敛：别名行与被隐藏的模型不进公开列表（enforce 关闭时不过滤）。
		if !isCatalogVisible(modelName) {
			continue
		}

		groups := []string{}
		for _, publicGroup := range publicGroups {
			if group[publicGroup] {
				groups = append(groups, publicGroup)
			}
		}

		if len(groups) == 0 {
			continue
		}

		if _, ok := availableModels[modelName]; !ok {
			price := model.PricingInstance.GetPrice(modelName)
			// 未配置的别名：顺着渠道 model_mapping 解析到映射后的真实模型价，
			// 避免别名显示「价格未配置」（计费本就按真实模型）。
			if price.Unconfigured {
				if mapped := model.ChannelGroup.ResolveMappedPrice(modelName); mapped != nil {
					price = mapped
				}
			}
			// 价格表推不出供应商（未配置价格）时，resolveModelVendor 会继续回退到实际服务
			// 该模型的渠道 Type，避免新建渠道后模型清一色显示「未知」。
			vendor := resolveModelVendor(modelName, price.ChannelType)
			endpoints := model.ModelEndpoints(modelName)
			availableModels[modelName] = &AvailableModelResponse{
				Groups:     groups,
				OwnedBy:    vendor.Name,
				Vendor:     vendor,
				Endpoints:  endpoints,
				Aliases:    model.ModelAliases(modelName),
				Price:      price,
				TTSVoices:  modelTTSVoices(modelName, endpoints),
				TTSFormats: modelTTSFormats(modelName, endpoints),
			}
		}
	}

	return availableModels
}
