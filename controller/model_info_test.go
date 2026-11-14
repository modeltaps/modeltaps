package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupModelInfoVisibilityDB 装配只含 model_info 的内存库，并打开隐藏约束。
func setupModelInfoVisibilityDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.ModelInfo{}, &model.ModelCatalogAuditLog{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	old := viper.GetBool("catalog.enforce_hidden")
	viper.Set("catalog.enforce_hidden", true)
	t.Cleanup(func() {
		model.DB = oldDB
		viper.Set("catalog.enforce_hidden", old)
	})
}

// 公开的 GET /api/model_info/ 无条件只暴露有渠道可路由、未隐藏的主名：隐藏行、无渠道行与
// 别名行不出现，且不受 catalog.enforce_hidden 影响（该开关只是 relay 侧的应急回滚阀门）。
func TestGetAllModelInfo_OnlyVisibleCanonical(t *testing.T) {
	prevModelGroup := model.ChannelGroup.ModelGroup
	model.ChannelGroup.ModelGroup = map[string]map[string]bool{
		"gpt-5":            {"default": true},
		"openai/gpt-5":     {"default": true},
		"internal-preview": {"default": true},
	}
	t.Cleanup(func() { model.ChannelGroup.ModelGroup = prevModelGroup })
	for _, enforce := range []bool{true, false} {
		t.Run("enforce_hidden="+strconv.FormatBool(enforce), func(t *testing.T) {
			setupModelInfoVisibilityDB(t)
			viper.Set("catalog.enforce_hidden", enforce)
			for _, row := range []*model.ModelInfo{
				{Model: "gpt-5"},
				{Model: "openai/gpt-5", AliasOf: "gpt-5"},
				{Model: "internal-preview", Hidden: true},
				{Model: "unrouted-model"},
			} {
				if err := model.DB.Create(row).Error; err != nil {
					t.Fatalf("建行失败: %v", err)
				}
			}

			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodGet, "/api/model_info/", nil)
			GetAllModelInfo(c)

			var resp struct {
				Success bool             `json:"success"`
				Data    []map[string]any `json:"data"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("解析响应失败: %v (body=%s)", err, rec.Body.String())
			}
			if !resp.Success {
				t.Fatalf("响应应成功: %s", rec.Body.String())
			}
			if len(resp.Data) != 1 || resp.Data[0]["model"] != "gpt-5" {
				t.Fatalf("公开列表应只含 gpt-5，实际 %s", rec.Body.String())
			}
			for _, field := range []string{"locked", "vendor_id", "alias_of"} {
				if _, ok := resp.Data[0][field]; ok {
					t.Fatalf("公开列表不应含管理字段 %s: %s", field, rec.Body.String())
				}
			}
			for _, field := range []string{"input_modalities", "output_modalities", "tags", "capabilities", "description"} {
				if _, ok := resp.Data[0][field]; !ok {
					t.Fatalf("公开列表缺少展示字段 %s: %s", field, rec.Body.String())
				}
			}
		})
	}
}

// postModelInfo 用给定 JSON 体调用写接口，返回状态码与响应体。
func postModelInfo(t *testing.T, handler gin.HandlerFunc, body string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/model_info/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	handler(c)

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("解析响应失败: %v (body=%s)", err, rec.Body.String())
	}
	return rec.Code, payload
}

// alias_of 写入校验：自指、指向不存在的模型、指向别名（含成环）都必须 400。
func TestModelInfoAliasTargetValidation(t *testing.T) {
	setupModelInfoVisibilityDB(t)
	for _, row := range []*model.ModelInfo{
		{Id: 1, Model: "gpt-5"},
		{Id: 2, Model: "openai/gpt-5", AliasOf: "gpt-5"},
	} {
		if err := model.DB.Create(row).Error; err != nil {
			t.Fatalf("建行失败: %v", err)
		}
	}

	cases := []struct {
		name string
		body string
	}{
		{"自指", `{"model":"self-loop","alias_of":"self-loop"}`},
		{"指向不存在", `{"model":"dangling","alias_of":"nope"}`},
		{"指向别名", `{"model":"second-hop","alias_of":"openai/gpt-5"}`},
	}
	for _, tc := range cases {
		if code, body := postModelInfo(t, CreateModelInfo, tc.body); code != http.StatusBadRequest {
			t.Fatalf("%s 应 400, got %d %v", tc.name, code, body)
		}
	}

	// 成环：把主名改成指向自己的别名行，等价于「目标是别名」，同样被拒。
	cycle := `{"id":1,"model":"gpt-5","alias_of":"openai/gpt-5"}`
	if code, body := postModelInfo(t, UpdateModelInfo, cycle); code != http.StatusBadRequest {
		t.Fatalf("成环应 400, got %d %v", code, body)
	}
}

// 被别名引用的主名不能改名、不能删除：拒绝并列出引用方。
func TestModelInfoRejectsRenameAndDeleteWithAliases(t *testing.T) {
	setupModelInfoVisibilityDB(t)
	for _, row := range []*model.ModelInfo{
		{Id: 1, Model: "gpt-5"},
		{Id: 2, Model: "openai/gpt-5", AliasOf: "gpt-5"},
	} {
		if err := model.DB.Create(row).Error; err != nil {
			t.Fatalf("建行失败: %v", err)
		}
	}

	code, body := postModelInfo(t, UpdateModelInfo, `{"id":1,"model":"gpt-5.1"}`)
	if code != http.StatusBadRequest {
		t.Fatalf("改名应 400, got %d %v", code, body)
	}
	refs, _ := body["data"].([]any)
	if len(refs) != 1 || refs[0] != "openai/gpt-5" {
		t.Fatalf("应列出引用方, got %v", body["data"])
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Params = gin.Params{{Key: "id", Value: "1"}}
	c.Request = httptest.NewRequest(http.MethodDelete, "/api/model_info/1", nil)
	DeleteModelInfo(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("删除应 400, got %d %s", rec.Code, rec.Body.String())
	}
	var still model.ModelInfo
	if err := model.DB.First(&still, 1).Error; err != nil {
		t.Fatalf("被引用的主名不应被删除: %v", err)
	}
}

// 隐藏 / 取消隐藏必须留痕：每个状态真正变化的条目一条审计，未变化的不记。
func TestHideModelInfoWritesAudit(t *testing.T) {
	setupModelInfoVisibilityDB(t)
	for _, row := range []*model.ModelInfo{
		{Id: 1, Model: "gpt-5"},
		{Id: 2, Model: "gemini-2.5-pro", Hidden: true},
		{Id: 3, Model: "claude-fable-5"},
	} {
		if err := model.DB.Create(row).Error; err != nil {
			t.Fatalf("建行失败: %v", err)
		}
	}

	hide := func(body string) {
		t.Helper()
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("id", 42)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/model_info/hide", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		HideModelInfo(c)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":true`) {
			t.Fatalf("隐藏应成功, got %d %s", rec.Code, rec.Body.String())
		}
	}

	hide(`{"ids":[1,2],"hidden":true}`)
	var logs []*model.ModelCatalogAuditLog
	if err := model.DB.Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("读取审计失败: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("只有状态变化的条目应留痕，实际 %d 条", len(logs))
	}
	entry := logs[0]
	if entry.ActorId != 42 || entry.Model != "gpt-5" || entry.Action != model.ModelCatalogActionHide {
		t.Fatalf("审计内容不符: %+v", entry)
	}
	if entry.Before != `{"hidden":false,"status":"draft"}` || entry.After != `{"hidden":true,"status":"draft"}` {
		t.Fatalf("审计应记录改前改后: before=%s after=%s", entry.Before, entry.After)
	}

	hide(`{"ids":[1],"hidden":false}`)
	logs = nil
	if err := model.DB.Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("读取审计失败: %v", err)
	}
	if len(logs) != 2 || logs[1].Action != model.ModelCatalogActionUnhide || logs[1].After != `{"hidden":false,"status":"published"}` {
		t.Fatalf("取消隐藏应留痕, got %+v", logs)
	}

	// Enabling a draft that was never hidden still publishes it, so it is recorded too.
	hide(`{"ids":[3],"hidden":false}`)
	logs = nil
	if err := model.DB.Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("读取审计失败: %v", err)
	}
	if len(logs) != 3 || logs[2].Model != "claude-fable-5" || logs[2].Before != `{"hidden":false,"status":"draft"}` {
		t.Fatalf("enabling a draft should be audited, got %+v", logs)
	}
}

