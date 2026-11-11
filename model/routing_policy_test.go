package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupRoutingPolicyTestDB 建一个含策略表与目录表的内存库，并清空两份进程内索引，
// 让按名查找走真实的 DB 回落路径。
func setupRoutingPolicyTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&RoutingPolicy{}, &ModelInfo{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := DB
	DB = testDB
	ResetRoutingPolicyCache()
	modelInfoCache.Lock()
	oldInfoLoaded := modelInfoCache.loaded
	modelInfoCache.loaded = false
	modelInfoCache.Unlock()
	t.Cleanup(func() {
		DB = oldDB
		ResetRoutingPolicyCache()
		modelInfoCache.Lock()
		modelInfoCache.loaded = oldInfoLoaded
		modelInfoCache.Unlock()
	})
	return testDB
}

func samplePolicy() *RoutingPolicy {
	return &RoutingPolicy{
		Name:          "auto/coder",
		DisplayName:   "Auto coder",
		Enabled:       true,
		DecisionModel: "claude-haiku-5-5",
		Question:      "How large is the change the user asks for?",
		Routes: []RoutingRoute{
			{Label: "small", Description: "additive change", TargetModel: "claude-haiku-5-5"},
			{Label: "medium", Description: "behaviour change in one place", TargetModel: "claude-sonnet-5-5"},
			{Label: "large", Description: "behaviour change across modules", TargetModel: "claude-opus-5-5"},
		},
		FallbackModel: "claude-opus-5-5",
		Threshold:     0.5,
		PinTTLSeconds: 3600,
	}
}

func apiCode(t *testing.T, err error) string {
	t.Helper()
	var apiErr *common.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("期望带 code 的 APIError，实际: %v", err)
	}
	return apiErr.Code
}

// TestValidateRoutingPolicyName 策略名会进 URL 与选择器，只放行字母数字与 . _ / -，不能以 / 开头或结尾
func TestValidateRoutingPolicyName(t *testing.T) {
	valid := []string{"auto", "auto/coder", "auto/coder-v2", "Auto.Coder_1", "a/b/c", strings.Repeat("a", 100)}
	for _, name := range valid {
		if err := ValidateRoutingPolicyName(name); err != nil {
			t.Fatalf("name %q 应合法，实际: %v", name, err)
		}
	}
	invalid := []string{"", "/auto", "auto/", "auto coder", "auto:coder", "自动", strings.Repeat("a", 101)}
	for _, name := range invalid {
		if err := ValidateRoutingPolicyName(name); err == nil {
			t.Fatalf("name %q 应被拒绝", name)
		}
	}
}

