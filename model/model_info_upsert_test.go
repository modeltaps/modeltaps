package model

import (
	"path/filepath"
	"testing"

	"github.com/modeltaps/modeltaps/common"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupModelInfoTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "mi_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&ModelInfo{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB, oldUsing := DB, common.UsingSQLite
	DB, common.UsingSQLite = testDB, true
	// 目录索引在单测里一律走 DB：避免上一个用例加载的索引泄漏到本用例。
	modelInfoCache.Lock()
	oldLoaded, oldByModel, oldAliases := modelInfoCache.loaded, modelInfoCache.byModel, modelInfoCache.aliasesByModel
	modelInfoCache.loaded, modelInfoCache.byModel, modelInfoCache.aliasesByModel = false, nil, nil
	modelInfoCache.Unlock()
	t.Cleanup(func() {
		DB, common.UsingSQLite = oldDB, oldUsing
		modelInfoCache.Lock()
		modelInfoCache.loaded, modelInfoCache.byModel, modelInfoCache.aliasesByModel = oldLoaded, oldByModel, oldAliases
		modelInfoCache.Unlock()
	})
}

// UpsertModelInfos：已存在(按 model)则更新元数据、不存在则插入；不影响其它行。
func TestUpsertModelInfos(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "a", Name: "old", ContextLength: 1}).Error; err != nil {
		t.Fatal(err)
	}

	err := UpsertModelInfos([]*ModelInfo{
		{Model: "a", Name: "new", ContextLength: 128000, MaxTokens: 4096, InputModalities: `["text"]`},
		{Model: "b", Name: "fresh", ContextLength: 200000},
	}, ModelInfoSourceModelsDev)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got := map[string]ModelInfo{}
	var all []ModelInfo
	if err := DB.Find(&all).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		got[m.Model] = m
	}
	if len(all) != 2 {
		t.Fatalf("want 2 rows, got %d", len(all))
	}
	if got["a"].Name != "new" || got["a"].ContextLength != 128000 || got["a"].InputModalities != `["text"]` {
		t.Errorf("a not updated: %+v", got["a"])
	}
	if got["b"].Name != "fresh" || got["b"].ContextLength != 200000 {
		t.Errorf("b not inserted: %+v", got["b"])
	}
	if got["a"].SyncedAt == 0 || got["b"].SyncedAt == 0 {
		t.Errorf("synced_at not written: a=%d b=%d", got["a"].SyncedAt, got["b"].SyncedAt)
	}
	if got["a"].Source != ModelInfoSourceModelsDev || got["b"].Source != ModelInfoSourceModelsDev {
		t.Errorf("source not written: a=%q b=%q", got["a"].Source, got["b"].Source)
	}
}

// 来源以调用参数为准：构造方填错的 Source 会被覆盖；来源缺失直接报错。
func TestUpsertModelInfosSourceFromArgument(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "a", Name: "old", Source: ModelInfoSourceManual}).Error; err != nil {
		t.Fatal(err)
	}

	err := UpsertModelInfos([]*ModelInfo{
		{Model: "a", Name: "new", Source: "wrong"},
		{Model: "b", Name: "fresh", Source: "wrong"},
	}, ModelInfoSourceOpenRouter)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var all []ModelInfo
	if err := DB.Find(&all).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if m.Source != ModelInfoSourceOpenRouter {
			t.Errorf("%s source = %q, want %q", m.Model, m.Source, ModelInfoSourceOpenRouter)
		}
	}

	if err := UpsertModelInfos([]*ModelInfo{{Model: "c"}}, ""); err == nil {
		t.Error("empty source should return an error")
	}
}

