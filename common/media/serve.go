package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var slots = make(chan struct{}, MaxConcurrent)

// throttledWriter writes at most BytesPerSecond.
type throttledWriter struct {
	ctx context.Context
	w   io.Writer
	lim *rate.Limiter
}

func (t throttledWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		n := min(len(p), 32<<10)
		if err := t.lim.WaitN(t.ctx, n); err != nil {
			return written, err
		}
		m, err := t.w.Write(p[:n])
		written += m
		if err != nil {
			return written, err
		}
		p = p[n:]
	}
	return written, nil
}

// Serve answers GET /media/:name.
func Serve(c *gin.Context) {
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	default:
		c.Header("Retry-After", "5")
		c.Status(http.StatusTooManyRequests)
		return
	}
	name := c.Param("name")
	if !namePattern.MatchString(name) {
		c.Status(http.StatusNotFound)
		return
	}
	data, ctype, left, err := Get(name)
	switch {
	case errors.Is(err, ErrExhausted):
		c.Status(http.StatusGone)
		return
	case errors.Is(err, ErrNotFound):
		if !serveRemote(c, name) {
			c.Status(http.StatusNotFound)
		}
		return
	case err != nil:
		c.Status(http.StatusServiceUnavailable)
		return
	}
	setMediaHeaders(c, ctype, left)
	c.Header("Content-Length", strconv.Itoa(len(data)))
	c.Status(http.StatusOK)
	_, _ = throttled(c).Write(data)
}
