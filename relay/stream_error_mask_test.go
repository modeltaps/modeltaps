package relay

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type errStream struct {
	data chan string
	errs chan error
}

func (s *errStream) Recv() (<-chan string, <-chan error) { return s.data, s.errs }
func (s *errStream) Close()                              {}

// TestStreamErrorMasked: an error that arrives after the stream started is written to the
// client directly; the upstream address and key in it must be masked.
func TestStreamErrorMasked(t *testing.T) {
	for name, run := range map[string]func(*gin.Context, *errStream){
		"openai":  func(c *gin.Context, s *errStream) { responseStreamClient(c, s, nil) },
		"general": func(c *gin.Context, s *errStream) { responseGeneralStreamClient(c, s, nil) },
	} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
		s := &errStream{data: make(chan string), errs: make(chan error, 1)}
		s.errs <- errors.New(`upstream https://secret-relay.example.com/v1 refused api_key: sk-abc123`)
		run(c, s)
		body := w.Body.String()
		if strings.Contains(body, "secret-relay") || strings.Contains(body, "sk-abc123") {
			t.Errorf("%s: unmasked stream error: %s", name, body)
		}
	}
}
