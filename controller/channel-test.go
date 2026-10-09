package controller

import (
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/notify"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers"
	providers_base "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	embeddingsRegex = regexp.MustCompile(`(?:^text-|embed|Embed|rerank|davinci|babbage|bge-|e5-|LLM2Vec|retrieval|uae-|gte-|jina-clip|jina-embeddings)`)
	imageRegex      = regexp.MustCompile(`flux|diffusion|stabilityai|sd-|dall|cogview|janus|image`)
	responseRegex   = regexp.MustCompile(`(?:^o[1-9])`)
	noSupportRegex  = regexp.MustCompile(`(?:^tts|rerank|whisper|speech|^mj_|^chirp)`)
)

// cheapExactModels 常见廉价测速模型（精确名，顺序即优先级）。命中 channel.Models 内同名者优先选用。
var cheapExactModels = []string{
	"gpt-4o-mini",
	"gpt-4.1-mini",
	"gpt-4.1-nano",
	"gpt-3.5-turbo",
	"deepseek-chat",
	"qwen-turbo",
	"glm-4-flash",
}

// cheapModelPatterns 常见廉价测速模型（模式匹配，顺序即优先级），在精确名未命中时按序尝试。
var cheapModelPatterns = []*regexp.Regexp{
	regexp.MustCompile(`claude.*haiku`),
	regexp.MustCompile(`gemini.*flash`),
}

// parseChannelModels 将逗号分隔的 channel.Models 拆为去空白、去空项的有序切片。
func parseChannelModels(models string) []string {
	parts := strings.Split(models, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			result = append(result, p)
		}
	}
	return result
}

// matchCheapModel 在渠道模型集合中按「精确名优先、模式匹配次之」选取常见廉价测速模型，未命中返回空串。
func matchCheapModel(models []string) string {
	set := make(map[string]bool, len(models))
	for _, m := range models {
		set[m] = true
	}
	for _, name := range cheapExactModels {
		if set[name] {
			return name
		}
	}
	for _, pattern := range cheapModelPatterns {
		for _, m := range models {
			if pattern.MatchString(strings.ToLower(m)) {
				return m
			}
		}
	}
	return ""
}

// baseModelName 去掉模型名的 # 后缀，返回用于类型判定与响应展示的实际模型名（如 gpt-5#low -> gpt-5）。
func baseModelName(modelName string) string {
	if idx := strings.Index(modelName, "#"); idx >= 0 {
		return modelName[:idx]
	}
	return modelName
}

// isChatTestable 判定渠道模型（去 # 后缀后）是否为可测速的 chat 类型，排除 response/embeddings/image/noSupport。
func isChatTestable(modelName string) bool {
	return getModelType(baseModelName(modelName)) == "chat"
}

// selectTestModel 按优先级为渠道选取测速模型：
// 1) 显式指定的 model；2) channel.TestModel；3) 常见廉价模型清单 ∩ channel.Models（仅 chat）；
// 4) channel.Models 中第一个 getModelType()=="chat" 的模型；5) 皆无则返回明确中文错误。
// 自动兜底候选（步骤 3、4）严格限定 chat 类型，不会选中 response(o1/o3 等)、embeddings、image、noSupport。
func selectTestModel(channel *model.Channel, testModel string) (string, error) {
	if testModel != "" {
		return testModel, nil
	}
	if channel.TestModel != "" {
		return channel.TestModel, nil
	}
	models := parseChannelModels(channel.Models)
	if len(models) == 0 {
		return "", errors.New("channel has no testable model configured")
	}
	chatModels := make([]string, 0, len(models))
	for _, m := range models {
		if isChatTestable(m) {
			chatModels = append(chatModels, m)
		}
	}
	if m := matchCheapModel(chatModels); m != "" {
		return m, nil
	}
	if len(chatModels) > 0 {
		return chatModels[0], nil
	}
	return "", errors.New("channel has no testable model configured")
}