// 锁定的行整行跳过；同步值为空时不覆盖已有非空值。
func TestUpsertModelInfosLockedAndEmpty(t *testing.T) {
	setupModelInfoTestDB(t)
	rows := []*ModelInfo{
		{Model: "locked", Name: "keep", Description: "keep desc", ContextLength: 1, Source: ModelInfoSourceManual, Locked: true},
		{Model: "open", Name: "keep", Description: "keep desc", ContextLength: 1, Source: ModelInfoSourceManual},
	}
	for _, r := range rows {
		if err := DB.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}

	err := UpsertModelInfos([]*ModelInfo{
		{Model: "locked", Name: "overwritten", ContextLength: 999},
		{Model: "open", Name: "", Description: "", ContextLength: 0, Tags: ""},
	}, ModelInfoSourceOpenRouter)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var locked, open ModelInfo
	if err := DB.Where("model = ?", "locked").First(&locked).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Where("model = ?", "open").First(&open).Error; err != nil {
		t.Fatal(err)
	}
	if locked.Name != "keep" || locked.ContextLength != 1 || locked.Source != ModelInfoSourceManual || locked.SyncedAt != 0 {
		t.Errorf("locked row was modified: %+v", locked)
	}
	if open.Name != "keep" || open.Description != "keep desc" || open.ContextLength != 1 {
		t.Errorf("empty values overwrote non-empty: %+v", open)
	}
	if open.Source != ModelInfoSourceOpenRouter || open.SyncedAt == 0 {
		t.Errorf("source/synced_at not updated: %+v", open)
	}
}

// capabilities 在更新白名单内（putStr 语义）；锁定行仍整行跳过。
func TestUpsertModelInfosCapabilities(t *testing.T) {
	setupModelInfoTestDB(t)
	rows := []*ModelInfo{
		{Model: "a", Capabilities: `["tool_call"]`, Source: ModelInfoSourceManual},
		{Model: "empty", Capabilities: `["reasoning"]`, Source: ModelInfoSourceManual},
		{Model: "locked", Capabilities: `["tool_call"]`, Source: ModelInfoSourceManual, Locked: true},
	}
	for _, r := range rows {
		if err := DB.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}

	err := UpsertModelInfos([]*ModelInfo{
		{Model: "a", Capabilities: `["tool_call","reasoning"]`},
		{Model: "empty", Capabilities: ""},
		{Model: "locked", Capabilities: `[]`},
		{Model: "fresh", Capabilities: `[]`},
	}, ModelInfoSourceModelsDev)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	want := map[string]string{
		"a":      `["tool_call","reasoning"]`,
		"empty":  `["reasoning"]`,
		"locked": `["tool_call"]`,
		"fresh":  `[]`,
	}
	var all []ModelInfo
	if err := DB.Find(&all).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if got := m.Capabilities; got != want[m.Model] {
			t.Errorf("%s capabilities = %q, want %q", m.Model, got, want[m.Model])
		}
	}
}

// mode 只由管理员维护：同步既不覆盖已有值，也不为新行写入。
func TestUpsertModelInfosKeepsMode(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "a", Name: "old", Mode: ModelModeChatImage, Source: ModelInfoSourceManual}).Error; err != nil {
		t.Fatal(err)
	}

	err := UpsertModelInfos([]*ModelInfo{
		{Model: "a", Name: "new", Mode: ModelModeImage},
		{Model: "b", Name: "fresh", Mode: ModelModeImage},
	}, ModelInfoSourceModelsDev)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var a, b ModelInfo
	if err := DB.Where("model = ?", "a").First(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Where("model = ?", "b").First(&b).Error; err != nil {
		t.Fatal(err)
	}
	if a.Mode != ModelModeChatImage {
		t.Errorf("existing mode overwritten: %q", a.Mode)
	}
	if a.Name != "new" {
		t.Errorf("other fields should still sync: %+v", a)
	}
	if b.Mode != "" {
		t.Errorf("mode written on insert: %q", b.Mode)
	}
}

