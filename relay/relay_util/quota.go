// 计费生命周期域:Quota 结构、预扣(PreQuotaConsumption)、实时累计(UpdateUserRealtimeQuota)、
// 结算(Consume*/completedQuotaConsumption)与回退(Undo)。
// 其余域见同包:quota_org.go(org 守护/预算)、quota_calc.go(计价计算)、quota_log.go(日志/归因)。
//
// 计费链路 fail-open/fail-closed 决策点清单(后续打补丁的裁决索引):
//   - 入场组织守护 fail-closed(A1/SEC-10):checkOrgMemberGuardrails / reserveOrgMemberBudget /
//     reserveOrganizationBudget 加载或预留失败返回 503 拒绝,不放行(见 quota_org.go)。
//   - 未定价模型 block 策略 fail-closed:unpricedReject 为真时 PreQuotaConsumption 直接 403 拒绝。
//   - 实时配额 Redis 关闭 = 降级(fail-open):UpdateUserRealtimeQuota 在 !config.RedisEnabled 时
//     跳过实时累计,不拦截请求。成员预算实时兜底(SEC-12)同此降级,退化为 SEC-9 入场预留硬闸;
//     Redis 可用但操作出错时与 token 实时路径同语义:返回错误中断流。
//   - 结算侧尽力而为(fail-open):completedQuotaConsumption 扣费/缓存失败仅记错误日志,
//     消费日志始终落库(避免上游已计费但本地无记录),不向调用方返回失败。
package relay_util

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

type Quota struct {
	// —— 计价上下文(quota_calc.go 消费):模型/分组与各类倍率,NewQuota 时确定 ——
	modelName       string
	promptTokens    int
	price           model.Price
	groupName       string
	isBackupGroup   bool // 新增字段记录是否使用备用分组
	backupGroupName string
	groupRatio      float64
	inputRatio      float64
	outputRatio     float64
	costRatio       float64

	// —— 结算状态:预扣额与实时累计额,结算/回退时对账 ——
	preConsumedQuota int
	cacheQuota       int
	// 本请求已累计入成员预算 Redis tally 的量(SEC-12),结算时回收(镜像 cacheQuota 模式)
	memberBudgetCacheQuota int

	// —— 身份/令牌上下文 ——
	userId         int
	createdBy      int // 令牌实际创建者(组织令牌:userId 为影子账户,createdBy 为实际成员;0=个人令牌)
	channelId      int
	tokenId        int
	unlimitedQuota bool

	// —— 生命周期/策略标志 ——
	hasPeriodLimit bool // token 配置了 quota_reset，结算/回退需同步累计周期用量
	HandelStatus   bool
	unpricedReject bool // 模型价格未配置且 unpriced_model_policy=block，PreQuotaConsumption 拒绝

	// 组织令牌成员级守护(T5):PreQuotaConsumption 入口校验白名单/预算后填充,
	// 配置了预算时结算/回退需同步累计成员周期用量(语义同 hasPeriodLimit)
	orgGuardrail *model.OrgMemberGuardrail

	startTime         time.Time
	firstResponseTime time.Time
	extraBillingData  map[string]ExtraBillingData

	// Prompt Caching 自动注入结果(由 provider 写入 gin.Context,经 ConsumeSnapshot 回填)。
	// promptCachingInjected 为 true 时才写入消费日志 metadata(additive)。
	promptCachingInjected bool
	promptCachingTTL      string
	promptCachingStrategy string

	// App 归因(W9-B,additive):NewQuota 时从请求头捕获并 sanitize,GetLogMeta 时非空才写。
	// appName 取 X-Title,缺省回退 appDomain;appDomain 取 Referer 域名;userAgent 取 User-Agent。
	appName   string
	appDomain string
	userAgent string

	// 请求 ID(W9-K1,additive):NewQuota 时从 gin.Context 捕获(中间件已 c.Set,值同响应头
	// X-Modeltaps-Request-Id),作为值字段存于 Quota,异步扣费闭包持有其副本,不读 c,无 c-pool 竞争。
	// GetLogMeta 时非空才写 metadata.request_id。
	requestId string

	// 消费模态(additive):NewQuota 时由请求路径判定的稳定标签(chat_completions/image_generations/
	// audio_speech 等),GetLogMeta 时非空才写 metadata.relay_mode,供日志页展示与筛选。
	relayMode string
}