func testChannel(channel *model.Channel, testModel string) (openaiErr *types.OpenAIErrorWithStatusCode, err error) {
	testModel, err = selectTestModel(channel, testModel)
	if err != nil {
		return nil, err
	}

	// 解析模型名称中的 # 后缀（用于 o1/o3-mini/gpt-5 的 reasoning_effort 等参数）
	// 例如：gpt-5#low -> gpt-5 (otherArg: low)
	var otherArg string
	parts := strings.Split(testModel, "#")
	if len(parts) > 1 {
		otherArg = parts[1]
		testModel = parts[0]
	}

	channelType := getModelType(testModel)
	channel.SetProxy()

	var url string
	switch channelType {
	case "embeddings":
		url = "/v1/embeddings"
	case "image":
		url = "/v1/images/generations"
	case "chat":
		url = "/v1/chat/completions"
	case "response":
		url = "/v1/responses"
	default:
		return nil, errors.New("unsupported model type")
	}

	// 创建测试上下文
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	// 获取并验证provider
	provider := providers.GetProvider(channel, c)
	if provider == nil {
		return nil, errors.New("channel not implemented")
	}

	// 设置 otherArg（用于 reasoning_effort 等参数）
	if otherArg != "" {
		provider.SetOtherArg(otherArg)
	}

	newModelName, err := provider.ModelMappingHandler(testModel)
	if err != nil {
		return nil, err
	}

	newModelName = strings.TrimPrefix(newModelName, "+")

	usage := &types.Usage{}
	provider.SetUsage(usage)

	// 执行测试请求
	var response any
	var openAIErrorWithStatusCode *types.OpenAIErrorWithStatusCode

	switch channelType {
	case "embeddings":
		embeddingsProvider, ok := provider.(providers_base.EmbeddingsInterface)
		if !ok {
			return nil, errors.New("channel not implemented")
		}
		testRequest := &types.EmbeddingRequest{
			Model: newModelName,
			Input: "hi",
		}
		response, openAIErrorWithStatusCode = embeddingsProvider.CreateEmbeddings(testRequest)
	case "image":
		imageProvider, ok := provider.(providers_base.ImageGenerationsInterface)
		if !ok {
			return nil, errors.New("channel not implemented")
		}

		testRequest := &types.ImageRequest{
			Model:  newModelName,
			Prompt: "A cute cat",
			N:      1,
		}
		response, openAIErrorWithStatusCode = imageProvider.CreateImageGenerations(testRequest)
	case "response":
		responseProvider, ok := provider.(providers_base.ResponsesInterface)
		if !ok {
			return nil, errors.New("channel not implemented")
		}

		testRequest := &types.OpenAIResponsesRequest{
			Input:  "You just need to output 'hi' next.",
			Model:  newModelName,
			Stream: false,
		}

		response, openAIErrorWithStatusCode = responseProvider.CreateResponses(testRequest)
	case "chat":
		// 仅实现 Claude 原生 Messages 接口的渠道（如 Bedrock Messages）不满足 ChatInterface，
		// 回退到 Claude 接口测速。
		if chatProvider, ok := provider.(providers_base.ChatInterface); ok {
			testRequest := &types.ChatCompletionRequest{
				Messages: []types.ChatCompletionMessage{
					{
						Role:    "user",
						Content: "You just need to output 'hi' next.",
					},
				},
				Model:  newModelName,
				Stream: false,
			}
			response, openAIErrorWithStatusCode = chatProvider.CreateChatCompletion(testRequest)
		} else if claudeProvider, ok := provider.(claude.ClaudeChatInterface); ok {
			testRequest := &claude.ClaudeRequest{
				Model:     newModelName,
				MaxTokens: 512,
				Messages: []claude.Message{
					{
						Role:    "user",
						Content: "You just need to output 'hi' next.",
					},
				},
			}
			response, openAIErrorWithStatusCode = claudeProvider.CreateClaudeChat(testRequest)
		} else {
			return nil, errors.New("channel not implemented")
		}
	default:
		return nil, errors.New("unsupported model type")
	}

	if openAIErrorWithStatusCode != nil {
		// 透传原始 err; UI/通知文案的脱敏由 caller 统一在调用 utils.MaskSensitiveInfo 时完成。
		return openAIErrorWithStatusCode, errors.New(openAIErrorWithStatusCode.Message)
	}

	// 转换为JSON字符串
	jsonBytes, _ := json.Marshal(response)
	logger.SysLog(fmt.Sprintf("test channel %s : %s response: %s", channel.Name, newModelName, string(jsonBytes)))

	return nil, nil
}

