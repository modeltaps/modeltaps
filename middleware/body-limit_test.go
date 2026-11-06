package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func TestRelayBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	orig := viper.Get("relay_max_body_mb")
	t.Cleanup(func() { viper.Set("relay_max_body_mb", orig) })
	viper.Set("relay_max_body_mb", 1)

	r := gin.New()
	r.Use(RelayBodyLimit())
	r.POST("/v1/chat/completions", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "ok")
	})

	send := func(body []byte, chunked bool) int {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
		if chunked {
			req.ContentLength = -1
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}

	small := bytes.Repeat([]byte("a"), 1<<10)
	big := bytes.Repeat([]byte("a"), 2<<20)
	if code := send(small, false); code != http.StatusOK {
		t.Fatalf("small body: %d", code)
	}
	if code := send(big, false); code != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared oversize body should be 413, got %d", code)
	}
	if code := send(big, true); code == http.StatusOK {
		t.Fatalf("chunked oversize body must not be read fully, got %d", code)
	}

	viper.Set("relay_max_body_mb", 0)
	if code := send(big, false); code != http.StatusOK {
		t.Fatalf("0 disables the limit, got %d", code)
	}
}
