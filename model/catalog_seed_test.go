package model

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupSeedTestDB(t *testing.T) {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "seed_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	testDB, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := testDB.AutoMigrate(&ModelInfo{}, &Channel{}, &ModelOwnedBy{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	oldDB, oldUsing := DB, common.UsingSQLite
	DB, common.UsingSQLite = testDB, true
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

// seedFixture 建最小数据集：一个别名行、一个白名单外的普通行、一个配置了别名的启用渠道。
func seedFixture(t *testing.T, entry CatalogSeedEntry) *Channel {
	t.Helper()
	alias := entry.Aliases[0]
	if err := DB.Create(&ModelInfo{Model: alias, Name: "Alias Row", ContextLength: 1024}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelInfo{Model: "legacy/model"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelOwnedBy{Id: 900, Name: "Vendor", Slug: NullableSlug(entry.VendorSlug)}).Error; err != nil {
		t.Fatal(err)
	}
	oldInstance := ModelOwnedBysInstance
	ModelOwnedBysInstance = &ModelOwnedBys{}
	if err := ModelOwnedBysInstance.Load(); err != nil {
		t.Fatalf("load vendors: %v", err)
	}
	t.Cleanup(func() { ModelOwnedBysInstance = oldInstance })
	weight := uint(1)
	channel := &Channel{Id: 1, Status: config.ChannelStatusEnabled, Name: "or", Models: alias, Weight: &weight}
	if err := DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	return channel
}

func firstSeedEntry(t *testing.T) CatalogSeedEntry {
	t.Helper()
	seed, err := LoadCatalogSeed()
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if len(seed.Models) == 0 {
		t.Fatal("seed is empty")
	}
	return seed.Models[0]
}

// 内置种子必须能解析，且每条都带主名、厂商 slug、至少一个别名与合法接口能力。
func TestLoadCatalogSeed(t *testing.T) {
	seed, err := LoadCatalogSeed()
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	if len(seed.Models) < 40 || len(seed.Models) > 60 {
		t.Fatalf("seed size = %d, want 40..60", len(seed.Models))
	}
	for _, entry := range seed.Models {
		if entry.VendorSlug == "" || len(entry.Aliases) == 0 || entry.Reason == "" {
			t.Fatalf("incomplete seed entry: %+v", entry)
		}
		if ModelEndpointsJSON(entry.Endpoints) == "" {
			t.Fatalf("entry %s has no valid endpoints", entry.Canonical)
		}
	}
}

// 词表外的接口能力必须在解析时报错：归一化会静默丢弃这些值，校验放在归一化之后等于没校验。
func TestParseCatalogSeedRejectsUnknownEndpoints(t *testing.T) {
	raw := []byte(`{"version":"t","models":[{"canonical":"m","vendor_slug":"openai","aliases":["a/m"],"endpoints":["chat","videos"],"reason":"r"}]}`)
	_, err := parseCatalogSeed(raw)
	if err == nil || !strings.Contains(err.Error(), "videos") {
		t.Fatalf("应报出词表外的 endpoint, got %v", err)
	}

	empty := []byte(`{"version":"t","models":[{"canonical":"m","vendor_slug":"openai","aliases":["a/m"],"endpoints":[],"reason":"r"}]}`)
	if _, err := parseCatalogSeed(empty); err == nil {
		t.Fatal("没有声明任何接口能力的条目应报错")
	}
}

// 种子条目的 endpoints 必须与自述能力自洽：自述图像生成 / 输出的要声明 images，
// 自述音频输出或转写的要声明 audio.*（只当输入模态讲的「音频输入」不在此列）。
// 否则图像 / 音频模型会被当成纯文本模型，经 EffectiveMode 影响任务类型判定与能力面展示。
func TestCatalogSeedEndpointsMatchReason(t *testing.T) {
	seed, err := LoadCatalogSeed()
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	for _, entry := range seed.Models {
		declared := make(map[string]bool, len(entry.Endpoints))
		audio := false
		for _, endpoint := range entry.Endpoints {
			declared[endpoint] = true
			if strings.HasPrefix(endpoint, "audio.") {
				audio = true
			}
		}
		reason := strings.ToLower(entry.Reason)
		imageOutput := strings.Contains(reason, "image output") || strings.Contains(reason, "image generation")
		if imageOutput && !declared[ModelEndpointImages] {
			t.Errorf("%s 自述图像能力却未声明 %s: %v", entry.Canonical, ModelEndpointImages, entry.Endpoints)
		}
		audioService := strings.Contains(reason, "audio output") || strings.Contains(reason, "transcription")
		if audioService && !audio {
			t.Errorf("%s 自述音频能力却未声明 audio.* 接口: %v", entry.Canonical, entry.Endpoints)
		}
	}
}

// 每条种子都要带模态与能力：模态为空前端的「图片理解 / 音频 / 推理」等分节全是空的；
// 同时模态要与 endpoints 自洽（images → 输出含 image；audio.speech → 输出含 audio；
// audio.transcription → 输入含 audio）。
func TestCatalogSeedModalitiesAndCapabilities(t *testing.T) {
	seed, err := LoadCatalogSeed()
	if err != nil {
		t.Fatalf("load seed: %v", err)
	}
	for _, entry := range seed.Models {
		if len(entry.InputModalities) == 0 || len(entry.OutputModalities) == 0 {
			t.Errorf("%s 模态为空: in=%v out=%v", entry.Canonical, entry.InputModalities, entry.OutputModalities)
			continue
		}
		if unknown := unknownSeedCapabilities(entry.Capabilities); len(unknown) > 0 {
			t.Errorf("%s 能力词表外取值: %v", entry.Canonical, unknown)
		}
		declared := make(map[string]bool, len(entry.Endpoints))
		for _, endpoint := range entry.Endpoints {
			declared[endpoint] = true
		}
		if declared[ModelEndpointImages] && !hasModality(entry.OutputModalities, "image") {
			t.Errorf("%s 声明 images 却不输出 image: %v", entry.Canonical, entry.OutputModalities)
		}
		if declared[ModelEndpointAudioSpeech] && !hasModality(entry.OutputModalities, "audio") {
			t.Errorf("%s 声明 audio.speech 却不输出 audio: %v", entry.Canonical, entry.OutputModalities)
		}
		if declared[ModelEndpointAudioTranscription] && !hasModality(entry.InputModalities, "audio") {
			t.Errorf("%s 声明 audio.transcription 却不接受 audio 输入: %v", entry.Canonical, entry.InputModalities)
		}
	}
}

// 模态缺失或词表外的取值必须在解析时报错，能力词同理。
func TestParseCatalogSeedRejectsBadModalities(t *testing.T) {
	missing := []byte(`{"version":"t","models":[{"canonical":"m","vendor_slug":"openai","aliases":["a/m"],"endpoints":["chat"],"reason":"r"}]}`)
	if _, err := parseCatalogSeed(missing); err == nil {
		t.Fatal("没有模态的条目应报错")
	}

	unknownModality := []byte(`{"version":"t","models":[{"canonical":"m","vendor_slug":"openai","aliases":["a/m"],"endpoints":["chat"],"input_modalities":["text","hologram"],"output_modalities":["text"],"reason":"r"}]}`)
	if _, err := parseCatalogSeed(unknownModality); err == nil || !strings.Contains(err.Error(), "hologram") {
		t.Fatalf("应报出词表外的模态, got %v", err)
	}

	unknownCap := []byte(`{"version":"t","models":[{"canonical":"m","vendor_slug":"openai","aliases":["a/m"],"endpoints":["chat"],"input_modalities":["text"],"output_modalities":["text"],"capabilities":["vision"],"reason":"r"}]}`)
	if _, err := parseCatalogSeed(unknownCap); err == nil || !strings.Contains(err.Error(), "vision") {
		t.Fatalf("应报出词表外的能力, got %v", err)
	}
}

// 实跑是全或无：渠道步骤失败时，前面已写入的目录行一并回滚，表状态与执行前一致。
func TestApplyCatalogSeedRollsBackOnFailure(t *testing.T) {
	setupSeedTestDB(t)
	entry := firstSeedEntry(t)
	seedFixture(t, entry)

	// 第二个启用渠道的 model_mapping 是坏 JSON：目录行写完之后、渠道改写阶段才会失败。
	badMapping := "{not json"
	weight := uint(1)
	broken := &Channel{Id: 2, Status: config.ChannelStatusEnabled, Name: "broken", Models: entry.Aliases[0], ModelMapping: &badMapping, Weight: &weight}
	if err := DB.Create(broken).Error; err != nil {
		t.Fatalf("预置坏渠道失败: %v", err)
	}

	before := snapshotModelInfoRows(t)
	if _, err := ApplyCatalogSeed(false); err == nil {
		t.Fatal("渠道改写失败时 apply_seed 应报错")
	}
	after := snapshotModelInfoRows(t)
	if len(before) != len(after) {
		t.Fatalf("回滚后目录行数应不变: before=%d after=%d", len(before), len(after))
	}
	for name, aliasOf := range before {
		if after[name] != aliasOf {
			t.Fatalf("回滚后 %s.alias_of = %q, want %q", name, after[name], aliasOf)
		}
	}
}

// snapshotModelInfoRows 取「模型名 → alias_of」的快照，用于比对回滚前后的表状态。
func snapshotModelInfoRows(t *testing.T) map[string]string {
	t.Helper()
	var infos []*ModelInfo
	if err := DB.Find(&infos).Error; err != nil {
		t.Fatalf("读取目录失败: %v", err)
	}
	snapshot := make(map[string]string, len(infos))
	for _, info := range infos {
		snapshot[info.Model] = info.AliasOf
	}
	return snapshot
}

// dry_run 返回摘要但不落库。
func TestApplyCatalogSeedDryRun(t *testing.T) {
	setupSeedTestDB(t)
	entry := firstSeedEntry(t)
	seedFixture(t, entry)

	result, err := ApplyCatalogSeed(true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if result.Created == 0 || len(result.AliasedModels) == 0 || result.ChannelsUpdated != 1 {
		t.Fatalf("unexpected summary: %+v", result)
	}

	var count int64
	DB.Model(&ModelInfo{}).Where("model = ?", entry.Canonical).Count(&count)
	if count != 0 {
		t.Fatal("dry run must not write model_info")
	}
	var channel Channel
	DB.First(&channel, 1)
	if channel.Models != entry.Aliases[0] {
		t.Fatalf("dry run must not write channel models, got %q", channel.Models)
	}
}

// 实跑：建主名、别名改写、清单外的行与 hidden 不动、渠道补主名与 model_mapping；再跑一次零变更。
func TestApplyCatalogSeedIdempotent(t *testing.T) {
	setupSeedTestDB(t)
	entry := firstSeedEntry(t)
	alias := entry.Aliases[0]
	seedFixture(t, entry)

	first, err := ApplyCatalogSeed(false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if first.Created == 0 || first.ChannelsUpdated != 1 {
		t.Fatalf("unexpected first summary: %+v", first)
	}

	canonical, err := GetModelInfoByModel(entry.Canonical)
	if err != nil {
		t.Fatalf("canonical row: %v", err)
	}
	if canonical.Hidden || canonical.AliasOf != "" || canonical.ContextLength != 1024 {
		t.Fatalf("canonical row not seeded from alias: %+v", canonical)
	}
	if canonical.VendorID != 900 {
		t.Fatalf("canonical row vendor = %d, want 900", canonical.VendorID)
	}
	// 模态与能力以白名单为准：别名行这几项是空的，主名行必须拿到种子里的取值。
	if canonical.InputModalities != catalogModalitiesToJSON(entry.InputModalities) ||
		canonical.OutputModalities != catalogModalitiesToJSON(entry.OutputModalities) ||
		canonical.Capabilities != ModelCapabilitiesJSON(entry.Capabilities) {
		t.Fatalf("canonical row 模态 / 能力未按种子写入: in=%q out=%q caps=%q",
			canonical.InputModalities, canonical.OutputModalities, canonical.Capabilities)
	}

	aliasRow, err := GetModelInfoByModel(alias)
	if err != nil {
		t.Fatalf("alias row: %v", err)
	}
	if aliasRow.Hidden || aliasRow.AliasOf != entry.Canonical {
		t.Fatalf("alias row not converted: %+v", aliasRow)
	}

	legacy, err := GetModelInfoByModel("legacy/model")
	if err != nil {
		t.Fatalf("legacy row: %v", err)
	}
	if legacy.Hidden || legacy.AliasOf != "" {
		t.Fatalf("rows outside the whitelist must stay untouched: %+v", legacy)
	}

	var channel Channel
	DB.First(&channel, 1)
	mapping := map[string]string{}
	if err := json.Unmarshal([]byte(channel.GetModelMapping()), &mapping); err != nil {
		t.Fatalf("mapping: %v", err)
	}
	if mapping[entry.Canonical] != alias {
		t.Fatalf("model_mapping not written: %v", mapping)
	}
	if channel.Models != alias+","+entry.Canonical {
		t.Fatalf("channel models = %q", channel.Models)
	}

	second, err := ApplyCatalogSeed(false)
	if err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if second.Created != 0 || len(second.AliasedModels) != 0 || second.ChannelsUpdated != 0 {
		t.Fatalf("second run must be a no-op: %+v", second)
	}
}

// 管理员锁定的行完全不被 apply_seed 改动：白名单里的锁定主名不被覆盖并计入 skipped_locked；
// 白名单外的行（含锁定行）本就不在种子管辖内，隐藏状态保持不变。
func TestApplyCatalogSeedSkipsLockedRows(t *testing.T) {
	setupSeedTestDB(t)
	entry := firstSeedEntry(t)
	seedFixture(t, entry)

	if err := DB.Create(&ModelInfo{Model: "private/model", Hidden: true, Locked: true}).Error; err != nil {
		t.Fatal(err)
	}
	if err := DB.Create(&ModelInfo{
		Model:   entry.Canonical,
		Name:    "Locked Canonical",
		Hidden:  true,
		Locked:  true,
		AliasOf: "legacy/model",
	}).Error; err != nil {
		t.Fatal(err)
	}

	dry, err := ApplyCatalogSeed(true)
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if dry.SkippedLocked != 1 {
		t.Fatalf("dry run skipped_locked = %d, want 1: %+v", dry.SkippedLocked, dry.SkippedLockedRows)
	}

	result, err := ApplyCatalogSeed(false)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if result.SkippedLocked != 1 {
		t.Fatalf("skipped_locked = %d, want 1: %+v", result.SkippedLocked, result.SkippedLockedRows)
	}

	private, err := GetModelInfoByModel("private/model")
	if err != nil {
		t.Fatalf("private row: %v", err)
	}
	if !private.Hidden {
		t.Fatal("rows outside the whitelist must keep their hidden state")
	}

	canonical, err := GetModelInfoByModel(entry.Canonical)
	if err != nil {
		t.Fatalf("canonical row: %v", err)
	}
	if !canonical.Hidden || canonical.AliasOf != "legacy/model" || canonical.VendorID != 0 {
		t.Fatalf("locked canonical row must not be overwritten: %+v", canonical)
	}
}
