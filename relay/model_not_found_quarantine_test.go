package relay

import (
	"context"
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

func TestQuarantineModelNotFound_Matrix(t *testing.T) {
	prev := config.ModelNotFoundCooldownSeconds
	defer func() { config.ModelNotFoundCooldownSeconds = prev }()

	tests := []struct {
		name    string
		seconds int
		err     *types.OpenAIErrorWithStatusCode
		seen429 bool
		want    bool
	}{
		{"openrouter no endpoints found", 21600, upstreamErr(http.StatusNotFound, false, "No endpoints found for "+qModel), false, true},
		{"openai does not exist", 21600, upstreamErr(http.StatusNotFound, false, "The model `x` does not exist or you do not have access to it."), false, true},
		{"disabled by option 0", 0, upstreamErr(http.StatusNotFound, false, "No endpoints found for "+qModel), false, false},
		{"negative option", -1, upstreamErr(http.StatusNotFound, false, "model not found"), false, false},
		{"429 status", 21600, upstreamErr(http.StatusTooManyRequests, false, "model not found"), false, false},
		{"upstream_seen_429 in chain", 21600, upstreamErr(http.StatusNotFound, false, "model not found"), true, false},
		{"local error", 21600, upstreamErr(http.StatusNotFound, true, "model not found"), false, false},
		{"unrelated upstream error", 21600, upstreamErr(http.StatusBadGateway, false, "upstream returned 502 bad gateway"), false, false},
		{"nil error", 21600, nil, false, false},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channelId := 9000 + i
			defer clearCooldown(channelId, qModel)

			config.ModelNotFoundCooldownSeconds = tt.seconds
			c := newTestContext()
			c.Set("routing_model", qModel)
			if tt.seen429 {
				c.Set("upstream_seen_429", true)
			}

			channel := &model.Channel{Id: channelId, Name: "ch", Type: config.ChannelTypeOpenAI}
			if got := quarantineModelNotFound(context.Background(), c, channel, tt.err); got != tt.want {
				t.Fatalf("quarantineModelNotFound: got %v, want %v", got, tt.want)
			}
			if got := model.ChannelGroup.IsInCooldown(channelId, qModel); got != tt.want {
				t.Fatalf("IsInCooldown(%d, %q): got %v, want %v", channelId, qModel, got, tt.want)
			}
		})
	}
}

// TestQuarantineModelNotFound_ModelMapping 锁定隔离键的口径：必须用 routing_model
// （balancer 选路 / IsInCooldown 用的名字），而不是 model_mapping 映射后的 new_model；
// 否则配了 model_mapping 的渠道隔离完全不生效。
func TestQuarantineModelNotFound_ModelMapping(t *testing.T) {
	prev := config.ModelNotFoundCooldownSeconds
	config.ModelNotFoundCooldownSeconds = 21600
	defer func() { config.ModelNotFoundCooldownSeconds = prev }()

	const routingModel = "jamba-large"
	const mappedModel = "ai21/jamba-large-1.7"
	const channelId = 9100
	defer clearCooldown(channelId, routingModel)
	defer clearCooldown(channelId, mappedModel)

	c := newTestContext()
	c.Set("original_model", routingModel)
	c.Set("routing_model", routingModel)
	c.Set("new_model", mappedModel) // model_mapping 映射后的上游名

	channel := &model.Channel{Id: channelId, Name: "ch", Type: config.ChannelTypeOpenAI}
	if !quarantineModelNotFound(context.Background(), c, channel, upstreamErr(http.StatusNotFound, false, "No endpoints found for "+mappedModel)) {
		t.Fatal("expected quarantine to apply")
	}

	if !model.ChannelGroup.IsInCooldown(channelId, routingModel) {
		t.Fatalf("expected cooldown keyed by routing model %q", routingModel)
	}
	if model.ChannelGroup.IsInCooldown(channelId, mappedModel) {
		t.Fatalf("cooldown must not be keyed by mapped model %q", mappedModel)
	}
}

// TestQuarantineModelNotFound_BalancerSkipsChannel 验证隔离后第二次请求在路由阶段
// 就拿不到该渠道（单渠道场景下直接无可用渠道），而不是再打一次上游。
func TestQuarantineModelNotFound_BalancerSkipsChannel(t *testing.T) {
	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels
	prevSeconds := config.ModelNotFoundCooldownSeconds
	defer func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
		config.ModelNotFoundCooldownSeconds = prevSeconds
	}()
	config.ModelNotFoundCooldownSeconds = 21600

	const channelId = 9200
	defer clearCooldown(channelId, qModel)

	weight := uint(1)
	channel := &model.Channel{Id: channelId, Name: "ch", Type: config.ChannelTypeOpenAI, Weight: &weight, Status: config.ChannelStatusEnabled}
	model.ChannelGroup.Rule = map[string]map[string][][]int{"g": {qModel: {{channelId}}}}
	model.ChannelGroup.Channels = map[int]*model.ChannelChoice{channelId: {Channel: channel}}

	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", nil)
	c.Set("token_group", "g")
	c.Set("routing_model", qModel)

	if n := model.ChannelGroup.CountAvailableChannels("g", qModel); n != 1 {
		t.Fatalf("precondition: expected 1 available channel, got %d", n)
	}

	if !quarantineModelNotFound(c.Request.Context(), c, channel, upstreamErr(http.StatusNotFound, false, "No endpoints found for "+qModel)) {
		t.Fatal("expected quarantine to apply")
	}

	if n := model.ChannelGroup.CountAvailableChannels("g", qModel); n != 0 {
		t.Fatalf("expected 0 available channels after quarantine, got %d", n)
	}
	if _, err := fetchChannelByModel(c, qModel); err == nil {
		t.Fatal("expected routing to fail after quarantine, got a channel")
	}
}

// TestNotifyChannelRelayError_Quarantine 锁定挂载点：隔离必须在 notifyChannelRelayError
// 里发生，即首次上游失败就生效，不依赖重试循环（单渠道时 actualRetryTimes=0，
// shouldCooldowns 根本不会被调用）。
func TestNotifyChannelRelayError_Quarantine(t *testing.T) {
	prevSeconds := config.ModelNotFoundCooldownSeconds
	prevAutoDisable := config.AutomaticDisableChannelEnabled
	config.ModelNotFoundCooldownSeconds = 21600
	config.AutomaticDisableChannelEnabled = false // 让异步的 processChannelRelayError 成为 no-op，不碰 DB
	defer func() {
		config.ModelNotFoundCooldownSeconds = prevSeconds
		config.AutomaticDisableChannelEnabled = prevAutoDisable
	}()
	// 异步的 processChannelRelayError 会读 AutomaticDisableChannelEnabled；defer 后进先出，
	// 这里先等它结束再执行上面的还原，否则 -race 会报测试与后台 goroutine 的数据竞争。
	defer relayErrorNotifyWG.Wait()

	const channelId = 9300
	defer clearCooldown(channelId, qModel)

	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", nil)
	c.Set("routing_model", qModel)

	channel := &model.Channel{Id: channelId, Name: "ch", Type: config.ChannelTypeOpenAI}
	notifyChannelRelayError(c.Request.Context(), c, channel, upstreamErr(http.StatusNotFound, false, "No endpoints found for "+qModel))

	if !model.ChannelGroup.IsInCooldown(channelId, qModel) {
		t.Fatal("expected notifyChannelRelayError to quarantine the (channel, model) pair")
	}
	if c.GetBool("upstream_seen_429") {
		t.Fatal("upstream_seen_429 must stay unset for a 404")
	}
}