// TestRoutingPolicyValidate 校验覆盖：路由数、标签重复（忽略大小写）、目标 / 兜底 / 决策模型不能是策略、阈值与上限
func TestRoutingPolicyValidate(t *testing.T) {
	isPolicy := func(name string) bool { return name == "auto/other" }

	cases := []struct {
		name   string
		mutate func(p *RoutingPolicy)
		code   string
	}{
		{"too few routes", func(p *RoutingPolicy) { p.Routes = p.Routes[:1] }, common.APICodeRoutingPolicyRoutesCount},
		{"duplicate label ignoring case", func(p *RoutingPolicy) { p.Routes[1].Label = "SMALL" }, common.APICodeRoutingPolicyLabelDuplicate},
		{"empty label", func(p *RoutingPolicy) { p.Routes[0].Label = "  " }, common.APICodeRoutingPolicyLabelInvalid},
		{"target is policy", func(p *RoutingPolicy) { p.Routes[0].TargetModel = "auto/other" }, common.APICodeRoutingPolicyModelIsPolicy},
		{"target is self", func(p *RoutingPolicy) { p.Routes[0].TargetModel = "auto/coder" }, common.APICodeRoutingPolicyModelIsPolicy},
		{"fallback is policy", func(p *RoutingPolicy) { p.FallbackModel = "auto/other" }, common.APICodeRoutingPolicyModelIsPolicy},
		{"decision is policy", func(p *RoutingPolicy) { p.DecisionModel = "auto/other" }, common.APICodeRoutingPolicyModelIsPolicy},
		{"missing fallback", func(p *RoutingPolicy) { p.FallbackModel = "" }, common.APICodeRoutingPolicyModelRequired},
		{"missing target", func(p *RoutingPolicy) { p.Routes[2].TargetModel = "" }, common.APICodeRoutingPolicyModelRequired},
		{"threshold above one", func(p *RoutingPolicy) { p.Threshold = 1.5 }, common.APICodeRoutingPolicyThresholdInvalid},
		{"negative ttl", func(p *RoutingPolicy) { p.PinTTLSeconds = -1 }, common.APICodeRoutingPolicyLimitInvalid},
		{"input chars too large", func(p *RoutingPolicy) { p.MaxInputChars = RoutingPolicyMaxInputChars + 1 }, common.APICodeRoutingPolicyLimitInvalid},
		{"bad name", func(p *RoutingPolicy) { p.Name = "auto coder" }, common.APICodeRoutingPolicyNameInvalid},
		{"unknown scoring", func(p *RoutingPolicy) { p.Scoring = "vibes" }, common.APICodeRoutingPolicyScoringInvalid},
	}
	for _, tc := range cases {
		p := samplePolicy()
		tc.mutate(p)
		p.Normalize()
		err := p.Validate(isPolicy)
		if err == nil {
			t.Fatalf("%s: 应被拒绝", tc.name)
		}
		if got := apiCode(t, err); got != tc.code {
			t.Fatalf("%s: code 期望 %s，实际 %s", tc.name, tc.code, got)
		}
	}

	p := samplePolicy()
	p.Normalize()
	if err := p.Validate(isPolicy); err != nil {
		t.Fatalf("合法策略应通过，实际: %v", err)
	}
	if p.Scoring != RoutingScoringAuto {
		t.Fatalf("Normalize 应把空打分方式补为 auto，实际 %q", p.Scoring)
	}
	if p.MaxInputChars != RoutingPolicyDefaultInputChars || p.DecisionTimeoutMs != RoutingPolicyDefaultTimeoutMs {
		t.Fatalf("Normalize 应补默认值，实际 chars=%d timeout=%d", p.MaxInputChars, p.DecisionTimeoutMs)
	}
	if p.PinTTLSeconds = 0; p.Validate(isPolicy) != nil {
		t.Fatalf("pin_ttl_seconds=0（每次分类）应合法")
	}
}

// TestRoutingPolicyTargetModels 目标列表去重且含兜底，保持出现顺序
func TestRoutingPolicyTargetModels(t *testing.T) {
	p := samplePolicy()
	got := p.TargetModels()
	want := []string{"claude-haiku-5-5", "claude-sonnet-5-5", "claude-opus-5-5"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("TargetModels 期望 %v，实际 %v", want, got)
	}
	if r := p.RouteFor("MEDIUM"); r == nil || r.TargetModel != "claude-sonnet-5-5" {
		t.Fatalf("RouteFor 应忽略大小写命中 medium，实际 %+v", r)
	}
	if p.RouteFor("none") != nil {
		t.Fatalf("RouteFor 未知标签应返回 nil")
	}
}

// TestRoutingPolicyInsertCreatesCatalogRow 建策略联动建目录行：kind=routing_policy、locked、名字与描述同步
func TestRoutingPolicyInsertCreatesCatalogRow(t *testing.T) {
	setupRoutingPolicyTestDB(t)

	p := samplePolicy()
	p.Description = "routes by change size"
	if err := p.Insert(); err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	if p.Id == 0 {
		t.Fatalf("Insert 后应有 id")
	}
	row, err := GetModelInfoByModel("auto/coder")
	if err != nil {
		t.Fatalf("应有联动目录行: %v", err)
	}
	if row.Kind != ModelInfoKindRoutingPolicy || !row.Locked || row.Name != "Auto coder" || row.Description != "routes by change size" {
		t.Fatalf("目录行字段不对: %+v", row)
	}
	if row.Mode != ModelModeChat {
		t.Fatalf("目录行 mode 应为 chat，实际 %q", row.Mode)
	}
	if !IsRoutingPolicyModel("auto/coder") || IsRoutingPolicyModel("claude-opus-5-5") {
		t.Fatalf("IsRoutingPolicyModel 判定不对")
	}
	if got := LookupRoutingPolicy("auto/coder"); got == nil || got.Id != p.Id || len(got.Routes) != 3 {
		t.Fatalf("LookupRoutingPolicy 应读回含路由的策略，实际 %+v", got)
	}
}

