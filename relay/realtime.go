package relay

import (
	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/metrics"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/relay/relay_util"
	"github.com/modeltaps/modeltaps/types"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

type RelayModeChatRealtime struct {
	relayBase
	userConn       *websocket.Conn
	messageHandler requester.MessageHandler
	providerConn   *websocket.Conn
	quota          *relay_util.Quota
	usage          *types.UsageEvent
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
	Subprotocols: []string{"realtime"},
}

// SEC-9/SEC-10/SEC-12 测试注入点:realtime 入场守护,单测可替换以模拟守护拒绝/额度不足
var preQuotaConsumptionFn = func(q *relay_util.Quota) *types.OpenAIErrorWithStatusCode {
	return q.PreQuotaConsumption()
}

func ChatRealtime(c *gin.Context) {
	modelName := c.Query("model")
	if modelName == "" {
		common.AbortWithMessage(c, http.StatusBadRequest, "model_name_required")
		return
	}

	userConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Println("upgrade failed", err)
		common.AbortWithMessage(c, http.StatusInternalServerError, "upgrade_failed")
		return
	}

	relay := &RelayModeChatRealtime{
		relayBase: relayBase{
			c: c,
		},
		userConn: userConn,
	}
	relay.setOriginalModel(modelName)

	// 入场守护(SEC-9/SEC-10/SEC-12/SEC-13)已前移进 getProvider:每次渠道选定后、建立上游
	// 连接之前执行 NewQuota + PreQuotaConsumption,守护拒绝时零上游连接、用户只收到错误事件。
	if !relay.getProvider() {
		return
	}

	relay.usage = &types.UsageEvent{}

	wsProxy := requester.NewWSProxy(relay.userConn, relay.providerConn, time.Minute*2, relay.messageHandler, relay.usageHandler)

	wsProxy.Start()

	// 在 spawn TrackedGoroutine 之前完成 snapshot：handler 在 wsProxy.Wait() 返回后
	// 就会 return，gin 会把 *gin.Context 归还 pool 并可能被新请求 Reset。闭包持有值不持指针，
	// 彻底消除 c-pool 复用的数据竞争窗口。
	snap := relay_util.NewConsumeSnapshot(relay.c)
	common.TrackedGoroutine(func() {
		var closedBy string
		select {
		case <-wsProxy.UserClosed():
			closedBy = "user"
		case <-wsProxy.SupplierClosed():
			closedBy = "provider"
		}

		logger.LogInfo(snap.Ctx, fmt.Sprintf("Connection closed by %s", closedBy))
		wsProxy.Close()
		relay.quota.ConsumeWithSnapshot(snap, relay.usage.ToChatUsage(), false)
	})

	wsProxy.Wait()
}

// enterQuotaGuard realtime 入场守护:PreQuotaConsumption 拒绝时向用户连接写错误事件并关闭用户连接。
// SEC-13 后守护先于建立上游连接执行(guardBeforeConnect),拒绝时 providerConn 必为 nil
// (本次尝试尚未连接,失败尝试的连接已在 getProvider 中清理),无需结算/Undo
// (PreQuotaConsumption 自身失败已在内部回冲部分预留)。
func (r *RelayModeChatRealtime) enterQuotaGuard() bool {
	if apiErr := preQuotaConsumptionFn(r.quota); apiErr != nil {
		r.abortWithMessage(apiErr.OpenAIError.Message)
		return false
	}
	return true
}

// guardBeforeConnect SEC-13「守护先行、连接在后」:入场守护拒绝时不调用 CreateChatRealtime,
// 零上游连接;守护通过但上游连接失败时立即回冲预扣(quota.Undo),不允许预扣泄漏。
// guardPassed=false 表示守护拒绝(错误事件已写给用户),调用方应直接终止。
func (r *RelayModeChatRealtime) guardBeforeConnect(realtimeProvider providersBase.RealtimeInterface) (providerConn *websocket.Conn, messageHandler requester.MessageHandler, apiErr *types.OpenAIErrorWithStatusCode, guardPassed bool) {
	if !r.enterQuotaGuard() {
		return nil, nil, nil, false
	}

	providerConn, messageHandler, apiErr = realtimeProvider.CreateChatRealtime(r.modelName)
	if apiErr != nil {
		r.quota.Undo(r.c)
	}
	return providerConn, messageHandler, apiErr, true
}

