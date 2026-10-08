package relay_util

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// ========== B1 特征化测试：锁定现有计费行为（重构安全网），不改任何生产代码 ==========

// setupQuotaTestDB 打开内存 sqlite、迁移计费相关表并替换 model.DB（controller/order_test.go 同款）。
// 独立 DSN 隔离用例；单连接串行化写入避免 sqlite "database is locked"。
var quotaTestDBSeq atomic.Int64

func setupQuotaTestDB(t *testing.T) {
	t.Helper()
	// Token.AfterCreate 钩子依赖令牌编码器（同 model.setupOrgTestDB）
	if viper.GetString("user_token_secret") == "" {
		viper.Set("user_token_secret", "quota-test-secret")
		if err := common.InitUserToken(); err != nil {
			t.Fatalf("初始化用户令牌编码器失败: %v", err)
		}
	}
	dsn := fmt.Sprintf("file:quota_test_%d?mode=memory&cache=shared", quotaTestDBSeq.Add(1))
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
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}, &model.LogDetail{},
		&model.Channel{}, &model.OrganizationMember{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
}

// seedUserAndToken 写入一个用户与其令牌，返回 token id。
func seedUserAndToken(t *testing.T, userId int, userQuota int, tokenRemain int) int {
	t.Helper()
	user := &model.User{Id: userId, Username: fmt.Sprintf("quota-user-%d-%d", userId, quotaTestDBSeq.Load()),
		Password: "placeholder", Quota: userQuota, Group: "default",
		AccessToken: fmt.Sprintf("quota-test-at-%d-%d", quotaTestDBSeq.Load(), userId),
		AffCode:     fmt.Sprintf("qt%d-%d", quotaTestDBSeq.Load(), userId)}
	if err := model.DB.Create(user).Error; err != nil {
		t.Fatalf("创建用户失败: %v", err)
	}
	token := &model.Token{UserId: userId, Name: "quota-test-token", RemainQuota: tokenRemain}
	if err := model.DB.Create(token).Error; err != nil {
		t.Fatalf("创建令牌失败: %v", err)
	}
	return token.Id
}

func getUserQuota(t *testing.T, userId int) int {
	t.Helper()
	q, err := model.GetUserQuota(userId)
	if err != nil {
		t.Fatalf("读取用户额度失败: %v", err)
	}
	return q
}

func getToken(t *testing.T, tokenId int) *model.Token {
	t.Helper()
	tok, err := model.GetTokenById(tokenId)
	if err != nil {
		t.Fatalf("读取令牌失败: %v", err)
	}
	return tok
}

// ---------- 1. calcQuota 全分支 ----------

