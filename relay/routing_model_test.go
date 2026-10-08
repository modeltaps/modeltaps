package relay

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

// MODEL-7：重试链路上的模型名必须是 balancer 口径的 routing_model（通配符匹配后、
// model_mapping 映射前），而不是发给上游的 new_model。以下用例锁定三处口径：
// cooldown 键、可用渠道计数、FilterDisabledStream。

const (
	rmRouting = "foo" // 渠道 models / balancer Rule 键 / 用户请求名
	rmMapped  = "bar" // model_mapping 映射后发给上游的名字
)

// newMappedContext 构造一个「配了 model_mapping」的请求上下文。
func newMappedContext() *gin.Context {
	c := newTestContext()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("token_group", "g")
	c.Set("original_model", rmRouting)
	c.Set("routing_model", rmRouting)
	c.Set("new_model", rmMapped)
	return c
}

func TestRoutingModelName_PrecedenceAndFallback(t *testing.T) {
	c := newMappedContext()
	if got := RoutingModelName(c); got != rmRouting {
		t.Fatalf("RoutingModelName: got %q, want %q", got, rmRouting)
	}

	// routing_model 缺失（未走 GetProvider 的边缘路径）时退回 original_model，
	// 与 MODEL-6 的 quarantine 口径一致。
	c.Set("routing_model", "")
	if got := RoutingModelName(c); got != rmRouting {
		t.Fatalf("RoutingModelName fallback: got %q, want %q", got, rmRouting)
	}
}

// TestShouldCooldowns_RoutingModelKey：429 冷却必须以 routing_model 为键，
// 否则 balancer 的 IsInCooldown 查不到，配了 model_mapping 的渠道冷却完全失效。
func TestShouldCooldowns_RoutingModelKey(t *testing.T) {
	prev := config.RetryCooldownSeconds
	config.RetryCooldownSeconds = 60
	defer func() { config.RetryCooldownSeconds = prev }()

	const channelId = 9400
	defer clearCooldown(channelId, rmRouting)
	defer clearCooldown(channelId, rmMapped)

	c := newMappedContext()
	channel := &model.Channel{Id: channelId, Name: "ch", Type: config.ChannelTypeOpenAI}

	if !shouldCooldowns(c, channel, apiErr(http.StatusTooManyRequests, false)) {
		t.Fatal("expected cooldown to be applied for 429")
	}
	if !model.ChannelGroup.IsInCooldown(channelId, rmRouting) {
		t.Fatalf("expected cooldown keyed by routing model %q", rmRouting)
	}
	if model.ChannelGroup.IsInCooldown(channelId, rmMapped) {
		t.Fatalf("cooldown must not be keyed by mapped model %q", rmMapped)
	}
}

// withRuleFixture 装配一个 group "g" 下配置了 rmRouting 的渠道集合，并在用例结束后还原。
func withRuleFixture(t *testing.T, channels map[int]*model.Channel) {
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
	model.ChannelGroup.Rule = map[string]map[string][][]int{"g": {rmRouting: {ids}}}
	model.ChannelGroup.Channels = choices
}

func testChannel(id int) *model.Channel {
	weight := uint(1)
	return &model.Channel{Id: id, Name: "ch", Type: config.ChannelTypeOpenAI, Weight: &weight, Status: config.ChannelStatusEnabled}
}

// TestCountAvailableChannels_RoutingModel：totalChannelsAtStart 用 routing_model 才非 0；
// 用 new_model 查 Rule 键不存在 → 0 → actualRetryTimes=0 → 映射模型完全不重试。
func TestCountAvailableChannels_RoutingModel(t *testing.T) {
	withRuleFixture(t, map[int]*model.Channel{9410: testChannel(9410), 9411: testChannel(9411)})

	c := newMappedContext()
	if n := model.ChannelGroup.CountAvailableChannels("g", RoutingModelName(c)); n != 2 {
		t.Fatalf("CountAvailableChannels by routing model: got %d, want 2", n)
	}
	if n := model.ChannelGroup.CountAvailableChannels("g", c.GetString("new_model")); n != 0 {
		t.Fatalf("CountAvailableChannels by mapped model: got %d, want 0", n)
	}
}

// TestFilterDisabledStream_RoutingModel：Channel.DisabledStream 存的是渠道 models 里的
// 名字（渠道侧 / routing 口径，见 web ChannelSheet 的 disabled_stream 自由文本输入），
// 因此禁流过滤必须用 routing_model 判定。
func TestFilterDisabledStream_RoutingModel(t *testing.T) {
	disabled := datatypes.NewJSONSlice([]string{rmRouting})
	blocked := testChannel(9420)
	blocked.DisabledStream = &disabled

	withRuleFixture(t, map[int]*model.Channel{9420: blocked, 9421: testChannel(9421)})

	c := newMappedContext()
	c.Set("is_stream", true)

	routingFilters := buildChannelFilters(c, RoutingModelName(c))
	if n := model.ChannelGroup.CountAvailableChannels("g", RoutingModelName(c), routingFilters...); n != 1 {
		t.Fatalf("stream filtering by routing model: got %d available, want 1", n)
	}

	// 用映射后的名字判定会漏掉禁流渠道（修复前的行为）。
	mappedFilters := buildChannelFilters(c, c.GetString("new_model"))
	if n := model.ChannelGroup.CountAvailableChannels("g", RoutingModelName(c), mappedFilters...); n != 2 {
		t.Fatalf("stream filtering by mapped model: got %d available, want 2 (name mismatch)", n)
	}
}