func NewQuota(c *gin.Context, modelName string, promptTokens int) *Quota {
	isBackupGroup := c.GetBool("is_backupGroup")

	quota := &Quota{
		modelName:      modelName,
		promptTokens:   promptTokens,
		userId:         c.GetInt("id"),
		createdBy:      c.GetInt("token_created_by"),
		channelId:      c.GetInt("channel_id"),
		tokenId:        c.GetInt("token_id"),
		unlimitedQuota: c.GetBool("token_unlimited_quota"),
		HandelStatus:   false,
		isBackupGroup:  isBackupGroup, // 记录是否使用备用分组
	}

	quota.price = *model.PricingInstance.GetPrice(quota.modelName)
	// 价格未配置时按策略处理(block 拒绝 / zero 不计费 / default 用默认倍率)，
	// 不再静默按旧的 30 倍率计费。须在下方倍率计算前应用。
	quota.unpricedReject = quota.price.ApplyUnpricedPolicy(config.UnpricedModelPolicy, config.UnpricedModelDefaultRatio)

	// 记录分组信息用于日志
	if isBackupGroup {
		// 发生了降级：记录原始分组 → 实际使用的分组
		quota.groupName = c.GetString("original_token_group") // 降级链的起点
		quota.backupGroupName = c.GetString("token_group")    // 实际使用的分组
	} else {
		// 没有降级：只记录使用的分组
		quota.groupName = c.GetString("token_group")
		quota.backupGroupName = ""
	}

	quota.groupRatio = c.GetFloat64("group_ratio") // 这里的倍率已经在 common.go 中正确设置了
	quota.inputRatio = quota.price.GetInput() * quota.groupRatio
	quota.outputRatio = quota.price.GetOutput() * quota.groupRatio

	if settingVal, ok := c.Get("token_setting"); ok {
		if tokenSetting, ok := settingVal.(*model.TokenSetting); ok && tokenSetting != nil {
			quota.hasPeriodLimit = tokenSetting.QuotaReset != nil
		}
	}

	// 成本倍率：仅用于成本/利润统计，不参与用户扣费。未配置或取不到渠道时为 0（不计成本）。
	quota.costRatio = 0
	if channel := model.ChannelGroup.GetChannel(quota.channelId); channel != nil {
		quota.costRatio = channel.GetCostRatio()
	}

	// App 归因(W9-B):c 在 handler 同步栈上活着,当场读取 App 请求头。只读 Referer/X-Title/User-Agent,
	// 绝不读取任何鉴权头。经 sanitize(去控制字符 + 长度上限)后暂存,GetLogMeta 时非空才写 metadata。
	quota.captureAppAttribution(c)

	// 请求 ID(W9-K1):c 仍在 handler 栈上时当场捕获中间件写入的请求 ID(值同响应头
	// X-Modeltaps-Request-Id),存为值字段,后续异步扣费不再读 c,GetLogMeta 时非空才写。
	quota.requestId = c.GetString(logger.RequestIdKey)

	// 消费模态:由请求路径判定(分支同 relay.Path2Relay),判不出时为空串不写 metadata。
	if c.Request != nil && c.Request.URL != nil {
		quota.relayMode = relayModeFromPath(c.Request.URL.Path)
	}

	return quota

}

