package model

import (
	"path/filepath"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupPriceTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "price_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开测试数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&Price{}); err != nil {
		t.Fatalf("迁移 Price 表失败: %v", err)
	}
	oldDB := DB
	oldUsing := common.UsingSQLite
	DB = testDB
	common.UsingSQLite = true
	t.Cleanup(func() {
		DB = oldDB
		common.UsingSQLite = oldUsing
	})
}

// UpsertPrices 语义：① 不存在 → 插入；② 存在且未锁 → 更新；③ 存在且锁定 → 保留；
// ④ 不在传入列表里的其它模型 → 完全不动（不能像 overwrite 那样误删其它渠道价）。
func TestUpsertPrices(t *testing.T) {
	setupPriceTestDB(t)

	if err := DB.Create(&Price{Model: "or/a", Type: TokensPriceType, Input: 1, Output: 1}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Price{Model: "or/b", Type: TokensPriceType, Input: 9, Output: 9, Locked: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&Price{Model: "other/keep", Type: TokensPriceType, Input: 2, Output: 3}).Error; err != nil {
		t.Fatal(err)
	}

	p := &Pricing{Prices: make(map[string]*Price), Match: []string{}}
	if err := p.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	in := []*Price{
		{Model: "or/a", Type: TokensPriceType, ChannelType: 20, Input: 1.5, Output: 7.5}, // 更新
		{Model: "or/b", Type: TokensPriceType, ChannelType: 20, Input: 1.5, Output: 7.5}, // 锁定→保留
		{Model: "or/c", Type: TokensPriceType, ChannelType: 20, Input: 0.5, Output: 0.5}, // 新增
	}
	if err := p.UpsertPrices(in); err != nil {
		t.Fatalf("UpsertPrices: %v", err)
	}

	got := map[string]Price{}
	var all []Price
	if err := DB.Find(&all).Error; err != nil {
		t.Fatal(err)
	}
	for _, x := range all {
		got[x.Model] = x
	}

	if got["or/a"].Input != 1.5 || got["or/a"].Output != 7.5 {
		t.Errorf("or/a 应被更新为 1.5/7.5，实际 %+v", got["or/a"])
	}
	if got["or/b"].Input != 9 || got["or/b"].Output != 9 {
		t.Errorf("锁定的 or/b 不应变，实际 %+v", got["or/b"])
	}
	if got["or/c"].Input != 0.5 {
		t.Errorf("or/c 应被插入，实际 %+v", got["or/c"])
	}
	if got["other/keep"].Input != 2 || got["other/keep"].Output != 3 {
		t.Errorf("无关的 other/keep 不应被动，实际 %+v", got["other/keep"])
	}
	if len(all) != 4 {
		t.Errorf("总数应为 4（a,b,c,other），实际 %d", len(all))
	}
}

// 目录写入（发布 / 元信息编辑）必须顺带刷新价格实例上的 model_info 快照，
// 否则 /v1/models 与 /api/available_model 读到的仍是进程启动时的旧元信息。
func TestCatalogWriteRefreshesPricingModelInfo(t *testing.T) {
	setupPriceTestDB(t)
	if err := DB.AutoMigrate(&ModelInfo{}); err != nil {
		t.Fatalf("迁移 ModelInfo 表失败: %v", err)
	}
	if err := DB.Create(&Price{Model: "cat/a", Type: TokensPriceType, Input: 1, Output: 1}).Error; err != nil {
		t.Fatal(err)
	}
	info := &ModelInfo{Model: "cat/a", Name: "before"}
	if err := DB.Create(info).Error; err != nil {
		t.Fatal(err)
	}

	oldPricing := PricingInstance
	PricingInstance = &Pricing{Prices: make(map[string]*Price), Match: []string{}}
	t.Cleanup(func() { PricingInstance = oldPricing })
	if err := PricingInstance.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if snapshot := PricingInstance.GetPrice("cat/a").ModelInfo; snapshot == nil || snapshot.Hidden {
		t.Fatalf("初始快照应为未隐藏，实际 %+v", snapshot)
	}

	if _, err := SetModelInfoHidden([]int{info.Id}, true); err != nil {
		t.Fatalf("SetModelInfoHidden: %v", err)
	}
	if snapshot := PricingInstance.GetPrice("cat/a").ModelInfo; snapshot == nil || !snapshot.Hidden {
		t.Fatalf("隐藏后价格快照应已刷新为已隐藏，实际 %+v", snapshot)
	}

	info.Name = "after"
	if err := UpdateModelInfo(info); err != nil {
		t.Fatalf("UpdateModelInfo: %v", err)
	}
	if snapshot := PricingInstance.GetPrice("cat/a").ModelInfo; snapshot == nil || snapshot.Name != "after" {
		t.Fatalf("元信息编辑后价格快照应已刷新，实际 %+v", snapshot)
	}
}