func (r *RelayModeChatRealtime) abortWithMessage(message string) {
	eventErr := types.NewErrorEvent("", "system_error", "system_error", message)

	r.userConn.WriteMessage(websocket.TextMessage, []byte(eventErr.Error()))
	r.userConn.Close()
}

func (r *RelayModeChatRealtime) getProvider() bool {
	retryTimes := config.RetryTimes
	if retryTimes == 0 {
		retryTimes = 1
	}

	for i := retryTimes; i > 0; i-- {
		// 找不到直接返回
		if err := r.setProvider(r.getOriginalModel()); err != nil {
			r.abortWithMessage(err.Error())
			return false
		}

		realtimeProvider, ok := r.provider.(providersBase.RealtimeInterface)
		if !ok {
			r.abortWithMessage("channel not implemented")
			return false
		}
		channel := r.provider.GetChannel()

		// SEC-13:渠道选定后(计价键仍为 getModelName() 的渠道映射后语义,不变)、
		// 建立上游连接之前完成 NewQuota + 入场守护;守护拒绝时零上游连接。
		r.quota = relay_util.NewQuota(r.getContext(), r.getModelName(), 0)
		providerConn, messageHandler, apiErr, guardPassed := r.guardBeforeConnect(realtimeProvider)
		if !guardPassed {
			return false
		}
		if apiErr != nil {
			// 预扣已在 guardBeforeConnect 内回冲(quota.Undo),此处仅跳过渠道重试
			r.skipChannelIds(channel.Id)
			logger.LogError(r.c.Request.Context(), fmt.Sprintf("using channel #%d(%s) Error: %s to retry (remain times %d)", channel.Id, channel.Name, apiErr.Error(), i))
			metrics.RecordProvider(r.c, apiErr.StatusCode)

			continue
		}

		r.messageHandler = messageHandler
		r.providerConn = providerConn

		if r.getRealtimeFirstMessage() {
			metrics.RecordProvider(r.c, 200)
			return true
		}

		// 首条消息失败:回冲本次预扣并关闭/清空上游连接,保证下一次尝试守护时零存活上游连接
		r.quota.Undo(r.c)
		r.providerConn.Close()
		r.providerConn = nil
		r.messageHandler = nil
		r.skipChannelIds(channel.Id)
	}

	r.abortWithMessage("get provider failed")
	return false
}

func (r *RelayModeChatRealtime) skipChannelIds(channelId int) {
	skipChannelIds, ok := utils.GetGinValue[[]int](r.c, "skip_channel_ids")
	if !ok {
		skipChannelIds = make([]int, 0)
	}

	skipChannelIds = append(skipChannelIds, channelId)

	r.c.Set("skip_channel_ids", skipChannelIds)
}

func (r *RelayModeChatRealtime) getRealtimeFirstMessage() bool {
	messageType, firstMessage, err := r.providerConn.ReadMessage()
	if err != nil {
		return false
	}

	if messageType != websocket.TextMessage {
		return false
	}

	shouldContinue, _, newMessage, err := r.messageHandler(requester.SupplierMessage, messageType, firstMessage)

	if !shouldContinue || err != nil {
		return false
	}

	if newMessage != nil {
		r.userConn.WriteMessage(websocket.TextMessage, newMessage)
	} else {
		r.userConn.WriteMessage(websocket.TextMessage, firstMessage)
	}

	return true
}

func (r *RelayModeChatRealtime) usageHandler(usage *types.UsageEvent) error {
	err := r.quota.UpdateUserRealtimeQuota(r.usage, usage)
	if err != nil {
		return types.NewErrorEvent("", "system_error", "system_error", err.Error())
	}

	return nil
}
