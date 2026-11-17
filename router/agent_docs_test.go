package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// AGENT-5: /skill.md and /llms.txt are served at the site root, briefly cacheable.
func TestAgentDocsAtSiteRoot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldAddr := config.ServerAddress
	config.ServerAddress = "https://api.example.com"
	t.Cleanup(func() { config.ServerAddress = oldAddr })
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
		if w.Header().Get("Cache-Control") != "public, max-age=300" {
			t.Fatalf("%s: should be cacheable for a few minutes, got %v", path, w.Header())
		}
	}
}

// SEC-88: without a configured address the documents name the request's own origin, so shared
// caches must not keep them.
func TestAgentDocsFromRequestAreNotSharedCacheable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldAddr := config.ServerAddress
	config.ServerAddress = ""
	t.Cleanup(func() { config.ServerAddress = oldAddr })
	r := gin.New()
	registerAgentDocs(r)
	for _, path := range []string{"/skill.md", "/llms.txt"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if cc := w.Header().Get("Cache-Control"); w.Code != http.StatusOK || strings.Contains(cc, "public") || !strings.Contains(cc, "private") {
			t.Fatalf("%s: got %d, Cache-Control %q, want private", path, w.Code, cc)
		}
		if !strings.Contains(w.Header().Get("Vary"), "Host") {
			t.Fatalf("%s: want Vary on Host, got %v", path, w.Header())
		}
	}
}
