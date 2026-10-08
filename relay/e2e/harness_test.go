package e2e

// relay 全链路 E2E 的装配基建：内存 sqlite + 真实 SetRelayRouter + seed user/token/channel，
// 上游由 fake_provider_test.go 的 httptest.Server 顶替（渠道 BaseURL 注入）。
//
// 全局单例（model.DB / ChannelGroup / GlobalUserGroupRatio / PricingInstance / config.*）
// 每个测试重置并在 t.Cleanup 还原，故本包测试不得使用 t.Parallel。

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/router"

	"github.com/gin-gonic/gin"
)

var relayTestDBSeq atomic.Int64

// ChannelSpec 描述一个待 seed 的渠道。BaseURL 一般指向 fake 上游。
type ChannelSpec struct {
	Name     string
	Type     int    // 0 视为 config.ChannelTypeOpenAI
	BaseURL  string
	Models   string // 逗号分隔；空则用 HarnessOptions.Model
	Group    string // 空则 "default"
	Priority int64
	Key      string // 空则用占位 key
}

// PriceSpec 描述一个测试模型的价格（Input/Output 为倍率，语义同 model.Price）。
type PriceSpec struct {
	Model  string
	Type   string // 空则 model.TokensPriceType
	Input  float64
	Output float64
}

// HarnessOptions 控制装配细节；零值即「单渠道 + 单模型 + 充足额度」的默认场景。
type HarnessOptions struct {
	Model       string        // 默认 "gpt-4o-mini"
	Group       string        // 用户/渠道分组，默认 "default"
	GroupRatio  float64       // 分组倍率，默认 1
	UserQuota   int           // 用户额度，默认 10_000_000
	TokenQuota  int           // 令牌剩余额度，默认 10_000_000
	Channels    []ChannelSpec // 空则按 FakeBaseURL seed 单渠道
	FakeBaseURL string        // 默认渠道的上游地址
	Prices      []PriceSpec   // 空则给 Model 配 1/1 的 tokens 价格
}

// Harness 是装配产物：就绪的 gin 引擎 + seed 好的实体。
type Harness struct {
	Engine *gin.Engine
	User   *model.User
	Token  *model.Token
	// Channel 为首个 seed 渠道；多渠道场景（如重试换渠道）用 Channels 逐个断言。
	Channel  *model.Channel
	Channels []*model.Channel
	Model    string
}

// NewHarness 一次调用装配好整条 relay 链路所需的 DB / 路由 / 数据 / 全局单例。
func NewHarness(t *testing.T, opts HarnessOptions) *Harness {
	t.Helper()
	opts = opts.withDefaults()

	setupTestConfig(t)
	setupTestDB(t)
	setupPricing(t, opts.Prices)
	seedUserGroup(t, opts.Group, opts.GroupRatio)

	user := seedUser(t, opts.Group, opts.UserQuota)
	token := seedToken(t, user.Id, opts.TokenQuota)
	channels := seedChannels(t, opts)

	model.GlobalUserGroupRatio.Load()
	model.ChannelGroup.Load()
	t.Cleanup(func() {
		model.GlobalUserGroupRatio = model.UserGroupRatio{}
		model.ChannelGroup = model.ChannelsChooser{}
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(newTestRequestContext())
	router.SetRelayRouter(engine)

	return &Harness{
		Engine:   engine,
		User:     user,
		Token:    token,
		Channel:  channels[0],
		Channels: channels,
		Model:    opts.Model,
	}
}

func (o HarnessOptions) withDefaults() HarnessOptions {
	if o.Model == "" {
		o.Model = "gpt-4o-mini"
	}
	if o.Group == "" {
		o.Group = "default"
	}
	if o.GroupRatio == 0 {
		o.GroupRatio = 1
	}
	if o.UserQuota == 0 {
		o.UserQuota = 10_000_000
	}
	if o.TokenQuota == 0 {
		o.TokenQuota = 10_000_000
	}
	if len(o.Channels) == 0 {
		o.Channels = []ChannelSpec{{Name: "fake-openai", BaseURL: o.FakeBaseURL}}
	}
	if len(o.Prices) == 0 {
		o.Prices = []PriceSpec{{Model: o.Model, Input: 1, Output: 1}}
	}
	return o
}

// Post 以 seed 的令牌向 relay 路由发起一次 JSON 请求。
func (h *Harness) Post(path, body string) *httptest.ResponseRecorder {
	return h.PostWithToken(h.Token.Key, path, body)
}

// PostWithToken 用指定令牌 key 发起请求，供组织令牌等非默认身份的场景使用。
func (h *Harness) PostWithToken(tokenKey, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenKey)
	w := httptest.NewRecorder()
	h.Engine.ServeHTTP(w, req)
	return w
}

// UserQuota 读取 seed 用户当前剩余额度。
func (h *Harness) UserQuota(t *testing.T) int {
	t.Helper()
	quota, err := model.GetUserQuota(h.User.Id)
	if err != nil {
		t.Fatalf("读取用户额度失败: %v", err)
	}
	return quota
}
