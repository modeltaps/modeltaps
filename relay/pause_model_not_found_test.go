package relay

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"
)

// clearCooldown 清掉指定 (渠道, 模型) 的冷却，避免用例之间互相污染
// （ChannelGroup 是包级单例，cooldown 存在进程内 sync.Map 里）。
func clearCooldown(channelId int, modelName string) {
	model.ChannelGroup.Cooldowns.Delete(fmt.Sprintf("%d:%s", channelId, modelName))
}

// upstreamErr 构造一个带文案的上游错误。
func upstreamErr(status int, local bool, message string) *types.OpenAIErrorWithStatusCode {
	e := apiErr(status, local)
	e.OpenAIError.Message = message
	return e
}

const qModel = "ai21/jamba-large-1.7"

// An upstream saying the model does not exist pauses that model on the channel through the
// built-in model_not_found rule, on the first failure (a single channel never runs the retry
// loop), keyed by the routing model so the balancer skips it.
func TestNotifyChannelRelayError_ModelNotFoundPauses(t *testing.T) {
	prevSeconds := config.ModelNotFoundCooldownSeconds
	// Under a minute, so the rule cools down in memory and nothing is written.
	config.ModelNotFoundCooldownSeconds = 30
	defer func() { config.ModelNotFoundCooldownSeconds = prevSeconds }()
	defer relayErrorNotifyWG.Wait()

	const channelId = 9300
	defer clearCooldown(channelId, qModel)

	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", nil)
	c.Set("routing_model", qModel)

	channel := &model.Channel{Id: channelId, Name: "ch", Type: config.ChannelTypeOpenAI}
	notifyChannelRelayError(c.Request.Context(), c, channel, upstreamErr(http.StatusNotFound, false, "No endpoints found for "+qModel))

	if !model.ChannelGroup.IsInCooldown(channelId, qModel) {
		t.Fatal("expected the model_not_found rule to pause the (channel, model) pair")
	}
	if !c.GetBool(lastFailurePausedKey) {
		t.Fatal("expected the failure to be recorded as paused")
	}
	if c.GetBool("upstream_seen_429") {
		t.Fatal("upstream_seen_429 must stay unset for a 404")
	}
}

// A gateway-local error says nothing about the channel and pauses nothing.
func TestNotifyChannelRelayError_LocalErrorDoesNotPause(t *testing.T) {
	const channelId = 9301
	defer clearCooldown(channelId, qModel)

	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("routing_model", qModel)

	channel := &model.Channel{Id: channelId, Name: "ch", Type: config.ChannelTypeOpenAI}
	notifyChannelRelayError(c.Request.Context(), c, channel, upstreamErr(http.StatusNotFound, true, "model not found"))

	if model.ChannelGroup.IsInCooldown(channelId, qModel) || c.GetBool(lastFailurePausedKey) {
		t.Fatal("a local error paused the channel")
	}
}
