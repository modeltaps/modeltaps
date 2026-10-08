package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

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
	if entry.Before != `{"hidden":false}` || entry.After != `{"hidden":true}` {
		t.Fatalf("审计应记录改前改后: before=%s after=%s", entry.Before, entry.After)
	}

	hide(`{"ids":[1],"hidden":false}`)
	logs = nil
	if err := model.DB.Order("id asc").Find(&logs).Error; err != nil {
		t.Fatalf("读取审计失败: %v", err)
	}
	if len(logs) != 2 || logs[1].Action != model.ModelCatalogActionUnhide || logs[1].After != `{"hidden":false}` {
		t.Fatalf("取消隐藏应留痕, got %+v", logs)
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
