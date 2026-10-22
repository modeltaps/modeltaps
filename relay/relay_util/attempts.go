package relay_util

import (
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
)

// relayAttemptsKey 在 gin.Context 上累积本次请求失败过的上游尝试。
const relayAttemptsKey = "relay_attempts"

// RelayAttempt 是一次失败的上游尝试，写入日志 metadata.retry_attempts 供日志详情展示决策流程。
// 只记录渠道与状态码：上游原文可能暴露供应商身份或密钥状态，用户也能看到自己的日志。
type RelayAttempt struct {
	ChannelID int  `json:"channel_id"`
	Status    int  `json:"status"`
	Cooldown  bool `json:"cooldown,omitempty"`
}

// RecordRelayAttempt 记下一次失败的尝试。
func RecordRelayAttempt(c *gin.Context, channelID int, apiErr *types.OpenAIErrorWithStatusCode) {
	if c == nil || apiErr == nil {
		return
	}
	attempts := RelayAttempts(c)
	c.Set(relayAttemptsKey, append(attempts, RelayAttempt{ChannelID: channelID, Status: apiErr.StatusCode}))
}

// MarkLastRelayAttemptCooldown 标记上一次失败的渠道被冷却（重试前判定）。
func MarkLastRelayAttemptCooldown(c *gin.Context, cooldown bool) {
	attempts := RelayAttempts(c)
	if len(attempts) == 0 || !cooldown {
		return
	}
	attempts[len(attempts)-1].Cooldown = true
}

// RelayAttempts 返回本次请求已记录的失败尝试。
func RelayAttempts(c *gin.Context) []RelayAttempt {
	if c == nil {
		return nil
	}
	if v, ok := c.Get(relayAttemptsKey); ok {
		if attempts, ok := v.([]RelayAttempt); ok {
			return attempts
		}
	}
	return nil
}
