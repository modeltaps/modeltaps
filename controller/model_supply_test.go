package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func setupSupplyHandlerDB(t *testing.T) {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := testDB.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := testDB.AutoMigrate(&model.Channel{}, &model.ModelInfo{}, &model.Price{}, &model.ModelSupply{},
		&model.RoutingPolicy{}, &model.ModelCatalogAuditLog{}); err != nil {
		t.Fatal(err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })
}

func callSupplyHandler(t *testing.T, handler gin.HandlerFunc, method, body string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/api/model_supply/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	handler(c)
	var resp map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w, resp
}

// The migration endpoint is a dry run unless told otherwise, and applying writes supplies.
func TestMigrateModelSuppliesHandler(t *testing.T) {
	setupSupplyHandlerDB(t)
	weight := uint(1)
	priority := int64(10)
	mapping := `{"gpt-4o-mini":"openai/gpt-4o-mini"}`
	if err := model.DB.Create(&model.Channel{Name: "c", Status: config.ChannelStatusEnabled, Group: "default",
		Weight: &weight, Priority: &priority, Models: "gpt-4o-mini", ModelMapping: &mapping}).Error; err != nil {
		t.Fatal(err)
	}

	for _, body := range []string{"", `{}`, `{"dry_run":true}`} {
		_, resp := callSupplyHandler(t, MigrateModelSupplies, http.MethodPost, body)
		data, _ := resp["data"].(map[string]any)
		if resp["success"] != true || data["dry_run"] != true || data["supplies_planned"] != float64(1) {
			t.Fatalf("body %q: response %v, want a dry run planning one supply", body, resp)
		}
	}
	if list, _ := model.GetModelSupplies("", 0); len(list) != 0 {
		t.Fatalf("dry runs wrote %d supplies", len(list))
	}

	_, resp := callSupplyHandler(t, MigrateModelSupplies, http.MethodPost, `{"dry_run":false}`)
	if resp["success"] != true {
		t.Fatalf("apply failed: %v", resp)
	}
	list, _ := model.GetModelSupplies("gpt-4o-mini", 0)
	if len(list) != 1 || list[0].UpstreamModel != "openai/gpt-4o-mini" || list[0].Priority != 10 {
		t.Fatalf("supplies after apply = %+v", list)
	}
}

// Validation errors come back with their API codes; enabled defaults to true on create.
func TestModelSupplyHandlers(t *testing.T) {
	setupSupplyHandlerDB(t)
	weight := uint(1)
	channel := &model.Channel{Name: "c", Status: config.ChannelStatusEnabled, Weight: &weight}
	if err := model.DB.Create(channel).Error; err != nil {
		t.Fatal(err)
	}
	if err := model.DB.Create(&model.ModelInfo{Model: "m", Status: model.ModelStatusPublished}).Error; err != nil {
		t.Fatal(err)
	}

	w, resp := callSupplyHandler(t, CreateModelSupply, http.MethodPost, `{"model":"missing","channel_id":1,"upstream_model":"x"}`)
	if w.Code != http.StatusBadRequest || resp["code"] != "model_supply_model_not_found" {
		t.Fatalf("unknown model: %d %v", w.Code, resp)
	}

	_, resp = callSupplyHandler(t, CreateModelSupply, http.MethodPost, `{"model":"m","channel_id":1,"upstream_model":"vendor/m","priority":3}`)
	data, _ := resp["data"].(map[string]any)
	if resp["success"] != true || data["enabled"] != true {
		t.Fatalf("create: %v, want an enabled supply", resp)
	}
	id := int(data["id"].(float64))

	w, resp = callSupplyHandler(t, CreateModelSupply, http.MethodPost, `{"model":"m","channel_id":1,"upstream_model":"other"}`)
	if w.Code != http.StatusBadRequest || resp["code"] != "model_supply_exists" {
		t.Fatalf("duplicate: %d %v", w.Code, resp)
	}

	// Leaving enabled out of an update keeps it.
	_, resp = callSupplyHandler(t, UpdateModelSupply, http.MethodPut, `{"id":`+itoa(id)+`,"upstream_model":"vendor/m2","priority":7,"weight":2}`)
	data, _ = resp["data"].(map[string]any)
	if resp["success"] != true || data["enabled"] != true || data["upstream_model"] != "vendor/m2" || data["priority"] != float64(7) {
		t.Fatalf("update: %v", resp)
	}
}

func itoa(i int) string {
	b, _ := json.Marshal(i)
	return string(b)
}