// apply_seed：dry_run 不留痕（什么都没改），实跑记一条整体动作。
func TestApplyModelInfoSeedAudit(t *testing.T) {
	setupModelInfoVisibilityDB(t)
	if err := model.DB.AutoMigrate(&model.Channel{}, &model.ModelOwnedBy{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldInstance := model.ModelOwnedBysInstance
	model.ModelOwnedBysInstance = &model.ModelOwnedBys{}
	if err := model.ModelOwnedBysInstance.Load(); err != nil {
		t.Fatalf("加载厂商失败: %v", err)
	}
	t.Cleanup(func() { model.ModelOwnedBysInstance = oldInstance })

	applySeed := func(body string) {
		t.Helper()
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		c.Set("id", 7)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/model_info/apply_seed", strings.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		ApplyModelInfoSeed(c)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"success":true`) {
			t.Fatalf("apply_seed 应成功, got %d %s", rec.Code, rec.Body.String())
		}
	}

	applySeed(`{"dry_run":true}`)
	var count int64
	if err := model.DB.Model(&model.ModelCatalogAuditLog{}).Count(&count).Error; err != nil {
		t.Fatalf("统计审计失败: %v", err)
	}
	if count != 0 {
		t.Fatalf("dry_run 不应留痕，实际 %d 条", count)
	}

	applySeed(`{"dry_run":false}`)
	var logs []*model.ModelCatalogAuditLog
	if err := model.DB.Find(&logs).Error; err != nil {
		t.Fatalf("读取审计失败: %v", err)
	}
	if len(logs) != 1 || logs[0].Action != model.ModelCatalogActionApplySeed || logs[0].ActorId != 7 {
		t.Fatalf("实跑应记一条 apply_seed 审计, got %+v", logs)
	}
	if logs[0].After == "" {
		t.Fatal("审计应带上变更摘要")
	}
}

// 管理端目录下发 vendor_slug：厂商展示名可以随时改写，按厂商归类只能认 slug。
func TestGetModelInfoCatalogExposesVendorSlug(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.ModelInfo{}, &model.ModelOwnedBy{}, &model.Channel{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB, oldInstance := model.DB, model.ModelOwnedBysInstance
	model.DB = testDB
	t.Cleanup(func() { model.DB, model.ModelOwnedBysInstance = oldDB, oldInstance })

	if err := testDB.Create(&model.ModelOwnedBy{Id: 25, Name: "Google Gemini", Slug: "google"}).Error; err != nil {
		t.Fatalf("预置厂商失败: %v", err)
	}
	if err := testDB.Create(&model.ModelInfo{Model: "gemini-2.5-pro", VendorID: 25}).Error; err != nil {
		t.Fatalf("预置目录行失败: %v", err)
	}
	model.ModelOwnedBysInstance = &model.ModelOwnedBys{}
	if err := model.ModelOwnedBysInstance.Load(); err != nil {
		t.Fatalf("加载厂商失败: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/model_info/catalog", nil)
	GetModelInfoCatalog(c)

	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Model      string `json:"model"`
			VendorSlug string `json:"vendor_slug"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v (body=%s)", err, rec.Body.String())
	}
	if !resp.Success || len(resp.Data) != 1 {
		t.Fatalf("响应不符: %s", rec.Body.String())
	}
	if resp.Data[0].VendorSlug != "google" {
		t.Fatalf("vendor_slug = %q, want google", resp.Data[0].VendorSlug)
	}
}

// 目录的 bound_channels 同时列出启用与禁用渠道并带路由信息；state 仍只认启用渠道。
func TestGetModelInfoCatalogBoundChannelsIncludeDisabled(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.ModelInfo{}, &model.ModelOwnedBy{}, &model.Channel{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB, oldInstance := model.DB, model.ModelOwnedBysInstance
	model.DB = testDB
	model.ModelOwnedBysInstance = &model.ModelOwnedBys{}
	t.Cleanup(func() { model.DB, model.ModelOwnedBysInstance = oldDB, oldInstance })

	for _, info := range []*model.ModelInfo{{Model: "gpt-a"}, {Model: "gpt-b"}} {
		if err := testDB.Create(info).Error; err != nil {
			t.Fatalf("预置目录行失败: %v", err)
		}
	}
	priority, weight := int64(7), uint(3)
	channels := []*model.Channel{
		{Id: 1, Name: "on", Type: 1, Status: 1, Group: "default,vip", Models: "gpt-a", Priority: &priority, Weight: &weight},
		{Id: 2, Name: "off", Type: 14, Status: 2, Group: "default", Models: "gpt-a,gpt-b"},
	}
	for _, channel := range channels {
		if err := testDB.Create(channel).Error; err != nil {
			t.Fatalf("预置渠道失败: %v", err)
		}
	}
	if err := testDB.Model(&model.Channel{}).Where("id = ?", 2).Update("status", 2).Error; err != nil {
		t.Fatalf("禁用渠道失败: %v", err)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/model_info/catalog", nil)
	GetModelInfoCatalog(c)

	var resp struct {
		Success bool `json:"success"`
		Data    []struct {
			Model         string         `json:"model"`
			State         string         `json:"state"`
			BoundChannels []boundChannel `json:"bound_channels"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应失败: %v (body=%s)", err, rec.Body.String())
	}
	byModel := map[string]int{}
	for i, item := range resp.Data {
		byModel[item.Model] = i
	}
	a, b := resp.Data[byModel["gpt-a"]], resp.Data[byModel["gpt-b"]]
	if a.State != modelInfoStateVisible || len(a.BoundChannels) != 2 {
		t.Fatalf("gpt-a 应可见且列出 2 个渠道: %+v", a)
	}
	for _, ch := range a.BoundChannels {
		switch ch.Id {
		case 1:
			if ch.Status != 1 || ch.Group != "default,vip" || ch.Priority != 7 || ch.Weight != 3 {
				t.Fatalf("启用渠道摘要不符: %+v", ch)
			}
		case 2:
			if ch.Status != 2 || ch.Type != 14 {
				t.Fatalf("禁用渠道摘要不符: %+v", ch)
			}
		}
	}
	if b.State != modelInfoStateUnrouted || len(b.BoundChannels) != 1 || b.BoundChannels[0].Status != 2 {
		t.Fatalf("gpt-b 只有禁用渠道，应为 unrouted 且仍列出该渠道: %+v", b)
	}
}

// 目录状态四态：别名优先，其次隐藏，未隐藏的按有无渠道分为可见 / 无渠道。
func TestModelInfoState(t *testing.T) {
	cases := []struct {
		name     string
		info     *model.ModelInfo
		channels int
		want     string
	}{
		{"alias", &model.ModelInfo{Model: "gemini", AliasOf: "google/gemini", Hidden: true}, 2, modelInfoStateAlias},
		{"visible", &model.ModelInfo{Model: "gpt-5"}, 1, modelInfoStateVisible},
		{"unrouted", &model.ModelInfo{Model: "gpt-5"}, 0, modelInfoStateUnrouted},
		{"hidden", &model.ModelInfo{Model: "gpt-5", Hidden: true}, 3, modelInfoStateHidden},
		{"hidden unrouted", &model.ModelInfo{Model: "gpt-5", Hidden: true}, 0, modelInfoStateHidden},
	}
	for _, c := range cases {
		if got := modelInfoState(c.info, c.channels); got != c.want {
			t.Errorf("%s: modelInfoState = %q, want %q", c.name, got, c.want)
		}
	}
}

func putModelInfo(t *testing.T, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPut, "/api/model_info/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	UpdateModelInfo(c)
	return w
}

// The console saves catalog rows whole and does not know about status or supply_mode yet: a
// save without them keeps the stored values, and unknown values are refused.
func TestUpdateModelInfoKeepsSupplyFields(t *testing.T) {
	setupModelInfoVisibilityDB(t)
	row := &model.ModelInfo{Model: "gpt-5", Status: model.ModelStatusPublished, SupplyMode: model.SupplyModeFailover}
	if err := model.DB.Create(row).Error; err != nil {
		t.Fatal(err)
	}

	w := putModelInfo(t, `{"id":`+strconv.Itoa(row.Id)+`,"model":"gpt-5","name":"GPT-5","description":"edited"}`)
	if !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("save failed: %s", w.Body.String())
	}
	saved, _ := model.GetModelInfo(row.Id)
	if saved.Status != model.ModelStatusPublished || saved.SupplyMode != model.SupplyModeFailover || saved.Description != "edited" {
		t.Fatalf("saved = status %q mode %q description %q", saved.Status, saved.SupplyMode, saved.Description)
	}

	for _, body := range []string{
		`{"id":` + strconv.Itoa(row.Id) + `,"model":"gpt-5","status":"live"}`,
		`{"id":` + strconv.Itoa(row.Id) + `,"model":"gpt-5","supply_mode":"random"}`,
	} {
		if w := putModelInfo(t, body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (%s)", body, w.Code, w.Body.String())
		}
	}
}

// With supplies routing, the public catalog lists published models only.
func TestPublicCatalogFollowsStatusWithSupplies(t *testing.T) {
	old := config.ModelSupplyEnabled
	config.ModelSupplyEnabled = true
	t.Cleanup(func() { config.ModelSupplyEnabled = old })
	routable := map[string]map[string]bool{"pub": {"default": true}, "drafted": {"default": true}, "old": {"default": true}, "hidden-pub": {"default": true}}
	infos := []*model.ModelInfo{
		{Model: "pub", Status: model.ModelStatusPublished},
		{Model: "drafted", Status: model.ModelStatusDraft},
		{Model: "old", Status: model.ModelStatusRetired},
		// hidden is no longer read once status decides
		{Model: "hidden-pub", Status: model.ModelStatusPublished, Hidden: true},
	}
	listed := map[string]bool{}
	for _, info := range visibleModelInfos(infos, routable) {
		listed[info.Model] = true
	}
	if !listed["pub"] || !listed["hidden-pub"] || listed["drafted"] || listed["old"] {
		t.Fatalf("listed = %v, want the two published models", listed)
	}
}

// While supplies route requests, a model's channels are those of its enabled supplies, with the
// supply's priority and weight, whatever the channels' own model lists say.
func TestGetModelInfoCatalogBoundChannelsFollowSupplies(t *testing.T) {
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.ModelInfo{}, &model.ModelOwnedBy{}, &model.Channel{}, &model.ModelSupply{}); err != nil {
		t.Fatal(err)
	}
	oldDB, oldInstance, oldEnabled := model.DB, model.ModelOwnedBysInstance, config.ModelSupplyEnabled
	model.DB = testDB
	model.ModelOwnedBysInstance = &model.ModelOwnedBys{}
	config.ModelSupplyEnabled = true
	t.Cleanup(func() { model.DB, model.ModelOwnedBysInstance, config.ModelSupplyEnabled = oldDB, oldInstance, oldEnabled })

	for _, info := range []*model.ModelInfo{{Model: "gpt-a", Status: model.ModelStatusPublished}, {Model: "gpt-b", Status: model.ModelStatusPublished}} {
		if err := testDB.Create(info).Error; err != nil {
			t.Fatal(err)
		}
	}
	for _, channel := range []*model.Channel{
		{Id: 1, Name: "lists-a", Type: 1, Status: 1, Group: "default", Models: "gpt-a"},
		{Id: 2, Name: "off", Type: 14, Status: 1, Group: "default"},
		{Id: 3, Name: "supplies-b", Type: 1, Status: 1, Group: "default,vip"},
	} {
		if err := testDB.Create(channel).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := testDB.Model(&model.Channel{}).Where("id = ?", 2).Update("status", 2).Error; err != nil {
		t.Fatal(err)
	}
	for _, supply := range []*model.ModelSupply{
		{Model: "gpt-a", ChannelId: 1, UpstreamModel: "gpt-a", Enabled: false},
		{Model: "gpt-b", ChannelId: 3, UpstreamModel: "up-b", Priority: 5, Weight: 2, Enabled: true},
		{Model: "gpt-b", ChannelId: 2, UpstreamModel: "up-b", Enabled: true},
		{Model: "gpt-b", ChannelId: 99, UpstreamModel: "up-b", Enabled: true},
	} {
		if err := testDB.Create(supply).Error; err != nil {
			t.Fatal(err)
		}
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/model_info/catalog", nil)
	GetModelInfoCatalog(c)

	var resp struct {
		SupplyRouting bool `json:"supply_routing"`
		Data          []struct {
			Model         string         `json:"model"`
			State         string         `json:"state"`
			BoundChannels []boundChannel `json:"bound_channels"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v (body=%s)", err, rec.Body.String())
	}
	if !resp.SupplyRouting {
		t.Fatal("supply_routing = false with ModelSupplyEnabled on")
	}
	byModel := map[string]int{}
	for i, item := range resp.Data {
		byModel[item.Model] = i
	}
	a, b := resp.Data[byModel["gpt-a"]], resp.Data[byModel["gpt-b"]]
	if a.State != modelInfoStateUnrouted || len(a.BoundChannels) != 0 {
		t.Fatalf("gpt-a has only a disabled supply (a channel listing it does not count): %+v", a)
	}
	if b.State != modelInfoStateVisible || len(b.BoundChannels) != 2 {
		t.Fatalf("gpt-b should list its two supplies' channels (not the deleted one): %+v", b)
	}
	for _, ch := range b.BoundChannels {
		switch ch.Id {
		case 3:
			if ch.Status != 1 || ch.Group != "default,vip" || ch.Priority != 5 || ch.Weight != 2 {
				t.Fatalf("supplied channel = %+v, want the supply's priority and weight", ch)
			}
		case 2:
			if ch.Status != 2 {
				t.Fatalf("disabled channel = %+v", ch)
			}
		default:
			t.Fatalf("unexpected channel %+v", ch)
		}
	}
}
