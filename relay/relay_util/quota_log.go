package relay_util

// 日志/归因域:消费日志 metadata 组装、App 归因捕获与 sanitize、LogIO 明细抓取、
// finish_reason 归一、上游 request id 透传。由 quota.go 的 NewQuota / Consume /
// completedQuotaConsumption 及各 RecordConsumeLog 调用方使用。

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"unicode"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// App 归因(W9-B):从中继请求头捕获调用方 App 信息写入消费日志 metadata。
// appMetaMaxLen 约束 app_name/app_domain 长度上限,userAgentMaxLen 约束 user_agent 长度上限。
const (
	appMetaMaxLen   = 256
	userAgentMaxLen = 256
)

// captureAppAttribution 从请求头提取调用方 App 归因:
//   - appDomain: HTTP-Referer(缺省回退 Referer)的域名
//   - appName:   X-Title;缺省回退 appDomain
//   - userAgent: User-Agent
//
// 全部经 sanitizeHeaderValue 去控制字符并截断,不涉及任何鉴权头。
func (q *Quota) captureAppAttribution(c *gin.Context) {
	referer := c.GetHeader("HTTP-Referer")
	if referer == "" {
		referer = c.GetHeader("Referer")
	}
	q.appDomain = extractRefererDomain(referer)

	if title := sanitizeHeaderValue(c.GetHeader("X-Title"), appMetaMaxLen); title != "" {
		q.appName = title
	} else {
		q.appName = q.appDomain
	}

	q.userAgent = sanitizeHeaderValue(c.GetHeader("User-Agent"), userAgentMaxLen)
}

// sanitizeHeaderValue 去除控制字符、首尾空白,并按 rune 截断到 maxLen(<=0 表示不截断)。
func sanitizeHeaderValue(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
	}
	cleaned := strings.TrimSpace(b.String())
	if maxLen > 0 {
		if runes := []rune(cleaned); len(runes) > maxLen {
			cleaned = string(runes[:maxLen])
		}
	}
	return cleaned
}

// extractRefererDomain 从 Referer 字面量解析出主机名(不含端口);无法解析出主机名时返回空串。
// 兼容带 scheme(https://demo.app/x)与裸域名(demo.app/x)两种形态。
func extractRefererDomain(referer string) string {
	referer = strings.TrimSpace(referer)
	if referer == "" {
		return ""
	}
	if u, err := url.Parse(referer); err == nil && u.Hostname() != "" {
		return sanitizeHeaderValue(u.Hostname(), appMetaMaxLen)
	}
	// 无 scheme 的裸域名/带路径:补 "//" 再解析,让 url 把首段当 host。
	if u, err := url.Parse("//" + referer); err == nil && u.Hostname() != "" {
		return sanitizeHeaderValue(u.Hostname(), appMetaMaxLen)
	}
	return ""
}

// relayModeLabels 把 config.RelayMode 枚举映射为写入消费日志 metadata.relay_mode 的稳定字符串标签。
// 用字符串而非裸数字入库:枚举值是 iota,后续插入常量会让历史日志语义漂移。未收录的枚举值不写该键。
var relayModeLabels = map[int]string{
	config.RelayModeChatCompletions:    "chat_completions",
	config.RelayModeCompletions:        "completions",
	config.RelayModeEmbeddings:         "embeddings",
	config.RelayModeModerations:        "moderations",
	config.RelayModeImagesGenerations:  "image_generations",
	config.RelayModeImagesEdits:        "image_edits",
	config.RelayModeImagesVariations:   "image_variations",
	config.RelayModeAudioSpeech:        "audio_speech",
	config.RelayModeAudioTranscription: "audio_transcription",
	config.RelayModeAudioTranslation:   "audio_translation",
	config.RelayModeRerank:             "rerank",
	config.RelayModeChatRealtime:       "realtime",
	config.RelayModeResponses:          "responses",
}

