package e2e

// harness 的内部装配步骤：config 归零、内存 DB、价格表、seed 实体。

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/cache"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// 进程级一次性初始化：token 签发密钥、HTTP client、cache manager 都是全局单例，
// 重复初始化没有意义且会在并发下打架。
var initOnce sync.Once

func setupTestConfig(t *testing.T) {
	t.Helper()

	initOnce.Do(func() {
		viper.Set("user_token_secret", "relay-e2e-token-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatalf("初始化令牌签发失败: %v", err)
		}
		requester.InitHttpClient()
		cache.InitCacheManager()
	})

	// Redis 关闭时缓存/限流自动退化为本地实现，无需外部依赖。
	setAndRestore(t, &config.RedisEnabled, false)
	// 批量更新会把扣费延后到定时 flush，E2E 需要请求返回后即可断言额度。
	setAndRestore(t, &config.BatchUpdateEnabled, false)
	// 关闭编码器省去 tiktoken 词表加载；token 数走近似估算，够 E2E 断言「扣费发生」。
	setAndRestore(t, &config.DisableTokenEncoders, true)
	setAndRestore(t, &config.RetryTimes, 0)
	setAndRestore(t, &config.LogConsumeEnabled, true)
	setAndRestore(t, &config.MemoryCacheEnabled, false)
	setAndRestore(t, &config.EnableSafe, false)
	setAndRestore(t, &config.UnpricedModelPolicy, "block")
}

// setAndRestore 设置全局变量并在测试结束还原原值。
func setAndRestore[T any](t *testing.T, target *T, value T) {
	t.Helper()
	old := *target
	*target = value
	t.Cleanup(func() { *target = old })
}

func setupTestDB(t *testing.T) {
	t.Helper()
	// shared-cache 让并发 goroutine 共享同一内存库；不同 DSN 名保证用例间隔离。
	dsn := fmt.Sprintf("file:relay_e2e_%d?mode=memory&cache=shared", relayTestDBSeq.Add(1))
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("获取底层 DB 失败: %v", err)
	}
	// 并发共享同一连接池会触发 sqlite "database is locked"；限制为单连接串行化写入。
	sqlDB.SetMaxOpenConns(1)

	if err := testDB.AutoMigrate(
		&model.User{}, &model.Token{}, &model.Channel{}, &model.UserGroup{},
		&model.Price{}, &model.ModelInfo{}, &model.Log{}, &model.LogDetail{},
		// 组织令牌场景（影子账户记账 / 成员守护）需要组织侧两张表
		&model.Organization{}, &model.OrganizationMember{},
	); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}

	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() {
		model.DB = oldDB
		_ = sqlDB.Close()
	})
}

func setupPricing(t *testing.T, prices []PriceSpec) {
	t.Helper()
	rows := make([]*model.Price, 0, len(prices))
	for _, p := range prices {
		priceType := p.Type
		if priceType == "" {
			priceType = model.TokensPriceType
		}
		rows = append(rows, &model.Price{
			Model:       p.Model,
			Type:        priceType,
			ChannelType: config.ChannelTypeOpenAI,
			Input:       p.Input,
			Output:      p.Output,
		})
	}
	if len(rows) > 0 {
		if err := model.DB.Create(rows).Error; err != nil {
			t.Fatalf("写入测试价格失败: %v", err)
		}
	}

	oldPricing := model.PricingInstance
	model.PricingInstance = &model.Pricing{
		Prices: make(map[string]*model.Price),
		Match:  make([]string, 0),
	}
	if err := model.PricingInstance.Init(); err != nil {
		t.Fatalf("初始化价格表失败: %v", err)
	}
	t.Cleanup(func() { model.PricingInstance = oldPricing })
}

func seedUserGroup(t *testing.T, symbol string, ratio float64) {
	t.Helper()
	enable := true
	group := &model.UserGroup{
		Symbol:  symbol,
		Name:    symbol,
		Ratio:   ratio,
		APIRate: 100000,
		Public:  true,
		Enable:  &enable,
	}
	if err := model.DB.Create(group).Error; err != nil {
		t.Fatalf("写入测试用户组失败: %v", err)
	}
}

func seedUser(t *testing.T, group string, quota int) *model.User {
	t.Helper()
	user := &model.User{
		Username:    "e2e_user",
		Password:    "e2e-password",
		DisplayName: "E2E User",
		Role:        config.RoleCommonUser,
		Status:      config.UserStatusEnabled,
		AccessToken: utils.GetUUID(),
		AffCode:     utils.GetRandomString(8),
		Quota:       quota,
		Group:       group,
		CreatedTime: utils.GetTimestamp(),
	}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("写入测试用户失败: %v", err)
	}
	return user
}

// seedToken 依赖 Token.AfterCreate 生成合法 Key（59 字符，满足 middleware/auth.go 的长度校验）。
func seedToken(t *testing.T, userId int, quota int) *model.Token {
	t.Helper()
	token := &model.Token{
		UserId:      userId,
		Name:        "e2e-token",
		Status:      config.TokenStatusEnabled,
		ExpiredTime: -1,
		RemainQuota: quota,
		CreatedTime: utils.GetTimestamp(),
	}
	if err := model.DB.Create(token).Error; err != nil {
		t.Fatalf("写入测试令牌失败: %v", err)
	}
	if len(token.Key) < 48 {
		t.Fatalf("生成的令牌 key 长度 %d 不满足鉴权要求", len(token.Key))
	}
	return token
}

func seedChannels(t *testing.T, opts HarnessOptions) []*model.Channel {
	t.Helper()
	channels := make([]*model.Channel, 0, len(opts.Channels))
	for i, spec := range opts.Channels {
		channelType := spec.Type
		if channelType == 0 {
			channelType = config.ChannelTypeOpenAI
		}
		models := spec.Models
		if models == "" {
			models = opts.Model
		}
		group := spec.Group
		if group == "" {
			group = opts.Group
		}
		key := spec.Key
		if key == "" {
			key = "sk-e2e-fake-key"
		}
		name := spec.Name
		if name == "" {
			name = fmt.Sprintf("fake-channel-%d", i+1)
		}
		baseURL := spec.BaseURL
		priority := spec.Priority
		weight := uint(1)

		channel := &model.Channel{
			Type:        channelType,
			Key:         key,
			Status:      config.ChannelStatusEnabled,
			Name:        name,
			Weight:      &weight,
			BaseURL:     &baseURL,
			Models:      models,
			Group:       group,
			Priority:    &priority,
			PreCost:     config.PreCostDefault,
			CreatedTime: utils.GetTimestamp(),
		}
		if err := model.DB.Create(channel).Error; err != nil {
			t.Fatalf("写入测试渠道失败: %v", err)
		}
		channels = append(channels, channel)
	}
	return channels
}

// newTestRequestContext 复刻 middleware.RequestId 里 relay 链路依赖的 context 键
// （requestStartTime 供计费计时，request id 供日志），不引入完整 server 中间件栈。
func newTestRequestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := utils.GetTimeString() + utils.GetRandomString(8)
		c.Set(logger.RequestIdKey, id)
		c.Set("requestStartTime", time.Now())
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), logger.RequestIdKey, id))
		c.Next()
	}
}
