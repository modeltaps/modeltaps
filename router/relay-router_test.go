package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// TestSetOpenAIRouter_ModelsCatchAll 验证 /v1/models 检索路由用 *model 通配注册，且与同级
// GET /v1/models、DELETE /v1/models/:model 共存时 gin 不 panic。
func TestSetOpenAIRouter_ModelsCatchAll(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("setOpenAIRouter panicked during route registration: %v", r)
		}
	}()

	engine := gin.New()
	setOpenAIRouter(engine)

	var retrieve, del bool
	for _, ri := range engine.Routes() {
		switch {
		case ri.Method == http.MethodGet && ri.Path == "/v1/models/*model":
			retrieve = true
		case ri.Method == http.MethodDelete && ri.Path == "/v1/models/:model":
			del = true
		}
	}
	if !retrieve {
		t.Fatalf("expected GET /v1/models/*model to be registered, routes=%v", engine.Routes())
	}
	if !del {
		t.Fatalf("expected DELETE /v1/models/:model to stay unchanged, routes=%v", engine.Routes())
	}
}

// TestModelsCatchAll_MatchesSlashAlias 验证通配路由能匹配含斜杠的别名（OpenRouter 风格），
// 且 handler 去掉前导斜杠后拿到完整模型名 —— 单段 :model 在这里会直接 404。
func TestModelsCatchAll_MatchesSlashAlias(t *testing.T) {
	engine := gin.New()

	var gotModel string
	engine.GET("/v1/models", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.GET("/v1/models/*model", func(c *gin.Context) {
		gotModel = strings.TrimPrefix(c.Param("model"), "/")
		c.Status(http.StatusOK)
	})
	engine.NoRoute(func(c *gin.Context) { c.String(http.StatusNotFound, "NOROUTE") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models/google/gemini-2.5-pro", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Body.String() == "NOROUTE" {
		t.Fatalf("catch-all did not match slash alias: status=%d body=%q", w.Code, w.Body.String())
	}
	if gotModel != "google/gemini-2.5-pro" {
		t.Fatalf("model param: got %q want google/gemini-2.5-pro", gotModel)
	}
}

// TestSetGeminiRouter_RegistersCatchAll 验证 Gemini 路由用 *action 通配注册，且与同级 GET
// /:version/models 共存时 gin 不 panic。含斜杠模型名（如 ai21/jamba-large-1.7）依赖该通配匹配。
func TestSetGeminiRouter_RegistersCatchAll(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("setGeminiRouter panicked during route registration: %v", r)
		}
	}()

	engine := gin.New()
	setGeminiRouter(engine)

	var found bool
	for _, ri := range engine.Routes() {
		if ri.Method == http.MethodPost && ri.Path == "/gemini/:version/models/*action" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected POST /gemini/:version/models/*action to be registered, routes=%v", engine.Routes())
	}
}

// TestGeminiCatchAll_MatchesSlashModel 验证 *action 通配能匹配含斜杠的模型路径，命中已注册
// handler 而非落到 NoRoute（SPA index.html 兜底）；这正是修复"含斜杠模型名返回 HTML"的关键。
func TestGeminiCatchAll_MatchesSlashModel(t *testing.T) {
	engine := gin.New()

	var gotVersion, gotAction string
	engine.GET("/gemini/:version/models", func(c *gin.Context) { c.Status(http.StatusOK) })
	engine.POST("/gemini/:version/models/*action", func(c *gin.Context) {
		gotVersion = c.Param("version")
		gotAction = c.Param("action")
		c.Status(http.StatusOK)
	})
	engine.NoRoute(func(c *gin.Context) { c.String(http.StatusNotFound, "NOROUTE") })

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/gemini/v1beta/models/ai21/jamba-large-1.7:generateContent", nil)
	engine.ServeHTTP(w, req)

	if w.Code != http.StatusOK || w.Body.String() == "NOROUTE" {
		t.Fatalf("catch-all did not match slash model: status=%d body=%q", w.Code, w.Body.String())
	}
	if gotVersion != "v1beta" {
		t.Fatalf("version param: got %q want v1beta", gotVersion)
	}
	if gotAction != "/ai21/jamba-large-1.7:generateContent" {
		t.Fatalf("action param: got %q want /ai21/jamba-large-1.7:generateContent", gotAction)
	}
}
