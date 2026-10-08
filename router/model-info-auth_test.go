package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

const (
	modelInfoAdminToken  = "modelinfoadmintoken0000000000001"
	modelInfoCommonToken = "modelinfocommontoken000000000002"
)

// setupModelInfoRouter 用内存库 + 真实 SetApiRouter 挂全量 /api 路由，
// 并准备一个管理员与一个普通用户的 access token。
func setupModelInfoRouter(t *testing.T) *gin.Engine {
	t.Helper()
	testDB, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err := testDB.AutoMigrate(&model.User{}, &model.ModelInfo{}, &model.Channel{}); err != nil {
		t.Fatalf("迁移测试表失败: %v", err)
	}
	oldDB := model.DB
	model.DB = testDB
	t.Cleanup(func() { model.DB = oldDB })

	users := []*model.User{
		{
			Username: "catalog-admin", DisplayName: "admin", Role: config.RoleAdminUser,
			Status: config.UserStatusEnabled, AccessToken: modelInfoAdminToken, AffCode: "affadmin",
		},
		{
			Username: "catalog-user", DisplayName: "user", Role: config.RoleCommonUser,
			Status: config.UserStatusEnabled, AccessToken: modelInfoCommonToken, AffCode: "affuser",
		},
	}
	for _, u := range users {
		if err := testDB.Create(u).Error; err != nil {
			t.Fatalf("创建测试用户 %s 失败: %v", u.Username, err)
		}
	}
	if err := testDB.Create(&model.ModelInfo{Model: "gpt-5", Name: "GPT-5"}).Error; err != nil {
		t.Fatalf("创建测试模型元信息失败: %v", err)
	}

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	SetApiRouter(engine)
	return engine
}

// requestModelInfo 以给定 access token（空串表示匿名）请求目标路径。
func requestModelInfo(engine *gin.Engine, target, accessToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

// modelInfoSuccess 取响应体的 success 字段；非 JSON 视为失败。
func modelInfoSuccess(t *testing.T, w *httptest.ResponseRecorder) bool {
	t.Helper()
	var body struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		return false
	}
	return body.Success
}

// TestModelInfoListIsPublic 验证 GET /api/model_info/ 留在 AdminAuth 之前：
// 匿名可取（公开价格页依赖它），隐藏行由处理器过滤而非鉴权拦截。
func TestModelInfoListIsPublic(t *testing.T) {
	engine := setupModelInfoRouter(t)

	w := requestModelInfo(engine, "/api/model_info/", "")
	if w.Code != http.StatusOK || !modelInfoSuccess(t, w) {
		t.Fatalf("匿名请求列表期望 200 且 success=true，实际 %d（body=%s）", w.Code, w.Body.String())
	}
}

// TestModelInfoCatalogRequiresAdmin 验证管理端目录仍在 AdminAuth 之内：匿名 401，
// 普通用户被拒（现有 authHelper 对权限不足返回 200 + success:false），管理员放行。
func TestModelInfoCatalogRequiresAdmin(t *testing.T) {
	engine := setupModelInfoRouter(t)
	const target = "/api/model_info/catalog"

	t.Run("匿名被拒", func(t *testing.T) {
		w := requestModelInfo(engine, target, "")
		if w.Code != http.StatusUnauthorized {
			t.Fatalf("匿名请求 %s 期望 401，实际 %d（body=%s）", target, w.Code, w.Body.String())
		}
		if modelInfoSuccess(t, w) {
			t.Fatalf("匿名请求 %s 不应返回成功响应: %s", target, w.Body.String())
		}
	})

	t.Run("普通用户被拒", func(t *testing.T) {
		w := requestModelInfo(engine, target, modelInfoCommonToken)
		if w.Code != http.StatusOK && w.Code != http.StatusUnauthorized && w.Code != http.StatusForbidden {
			t.Fatalf("普通用户请求 %s 状态码异常: %d（body=%s）", target, w.Code, w.Body.String())
		}
		if modelInfoSuccess(t, w) {
			t.Fatalf("普通用户请求 %s 不应拿到数据: %s", target, w.Body.String())
		}
	})

	t.Run("管理员放行", func(t *testing.T) {
		w := requestModelInfo(engine, target, modelInfoAdminToken)
		if w.Code != http.StatusOK || !modelInfoSuccess(t, w) {
			t.Fatalf("管理员请求 %s 期望 200 且 success=true，实际 %d（body=%s）", target, w.Code, w.Body.String())
		}
	})
}

// TestModelInfoRoutesRegistered 直接检查路由表：路径没有被改名或漏注册，
// 鉴权层级由上面的请求级测试保证。
func TestModelInfoRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("test-secret"))))
	SetApiRouter(engine)

	var listFound, catalogFound bool
	for _, ri := range engine.Routes() {
		if ri.Method != http.MethodGet {
			continue
		}
		switch ri.Path {
		case "/api/model_info/":
			listFound = true
		case "/api/model_info/catalog":
			catalogFound = true
		}
	}
	if !listFound || !catalogFound {
		t.Fatalf("期望注册 GET /api/model_info/ 与 /api/model_info/catalog，实际 routes=%v", engine.Routes())
	}
}

// TestBrandIconAdminRoutesRequireAdmin 验证图标上传 / 刷新挂在 AdminAuth 之内：
// 匿名 401、普通用户被拒；管理员能进到处理器（非法 domain 由处理器返回 400）。
func TestBrandIconAdminRoutesRequireAdmin(t *testing.T) {
	engine := setupModelInfoRouter(t)
	post := func(target, body, accessToken string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if accessToken != "" {
			req.Header.Set("Authorization", "Bearer "+accessToken)
		}
		w := httptest.NewRecorder()
		engine.ServeHTTP(w, req)
		return w
	}

	for _, target := range []string{"/api/brand-icon/upload", "/api/brand-icon/refresh"} {
		if w := post(target, `{"domain":"localhost"}`, ""); w.Code != http.StatusUnauthorized || modelInfoSuccess(t, w) {
			t.Errorf("匿名请求 %s 期望 401，实际 %d（body=%s）", target, w.Code, w.Body.String())
		}
		if w := post(target, `{"domain":"localhost"}`, modelInfoCommonToken); modelInfoSuccess(t, w) ||
			w.Code == http.StatusBadRequest {
			t.Errorf("普通用户请求 %s 不应进入处理器: %d（body=%s）", target, w.Code, w.Body.String())
		}
	}
	if w := post("/api/brand-icon/refresh", `{"domain":"localhost"}`, modelInfoAdminToken); w.Code != http.StatusBadRequest {
		t.Errorf("管理员请求 refresh 应进入处理器并拒绝非法 domain，实际 %d（body=%s）", w.Code, w.Body.String())
	}
	if w := post("/api/brand-icon/upload", `{}`, modelInfoAdminToken); w.Code != http.StatusBadRequest {
		t.Errorf("管理员请求 upload 缺少文件应返回 400，实际 %d（body=%s）", w.Code, w.Body.String())
	}
}