func (q *Quota) PreQuotaConsumption() *types.OpenAIErrorWithStatusCode {
	// 成员级守护在任何预扣发生前执行,拒绝时零费用
	if errResp := q.checkOrgMemberGuardrails(); errResp != nil {
		return errResp
	}

	// 价格未配置 + block 策略:拒绝请求(零费用),避免按错误价格计费
	if q.unpricedReject {
		return common.ErrorWrapperLocal(
			fmt.Errorf("model %s has no configured price; pricing must be synced or set before use", q.modelName),
			"model_price_not_configured", http.StatusForbidden)
	}

	if q.price.Type == model.TimesPriceType {
		q.preConsumedQuota = common.QuotaFromFloat(1000 * q.inputRatio)
	} else if q.price.Input != 0 || q.price.Output != 0 {
		q.preConsumedQuota = common.QuotaFromFloat(float64(q.promptTokens)*q.inputRatio) + config.PreConsumedQuota
	}

	if q.preConsumedQuota == 0 {
		return nil
	}

	userQuota, err := model.CacheGetUserQuota(q.userId)
	if err != nil {
		return common.ErrorWrapper(err, "get_user_quota_failed", http.StatusInternalServerError)
	}

	// 预算受限的组织成员令牌不走 100x 跳过(SEC-9):必须逐请求对成员预算做原子预留(硬闸),
	// 且保持 preConsumedQuota 非零,使结算/回退的成员预算对账语义与既有一致。配置了团队级预算
	// 的组织令牌同理(T1)。个人令牌与未配置预算的组织令牌仍走跳过(orgGuardrail 为 nil 时
	// HasBudget()/HasOrgBudget() 安全返回 false),保留低延迟优化。
	if !q.orgGuardrail.HasBudget() && !q.orgGuardrail.HasOrgBudget() && userQuota > 100*q.preConsumedQuota {
		q.preConsumedQuota = 0
		return nil
	}

	if userQuota < q.preConsumedQuota {
		// 组织令牌:userId 为影子账户,额度不足意味着组织积分池耗尽,与成员预算超限错误明确区分
		if q.isOrgToken() {
			return common.ErrorWrapperLocal(errors.New("organization quota is not enough"), "insufficient_org_quota", http.StatusPaymentRequired)
		}
		return common.ErrorWrapperLocal(errors.New("user quota is not enough"), "insufficient_user_quota", http.StatusPaymentRequired)
	}

	if q.preConsumedQuota > 0 {
		// 先对成员预算原子预留(硬闸,合计不超周期上限):超限则零费用拒绝、不预扣额度池。
		// 预留即完成入场累计(替代旧 accrueOrgMemberBudget 入场累计),结算/回退仍按 preConsumedQuota 对账。
		if errResp := q.reserveOrgMemberBudget(q.preConsumedQuota); errResp != nil {
			return errResp
		}
		// 再对团队级预算原子预留(T1 硬闸):成员未超但团队已超同样零费用拒绝;
		// 团队预留失败须回冲刚才的成员预算预留,避免成员预算被无效占用
		if errResp := q.reserveOrganizationBudget(q.preConsumedQuota); errResp != nil {
			q.accrueOrgMemberBudget(-q.preConsumedQuota)
			return errResp
		}
		err := model.PreConsumeTokenQuota(q.tokenId, q.preConsumedQuota)
		if err != nil {
			// 池预扣失败:回冲刚才的成员/团队预算预留,避免预算被无效占用
			q.accrueOrgMemberBudget(-q.preConsumedQuota)
			q.accrueOrganizationBudget(-q.preConsumedQuota)
			return common.ErrorWrapperLocal(err, "pre_consume_token_quota_failed", http.StatusForbidden)
		}
		_ = model.CacheUpdateUserQuota(q.userId)
		q.HandelStatus = true
	}

	return nil
}

// 更新用户实时配额
func (q *Quota) UpdateUserRealtimeQuota(usage *types.UsageEvent, nowUsage *types.UsageEvent) error {
	usage.Merge(nowUsage)

	// 不开启Redis，则不更新实时配额
	if !config.RedisEnabled {
		return nil
	}

	promptTokens, completionTokens := q.getComputeTokensByUsageEvent(nowUsage)
	// 长上下文分档：实时路径逐增量结算，用累计输入 token（而非单次增量）判断档位，
	// 与最终结算按整次请求原始输入判档的效果保持收敛；对本次增量套用分档倍率。
	inRatio, outRatio := q.price.GetLongContextMultiplier(usage.InputTokens)
	increaseQuota := q.calcQuota(promptTokens, completionTokens, q.inputRatio*inRatio, q.outputRatio*outRatio, q.groupRatio)

	cacheQuota, err := model.CacheIncreaseUserRealtimeQuota(q.userId, increaseQuota)
	if err != nil {
		return errors.New("error update user realtime quota cache: " + err.Error())
	}

	q.cacheQuota += increaseQuota
	userQuota, err := model.CacheGetUserQuota(q.userId)
	if err != nil {
		return errors.New("error get user quota cache: " + err.Error())
	}

	if cacheQuota >= int64(userQuota) {
		return errors.New("user quota is not enough")
	}

	// 成员预算流式实时兜底(SEC-12):镜像上方 token 实时路径,把本次增量累计入成员预算 Redis tally,
	// 「入场 BudgetUsed + tally ≥ 周期上限」时返回错误中断流。
	if err := q.accrueOrgMemberRealtimeBudget(increaseQuota); err != nil {
		return err
	}

	return nil
}

