package model

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/brandicon"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/safefetch"

	"github.com/go-gormigrate/gormigrate/v2"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupModelOwnedByTestDB 建一个只含 model_owned_by 表的临时库，并接管全局实例。
func setupModelOwnedByTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "mob_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&ModelOwnedBy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB, oldInstance := DB, ModelOwnedBysInstance
	DB, ModelOwnedBysInstance = testDB, &ModelOwnedBys{}
	t.Cleanup(func() { DB, ModelOwnedBysInstance = oldDB, oldInstance })
}

// 默认数据为主流厂商补齐 slug；聚合型渠道不给 slug。
func TestDefaultModelOwnedBySlugs(t *testing.T) {
	bySlug := map[string]int{}
	for _, owner := range GetDefaultModelOwnedBy() {
		if owner.Slug == "" {
			continue
		}
		if prev, ok := bySlug[string(owner.Slug)]; ok {
			t.Fatalf("slug %q 被 %d 与 %d 共用", owner.Slug, prev, owner.Id)
		}
		bySlug[string(owner.Slug)] = owner.Id
	}

	want := map[string]int{
		"openai":     config.ChannelTypeOpenAI,
		"anthropic":  config.ChannelTypeAnthropic,
		"google":     config.ChannelTypeGemini,
		"x-ai":       config.ChannelTypeXAI,
		"deepseek":   config.ChannelTypeDeepseek,
		"qwen":       config.ChannelTypeAli,
		"moonshotai": config.ChannelTypeMoonshot,
		"z-ai":       config.ChannelTypeZhipu,
		"meta-llama": config.ChannelTypeLLAMA,
		"mistralai":  config.ChannelTypeMistral,
	}
	for slug, id := range want {
		if bySlug[slug] != id {
			t.Errorf("slug %q = %d, want %d", slug, bySlug[slug], id)
		}
	}
	for _, owner := range GetDefaultModelOwnedBy() {
		if owner.Id == config.ChannelTypeOpenRouter && owner.Slug != "" {
			t.Errorf("聚合型渠道不应有 slug: %+v", owner)
		}
	}
}

// 同步只回填存量行缺失的 slug，不动管理员改过的名称 / 图标。
func TestSyncModelOwnedByBackfillsSlug(t *testing.T) {
	setupModelOwnedByTestDB(t)
	legacy := &ModelOwnedBy{Id: config.ChannelTypeOpenAI, Name: "我的 OpenAI", Icon: "custom.svg"}
	if err := DB.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := ModelOwnedBysInstance.Load(); err != nil {
		t.Fatal(err)
	}

	ModelOwnedBysInstance.SyncModelOwnedBy(GetDefaultModelOwnedBy())

	var got ModelOwnedBy
	if err := DB.Where("id = ?", config.ChannelTypeOpenAI).First(&got).Error; err != nil {
		t.Fatal(err)
	}
	if got.Slug != "openai" {
		t.Errorf("slug 未回填: %+v", got)
	}
	if got.Name != "我的 OpenAI" || got.Icon != "custom.svg" {
		t.Errorf("同步覆盖了管理员改过的字段: %+v", got)
	}
	if id := ModelOwnedBysInstance.GetIdBySlug("x-ai"); id != config.ChannelTypeXAI {
		t.Errorf("GetIdBySlug(x-ai) = %d, want %d", id, config.ChannelTypeXAI)
	}
	if id := ModelOwnedBysInstance.GetIdBySlug("nope"); id != 0 {
		t.Errorf("未登记 slug 应返回 0, got %d", id)
	}
}

// VendorIDFromModelName：按 "厂商slug/模型名" 前缀匹配，无前缀或未登记返回 0。
func TestVendorIDFromModelName(t *testing.T) {
	setupModelOwnedByTestDB(t)
	ModelOwnedBysInstance.SyncModelOwnedBy(GetDefaultModelOwnedBy())

	cases := map[string]int{
		"google/gemini-2.5-pro":         config.ChannelTypeGemini,
		"x-ai/grok-4":                   config.ChannelTypeXAI,
		"unknown-vendor/model":          0,
		"gpt-4o":                        0,
		"/leading-slash":                0,
		"~anthropic/claude-opus-latest": config.ChannelTypeAnthropic,
		"~/leading-slash":               0,
	}
	for name, want := range cases {
		if got := VendorIDFromModelName(name); got != want {
			t.Errorf("VendorIDFromModelName(%q) = %d, want %d", name, got, want)
		}
	}
}

// 默认厂商表的图标全部是内置 key，不再出现外链。
func TestDefaultModelOwnedByIconsAreBrandKeys(t *testing.T) {
	for _, owner := range GetDefaultModelOwnedBy() {
		if owner.Icon == "" {
			continue
		}
		key, ok := strings.CutPrefix(owner.Icon, VendorIconBrandPrefix)
		if !ok {
			t.Errorf("vendor %d icon %q is not a brand key", owner.Id, owner.Icon)
			continue
		}
		if canonical, ok := brandicon.Resolve(key); !ok || canonical != key {
			t.Errorf("vendor %d icon %q does not resolve to a canonical manifest key", owner.Id, owner.Icon)
		}
	}
}

func setupVendorIconMigrationDB(t *testing.T, fetch func(context.Context, string, safefetch.Options) (*safefetch.Result, error)) {
	t.Helper()
	setupModelOwnedByTestDB(t)
	if err := DB.AutoMigrate(&BrandIconUpload{}); err != nil {
		t.Fatal(err)
	}
	old := vendorIconFetch
	vendorIconFetch = fetch
	t.Cleanup(func() { vendorIconFetch = old })
}

const testPNG = "\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR"