// pathToRelayMode 由请求路径判定 config.RelayMode,分支与 relay.Path2Relay 一一对应,
// 另补 Path2Relay 不经手但同样计费的 /v1/rerank、/v1/realtime、/recraftAI 图像端点。
// 无法判定时返回 RelayModeUnknown(不写 metadata,前端显示 "-")。
func pathToRelayMode(path string) int {
	switch {
	case strings.HasPrefix(path, "/v1/chat/completions"), strings.HasPrefix(path, "/claude/v1/messages"):
		return config.RelayModeChatCompletions
	case strings.HasPrefix(path, "/v1/completions"):
		return config.RelayModeCompletions
	case strings.HasPrefix(path, "/v1/embeddings"):
		return config.RelayModeEmbeddings
	case strings.HasPrefix(path, "/v1/moderations"):
		return config.RelayModeModerations
	case strings.HasPrefix(path, "/v1/images/generations"), strings.HasPrefix(path, "/recraftAI/v1/images"):
		return config.RelayModeImagesGenerations
	case strings.HasPrefix(path, "/v1/images/edits"):
		return config.RelayModeImagesEdits
	case strings.HasPrefix(path, "/v1/images/variations"):
		return config.RelayModeImagesVariations
	case strings.HasPrefix(path, "/v1/audio/speech"):
		return config.RelayModeAudioSpeech
	case strings.HasPrefix(path, "/v1/audio/transcriptions"):
		return config.RelayModeAudioTranscription
	case strings.HasPrefix(path, "/v1/audio/translations"):
		return config.RelayModeAudioTranslation
	case strings.HasPrefix(path, "/v1/rerank"):
		return config.RelayModeRerank
	case strings.HasPrefix(path, "/v1/realtime"):
		return config.RelayModeChatRealtime
	case strings.HasPrefix(path, "/v1/responses"):
		return config.RelayModeResponses
	default:
		return config.RelayModeUnknown
	}
}

// relayModeFromPath 返回写入 metadata.relay_mode 的标签;判定不出时返回空串(不写该键)。
// /gemini 前缀按 Path2Relay 的同款细分:veo 长任务=视频,:predict=图像,其余=对话;
// 视频无对应 RelayMode 枚举值,单独用 "video" 标签。
func relayModeFromPath(path string) string {
	if strings.HasPrefix(path, "/gemini") {
		switch {
		case strings.Contains(path, "veo") && strings.Contains(path, ":predictLongRunning"):
			return "video"
		case strings.Contains(path, ":predict"):
			return relayModeLabels[config.RelayModeImagesGenerations]
		default:
			return relayModeLabels[config.RelayModeChatCompletions]
		}
	}
	return relayModeLabels[pathToRelayMode(path)]
}

// buildLogIODetail 在写入闸门(站点级 LogIOEnabled && 令牌级 token.log_io)为真时,
// 从 gin.Context 抓取请求体、从响应缓存/累计文本抓取响应体,截断到 64KB 后构造明细载体。
// 闸门为假时直接返回 nil,不读 body、不分配,保证默认零开销。仅留存 body,绝不含鉴权头/密钥。
// 非文本类请求体(multipart/form-data 音频/图片上传等)只写占位说明,不入库二进制字节。
func (q *Quota) buildLogIODetail(c *gin.Context, usage *types.Usage, isStream bool) *model.LogIODetail {
	if !config.LogIOEnabled || !c.GetBool("token_log_io") {
		return nil
	}

	detail := &model.LogIODetail{
		TokenId:   q.tokenId,
		CreatedBy: q.createdBy,
	}

	if reqBytes := cachedRequestBytes(c); len(reqBytes) > 0 {
		if contentType := c.ContentType(); isTextualLogIOContentType(contentType) {
			detail.RequestBody, detail.RequestTruncated = model.TruncateLogIOBody(reqBytes)
		} else {
			// 主动省略而非截断:RequestTruncated 保持 false。
			detail.RequestBody = omittedRequestBodyPlaceholder(contentType, len(reqBytes))
		}
	}

	var respBytes []byte
	if isStream {
		if usage != nil && usage.TextBuilder.Len() > 0 {
			respBytes = []byte(usage.TextBuilder.String())
		}
	} else if v, ok := c.Get(config.GinResponseBodyKey); ok {
		if b, ok := v.([]byte); ok {
			respBytes = b
		}
	}
	if len(respBytes) > 0 {
		detail.ResponseBody, detail.ResponseTruncated = model.TruncateLogIOBody(respBytes)
	}

	return detail
}