func (q *Quota) completedQuotaConsumption(usage *types.Usage, tokenName string, isStream bool, sourceIp string, ctx context.Context, logIODetail *model.LogIODetail) error {
	defer func() {
		if q.cacheQuota > 0 {
			model.CacheDecreaseUserRealtimeQuota(q.userId, q.cacheQuota)
		}
		// SEC-12:同步回收成员预算实时 tally(镜像 cacheQuota 回收模式)
		q.releaseOrgMemberRealtimeBudget()
	}()

	quota := q.GetTotalQuotaByUsage(usage)
	costQuota := q.GetCostQuotaByUsage(usage)

	quotaDelta := quota - q.preConsumedQuota
	var quotaErr error
	if quotaDelta != 0 {
		err := model.PostConsumeTokenQuotaWithInfo(q.tokenId, q.userId, q.unlimitedQuota, quotaDelta)
		if err != nil {
			quotaErr = errors.New("error consuming token remain quota: " + err.Error())
			logger.LogError(ctx, quotaErr.Error())
		} else {
			if q.hasPeriodLimit {
				if perr := model.AccrueTokenPeriodUsed(q.tokenId, quotaDelta); perr != nil {
					logger.LogError(ctx, "error accrue token period used: "+perr.Error())
				}
			}
			q.accrueOrgMemberBudget(quotaDelta)
			q.accrueOrganizationBudget(quotaDelta)
			err = model.CacheUpdateUserQuota(q.userId)
			if err != nil {
				quotaErr = errors.New("error update user quota cache: " + err.Error())
				logger.LogError(ctx, quotaErr.Error())
			}
		}
	}
	if quota > 0 {
		model.UpdateChannelUsedQuota(q.channelId, quota)
	}

	// 无论配额操作是否成功，都要记录日志，避免上游已计费但本地无记录
	model.RecordConsumeLog(
		ctx,
		q.userId,
		q.channelId,
		usage.PromptTokens,
		usage.CompletionTokens,
		q.modelName,
		tokenName,
		quota,
		costQuota,
		"",
		q.getRequestTime(),
		isStream,
		q.GetLogMeta(usage),
		sourceIp,
		logIODetail,
	)
	model.UpdateUserUsedQuotaAndRequestCount(q.userId, quota)

	return quotaErr
}

func (q *Quota) Undo(c *gin.Context) {
	// SEC-12:回退路径同步回收成员预算实时 tally(与 completedQuotaConsumption 的 defer 对称),
	// 无累计时 noop;须在 HandelStatus 短路前执行——实时累计与预扣是否成功无关。
	q.releaseOrgMemberRealtimeBudget()
	if !q.HandelStatus {
		return
	}
	// Undo 所有调用方都在 gin handler 同步路径上，panic 由 gin.Recovery 兜底（带 stack）。
	// 不再加本地 recover：之前的"defense-in-depth"在已有 gin.Recovery 时是 anti-pattern：
	// 截胡 panic 让上层拿不到信号、日志失去堆栈、可调试性反而下降。
	ctx := c.Request.Context()
	if err := model.PostConsumeTokenQuotaWithInfo(q.tokenId, q.userId, q.unlimitedQuota, -q.preConsumedQuota); err != nil {
		logger.LogError(ctx, "error return pre-consumed quota: "+err.Error())
	} else {
		if q.hasPeriodLimit {
			// 回退预扣时同步回冲周期用量（预扣时已在 PreConsumeTokenQuota 中累计）
			if perr := model.AccrueTokenPeriodUsed(q.tokenId, -q.preConsumedQuota); perr != nil {
				logger.LogError(ctx, "error accrue token period used: "+perr.Error())
			}
		}
		// 回退预扣时同步回冲成员/团队预算用量（预扣时已在 PreQuotaConsumption 中累计）
		q.accrueOrgMemberBudget(-q.preConsumedQuota)
		q.accrueOrganizationBudget(-q.preConsumedQuota)
	}
	_ = model.CacheUpdateUserQuota(q.userId)
}

func (q *Quota) Consume(c *gin.Context, usage *types.Usage, isStream bool) {
	// 同步调用方（handler 主流程）走这里：c 在调用栈上活着，直接当场 snapshot 即可。
	// 异步调用方（TrackedGoroutine 路径）应直接调 ConsumeWithSnapshot，
	// 在 spawn goroutine 之前先 NewConsumeSnapshot(c)，闭包持有快照值而非 c 指针，
	// 彻底避免 handler return 后 c 被 gin pool 复用造成的数据竞争。
	snap := NewConsumeSnapshot(c)
	// T50d: c 仍活着时抓取请求/响应 body 明细(LogIOWriteEnabled 为假时返回 nil,零开销)。
	snap.LogIODetail = q.buildLogIODetail(c, usage, isStream)
	q.ConsumeWithSnapshot(snap, usage, isStream)
}