// TestCalcQuotaTokensAndTimes 锁定 token/times 两种计价与空回复/零 token 闸的现状。
func TestCalcQuotaTokensAndTimes(t *testing.T) {
	cases := []struct {
		name                     string
		priceType                string
		promptTokens, completion int
		inputRatio, outputRatio  float64
		want                     int
	}{
		// tokens 计价：ceil(pt*in + ct*out)
		{"tokens 基本", model.TokensPriceType, 1000, 500, 1.5, 2.0, 2500},
		// tokens 计价向上取整
		{"tokens 取整", model.TokensPriceType, 3, 0, 0.4, 0, 2},
		// times 计价：1000*inputRatio，与 token 数无关
		{"times 基本", model.TimesPriceType, 10, 0, 0.05, 0, 50},
		// 最小计 1：量*率不足 1 但 inputRatio != 0
		{"最小记 1", model.TokensPriceType, 0, 5, 0.5, 0, 1},
		// totalTokens==0 闸：上游未成功返回，两种计价都归零
		{"零 token 闸 tokens", model.TokensPriceType, 0, 0, 1.5, 2.0, 0},
		{"零 token 闸 times", model.TimesPriceType, 0, 0, 0.05, 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &Quota{price: model.Price{Type: tc.priceType}}
			got := q.calcQuota(tc.promptTokens, tc.completion, tc.inputRatio, tc.outputRatio, 1)
			if got != tc.want {
				t.Fatalf("calcQuota = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestCalcQuotaOverflowSaturation 溢出饱和：巨量 token*倍率经 QuotaFromFloat 饱和到 MaxQuota，
// 不得回绕为负（扣费变退款）。
func TestCalcQuotaOverflowSaturation(t *testing.T) {
	q := &Quota{price: model.Price{Type: model.TokensPriceType}}
	got := q.calcQuota(math.MaxInt32, 0, 1e10, 0, 1)
	if got != common.MaxQuota {
		t.Fatalf("calcQuota = %d, want MaxQuota %d", got, common.MaxQuota)
	}
}

// TestCalcQuotaExtraBilling 额外计费项：每项 QuotaFromFloat(ceil(price*QuotaPerUnit)*callCount)，
// 合计后再乘 ratio 并向上取整加到 quota。
func TestCalcQuotaExtraBilling(t *testing.T) {
	// 默认 QuotaPerUnit = 500000：0.025$ * 500000 = 12500/次
	q := &Quota{
		price: model.Price{Type: model.TokensPriceType},
		extraBillingData: map[string]ExtraBillingData{
			types.APITollTypeWebSearchPreview: {Type: "", CallCount: 2, Price: 0.025},
		},
	}
	// 基础 100*1 + 额外 12500*2*ratio(1)
	if got := q.calcQuota(100, 0, 1, 0, 1); got != 25100 {
		t.Fatalf("ratio=1: calcQuota = %d, want 25100", got)
	}
	// ratio=0.5：额外部分 ceil(25000*0.5)=12500
	if got := q.calcQuota(100, 0, 1, 0, 0.5); got != 12600 {
		t.Fatalf("ratio=0.5: calcQuota = %d, want 12600", got)
	}
}

// TestCalcQuotaEmptyResponseGate 空回复计费闸：EmptyResponseBillingEnabled=false 且
// completionTokens==0 时 token 计价归零；times 计价不受影响（按次语义成功即全额收）。
func TestCalcQuotaEmptyResponseGate(t *testing.T) {
	old := config.EmptyResponseBillingEnabled
	defer func() { config.EmptyResponseBillingEnabled = old }()

	config.EmptyResponseBillingEnabled = false
	qTokens := &Quota{price: model.Price{Type: model.TokensPriceType}}
	if got := qTokens.calcQuota(100, 0, 1, 1, 1); got != 0 {
		t.Fatalf("tokens 空回复应归零, got %d", got)
	}
	qTimes := &Quota{price: model.Price{Type: model.TimesPriceType}}
	if got := qTimes.calcQuota(10, 0, 0.05, 0, 1); got != 50 {
		t.Fatalf("times 不受空回复闸影响, got %d, want 50", got)
	}

	config.EmptyResponseBillingEnabled = true
	if got := qTokens.calcQuota(100, 0, 1, 1, 1); got != 100 {
		t.Fatalf("开关开启时应正常计费, got %d, want 100", got)
	}
}

// ---------- 3. token 折算与总额/成本计算 ----------

// TestGetComputeTokensByUsage 缓存读/写折算：extra token 按 (ratio-1) 增减对应侧
// （cached_read 0.1x → -0.9*n；cached_write 1.25x → +0.25*n）。
func TestGetComputeTokensByUsage(t *testing.T) {
	q := &Quota{price: model.Price{Type: model.TokensPriceType, Input: 1, Output: 1}}
	usage := &types.Usage{PromptTokens: 10000, CompletionTokens: 1000}
	usage.PromptTokensDetails.CachedReadTokens = 1000
	usage.PromptTokensDetails.CachedWriteTokens = 1000

	pt, ct := q.getComputeTokensByUsage(usage)
	if pt != 9350 { // 10000 - 900 + 250
		t.Fatalf("promptTokens = %d, want 9350", pt)
	}
	if ct != 1000 {
		t.Fatalf("completionTokens = %d, want 1000", ct)
	}
}

// TestGetComputeTokensByUsageAnthropicFallback 顶层扁平缓存字段回退：
// cache_read_input_tokens 在 prompt_tokens_details 为空时计入 CachedReadTokens（0.1x 折算）。
func TestGetComputeTokensByUsageAnthropicFallback(t *testing.T) {
	q := &Quota{price: model.Price{Type: model.TokensPriceType}}
	usage := &types.Usage{PromptTokens: 10000, CompletionTokens: 100, CacheReadInputTokens: 1000}
	pt, _ := q.getComputeTokensByUsage(usage)
	if pt != 9100 { // 10000 - 900
		t.Fatalf("promptTokens = %d, want 9100", pt)
	}
}

// TestGetComputeTokensByUsageEvent 实时事件路径的折算（OpenAI cache write 1.25x → +0.25*n）。
func TestGetComputeTokensByUsageEvent(t *testing.T) {
	q := &Quota{price: model.Price{Type: model.TokensPriceType}}
	usage := &types.UsageEvent{InputTokens: 1000, OutputTokens: 100}
	usage.InputTokenDetails.OpenAICacheWriteTokens = 400

	pt, ct := q.getComputeTokensByUsageEvent(usage)
	if pt != 1100 { // 1000 + 400*0.25
		t.Fatalf("promptTokens = %d, want 1100", pt)
	}
	if ct != 100 {
		t.Fatalf("completionTokens = %d, want 100", ct)
	}
}

// TestGetTotalQuotaByUsageLongContext 长上下文分档：按原始输入 token 判档，
// 超阈值时整次请求套用分档倍率；未超阈值 (1,1)。
func TestGetTotalQuotaByUsageLongContext(t *testing.T) {
	lc := datatypes.NewJSONType(model.LongContextTier{Threshold: 1000, InputRatio: 2, OutputRatio: 1.5})
	q := &Quota{
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1, LongContext: &lc},
		inputRatio: 1, outputRatio: 1, groupRatio: 1,
	}

	// 超阈值：2000*1*2 + 1000*1*1.5 = 5500
	over := &types.Usage{PromptTokens: 2000, CompletionTokens: 1000}
	if got := q.GetTotalQuotaByUsage(over); got != 5500 {
		t.Fatalf("超阈值 quota = %d, want 5500", got)
	}
	// 边界（等于阈值不触发）：1000 + 1000 = 2000
	at := &types.Usage{PromptTokens: 1000, CompletionTokens: 1000}
	if got := q.GetTotalQuotaByUsage(at); got != 2000 {
		t.Fatalf("等于阈值 quota = %d, want 2000", got)
	}
}

// TestGetTotalQuotaByUsageExtraBilling GetTotalQuotaByUsage 会从 usage.ExtraBilling
// 组装 extraBillingData（价格按模型 tier 查默认价表）并计入总额。
func TestGetTotalQuotaByUsageExtraBilling(t *testing.T) {
	q := &Quota{
		modelName: "gpt-4o", // high_tier → web search 0.025$/次 → 12500/次
		price:     model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, outputRatio: 1, groupRatio: 1,
	}
	usage := &types.Usage{PromptTokens: 100, CompletionTokens: 0}
	usage.ExtraBilling = map[string]types.ExtraBilling{
		types.APITollTypeWebSearchPreview: {Type: "", CallCount: 2},
	}
	if got := q.GetTotalQuotaByUsage(usage); got != 25100 {
		t.Fatalf("quota = %d, want 25100", got)
	}
}

// TestGetCostQuotaByUsage 成本配额：costRatio<=0 → 0；否则用 price 原始倍率*costRatio。
func TestGetCostQuotaByUsage(t *testing.T) {
	usage := &types.Usage{PromptTokens: 1000, CompletionTokens: 500}

	q := &Quota{price: model.Price{Type: model.TokensPriceType, Input: 2, Output: 4}, costRatio: 0}
	if got := q.GetCostQuotaByUsage(usage); got != 0 {
		t.Fatalf("costRatio=0 应返回 0, got %d", got)
	}

	q = &Quota{price: model.Price{Type: model.TokensPriceType, Input: 2, Output: 4}, costRatio: 0.5}
	// 1000*(2*0.5) + 500*(4*0.5) = 1000 + 1000
	if got := q.GetCostQuotaByUsage(usage); got != 2000 {
		t.Fatalf("costQuota = %d, want 2000", got)
	}
}

// TestGetExtraBillingData 默认额外服务价表：web search 按模型 tier，image generation
// 按 quality-size 组合，未知/畸形输入回 0；空 map 不设置 extraBillingData。
func TestGetExtraBillingData(t *testing.T) {
	q := &Quota{modelName: "gpt-4o-mini"} // gpt-4o 前缀 → high_tier
	q.GetExtraBillingData(map[string]types.ExtraBilling{
		types.APITollTypeWebSearchPreview: {Type: "", CallCount: 1},
	})
	if q.extraBillingData[types.APITollTypeWebSearchPreview].Price != 0.025 {
		t.Fatalf("gpt-4o* web search 应为 0.025, got %v", q.extraBillingData)
	}

	q = &Quota{modelName: "gpt-3.5-turbo"}
	q.GetExtraBillingData(map[string]types.ExtraBilling{
		types.APITollTypeWebSearchPreview: {Type: "", CallCount: 1},
		types.APITollTypeFileSearch:       {Type: "", CallCount: 1},
		types.APITollTypeCodeInterpreter:  {Type: "", CallCount: 1},
		types.APITollTypeImageGeneration:  {Type: "high-1024x1024", CallCount: 1},
	})
	want := map[string]float64{
		types.APITollTypeWebSearchPreview: 0.01, // standard tier
		types.APITollTypeFileSearch:       0.0025,
		types.APITollTypeCodeInterpreter:  0.03,
		types.APITollTypeImageGeneration:  0.167,
	}
	for k, w := range want {
		if got := q.extraBillingData[k].Price; got != w {
			t.Fatalf("%s price = %v, want %v", k, got, w)
		}
	}

	// 畸形 image type → 0
	q = &Quota{modelName: "gpt-image-1"}
	q.GetExtraBillingData(map[string]types.ExtraBilling{
		types.APITollTypeImageGeneration: {Type: "not-a-valid", CallCount: 1},
	})
	if got := q.extraBillingData[types.APITollTypeImageGeneration].Price; got != 0 {
		t.Fatalf("畸形 image type price = %v, want 0", got)
	}

	// 空 map → 不设置
	q = &Quota{}
	q.GetExtraBillingData(map[string]types.ExtraBilling{})
	if q.extraBillingData != nil {
		t.Fatal("空 map 不应设置 extraBillingData")
	}
	q.GetExtraBillingData(nil)
	if q.extraBillingData != nil {
		t.Fatal("nil 不应设置 extraBillingData")
	}
}

// ---------- 2. PreQuotaConsumption 决策矩阵 ----------

// mockGuardrailFns 替换守护注入点，测试结束后还原。
func mockGuardrailFns(t *testing.T, guardrail *model.OrgMemberGuardrail, reserveOk bool) {
	t.Helper()
	origGet, origReserve := cacheGetOrgMemberGuardrailFn, reserveOrgMemberBudgetFn
	t.Cleanup(func() {
		cacheGetOrgMemberGuardrailFn, reserveOrgMemberBudgetFn = origGet, origReserve
	})
	cacheGetOrgMemberGuardrailFn = func(shadowUserId, memberUserId int) (*model.OrgMemberGuardrail, error) {
		return guardrail, nil
	}
	reserveOrgMemberBudgetFn = func(memberId, amount int) (bool, error) {
		return reserveOk, nil
	}
}

// TestPreQuotaConsumptionUnpricedReject unpriced block 策略：403 零费用拒绝，不触达 DB。
func TestPreQuotaConsumptionUnpricedReject(t *testing.T) {
	q := &Quota{unpricedReject: true, modelName: "unknown-model"}
	errResp := q.PreQuotaConsumption()
	if errResp == nil || errResp.StatusCode != http.StatusForbidden {
		t.Fatalf("应 403 拒绝: %+v", errResp)
	}
	if errResp.OpenAIError.Code != "model_price_not_configured" {
		t.Fatalf("Code = %v, want model_price_not_configured", errResp.OpenAIError.Code)
	}
	if !errResp.LocalError || q.HandelStatus || q.preConsumedQuota != 0 {
		t.Fatalf("应零费用 LocalError 拒绝: local=%v handel=%v pre=%d",
			errResp.LocalError, q.HandelStatus, q.preConsumedQuota)
	}
}

// TestPreQuotaConsumptionZeroPrice 零价格模型（zero 策略放行后）：pre=0 直接放行，不触达 DB。
func TestPreQuotaConsumptionZeroPrice(t *testing.T) {
	q := &Quota{price: model.Price{Type: model.TokensPriceType, Input: 0, Output: 0}}
	if errResp := q.PreQuotaConsumption(); errResp != nil {
		t.Fatalf("零价格应放行: %+v", errResp)
	}
	if q.preConsumedQuota != 0 || q.HandelStatus {
		t.Fatalf("不应产生预扣: pre=%d handel=%v", q.preConsumedQuota, q.HandelStatus)
	}
}

// TestPreQuotaConsumption100xSkip 个人令牌 userQuota > 100*pre → 跳过预扣（低延迟优化）。
func TestPreQuotaConsumption100xSkip(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 1, 100000, 0)

	q := &Quota{
		userId: 1, tokenId: tokenId, promptTokens: 100,
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, groupRatio: 1,
	}
	if errResp := q.PreQuotaConsumption(); errResp != nil {
		t.Fatalf("应放行: %+v", errResp)
	}
	// pre = 100*1 + PreConsumedQuota(500) = 600；100000 > 60000 → 跳过并清零
	if q.preConsumedQuota != 0 || q.HandelStatus {
		t.Fatalf("100x 应跳过预扣: pre=%d handel=%v", q.preConsumedQuota, q.HandelStatus)
	}
	if got := getUserQuota(t, 1); got != 100000 {
		t.Fatalf("用户额度不应变化: %d", got)
	}
}

// TestPreQuotaConsumptionBudgetMemberNoSkip 预算成员不走 100x 跳过（SEC-9）：
// 逐请求原子预留 + 保持 preConsumedQuota 非零 + 真实预扣额度池。
func TestPreQuotaConsumptionBudgetMemberNoSkip(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 900, 100000, 100000)
	guardrail := &model.OrgMemberGuardrail{
		MemberId: 7,
		Budget:   &model.QuotaResetSetting{Period: "daily", Limit: 100000},
	}
	mockGuardrailFns(t, guardrail, true)

	q := &Quota{
		userId: 900, createdBy: 5, tokenId: tokenId, promptTokens: 100,
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, groupRatio: 1,
	}
	if errResp := q.PreQuotaConsumption(); errResp != nil {
		t.Fatalf("应放行: %+v", errResp)
	}
	if q.preConsumedQuota != 600 || !q.HandelStatus {
		t.Fatalf("预算成员应保持预扣: pre=%d handel=%v", q.preConsumedQuota, q.HandelStatus)
	}
	if got := getUserQuota(t, 900); got != 99400 {
		t.Fatalf("影子账户额度应预扣 600: %d", got)
	}
	tok := getToken(t, tokenId)
	if tok.RemainQuota != 99400 || tok.UsedQuota != 600 {
		t.Fatalf("令牌额度应预扣: remain=%d used=%d", tok.RemainQuota, tok.UsedQuota)
	}
}

// TestPreQuotaConsumptionInsufficientQuota 池不足两种错误：个人 insufficient_user_quota，
// 组织令牌 insufficient_org_quota（影子账户即组织积分池）。
func TestPreQuotaConsumptionInsufficientQuota(t *testing.T) {
	setupQuotaTestDB(t)
	seedUserAndToken(t, 1, 100, 0)     // 个人：100 < pre(600)
	seedUserAndToken(t, 900, 100, 0)   // 影子账户：100 < pre(600)
	mockGuardrailFns(t, &model.OrgMemberGuardrail{}, true) // 组织成员无预算限制

	newQ := func(userId, createdBy int) *Quota {
		return &Quota{
			userId: userId, createdBy: createdBy, promptTokens: 100,
			price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
			inputRatio: 1, groupRatio: 1,
		}
	}

	errResp := newQ(1, 0).PreQuotaConsumption()
	if errResp == nil || errResp.StatusCode != http.StatusPaymentRequired ||
		errResp.OpenAIError.Code != "insufficient_user_quota" {
		t.Fatalf("个人池不足应 402 insufficient_user_quota: %+v", errResp)
	}

	errResp = newQ(900, 5).PreQuotaConsumption()
	if errResp == nil || errResp.StatusCode != http.StatusPaymentRequired ||
		errResp.OpenAIError.Code != "insufficient_org_quota" {
		t.Fatalf("组织池不足应 402 insufficient_org_quota: %+v", errResp)
	}
}

// TestPreQuotaConsumptionReserveRefundOnPoolFailure 预留成功但池预扣失败 →
// 回冲刚才的成员预算预留（budget_used 减回），403 pre_consume_token_quota_failed。
func TestPreQuotaConsumptionReserveRefundOnPoolFailure(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 900, 100000, 0) // 令牌余额 0 → PreConsumeTokenQuota 失败

	member := &model.OrganizationMember{OrganizationId: 1, UserId: 5, Role: "member"}
	member.Setting.Set(model.OrgMemberSetting{
		Budget: &model.QuotaResetSetting{Period: "daily", Limit: 100000},
	})
	if err := model.DB.Create(member).Error; err != nil {
		t.Fatalf("创建成员失败: %v", err)
	}
	// 伪造本周期已用 700（budget_start=当前时间，处于本周期内）
	if err := model.DB.Model(&model.OrganizationMember{}).Where("id = ?", member.Id).
		UpdateColumns(map[string]interface{}{"budget_used": 700, "budget_start": time.Now().Unix()}).Error; err != nil {
		t.Fatalf("写入伪造用量失败: %v", err)
	}

	guardrail := &model.OrgMemberGuardrail{
		MemberId: member.Id,
		Budget:   &model.QuotaResetSetting{Period: "daily", Limit: 100000},
	}
	mockGuardrailFns(t, guardrail, true)

	q := &Quota{
		userId: 900, createdBy: 5, tokenId: tokenId, promptTokens: 100,
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, groupRatio: 1,
	}
	errResp := q.PreQuotaConsumption()
	if errResp == nil || errResp.StatusCode != http.StatusForbidden ||
		errResp.OpenAIError.Code != "pre_consume_token_quota_failed" {
		t.Fatalf("池预扣失败应 403 pre_consume_token_quota_failed: %+v", errResp)
	}
	// 回冲：700 - 600 = 100
	var got model.OrganizationMember
	if err := model.DB.First(&got, "id = ?", member.Id).Error; err != nil {
		t.Fatalf("读取成员失败: %v", err)
	}
	if got.BudgetUsed != 100 {
		t.Fatalf("预算应回冲至 100, got %d", got.BudgetUsed)
	}
	if q.HandelStatus {
		t.Fatal("预扣失败不应置 HandelStatus")
	}
}

// mockOrgBudgetReserveFn 替换团队预算预留注入点，测试结束后还原。
func mockOrgBudgetReserveFn(t *testing.T, ok bool, err error) {
	t.Helper()
	orig := reserveOrganizationBudgetFn
	t.Cleanup(func() { reserveOrganizationBudgetFn = orig })
	reserveOrganizationBudgetFn = func(orgId, amount int) (bool, error) {
		return ok, err
	}
}

// TestPreQuotaConsumptionOrgBudgetExceeded 成员预算未超但团队预算已超（T1）：
// 402 org_budget_exceeded 零费用拒绝，成员预算预留被回冲，积分池与令牌均未预扣。
func TestPreQuotaConsumptionOrgBudgetExceeded(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 900, 100000, 100000)

	member := &model.OrganizationMember{OrganizationId: 1, UserId: 5, Role: "member"}
	member.Setting.Set(model.OrgMemberSetting{
		Budget: &model.QuotaResetSetting{Period: "daily", Limit: 100000},
	})
	if err := model.DB.Create(member).Error; err != nil {
		t.Fatalf("创建成员失败: %v", err)
	}
	if err := model.DB.Model(&model.OrganizationMember{}).Where("id = ?", member.Id).
		UpdateColumns(map[string]interface{}{"budget_used": 700, "budget_start": time.Now().Unix()}).Error; err != nil {
		t.Fatalf("写入伪造用量失败: %v", err)
	}

	guardrail := &model.OrgMemberGuardrail{
		OrganizationId: 1,
		MemberId:       member.Id,
		Budget:         &model.QuotaResetSetting{Period: "daily", Limit: 100000},
		OrgBudget:      &model.QuotaResetSetting{Period: "daily", Limit: 1000},
	}
	mockGuardrailFns(t, guardrail, true) // 成员预算预留成功
	mockOrgBudgetReserveFn(t, false, nil)

	q := &Quota{
		userId: 900, createdBy: 5, tokenId: tokenId, promptTokens: 100,
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, groupRatio: 1,
	}
	errResp := q.PreQuotaConsumption()
	if errResp == nil || errResp.StatusCode != http.StatusPaymentRequired ||
		errResp.OpenAIError.Code != "org_budget_exceeded" {
		t.Fatalf("团队预算超限应 402 org_budget_exceeded: %+v", errResp)
	}
	// 成员预算回冲：700 - 600 = 100
	var got model.OrganizationMember
	if err := model.DB.First(&got, "id = ?", member.Id).Error; err != nil {
		t.Fatalf("读取成员失败: %v", err)
	}
	if got.BudgetUsed != 100 {
		t.Fatalf("成员预算应回冲至 100, got %d", got.BudgetUsed)
	}
	if q.HandelStatus {
		t.Fatal("拒绝不应置 HandelStatus")
	}
	if quota := getUserQuota(t, 900); quota != 100000 {
		t.Fatalf("拒绝必须零费用，影子账户额度 = %d", quota)
	}
	if tok := getToken(t, tokenId); tok.RemainQuota != 100000 || tok.UsedQuota != 0 {
		t.Fatalf("拒绝不应预扣令牌: remain=%d used=%d", tok.RemainQuota, tok.UsedQuota)
	}
}