func getModelType(modelName string) string {
	if noSupportRegex.MatchString(modelName) {
		return "noSupport"
	}

	if embeddingsRegex.MatchString(modelName) {
		return "embeddings"
	}

	// 对话出图模型（Gemini 原生生图等）只支持 generateContent，必须走 chat 测速；
	// 否则会被下面 imageRegex 的 image 关键词吞进 image(predict) 分支 → 404 → 自动禁用。
	// 放在 imageRegex 之前；imagen-* 判为 image，仍走 image(predict)。
	switch model.ResolveModelMode(modelName) {
	case model.ModelModeChatImage:
		return "chat"
	case model.ModelModeImage:
		return "image"
	}

	// 库里显式标了 chat 的模型不再被 imageRegex 的关键词误判为生图。
	if model.ModelModeFromInfo(modelName) != model.ModelModeChat && imageRegex.MatchString(modelName) {
		return "image"
	}

	if responseRegex.MatchString(modelName) {
		return "response"
	}

	return "chat"
}

func TestChannel(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	channel, err := model.GetChannelById(id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	testModel, err := selectTestModel(channel, c.Query("model"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	tik := time.Now()
	openaiErr, err := testChannel(channel, testModel)
	tok := time.Now()
	milliseconds := tok.Sub(tik).Milliseconds()
	consumedTime := float64(milliseconds) / 1000.0

	success := false
	msg := ""
	if openaiErr != nil {
		if ShouldDisableChannel(channel.Type, openaiErr) {
			msg = fmt.Sprintf("speed test failed, channel disabled, reason: %s", utils.MaskSensitiveInfo(err.Error()))
			DisableChannel(channel.Id, channel.Name, testModel, err.Error(), false)
		} else {
			msg = fmt.Sprintf("speed test failed, reason: %s", utils.MaskSensitiveInfo(err.Error()))
		}
	} else if err != nil {
		msg = fmt.Sprintf("speed test failed, reason: %s", utils.MaskSensitiveInfo(err.Error()))
	} else {
		success = true
		msg = "speed test succeeded"
		go channel.UpdateResponseTime(milliseconds)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": success,
		"message": msg,
		"time":    consumedTime,
		"model":   baseModelName(testModel),
	})
}

var testAllChannelsLock sync.Mutex
var testAllChannelsRunning = false
var testAllChannelsStartTime time.Time
var testAllChannelsCount int
var testAllChannelsGen uint64

const testChannelTimeout = 60 * time.Second

// 注意：底层 testChannel 不接受 context，超时后内部 goroutine 仍会等待
// HTTP 请求自身超时返回。testAllChannels 串行调用本函数，最多一个滞留 goroutine。
func testChannelWithTimeout(channel *model.Channel, testModel string, timeout time.Duration) (*types.OpenAIErrorWithStatusCode, error) {
	type result struct {
		openaiErr *types.OpenAIErrorWithStatusCode
		err       error
	}
	ch := make(chan result, 1)
	go func() {
		openaiErr, err := testChannel(channel, testModel)
		ch <- result{openaiErr, err}
	}()
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case res := <-ch:
		return res.openaiErr, res.err
	case <-timer.C:
		return nil, fmt.Errorf("channel test timed out (over %s)", timeout)
	}
}

func testAllChannels(isNotify bool) error {
	channels, err := model.GetAllChannels()
	if err != nil {
		return err
	}

	testAllChannelsLock.Lock()
	if testAllChannelsRunning {
		// 动态计算超时：渠道数 × (单次超时 + RequestInterval) + 120s 缓冲
		maxDuration := time.Duration(testAllChannelsCount)*(testChannelTimeout+config.RequestInterval) + 120*time.Second
		if time.Since(testAllChannelsStartTime) < maxDuration {
			testAllChannelsLock.Unlock()
			return errors.New("a test is already running")
		}
		// 强制重置：bump generation，让旧 goroutine 的 defer 不再清理状态，
		// 避免新旧两轮并发时旧 goroutine 把 running 错误地置为 false。
		testAllChannelsGen++
		logger.SysError("testing all channels timed out, forcing reset")
	}
	testAllChannelsRunning = true
	testAllChannelsStartTime = time.Now()
	testAllChannelsCount = len(channels)
	myGen := testAllChannelsGen
	testAllChannelsLock.Unlock()

	var disableThreshold = int64(config.ChannelDisableThreshold * 1000)
	if disableThreshold == 0 {
		disableThreshold = 10000000 // a impossible value
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logger.SysError(fmt.Sprintf("testAllChannels panic: %v\n%s", r, debug.Stack()))
			}
			testAllChannelsLock.Lock()
			if testAllChannelsGen == myGen {
				testAllChannelsRunning = false
			}
			testAllChannelsLock.Unlock()
			debug.FreeOSMemory()
		}()

		var sb strings.Builder
		for _, channel := range channels {
			// 定时任务跳过手动禁用的渠道：它不会被自动恢复，测试只会消耗配额，无实际意义。
			// 手动触发（isNotify=true）仍测试全部渠道。
			if !isNotify && channel.Status == config.ChannelStatusManuallyDisabled {
				continue
			}

			time.Sleep(config.RequestInterval)

			isChannelEnabled := channel.Status == config.ChannelStatusEnabled
			sb.WriteString(fmt.Sprintf("**Channel %s - #%d - %s** : \n\n", utils.EscapeMarkdownText(channel.Name), channel.Id, channel.StatusToStr()))
			tik := time.Now()
			openaiErr, err := testChannelWithTimeout(channel, "", testChannelTimeout)
			tok := time.Now()
			milliseconds := tok.Sub(tik).Milliseconds()
			// 通道为禁用状态，并且还是请求错误 或者 响应时间超过阈值 直接跳过，也不需要更新响应时间。
			if !isChannelEnabled {
				if err != nil {
					sb.WriteString(fmt.Sprintf("- Test error: %s \n\n- No status change needed, skipped\n\n", utils.EscapeMarkdownText(utils.MaskSensitiveInfo(err.Error()))))
					continue
				}
				if milliseconds > disableThreshold {
					sb.WriteString(fmt.Sprintf("- Response time %.2fs exceeds threshold %.2fs \n\n- No status change needed, skipped\n\n", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0))
					continue
				}
				// 如果已被禁用，但是请求成功，需要判断是否需要恢复
				// 手动禁用的通道，不会自动恢复
				if shouldEnableChannel(err, openaiErr) {
					if channel.Status == config.ChannelStatusAutoDisabled {
						EnableChannel(channel.Id, channel.Name, false)
						sb.WriteString("- Enabled \n\n")
					} else {
						sb.WriteString("- Manually disabled channel, will not be re-enabled automatically \n\n")
					}
				}
			} else {
				// 如果通道启用状态，但是返回了错误 或者 响应时间超过阈值，需要判断是否需要禁用
				if milliseconds > disableThreshold {
					errMsg := fmt.Sprintf("Response time %.2fs exceeds threshold %.2fs ", float64(milliseconds)/1000.0, float64(disableThreshold)/1000.0)
					sb.WriteString(fmt.Sprintf("- %s \n\n- Disabled\n\n", errMsg))
					DisableChannel(channel.Id, channel.Name, channel.TestModel, errMsg, false)
					continue
				}

				if ShouldDisableChannel(channel.Type, openaiErr) {
					sb.WriteString(fmt.Sprintf("- Disabled, reason: %s\n\n", utils.EscapeMarkdownText(utils.MaskSensitiveInfo(err.Error()))))
					DisableChannel(channel.Id, channel.Name, channel.TestModel, err.Error(), false)
					continue
				}

				if err != nil {
					sb.WriteString(fmt.Sprintf("- Test error: %s \n\n", utils.EscapeMarkdownText(utils.MaskSensitiveInfo(err.Error()))))
					continue
				}
			}
			channel.UpdateResponseTime(milliseconds)
			sb.WriteString(fmt.Sprintf("- Test completed in %.2fs\n\n", float64(milliseconds)/1000.0))
		}
		if isNotify {
			notify.Send("Channel test completed", sb.String())
		}
	}()
	return nil
}

func TestAllChannels(c *gin.Context) {
	err := testAllChannels(true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
	})
}

func AutomaticallyTestChannels(frequency int) {
	if frequency <= 0 {
		return
	}

	for {
		time.Sleep(time.Duration(frequency) * time.Minute)
		logger.SysLog("testing all channels")
		_ = testAllChannels(false)
		logger.SysLog("channel test finished")
	}
}
