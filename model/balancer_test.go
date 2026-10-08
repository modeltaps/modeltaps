package model

import (
	"path/filepath"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// TestModelSupportsChannelTypes 覆盖"模型在分组下是否存在类型匹配渠道"的判定，
// 用于区分"该协议根本不支持此模型"（配置类）与"渠道暂时不可用"（运行时）。
func TestModelSupportsChannelTypes(t *testing.T) {
	weight := uint(1)
	openaiChannel := &Channel{Id: 1, Type: config.ChannelTypeOpenAI, Weight: &weight, Status: config.ChannelStatusEnabled}
	geminiChannel := &Channel{Id: 2, Type: config.ChannelTypeGemini, Weight: &weight, Status: config.ChannelStatusEnabled}

	cc := &ChannelsChooser{
		Rule: map[string]map[string][][]int{
			"g": {
				"openai-only": {{1}},
				"mixed":       {{1}, {2}},
			},
		},
		Channels: map[int]*ChannelChoice{
			1: {Channel: openaiChannel},
			2: {Channel: geminiChannel},
		},
	}

	geminiTypes := []int{config.ChannelTypeGemini}

	tests := []struct {
		name  string
		group string
		model string
		types []int
		want  bool
	}{
		{"openai-only model via gemini protocol", "g", "openai-only", geminiTypes, false},
		{"mixed model has a gemini channel", "g", "mixed", geminiTypes, true},
		{"openai-only model via openai protocol", "g", "openai-only", []int{config.ChannelTypeOpenAI}, true},
		{"unknown model", "g", "nope", geminiTypes, false},
		{"unknown group", "other", "openai-only", geminiTypes, false},
		{"empty types", "g", "mixed", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cc.ModelSupportsChannelTypes(tt.group, tt.model, tt.types); got != tt.want {
				t.Fatalf("ModelSupportsChannelTypes(%q, %q, %v) = %v, want %v",
					tt.group, tt.model, tt.types, got, tt.want)
			}
		})
	}
}

// TestModelSupportsChannelTypes_MissingChannelEntry 验证 Rule 引用了 Channels 里不存在的渠道 id 时
// 不 panic 且按"无匹配"处理（防御数据库一致性被破坏的场景）。
func TestModelSupportsChannelTypes_MissingChannelEntry(t *testing.T) {
	cc := &ChannelsChooser{
		Rule: map[string]map[string][][]int{
			"g": {"m": {{99}}},
		},
		Channels: map[int]*ChannelChoice{},
	}
	if got := cc.ModelSupportsChannelTypes("g", "m", []int{config.ChannelTypeGemini}); got != false {
		t.Fatalf("expected false for dangling channel id, got %v", got)
	}
}

// TestLoad_NilPriority 覆盖渠道 priority 为 NULL 的分支：管理员 PUT /api/channel/ 不带 priority 时
// 该列会被写成 NULL，Load 过去在此解引用空指针 panic。现按 0 处理，与新建渠道默认值一致。
func TestLoad_NilPriority(t *testing.T) {
	setupBalancerTestDB(t)

	weight := uint(1)
	priority := int64(10)
	nullPriority := &Channel{
		Id: 1, Type: config.ChannelTypeOpenAI, Status: config.ChannelStatusEnabled,
		Group: "default", Models: "gpt-5", Weight: &weight,
	}
	prioritized := &Channel{
		Id: 2, Type: config.ChannelTypeOpenAI, Status: config.ChannelStatusEnabled,
		Group: "default", Models: "gpt-5", Weight: &weight, Priority: &priority,
	}
	if err := DB.Create([]*Channel{nullPriority, prioritized}).Error; err != nil {
		t.Fatalf("create channels: %v", err)
	}
	// 建表默认值会补 0，需显式置 NULL 才能复现 Select("*") 全量更新写入的空值。
	if err := DB.Exec("UPDATE channels SET priority = NULL WHERE id = ?", nullPriority.Id).Error; err != nil {
		t.Fatalf("null out priority: %v", err)
	}

	cc := &ChannelsChooser{}
	cc.Load()

	got := cc.Rule["default"]["gpt-5"]
	// 优先级从大到小：priority=10 的渠道排在按 0 处理的空值渠道之前。
	if len(got) != 2 || len(got[0]) != 1 || got[0][0] != 2 || len(got[1]) != 1 || got[1][0] != 1 {
		t.Fatalf("空 priority 应按 0 参与排序, got %v", got)
	}
}

// TestLoad_NilWeight 覆盖渠道 weight 为 NULL 的分支：管理员 PUT /api/channel/ 不带 weight 时
// 该列会被写成 NULL，Load 与权重选路过去在此解引用空指针 panic。现按默认权重处理。
func TestLoad_NilWeight(t *testing.T) {
	setupBalancerTestDB(t)

	weight := uint(1)
	nullWeight := &Channel{
		Id: 1, Type: config.ChannelTypeOpenAI, Status: config.ChannelStatusEnabled,
		Group: "default", Models: "gpt-5",
	}
	weighted := &Channel{
		Id: 2, Type: config.ChannelTypeOpenAI, Status: config.ChannelStatusEnabled,
		Group: "default", Models: "gpt-5", Weight: &weight,
	}
	if err := DB.Create([]*Channel{nullWeight, weighted}).Error; err != nil {
		t.Fatalf("create channels: %v", err)
	}
	// 建表默认值会补 1，需显式置 NULL 才能复现 Select("*") 全量更新写入的空值。
	if err := DB.Exec("UPDATE channels SET weight = NULL WHERE id = ?", nullWeight.Id).Error; err != nil {
		t.Fatalf("null out weight: %v", err)
	}

	cc := &ChannelsChooser{}
	cc.Load()

	if got := cc.Channels[nullWeight.Id].Channel.GetWeight(); got != int64(config.DefaultChannelWeight) {
		t.Fatalf("空 weight 应按默认权重处理, got %d", got)
	}
	// 权重选路必须能选中空 weight 渠道，且全程不 panic。
	selected := make(map[int]bool)
	for i := 0; i < 50; i++ {
		channel := cc.balancer([]int{1, 2}, nil, "gpt-5", nil)
		if channel == nil {
			t.Fatal("balancer 不应返回空渠道")
		}
		selected[channel.Id] = true
	}
	if !selected[nullWeight.Id] {
		t.Fatal("空 weight 渠道应能被选中")
	}
}

func setupBalancerTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "balancer_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&Channel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB, oldUsing := DB, common.UsingSQLite
	DB, common.UsingSQLite = testDB, true
	t.Cleanup(func() { DB, common.UsingSQLite = oldDB, oldUsing })
}