// hidden / alias_of 只由管理员维护：同步既不覆盖已有值，新行也一律 false / 空。
func TestUpsertModelInfosKeepsHiddenAndAlias(t *testing.T) {
	setupModelInfoTestDB(t)
	if err := DB.Create(&ModelInfo{Model: "a", Name: "old", Hidden: true, AliasOf: "canonical", Source: ModelInfoSourceManual}).Error; err != nil {
		t.Fatal(err)
	}

	err := UpsertModelInfos([]*ModelInfo{
		{Model: "a", Name: "new", Hidden: false, AliasOf: ""},
		{Model: "b", Name: "fresh", Hidden: true, AliasOf: "a"},
	}, ModelInfoSourceModelsDev)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	var a, b ModelInfo
	if err := DB.Where("model = ?", "a").First(&a).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Where("model = ?", "b").First(&b).Error; err != nil {
		t.Fatal(err)
	}
	if !a.Hidden || a.AliasOf != "canonical" {
		t.Errorf("existing hidden/alias_of overwritten: %+v", a)
	}
	if a.Name != "new" {
		t.Errorf("other fields should still sync: %+v", a)
	}
	if b.Hidden || b.AliasOf != "" {
		t.Errorf("hidden/alias_of written on insert: %+v", b)
	}
}

// endpoints：库里为空才填入，已有值（管理员改过的）不被同步覆盖；vendor_id 随同步更新；
// 锁定行三者都不动。
func TestUpsertModelInfosEndpointsAndVendor(t *testing.T) {
	setupModelInfoTestDB(t)
	rows := []*ModelInfo{
		{Model: "admin", Endpoints: `["images"]`, VendorID: 1, Source: ModelInfoSourceManual},
		{Model: "empty", Source: ModelInfoSourceManual},
		{Model: "locked", Endpoints: `["chat"]`, VendorID: 1, Source: ModelInfoSourceManual, Locked: true},
	}
	for _, r := range rows {
		if err := DB.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}

	err := UpsertModelInfos([]*ModelInfo{
		{Model: "admin", Endpoints: `["chat","responses"]`, VendorID: 2},
		{Model: "empty", Endpoints: `["chat","responses"]`, VendorID: 2},
		{Model: "locked", Endpoints: `["images"]`, VendorID: 2},
		{Model: "fresh", Endpoints: `["chat"]`, VendorID: 2},
	}, ModelInfoSourceOpenRouter)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	wantEndpoints := map[string]string{
		"admin":  `["images"]`,
		"empty":  `["chat","responses"]`,
		"locked": `["chat"]`,
		"fresh":  `["chat"]`,
	}
	wantVendor := map[string]int{"admin": 2, "empty": 2, "locked": 1, "fresh": 2}
	var all []ModelInfo
	if err := DB.Find(&all).Error; err != nil {
		t.Fatal(err)
	}
	for _, m := range all {
		if m.Endpoints != wantEndpoints[m.Model] {
			t.Errorf("%s endpoints = %q, want %q", m.Model, m.Endpoints, wantEndpoints[m.Model])
		}
		if m.VendorID != wantVendor[m.Model] {
			t.Errorf("%s vendor_id = %d, want %d", m.Model, m.VendorID, wantVendor[m.Model])
		}
	}
}

// ResolveCanonicalModel / IsModelHidden：别名解析到主名，隐藏状态随主名；无目录行不隐藏；环有跳数上限。
func TestResolveCanonicalModelAndHidden(t *testing.T) {
	setupModelInfoTestDB(t)
	rows := []*ModelInfo{
		{Model: "gpt-5", Hidden: true},
		{Model: "gpt", AliasOf: "gpt-5"},
		{Model: "gpt-old", AliasOf: "gpt"},
		{Model: "draft"},
		{Model: "loop-a", AliasOf: "loop-b"},
		{Model: "loop-b", AliasOf: "loop-a"},
	}
	for _, r := range rows {
		if err := DB.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name      string
		canonical string
		isAlias   bool
		hidden    bool
	}{
		{"gpt-5", "gpt-5", false, true},
		{"gpt", "gpt-5", true, true},
		{"gpt-old", "gpt-5", true, true},
		{"draft", "draft", false, false},
		{"missing", "missing", false, false},
	}
	for _, c := range cases {
		canonical, isAlias := ResolveCanonicalModel(c.name)
		if canonical != c.canonical || isAlias != c.isAlias {
			t.Errorf("ResolveCanonicalModel(%q) = (%q,%v), want (%q,%v)", c.name, canonical, isAlias, c.canonical, c.isAlias)
		}
		if got := IsModelHidden(c.name); got != c.hidden {
			t.Errorf("IsModelHidden(%q) = %v, want %v", c.name, got, c.hidden)
		}
	}

	// 环形别名只要不挂死即可，解析结果落在环上的任一节点。
	if canonical, _ := ResolveCanonicalModel("loop-a"); canonical != "loop-a" && canonical != "loop-b" {
		t.Errorf("cyclic alias resolved to %q, want a node on the cycle", canonical)
	}
}