// isTextualLogIOContentType 判定请求体是否可原样留存到 log_details:仅放行 JSON/文本类
// Content-Type。multipart/form-data(STT/图片编辑的音频、图片上传)、application/octet-stream
// 等二进制载荷一律不留存原始字节:TruncateLogIOBody 只修剪末尾不完整 UTF-8,中段非法序列会被
// MySQL utf8mb4 列拒写(明细静默丢失)或在 SQLite 存成乱码。
// Content-Type 缺省按 JSON 放行,与 common.UnmarshalBodyReusable 同口径(缺省 CT 走
// json.Unmarshal,能走到计费链路的此类请求已是合法 JSON 文本)。
func isTextualLogIOContentType(contentType string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		return true
	}
	if strings.HasPrefix(ct, "text/") || strings.HasSuffix(ct, "+json") || strings.HasSuffix(ct, "+xml") {
		return true
	}
	switch {
	case strings.HasPrefix(ct, "application/json"),
		strings.HasPrefix(ct, "application/x-ndjson"),
		strings.HasPrefix(ct, "application/xml"),
		strings.HasPrefix(ct, "application/x-www-form-urlencoded"):
		return true
	}
	return false
}

// omittedRequestBodyPlaceholder 为被省略的非文本请求体生成占位说明,保留 Content-Type
// 与原始字节数供排查;Content-Type 经 sanitizeHeaderValue 去控制字符并截断。
func omittedRequestBodyPlaceholder(contentType string, size int) string {
	ct := sanitizeHeaderValue(contentType, appMetaMaxLen)
	if ct == "" {
		ct = "unknown"
	}
	return fmt.Sprintf("[request body omitted: content-type=%s, %d bytes]", ct, size)
}

// cachedRequestBytes 返回已发送至上游的请求体字节:优先已处理字节,回退原始缓存字节。
func cachedRequestBytes(c *gin.Context) []byte {
	for _, key := range []string{config.GinProcessedBytesKey, config.GinRequestBodyKey} {
		if v, ok := c.Get(key); ok {
			if b, ok := v.([]byte); ok && len(b) > 0 {
				return b
			}
		}
	}
	return nil
}

// WithUpstreamRequestID 把 provider 在响应阶段暂存到 gin.Context 的上游 request id
// （如 bedrock x-amzn-requestid）注入 ctx。model.RecordConsumeLog 约定从 ctx 读该值，
// 因此每个 RecordConsumeLog 调用方都必须经本函数派生 ctx，否则 upstream_request_id
// 列会静默留空。未暂存时原样返回 ctx（realtime/WS 的快照先于上游响应，拿不到值，
// 日志该列留空即可）。
func WithUpstreamRequestID(ctx context.Context, c *gin.Context) context.Context {
	if upstreamRequestID := c.GetString(config.GinUpstreamRequestIdKey); upstreamRequestID != "" {
		return context.WithValue(ctx, config.GinUpstreamRequestIdKey, upstreamRequestID)
	}
	return ctx
}

