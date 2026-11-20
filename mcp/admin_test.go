package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/model"
)

func adminToolRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/tools", HandleAdminToolList)
	r.POST("/call", HandleAdminToolCall)
	return r
}

func adminToolDo(t *testing.T, r *gin.Engine, method, path string, body any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, bytes.NewReader(raw)))
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("%s %s: %v (%s)", method, path, err, w.Body.String())
	}
	return out
}

// The console's settings assistant lists the admin tools as chat completion functions and runs
// them with the arguments a model wrote (a JSON string) or an object.
func TestAdminToolRoutes(t *testing.T) {
	model.RegisterOptions()
	r := adminToolRouter()

	list := adminToolDo(t, r, http.MethodGet, "/tools", nil)
	tools, _ := list["data"].([]any)
	if len(tools) != 4 {
		t.Fatalf("tools = %v", list)
	}
	first, _ := tools[0].(map[string]any)
	fn, _ := first["function"].(map[string]any)
	if first["type"] != "function" || fn["name"] != "settings_search" || fn["parameters"] == nil {
		t.Fatalf("first tool = %v", first)
	}

	for _, args := range []any{`{"query":"fee rate"}`, map[string]any{"query": "fee rate"}} {
		out := adminToolDo(t, r, http.MethodPost, "/call", map[string]any{"name": "settings_search", "arguments": args})
		data, _ := out["data"].(map[string]any)
		if text, _ := data["text"].(string); out["success"] != true || !strings.Contains(text, "channel.fee_rate") {
			t.Fatalf("settings_search(%v) = %v", args, out)
		}
	}

	out := adminToolDo(t, r, http.MethodPost, "/call", map[string]any{"name": "settings_describe", "arguments": "{}"})
	data, _ := out["data"].(map[string]any)
	if text, _ := data["text"].(string); !strings.HasPrefix(text, "error:") {
		t.Fatalf("a tool error is returned to the model as text: %v", out)
	}
	if out := adminToolDo(t, r, http.MethodPost, "/call", map[string]any{"name": "drop_tables"}); out["success"] != false {
		t.Fatalf("an unknown tool ran: %v", out)
	}
}