// ModelAliases / ModelEndpoints：别名反查（含多跳）与接口能力读取，索引加载前后口径一致。
func TestModelAliasesAndEndpoints(t *testing.T) {
	setupModelInfoTestDB(t)
	rows := []*ModelInfo{
		{Model: "gpt-5", Endpoints: `["chat","responses"]`},
		{Model: "gpt", AliasOf: "gpt-5"},
		{Model: "gpt-old", AliasOf: "gpt"},
		{Model: "draft"},
	}
	for _, r := range rows {
		if err := DB.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}

	// 索引未加载：回落 DB，只查直接别名。
	if got := ModelAliases("gpt-5"); len(got) != 1 || got[0] != "gpt" {
		t.Errorf("ModelAliases(gpt-5) without cache = %v, want [gpt]", got)
	}

	if err := ReloadModelInfoCache(); err != nil {
		t.Fatal(err)
	}
	// 索引加载后：多跳别名也归到主名下。
	got := ModelAliases("gpt-5")
	if len(got) != 2 || got[0] != "gpt" || got[1] != "gpt-old" {
		t.Errorf("ModelAliases(gpt-5) = %v, want [gpt gpt-old]", got)
	}
	if got := ModelAliases("draft"); len(got) != 0 {
		t.Errorf("ModelAliases(draft) = %v, want empty", got)
	}

	if got := ModelEndpoints("gpt-5"); len(got) != 2 || got[0] != "chat" || got[1] != "responses" {
		t.Errorf("ModelEndpoints(gpt-5) = %v, want [chat responses]", got)
	}
	// 别名随主名；未设置 endpoints 与无目录行都返回空切片。
	if got := ModelEndpoints("gpt-old"); len(got) != 2 {
		t.Errorf("ModelEndpoints(gpt-old) = %v, want 主名的两项", got)
	}
	if got := ModelEndpoints("draft"); len(got) != 0 {
		t.Errorf("ModelEndpoints(draft) = %v, want empty", got)
	}
	if got := ModelEndpoints("missing"); len(got) != 0 {
		t.Errorf("ModelEndpoints(missing) = %v, want empty", got)
	}
}

// ResolveVendorID：本行 vendor_id 优先，为空时取别名主名的 vendor；都没有返回 0。
func TestResolveVendorID(t *testing.T) {
	setupModelInfoTestDB(t)
	rows := []*ModelInfo{
		{Model: "gpt-5", VendorID: 1},
		{Model: "gpt", AliasOf: "gpt-5"},
		{Model: "own", VendorID: 3, AliasOf: "gpt-5"},
		{Model: "none"},
	}
	for _, r := range rows {
		if err := DB.Create(r).Error; err != nil {
			t.Fatal(err)
		}
	}

	want := map[string]int{"gpt-5": 1, "gpt": 1, "own": 3, "none": 0, "missing": 0}
	for name, vendorID := range want {
		if got := ResolveVendorID(name); got != vendorID {
			t.Errorf("ResolveVendorID(%q) = %d, want %d", name, got, vendorID)
		}
	}
}