// TestPreQuotaConsumptionOrgBudgetUnavailable 团队预算预留 DB 失败 → fail-closed 503（同 SEC-10）。
func TestPreQuotaConsumptionOrgBudgetUnavailable(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 900, 100000, 100000)
	guardrail := &model.OrgMemberGuardrail{
		OrganizationId: 1,
		OrgBudget:      &model.QuotaResetSetting{Period: "daily", Limit: 1000},
	}
	mockGuardrailFns(t, guardrail, true)
	mockOrgBudgetReserveFn(t, false, errors.New("db down"))

	q := &Quota{
		userId: 900, createdBy: 5, tokenId: tokenId, promptTokens: 100,
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, groupRatio: 1,
	}
	errResp := q.PreQuotaConsumption()
	if errResp == nil || errResp.StatusCode != http.StatusServiceUnavailable ||
		errResp.OpenAIError.Code != "org_guardrail_unavailable" {
		t.Fatalf("团队预算预留失败应 503 org_guardrail_unavailable: %+v", errResp)
	}
	if q.HandelStatus {
		t.Fatal("拒绝不应置 HandelStatus")
	}
}

// ---------- 4. UpdateUserRealtimeQuota ----------

// TestUpdateUserRealtimeQuotaRedisDisabled Redis 关闭：仅 Merge 累计用量后 noop 返回 nil。
// 长上下文收敛语义：分档判据是 Merge 后的累计 InputTokens（见 GetLongContextMultiplier 用例）。
func TestUpdateUserRealtimeQuotaRedisDisabled(t *testing.T) {
	if config.RedisEnabled {
		t.Skip("Redis enabled in test env")
	}
	q := &Quota{price: model.Price{Type: model.TokensPriceType, Input: 1, Output: 1}}
	usage := &types.UsageEvent{InputTokens: 900, OutputTokens: 10, TotalTokens: 910}
	nowUsage := &types.UsageEvent{InputTokens: 300, OutputTokens: 20, TotalTokens: 320}

	if err := q.UpdateUserRealtimeQuota(usage, nowUsage); err != nil {
		t.Fatalf("Redis 关闭应 noop 返回 nil: %v", err)
	}
	// Merge 先于 Redis 闸执行：累计输入 1200（分档判据用它，而非单次增量 300）
	if usage.InputTokens != 1200 || usage.OutputTokens != 30 || usage.TotalTokens != 1230 {
		t.Fatalf("Merge 应累计: %+v", usage)
	}
	if q.cacheQuota != 0 {
		t.Fatalf("Redis 关闭不应累计 cacheQuota: %d", q.cacheQuota)
	}
}