func (q *Quota) GetLogMeta(usage *types.Usage) map[string]any {
	meta := map[string]any{
		"group_name":        q.groupName,
		"backup_group_name": q.backupGroupName,
		"is_backup_group":   q.isBackupGroup, // 添加是否使用备用分组的标识
		"price_type":        q.price.Type,
		"group_ratio":       q.groupRatio,
		"input_ratio":       q.price.GetInput(),
		"output_ratio":      q.price.GetOutput(),
	}

	// 组织令牌:记账主体为影子账户,metadata 记录实际成员 ID 用于按成员聚合(规格 §5.2 散点③)
	if q.createdBy > 0 && q.createdBy != q.userId {
		meta["org_member_id"] = q.createdBy
	}

	// App 归因(W9-B,additive):三者由 NewQuota 从请求头捕获并 sanitize,非空才写,皆缺省时不写字段。
	if q.appName != "" {
		meta["app_name"] = q.appName
	}
	if q.appDomain != "" {
		meta["app_domain"] = q.appDomain
	}
	if q.userAgent != "" {
		meta["user_agent"] = q.userAgent
	}

	// 请求 ID(W9-K1,additive):由 NewQuota 从 gin.Context 捕获,与响应头 X-Modeltaps-Request-Id 一致,
	// 非空才写;取不到时不写该键(不写空串),旧日志无该键不受影响。
	if q.requestId != "" {
		meta["request_id"] = q.requestId
	}

	// 消费模态(additive):NewQuota 时由请求路径判定,非空才写;判不出/旧日志无该键,前端显示 "-"。
	if q.relayMode != "" {
		meta["relay_mode"] = q.relayMode
	}

	firstResponseTime := q.GetFirstResponseTime()
	if firstResponseTime > 0 {
		meta["first_response"] = firstResponseTime
	}

	if usage != nil {
		extraTokens := usage.GetExtraTokens()

		for key, value := range extraTokens {
			meta[key] = value
			extraRatio := q.price.GetExtraRatio(key)
			meta[key+"_ratio"] = extraRatio
		}

		// 长上下文分档命中时记录分档倍率，供日志详情展示。
		if inRatio, outRatio := q.price.GetLongContextMultiplier(usage.PromptTokens); inRatio != 1 || outRatio != 1 {
			meta["long_context_input_ratio"] = inRatio
			meta["long_context_output_ratio"] = outRatio
		}

		// finish_reason 归一(W8-B,additive):原文另存 native_finish_reason,归一值写 finish_reason。
		// 无法取得时(空串)不写,不写空串。
		if usage.FinishReason != "" {
			meta["native_finish_reason"] = usage.FinishReason
			meta["finish_reason"] = normalizeFinishReason(usage.FinishReason)
		}
	}

	if q.extraBillingData != nil {
		meta["extra_billing"] = q.extraBillingData
	}

	// Prompt Caching 注入标识(仅注入成功时写入,additive)。
	if q.promptCachingInjected {
		meta["prompt_caching_injected"] = true
		if q.promptCachingTTL != "" {
			meta["prompt_caching_ttl"] = q.promptCachingTTL
		}
		if q.promptCachingStrategy != "" {
			meta["prompt_caching_strategy"] = q.promptCachingStrategy
		}
	}

	return meta
}

// normalizeFinishReason 把上游各家 finish_reason 原文归一到标准集:
// stop | length | content_filter | tool_calls | error。大小写/首尾空白不敏感。
// 覆盖 OpenAI 兼容 provider 常见值,并容纳 Anthropic/Gemini 等常见别名(这些 provider 多在
// 各自适配层已转 OpenAI 值,别名仅作兜底)。未知非空值保守归一为 error(原文另存
// native_finish_reason,信息不丢失)。空串由调用方拦截,不会进入本函数写日志。
func normalizeFinishReason(native string) string {
	switch strings.ToLower(strings.TrimSpace(native)) {
	case "stop", "end_turn", "stop_sequence", "eos", "complete", "completed", "finished":
		return "stop"
	case "length", "max_tokens", "max_output_tokens", "model_length", "token_limit":
		return "length"
	case "content_filter", "safety", "recitation", "blocklist", "blocked", "prohibited_content", "spii", "image_safety":
		return "content_filter"
	case "tool_calls", "tool_use", "function_call", "tool_call":
		return "tool_calls"
	case "error", "failed", "malformed_function_call", "unexpected_tool_call":
		return "error"
	default:
		return "error"
	}
}
