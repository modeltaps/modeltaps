package middleware

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common"
)

// TestMultipartTempFilesRemoved uploads files larger than the in-memory limit of
// multipart parsing (32 MB), so they spill to multipart-* temp files, through a real
// server and the middlewares that replace the request with a WithContext copy. The temp
// files must be gone once each request ends.
func TestMultipartTempFilesRemoved(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)

	r := gin.New()
	r.Use(RequestId())
	r.Use(func(c *gin.Context) { c.Set("id", 1) })
	r.Use(ContextUserId())
	r.POST("/v1/audio/transcriptions", func(c *gin.Context) {
		var req struct {
			File  *multipart.FileHeader `form:"file" binding:"required"`
			Model string                `form:"model"`
		}
		if err := common.UnmarshalBodyReusable(c, &req); err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		if countTempFiles(t, tmp) == 0 {
			c.String(http.StatusInternalServerError, "upload did not spill to disk")
			return
		}
		c.String(http.StatusOK, "ok")
	})
	r.POST("/api/upload", func(c *gin.Context) {
		if _, err := c.FormFile("file"); err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})
	srv := httptest.NewServer(r)

	for _, path := range []string{"/v1/audio/transcriptions", "/api/upload", "/v1/audio/transcriptions"} {
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		_ = w.WriteField("model", "whisper-1")
		part, _ := w.CreateFormFile("file", "a.wav")
		_, _ = part.Write(make([]byte, 33<<20))
		_ = w.Close()
		resp, err := http.Post(srv.URL+path, w.FormDataContentType(), &body)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
	}
	srv.Close() // waits for the handlers to return

	if n := countTempFiles(t, tmp); n != 0 {
		t.Errorf("%d multipart temp files left in %s", n, tmp)
	}
}

func countTempFiles(t *testing.T, dir string) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "multipart-*"))
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			n++
		}
	}
	return n
}