// ---------- 5. 结算 / 回退 ----------

// TestCompletedQuotaConsumptionSettlement 结算：quotaDelta = 实际 - 预扣（负数回冲），
// 同步落消费日志、渠道用量、用户 used_quota/request_count。
func TestCompletedQuotaConsumptionSettlement(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 1, 10000, 5000)
	channel := &model.Channel{Name: "quota-test-channel"}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatalf("创建渠道失败: %v", err)
	}

	q := &Quota{
		userId: 1, tokenId: tokenId, channelId: channel.Id,
		modelName:  "gpt-4o",
		price:      model.Price{Type: model.TokensPriceType, Input: 1, Output: 1},
		inputRatio: 1, outputRatio: 1, groupRatio: 1,
		preConsumedQuota: 600, HandelStatus: true,
		startTime: time.Now(),
	}
	usage := &types.Usage{PromptTokens: 100, CompletionTokens: 100, TotalTokens: 200}
	if err := q.completedQuotaConsumption(usage, "tok", false, "127.0.0.1", context.Background(), nil); err != nil {
		t.Fatalf("结算失败: %v", err)
	}

	// 实际 200，预扣 600 → 回冲 400
	if got := getUserQuota(t, 1); got != 10400 {
		t.Fatalf("用户额度应回冲 400: %d", got)
	}
	tok := getToken(t, tokenId)
	if tok.RemainQuota != 5400 || tok.UsedQuota != -400 {
		t.Fatalf("令牌应回冲: remain=%d used=%d", tok.RemainQuota, tok.UsedQuota)
	}
	var user model.User
	if err := model.DB.First(&user, "id = ?", 1).Error; err != nil {
		t.Fatalf("读取用户失败: %v", err)
	}
	if user.UsedQuota != 200 || user.RequestCount != 1 {
		t.Fatalf("used_quota=%d request_count=%d, want 200/1", user.UsedQuota, user.RequestCount)
	}
	var ch model.Channel
	if err := model.DB.First(&ch, "id = ?", channel.Id).Error; err != nil {
		t.Fatalf("读取渠道失败: %v", err)
	}
	if ch.UsedQuota != 200 {
		t.Fatalf("渠道用量 = %d, want 200", ch.UsedQuota)
	}
	var logRow model.Log
	if err := model.DB.First(&logRow, "user_id = ? AND type = ?", 1, model.LogTypeConsume).Error; err != nil {
		t.Fatalf("消费日志应写入: %v", err)
	}
	if logRow.Quota != 200 || logRow.ModelName != "gpt-4o" {
		t.Fatalf("日志 quota=%d model=%s, want 200/gpt-4o", logRow.Quota, logRow.ModelName)
	}
}

// TestUndo 回退预扣：HandelStatus=false 时 noop；true 时全额退回用户与令牌额度。
func TestUndo(t *testing.T) {
	setupQuotaTestDB(t)
	tokenId := seedUserAndToken(t, 1, 9400, 4400) // 已预扣 600 后的状态

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/", nil)

	// HandelStatus=false → noop
	q := &Quota{userId: 1, tokenId: tokenId, preConsumedQuota: 600, HandelStatus: false}
	q.Undo(c)
	if got := getUserQuota(t, 1); got != 9400 {
		t.Fatalf("未预扣不应回退: %d", got)
	}

	// HandelStatus=true → 退回 600
	q.HandelStatus = true
	q.Undo(c)
	if got := getUserQuota(t, 1); got != 10000 {
		t.Fatalf("应退回 600: %d", got)
	}
	tok := getToken(t, tokenId)
	if tok.RemainQuota != 5000 || tok.UsedQuota != -600 {
		t.Fatalf("令牌应退回: remain=%d used=%d", tok.RemainQuota, tok.UsedQuota)
	}
}
