package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// AGENT-5: /skill.md and /llms.txt are served at the site root, briefly cacheable.
func TestAgentDocsAtSiteRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	registerAgentDocs(r)
	for path, contentType := range map[string]string{
		"/skill.md": "text/markdown; charset=utf-8",
		"/llms.txt": "text/plain; charset=utf-8",
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusOK || w.Header().Get("Content-Type") != contentType {
			t.Fatalf("%s: got %d %q", path, w.Code, w.Header().Get("Content-Type"))
		}
		if w.Header().Get("Cache-Control") != "public, max-age=300" || !strings.Contains(w.Header().Get("Vary"), "Host") {
			t.Fatalf("%s: should be cacheable for a few minutes per host, got %v", path, w.Header())
		}
	}
}