// 迁移覆盖三类外链：lobehub 图标 → brand:，旧默认图 → 空，其他外链 → upload:（失败置空）；
// 原值进 icon_legacy，重复执行无变化，Rollback 恢复原值。
func TestMigrateModelOwnedByIcon(t *testing.T) {
	var fetched []string
	setupVendorIconMigrationDB(t, func(_ context.Context, rawURL string, _ safefetch.Options) (*safefetch.Result, error) {
		fetched = append(fetched, rawURL)
		if strings.Contains(rawURL, "broken") {
			return nil, errors.New("boom")
		}
		return &safefetch.Result{Body: []byte(testPNG)}, nil
	})
	rows := []*ModelOwnedBy{
		{Id: 1, Name: "OpenAI", Icon: "https://registry.npmmirror.com/@lobehub/icons-static-svg/latest/files/icons/openai.svg"},
		{Id: 2, Name: "Claude", Icon: "https://registry.npmmirror.com/@lobehub/icons-static-svg/1.24.0/files/icons/claude-color.svg"},
		{Id: 3, Name: "xAI", Icon: "https://registry.npmmirror.com/@lobehub/icons-static-webp/1.24.0/files/light/xai.webp"},
		{Id: 4, Name: "Default", Icon: legacyDefaultModelIcon},
		{Id: 5, Name: "Custom", Icon: "https://example.com/logo.png"},
		{Id: 6, Name: "Broken", Icon: "https://broken.example.com/logo.png"},
		{Id: 7, Name: "Relative", Icon: "custom.svg"},
		{Id: 8, Name: "Kept", Icon: "brand:qwen"},
		{Id: 9, Name: "Auto", Icon: ""},
	}
	if err := DB.Create(rows).Error; err != nil {
		t.Fatal(err)
	}

	run := func() {
		t.Helper()
		m := gormigrate.New(DB, gormigrate.DefaultOptions, []*gormigrate.Migration{migrateModelOwnedByIcon()})
		if err := m.Migrate(); err != nil {
			t.Fatal(err)
		}
	}
	load := func() map[int]ModelOwnedBy {
		t.Helper()
		var got []ModelOwnedBy
		if err := DB.Order("id").Find(&got).Error; err != nil {
			t.Fatal(err)
		}
		out := map[int]ModelOwnedBy{}
		for _, r := range got {
			out[r.Id] = r
		}
		return out
	}

	run()
	got := load()
	uploadID, err := SaveBrandIconUpload([]byte(testPNG))
	if err != nil {
		t.Fatal(err)
	}
	want := map[int]string{1: "brand:openai", 2: "brand:claude", 3: "brand:xai", 4: "", 5: "upload:" + uploadID,
		6: "", 7: "", 8: "brand:qwen", 9: ""}
	for id, icon := range want {
		if got[id].Icon != icon {
			t.Errorf("vendor %d icon = %q, want %q", id, got[id].Icon, icon)
		}
	}
	for _, r := range rows {
		wantLegacy := r.Icon
		if r.Id >= 8 {
			wantLegacy = ""
		}
		if got[r.Id].IconLegacy != wantLegacy {
			t.Errorf("vendor %d icon_legacy = %q, want %q", r.Id, got[r.Id].IconLegacy, wantLegacy)
		}
	}
	if len(fetched) != 2 {
		t.Errorf("only non-lobehub external urls should be fetched, got %v", fetched)
	}

	// 直接重跑迁移函数：已是新取值的行不再处理。
	if err := migrateModelOwnedByIcon().Migrate(DB); err != nil {
		t.Fatal(err)
	}
	if again := load(); again[5] != got[5] || again[1] != got[1] || len(fetched) != 2 {
		t.Errorf("migration is not idempotent: %+v vs %+v, fetched %v", again[5], got[5], fetched)
	}

	if err := migrateModelOwnedByIcon().Rollback(DB); err != nil {
		t.Fatal(err)
	}
	restored := load()
	for _, r := range rows {
		if restored[r.Id].Icon != r.Icon || restored[r.Id].IconLegacy != "" {
			t.Errorf("rollback vendor %d = %+v, want icon %q", r.Id, restored[r.Id], r.Icon)
		}
	}
}

// 管理员编辑厂商不会冲掉 icon_legacy 备份。
func TestUpdateModelOwnedByKeepsIconLegacy(t *testing.T) {
	setupModelOwnedByTestDB(t)
	if err := DB.Create(&ModelOwnedBy{Id: 1001, Name: "A", Icon: "", IconLegacy: "https://example.com/a.png"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := UpdateModelOwnedBy(&ModelOwnedBy{Id: 1001, Name: "B", Icon: "brand:openai"}); err != nil {
		t.Fatal(err)
	}
	var got ModelOwnedBy
	DB.First(&got, 1001)
	if got.Name != "B" || got.Icon != "brand:openai" || got.IconLegacy != "https://example.com/a.png" {
		t.Errorf("unexpected row after update: %+v", got)
	}
}

func TestNormalizeVendorIcon(t *testing.T) {
	setupVendorIconMigrationDB(t, nil)
	id, err := SaveBrandIconUpload([]byte(testPNG))
	if err != nil {
		t.Fatal(err)
	}
	ok := map[string]string{"": "", "  ": "", "brand:OpenAI": "brand:openai", "upload:" + id: "upload:" + id}
	for in, want := range ok {
		if got, err := NormalizeVendorIcon(in); err != nil || got != want {
			t.Errorf("NormalizeVendorIcon(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, in := range []string{"https://example.com/a.png", "brand:nope", "upload:missing", "upload:", "custom.svg"} {
		if _, err := NormalizeVendorIcon(in); !errors.Is(err, ErrInvalidVendorIcon) {
			t.Errorf("NormalizeVendorIcon(%q) should be rejected, got %v", in, err)
		}
	}
}
