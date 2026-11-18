package media

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

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
	data, ctype, left, err := Get(c.Param("name"))
	switch {
	case errors.Is(err, ErrExhausted):
		c.Status(http.StatusGone)
		return
	case errors.Is(err, ErrNotFound):
		c.Status(http.StatusNotFound)
		return
	case err != nil:
		c.Status(http.StatusServiceUnavailable)
		return
	}
	h := c.Writer.Header()
	h.Set("Content-Type", ctype)
	h.Set("Content-Length", strconv.Itoa(len(data)))
	h.Set("Content-Disposition", "inline")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Cache-Control", "private, max-age="+strconv.Itoa(int(left/time.Second)))
	c.Status(http.StatusOK)
	lim := rate.NewLimiter(rate.Limit(BytesPerSecond), 64<<10)
	_, _ = throttledWriter{ctx: c.Request.Context(), w: c.Writer, lim: lim}.Write(data)
}