// TestRoutingPolicyInsertRejectsTakenName 名字被目录模型或另一条策略占用时拒绝，不留半截目录行
func TestRoutingPolicyInsertRejectsTakenName(t *testing.T) {
	db := setupRoutingPolicyTestDB(t)

	if err := db.Create(&ModelInfo{Model: "gpt-5", Name: "GPT-5"}).Error; err != nil {
		t.Fatalf("建目录行失败: %v", err)
	}
	p := samplePolicy()
	p.Name = "gpt-5"
	err := p.Insert()
	if err == nil || apiCode(t, err) != common.APICodeRoutingPolicyNameTaken {
		t.Fatalf("与目录模型重名应被拒绝，实际: %v", err)
	}

	first := samplePolicy()
	if err := first.Insert(); err != nil {
		t.Fatalf("第一条 Insert 失败: %v", err)
	}
	second := samplePolicy()
	err = second.Insert()
	if err == nil || apiCode(t, err) != common.APICodeRoutingPolicyNameTaken {
		t.Fatalf("与策略重名应被拒绝，实际: %v", err)
	}
	var count int64
	db.Model(&ModelInfo{}).Where("model = ?", "auto/coder").Count(&count)
	if count != 1 {
		t.Fatalf("目录里应只有一行 auto/coder，实际 %d", count)
	}
}

// TestRoutingPolicyInsertRejectsNestedPolicy 目标 / 兜底指向已存在的策略时拒绝（禁止嵌套）
func TestRoutingPolicyInsertRejectsNestedPolicy(t *testing.T) {
	setupRoutingPolicyTestDB(t)

	first := samplePolicy()
	if err := first.Insert(); err != nil {
		t.Fatalf("第一条 Insert 失败: %v", err)
	}
	second := samplePolicy()
	second.Name = "auto/outer"
	second.FallbackModel = "auto/coder"
	err := second.Insert()
	if err == nil || apiCode(t, err) != common.APICodeRoutingPolicyModelIsPolicy {
		t.Fatalf("兜底为策略应被拒绝，实际: %v", err)
	}
}

// TestRoutingPolicyUpdateRenamesCatalogRow 改名时目录行跟着改名；被别名引用的旧名不能改
func TestRoutingPolicyUpdateRenamesCatalogRow(t *testing.T) {
	db := setupRoutingPolicyTestDB(t)

	p := samplePolicy()
	if err := p.Insert(); err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	p.Name = "auto/coding"
	p.DisplayName = "Auto coding"
	p.Routes = append(p.Routes, RoutingRoute{Label: "refactor", TargetModel: "claude-opus-5-5"})
	if err := p.Update(); err != nil {
		t.Fatalf("Update 失败: %v", err)
	}
	if _, err := GetModelInfoByModel("auto/coder"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("旧名目录行应已改名，实际 err=%v", err)
	}
	row, err := GetModelInfoByModel("auto/coding")
	if err != nil || row.Kind != ModelInfoKindRoutingPolicy || row.Name != "Auto coding" {
		t.Fatalf("新名目录行不对: %+v err=%v", row, err)
	}
	if got := LookupRoutingPolicy("auto/coding"); got == nil || len(got.Routes) != 4 {
		t.Fatalf("更新后的路由数应为 4，实际 %+v", got)
	}

	// 别名指向策略名后，改名会让别名悬空：拒绝。
	if err := db.Create(&ModelInfo{Model: "coder", AliasOf: "auto/coding"}).Error; err != nil {
		t.Fatalf("建别名失败: %v", err)
	}
	p.Name = "auto/coder"
	err = p.Update()
	if err == nil || apiCode(t, err) != common.APICodeRoutingPolicyReferenced {
		t.Fatalf("被别名引用时改名应被拒绝，实际: %v", err)
	}
}

// TestRoutingPolicyDeleteRemovesCatalogRow 删除策略连带删目录行；被别名引用时拒绝
func TestRoutingPolicyDeleteRemovesCatalogRow(t *testing.T) {
	db := setupRoutingPolicyTestDB(t)

	p := samplePolicy()
	if err := p.Insert(); err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	if err := db.Create(&ModelInfo{Model: "coder", AliasOf: "auto/coder"}).Error; err != nil {
		t.Fatalf("建别名失败: %v", err)
	}
	err := DeleteRoutingPolicyById(p.Id)
	if err == nil || apiCode(t, err) != common.APICodeRoutingPolicyReferenced {
		t.Fatalf("被别名引用时删除应被拒绝，实际: %v", err)
	}
	if err := db.Where("model = ?", "coder").Delete(&ModelInfo{}).Error; err != nil {
		t.Fatalf("删别名失败: %v", err)
	}
	if err := DeleteRoutingPolicyById(p.Id); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if _, err := GetModelInfoByModel("auto/coder"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("目录行应已删除，实际 err=%v", err)
	}
	if LookupRoutingPolicy("auto/coder") != nil {
		t.Fatalf("删除后不应再查到策略")
	}
	if err := DeleteRoutingPolicyById(p.Id); err == nil || apiCode(t, err) != common.APICodeRoutingPolicyNotFound {
		t.Fatalf("重复删除应返回 not found，实际: %v", err)
	}
}

