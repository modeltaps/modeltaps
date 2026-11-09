package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/middleware"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// setupOrgRoutingControllerTest creates organizations 1 and 2 (shadow users 11 and 12 in group
// "default") and a router that mounts the organization handlers behind the feature gate, with the
// organization taken from the X-Org header in place of OrgAuth.
func setupOrgRoutingControllerTest(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.Organization{}, &model.OrganizationAuditLog{},
		&model.RoutingPolicy{}, &model.ModelInfo{}); err != nil {
		t.Fatal(err)
	}
	prevDB, prevRule, prevEnabled := model.DB, model.ChannelGroup.Rule, config.OrganizationRoutingPolicyEnabled
	model.DB = testDB
	model.ChannelGroup.Rule = map[string]map[string][][]int{"default": {"decider": {{1}}, "cheap-model": {{1}}, "big-model": {{1}}}}
	config.OrganizationRoutingPolicyEnabled = true
	model.ResetRoutingPolicyCache()
	t.Cleanup(func() {
		model.DB, model.ChannelGroup.Rule, config.OrganizationRoutingPolicyEnabled = prevDB, prevRule, prevEnabled
		model.ResetRoutingPolicyCache()
	})
	orgs := map[string]*model.Organization{}
	for i := 1; i <= 2; i++ {
		shadow := &model.User{Id: 10 + i, Username: "shadow" + strconv.Itoa(i), AccessToken: "t" + strconv.Itoa(i), AffCode: "a" + strconv.Itoa(i), Group: "default"}
		if err := testDB.Create(shadow).Error; err != nil {
			t.Fatal(err)
		}
		org := &model.Organization{Id: i, Name: "Org", Slug: "org" + strconv.Itoa(i), Status: model.OrganizationStatusEnabled, ShadowUserId: shadow.Id, CreatedBy: 1}
		if err := testDB.Create(org).Error; err != nil {
			t.Fatal(err)
		}
		orgs[strconv.Itoa(i)] = org
	}
	r := gin.New()
	group := r.Group("/api/org/:id/routing_policy", func(c *gin.Context) {
		c.Set("id", 1)
		c.Set(middleware.OrgContextKey, orgs[c.Param("id")])
		c.Next()
	}, middleware.OrgRoutingPolicyEnabled())
	group.GET("/", GetOrgRoutingPolicies)
	group.POST("/", CreateOrgRoutingPolicy)
	group.GET("/:policy_id", GetOrgRoutingPolicy)
	group.PUT("/:policy_id", UpdateOrgRoutingPolicy)
	group.PUT("/:policy_id/status", UpdateOrgRoutingPolicyStatus)
	group.DELETE("/:policy_id", DeleteOrgRoutingPolicy)
	return r
}

func orgRoutingCall(t *testing.T, r *gin.Engine, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	resp := map[string]any{}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	return w.Code, resp
}

func orgRoutingDraft(name string) map[string]any {
	return map[string]any{
		"name": name, "decision_model": "decider", "fallback_model": "big-model", "threshold": 0.5,
		// A client-supplied organization_id is ignored: the path decides the owner.
		"organization_id": 2,
		"routes": []map[string]any{
			{"label": "small", "target_model": "cheap-model"},
			{"label": "large", "target_model": "big-model"},
		},
	}
}

func TestOrgRoutingPolicyEndpoints(t *testing.T) {
	r := setupOrgRoutingControllerTest(t)

	code, resp := orgRoutingCall(t, r, http.MethodPost, "/api/org/1/routing_policy/", orgRoutingDraft("auto/coder"))
	if code != http.StatusOK {
		t.Fatalf("create: %d %v", code, resp)
	}
	data := resp["data"].(map[string]any)
	id := int(data["id"].(float64))
	if int(data["organization_id"].(float64)) != 1 {
		t.Fatalf("the policy belongs to the organization in the path, got %v", data["organization_id"])
	}

	// Another organization sees neither the policy nor its id.
	if code, _ := orgRoutingCall(t, r, http.MethodGet, "/api/org/2/routing_policy/"+strconv.Itoa(id), nil); code != http.StatusNotFound {
		t.Fatalf("cross-organization read: want 404, got %d", code)
	}
	if code, _ := orgRoutingCall(t, r, http.MethodDelete, "/api/org/2/routing_policy/"+strconv.Itoa(id), nil); code != http.StatusNotFound {
		t.Fatalf("cross-organization delete: want 404, got %d", code)
	}
	if code, _ := orgRoutingCall(t, r, http.MethodPut, "/api/org/2/routing_policy/"+strconv.Itoa(id)+"/status", map[string]any{"enabled": false}); code != http.StatusNotFound {
		t.Fatalf("cross-organization status change: want 404, got %d", code)
	}
	_, list := orgRoutingCall(t, r, http.MethodGet, "/api/org/2/routing_policy/", nil)
	if items := list["data"].([]any); len(items) != 0 {
		t.Fatalf("organization 2 should list nothing, got %v", items)
	}

	// Organization 2 may use the same name.
	if code, resp := orgRoutingCall(t, r, http.MethodPost, "/api/org/2/routing_policy/", orgRoutingDraft("auto/coder")); code != http.StatusOK {
		t.Fatalf("same name in another organization: %d %v", code, resp)
	}

	// A model outside the organization's group is refused.
	bad := orgRoutingDraft("auto/bad")
	bad["fallback_model"] = "elsewhere-model"
	if code, resp := orgRoutingCall(t, r, http.MethodPost, "/api/org/1/routing_policy/", bad); code != http.StatusBadRequest {
		t.Fatalf("unavailable model: want 400, got %d %v", code, resp)
	}

	if code, _ := orgRoutingCall(t, r, http.MethodPut, "/api/org/1/routing_policy/"+strconv.Itoa(id)+"/status", map[string]any{"enabled": false}); code != http.StatusOK {
		t.Fatalf("status: %d", code)
	}
	if code, _ := orgRoutingCall(t, r, http.MethodDelete, "/api/org/1/routing_policy/"+strconv.Itoa(id), nil); code != http.StatusOK {
		t.Fatalf("delete: %d", code)
	}

	var actions []string
	model.DB.Model(&model.OrganizationAuditLog{}).Where("organization_id = ?", 1).Order("id").Pluck("action", &actions)
	want := []string{"routing_policy.create", "routing_policy.status", "routing_policy.delete"}
	if len(actions) != len(want) {
		t.Fatalf("audit actions: want %v, got %v", want, actions)
	}
	for i := range want {
		if actions[i] != want[i] {
			t.Fatalf("audit actions: want %v, got %v", want, actions)
		}
	}

	// With the site option off the endpoints are closed.
	config.OrganizationRoutingPolicyEnabled = false
	if code, resp := orgRoutingCall(t, r, http.MethodGet, "/api/org/1/routing_policy/", nil); code != http.StatusForbidden || resp["code"] != "org_routing_policy_disabled" {
		t.Fatalf("option off: want 403 org_routing_policy_disabled, got %d %v", code, resp)
	}
}