// ConsumeSnapshot 是 Quota.Consume 需要的 gin.Context 字段的不可变快照。
// 用于把 handler 派生 goroutine 上的 c 访问全部前置到 handler 还活着的时刻，
// 闭包只持有值，杜绝 c-pool 复用窗口。
type ConsumeSnapshot struct {
	TokenName string
	SourceIP  string
	StartTime time.Time
	Ctx       context.Context

	// Prompt Caching 自动注入结果，由 provider 在 handler 内写入 gin.Context。
	// 在此快照抓取，避免异步扣费 goroutine 持有 c 指针造成 c-pool 复用数据竞争。
	PromptCachingInjected bool
	PromptCachingTTL      string
	PromptCachingStrategy string

	// LogIODetail 为 LogIO opt-in 时抓取的请求/响应 body 明细(同步路径在 Consume 内填充)。
	// 异步路径(NewConsumeSnapshot 直调)保持 nil,不写 log_details。
	LogIODetail *model.LogIODetail
}

// NewConsumeSnapshot 立即从 c 抓取计费所需字段，构造不再持有 c 指针的快照。
// 必须在 handler 还在调用栈上（c 仍归本请求所有）时调用。
func NewConsumeSnapshot(c *gin.Context) ConsumeSnapshot {
	// 上游 request id 随快照带走，供异步消费日志落库。普通 HTTP 路径在 send 之后取快照能拿到。
	ctx := WithUpstreamRequestID(c.Request.Context(), c)
	return ConsumeSnapshot{
		TokenName: c.GetString("token_name"),
		SourceIP:  c.ClientIP(),
		StartTime: c.GetTime("requestStartTime"),
		Ctx:       ctx,

		PromptCachingInjected: c.GetBool("prompt_caching_injected"),
		PromptCachingTTL:      c.GetString("prompt_caching_ttl"),
		PromptCachingStrategy: c.GetString("prompt_caching_strategy"),
	}
}

// ConsumeWithSnapshot 用预先抓取好的快照执行同步扣费 + 写消费日志。
// 同 Consume：DB/Cache 调用刻意不传 ctx，防客户端断连取消计费。
//
// recover 保留的理由：本函数被 realtime / task 的 TrackedGoroutine 异步路径调用，
// 虽然外层 TrackedGoroutine 自带 stack-aware recover 兜底，但本地 recover 能给 panic
// 加上 "Quota.ConsumeWithSnapshot" 的上下文标签 + 完整堆栈，定位 quota 步骤更直接。
// 同步路径（Consume wrapper）下本地 recover 会截胡 gin.Recovery，但权衡后保留这层
// 是因为异步路径 panic 没有 gin.Recovery 接，需要 quota 这层就抓住堆栈。
//
// 延迟成本（同步化的已知代价）：
//   - BatchUpdateEnabled=true（推荐生产配置）：扣费/日志/统计全部走 in-memory append，
//     同步路径上唯一真 IO 是 CacheUpdateUserQuota 一次 Redis 调用，~几 ms。
//   - BatchUpdateEnabled=false：5 个调用里 4 个走真 DB UPDATE/INSERT，TTLB 增加 ~10-20ms。
//
// 这是反压换一致性的有意取舍：异步会让 handler 早返 200 但扣费 goroutine 在 DB 池满时堆积，
// 正是"上游已计费、本地未记账"的真凶。
func (q *Quota) ConsumeWithSnapshot(snap ConsumeSnapshot, usage *types.Usage, isStream bool) {
	q.startTime = snap.StartTime
	q.promptCachingInjected = snap.PromptCachingInjected
	q.promptCachingTTL = snap.PromptCachingTTL
	q.promptCachingStrategy = snap.PromptCachingStrategy
	ctx := snap.Ctx
	defer func() {
		if r := recover(); r != nil {
			logger.LogError(ctx, fmt.Sprintf("panic in Quota.ConsumeWithSnapshot: %v, stack: %s", r, string(debug.Stack())))
		}
	}()
	if err := q.completedQuotaConsumption(usage, snap.TokenName, isStream, snap.SourceIP, ctx, snap.LogIODetail); err != nil {
		logger.LogError(ctx, err.Error())
	}
}

func (q *Quota) GetFirstResponseTime() int64 {
	// 先判断 firstResponseTime 是否为0
	if q.firstResponseTime.IsZero() {
		return 0
	}

	return q.firstResponseTime.Sub(q.startTime).Milliseconds()
}

func (q *Quota) SetFirstResponseTime(firstResponseTime time.Time) {
	q.firstResponseTime = firstResponseTime
}