// TestRoutingPolicyCacheFollowsWrites 索引加载后，增删改与启停都立即反映到按名查找
func TestRoutingPolicyCacheFollowsWrites(t *testing.T) {
	setupRoutingPolicyTestDB(t)
	if err := ReloadRoutingPolicyCache(); err != nil {
		t.Fatalf("加载索引失败: %v", err)
	}

	p := samplePolicy()
	if err := p.Insert(); err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	if LookupRoutingPolicy("auto/coder") == nil {
		t.Fatalf("Insert 后索引应命中")
	}
	if names := policyNames(EnabledRoutingPolicies()); strings.Join(names, ",") != "auto/coder" {
		t.Fatalf("启用列表期望 [auto/coder]，实际 %v", names)
	}

	if _, err := SetRoutingPolicyEnabled(p.Id, false); err != nil {
		t.Fatalf("停用失败: %v", err)
	}
	if got := LookupRoutingPolicy("auto/coder"); got == nil || got.Enabled {
		t.Fatalf("停用后索引应仍命中且 enabled=false，实际 %+v", got)
	}
	if len(EnabledRoutingPolicies()) != 0 {
		t.Fatalf("停用后启用列表应为空")
	}

	if err := DeleteRoutingPolicyById(p.Id); err != nil {
		t.Fatalf("Delete 失败: %v", err)
	}
	if LookupRoutingPolicy("auto/coder") != nil {
		t.Fatalf("Delete 后索引不应命中")
	}
}

// TestRoutingPolicyModelsFor 只并入启用且兜底可路由的策略，已在列表里的不重复
func TestRoutingPolicyModelsFor(t *testing.T) {
	setupRoutingPolicyTestDB(t)

	reachable := samplePolicy()
	if err := reachable.Insert(); err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	unreachable := samplePolicy()
	unreachable.Name = "auto/elsewhere"
	unreachable.FallbackModel = "gpt-5"
	if err := unreachable.Insert(); err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	disabled := samplePolicy()
	disabled.Name = "auto/off"
	disabled.Enabled = false
	if err := disabled.Insert(); err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}

	routable := func(name string) bool { return name == "claude-opus-5-5" }
	got := RoutingPolicyModelsFor(PlatformRoutingScope, []string{"claude-opus-5-5"}, routable)
	if strings.Join(got, ",") != "auto/coder" {
		t.Fatalf("期望只并入 auto/coder，实际 %v", got)
	}
	got = RoutingPolicyModelsFor(PlatformRoutingScope, []string{"claude-opus-5-5", "auto/coder"}, routable)
	if len(got) != 0 {
		t.Fatalf("已在列表里的策略不应重复加入，实际 %v", got)
	}
}

func policyNames(policies []*RoutingPolicy) []string {
	names := make([]string, 0, len(policies))
	for _, p := range policies {
		names = append(names, p.Name)
	}
	return names
}

// TestRoutingPolicyZeroValuesRoundTrip 阈值 0 与 pin_ttl_seconds 0 是合法取值，落库后不能被默认值改写
func TestRoutingPolicyZeroValuesRoundTrip(t *testing.T) {
	setupRoutingPolicyTestDB(t)
	p := samplePolicy()
	p.Threshold = 0
	p.PinTTLSeconds = 0
	if err := p.Insert(); err != nil {
		t.Fatalf("Insert 失败: %v", err)
	}
	got := LookupRoutingPolicy(p.Name)
	if got == nil || got.Threshold != 0 || got.PinTTLSeconds != 0 {
		t.Fatalf("零值应原样读回，实际 %+v", got)
	}
	if got.MaxInputChars != RoutingPolicyDefaultInputChars || got.DecisionTimeoutMs != RoutingPolicyDefaultTimeoutMs {
		t.Fatalf("0 = 默认 的字段应在 Normalize 补值，实际 %+v", got)
	}
}
