package task

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay"
	"github.com/modeltaps/modeltaps/relay/task/base"

	"github.com/gin-gonic/gin"
)

// MODEL-8：任务型 relay（RelayTaskSubmit）的重试链路里，「计数 / cooldown」用的模型名必须是
// balancer 口径的 routing_model，而不是 taskAdaptor.GetModelName()——后者是 model_mapping
// 映射后的上游名，且受 billing_original_model 开关影响，两者都不是 balancer 的 Rule /
// IsInCooldown 键。以下用例锁定 main.go 两处调用点的表达式口径。

const (
	tmRouting = "foo" // 渠道 models / balancer Rule 键 / 用户请求名
	tmMapped  = "bar" // model_mapping 映射后发给上游的名字
)

// newMappedTaskContext 构造一个「配了 model_mapping」的任务请求上下文，
// 三个模型名键与 relay.GetProvider 的写入口径一致。
func newMappedTaskContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("token_group", "g")
	c.Set("original_model", tmRouting)
	c.Set("routing_model", tmRouting)
	c.Set("new_model", tmMapped)
	return c
}

// newMappedTaskAdaptor 构造一个 ModelName 已被 GetProviderByModel 赋为映射后上游名的 TaskBase。
func newMappedTaskAdaptor(c *gin.Context) *base.TaskBase {
	return &base.TaskBase{C: c, OriginalModel: tmRouting, ModelName: tmMapped}
}

func clearTaskCooldown(channelId int, modelName string) {
	model.ChannelGroup.Cooldowns.Delete(fmt.Sprintf("%d:%s", channelId, modelName))
}

// withTaskRuleFixture 装配 group "g" 下配置了 tmRouting 的渠道集合，用例结束后还原。
func withTaskRuleFixture(t *testing.T, channels map[int]*model.Channel) {
	t.Helper()

	prevRule := model.ChannelGroup.Rule
	prevChannels := model.ChannelGroup.Channels
	t.Cleanup(func() {
		model.ChannelGroup.Rule = prevRule
		model.ChannelGroup.Channels = prevChannels
	})

	ids := make([]int, 0, len(channels))
	choices := make(map[int]*model.ChannelChoice, len(channels))
	for id, ch := range channels {
		ids = append(ids, id)
		choices[id] = &model.ChannelChoice{Channel: ch}
	}
	model.ChannelGroup.Rule = map[string]map[string][][]int{"g": {tmRouting: {ids}}}
	model.ChannelGroup.Channels = choices
}

func taskTestChannel(id int) *model.Channel {
	weight := uint(1)
	return &model.Channel{Id: id, Name: "ch", Type: config.ChannelTypeOpenAI, Weight: &weight, Status: config.ChannelStatusEnabled}
}

// TestTaskRetryCooldownKey_RoutingModel：重试循环里的 SetCooldowns 必须以 routing_model 为键，
// 否则 balancer 的 IsInCooldown 查不到，失败的任务型渠道会被立刻重选。
func TestTaskRetryCooldownKey_RoutingModel(t *testing.T) {
	prev := config.RetryCooldownSeconds
	config.RetryCooldownSeconds = 60
	defer func() { config.RetryCooldownSeconds = prev }()

	const channelId = 9500
	defer clearTaskCooldown(channelId, tmRouting)
	defer clearTaskCooldown(channelId, tmMapped)

	c := newMappedTaskContext()

	if !model.ChannelGroup.SetCooldowns(channelId, relay.RoutingModelName(c)) {
		t.Fatal("expected cooldown to be applied")
	}
	if !model.ChannelGroup.IsInCooldown(channelId, tmRouting) {
		t.Fatalf("expected cooldown keyed by routing model %q", tmRouting)
	}
	if model.ChannelGroup.IsInCooldown(channelId, tmMapped) {
		t.Fatalf("cooldown must not be keyed by mapped model %q", tmMapped)
	}
}

// TestTaskTotalChannelsAtStart_RoutingModel：totalChannelsAtStart 用 routing_model 才非 0；
// 用 GetModelName()（映射后名字）查 Rule 键不存在 → 0 → actualRetryTimes=0 → 完全不重试。
func TestTaskTotalChannelsAtStart_RoutingModel(t *testing.T) {
	withTaskRuleFixture(t, map[int]*model.Channel{9510: taskTestChannel(9510), 9511: taskTestChannel(9511)})

	c := newMappedTaskContext()
	taskAdaptor := newMappedTaskAdaptor(c)

	if n := model.ChannelGroup.CountAvailableChannels("g", relay.RoutingModelName(c)); n != 2 {
		t.Fatalf("CountAvailableChannels by routing model: got %d, want 2", n)
	}
	if n := model.ChannelGroup.CountAvailableChannels("g", taskAdaptor.GetModelName()); n != 0 {
		t.Fatalf("CountAvailableChannels by mapped model: got %d, want 0", n)
	}
}

// TestTaskRoutingModel_UnaffectedByBillingOriginalModel：billing_original_model 只决定计费
// 模型名（NewQuota / CompletedTask），不得再影响计数与 cooldown 键。
func TestTaskRoutingModel_UnaffectedByBillingOriginalModel(t *testing.T) {
	withTaskRuleFixture(t, map[int]*model.Channel{9520: taskTestChannel(9520)})

	for _, billingOriginal := range []bool{false, true} {
		c := newMappedTaskContext()
		c.Set("billing_original_model", billingOriginal)
		taskAdaptor := newMappedTaskAdaptor(c)

		if got := relay.RoutingModelName(c); got != tmRouting {
			t.Fatalf("billing_original_model=%v: routing model got %q, want %q", billingOriginal, got, tmRouting)
		}
		if n := model.ChannelGroup.CountAvailableChannels("g", relay.RoutingModelName(c)); n != 1 {
			t.Fatalf("billing_original_model=%v: got %d available, want 1", billingOriginal, n)
		}
		// 计费口径保持由开关决定（本项不改）。
		wantBilling := tmMapped
		if billingOriginal {
			wantBilling = tmRouting
		}
		if got := taskAdaptor.GetModelName(); got != wantBilling {
			t.Fatalf("billing_original_model=%v: GetModelName got %q, want %q", billingOriginal, got, wantBilling)
		}
	}
}

// TestTaskRoutingModel_NoMapping：无 model_mapping 时三个名字相同，行为不变。
func TestTaskRoutingModel_NoMapping(t *testing.T) {
	withTaskRuleFixture(t, map[int]*model.Channel{9530: taskTestChannel(9530)})

	c := newMappedTaskContext()
	c.Set("routing_model", tmRouting)
	c.Set("new_model", tmRouting)
	taskAdaptor := &base.TaskBase{C: c, OriginalModel: tmRouting, ModelName: tmRouting}

	if got, want := relay.RoutingModelName(c), taskAdaptor.GetModelName(); got != want {
		t.Fatalf("without model_mapping both must agree: routing %q vs GetModelName %q", got, want)
	}
}
