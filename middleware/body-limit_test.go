package middleware

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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

// countingReader serves n bytes of 'a' and counts how many were read.
type countingReader struct {
	left int64
	read int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	if r.left <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.left {
		p = p[:r.left]
	}
	for i := range p {
		p[i] = 'a'
	}
	r.left -= int64(len(p))
	r.read += int64(len(p))
	return len(p), nil
}

func TestRouteBodyLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(RouteBodyLimit(1<<20, map[string]int64{"/upload": 4 << 20, "/notify/:id": 64 << 10}))
	readAll := func(c *gin.Context) {
		n, err := io.Copy(io.Discard, c.Request.Body)
		if err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}
		c.String(http.StatusOK, "%d", n)
	}
	r.POST("/login", readAll)
	r.POST("/upload", readAll)
	r.POST("/notify/:id", readAll)

	send := func(path string, size int64, chunked bool) (*httptest.ResponseRecorder, *countingReader) {
		body := &countingReader{left: size}
		req := httptest.NewRequest(http.MethodPost, path, body)
		req.ContentLength = size
		if chunked {
			req.ContentLength = -1
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w, body
	}

	w, body := send("/login", 200<<20, false)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("declared oversize body should be 413, got %d", w.Code)
	}
	if body.read != 0 {
		t.Fatalf("declared oversize body must not be read, read %d bytes", body.read)
	}
	if !strings.Contains(w.Body.String(), "request body exceeds the 1 MB limit") {
		t.Fatalf("unexpected 413 body: %s", w.Body.String())
	}
	if w.Header().Get("Connection") != "close" {
		t.Fatalf("413 should close the connection, headers %v", w.Header())
	}

	w, body = send("/login", 200<<20, true)
	if w.Code == http.StatusOK {
		t.Fatalf("chunked oversize body must not be accepted")
	}
	if body.read > 1<<20+1 {
		t.Fatalf("chunked oversize body read past the limit: %d bytes", body.read)
	}

	if w, _ := send("/login", 512<<10, false); w.Code != http.StatusOK {
		t.Fatalf("body under the default limit: %d %s", w.Code, w.Body.String())
	}
	if w, _ := send("/upload", 3<<20, false); w.Code != http.StatusOK || w.Body.String() != "3145728" {
		t.Fatalf("route with a larger limit should read the whole body: %d %s", w.Code, w.Body.String())
	}
	if w, _ := send("/upload", 3<<20, true); w.Code != http.StatusOK {
		t.Fatalf("chunked body under the route limit: %d %s", w.Code, w.Body.String())
	}
	if w, _ := send("/upload", 5<<20, false); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("body over the route limit should be 413, got %d", w.Code)
	}
	if w, _ := send("/notify/1", 128<<10, false); w.Code != http.StatusRequestEntityTooLarge ||
		!strings.Contains(w.Body.String(), "64 KB") {
		t.Fatalf("route with a smaller limit should be 413, got %d %s", w.Code, w.Body.String())
	}
}

func TestBodyLimitZeroDisables(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(BodyLimit(0))
	r.POST("/", func(c *gin.Context) {
		n, _ := io.Copy(io.Discard, c.Request.Body)
		c.String(http.StatusOK, "%d", n)
	})
	req := httptest.NewRequest(http.MethodPost, "/", &countingReader{left: 3 << 20})
	req.ContentLength = 3 << 20
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK || w.Body.String() != "3145728" {
		t.Fatalf("0 disables the limit, got %d %s", w.Code, w.Body.String())
	}
}

// BodyReadTimeout needs a real connection: the deadline is set on it.
func TestBodyReadTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const timeout = 300 * time.Millisecond
	type result struct {
		readErr error
		ctxErr  error
		elapsed time.Duration
	}
	results := make(chan result, 1)
	r := gin.New()
	r.Use(BodyReadTimeout(timeout))
	r.POST("/slow", func(c *gin.Context) {
		start := time.Now()
		_, err := io.ReadAll(c.Request.Body)
		results <- result{readErr: err, elapsed: time.Since(start)}
		c.Status(http.StatusRequestTimeout)
	})
	// Reads the whole body, then keeps working past the deadline.
	r.POST("/work", func(c *gin.Context) {
		if _, err := io.ReadAll(c.Request.Body); err != nil {
			results <- result{readErr: err}
			c.Status(http.StatusBadRequest)
			return
		}
		time.Sleep(3 * timeout)
		results <- result{ctxErr: c.Request.Context().Err()}
		c.String(http.StatusOK, "done")
	})
	r.GET("/work", func(c *gin.Context) {
		time.Sleep(3 * timeout)
		results <- result{ctxErr: c.Request.Context().Err()}
		c.String(http.StatusOK, "done")
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	// A client that declares 100 bytes, sends 10 and stalls.
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "POST /slow HTTP/1.1\r\nHost: x\r\nContent-Length: 100\r\n\r\n0123456789"); err != nil {
		t.Fatal(err)
	}
	select {
	case res := <-results:
		if res.readErr == nil {
			t.Fatal("a stalled body should fail to read")
		}
		if res.elapsed > 5*timeout {
			t.Fatalf("stalled body read took %v, deadline %v", res.elapsed, timeout)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("stalled body read was not cut off")
	}

	for _, method := range []string{http.MethodPost, http.MethodGet} {
		var body io.Reader
		if method == http.MethodPost {
			body = strings.NewReader(`{"a":1}`)
		}
		req, _ := http.NewRequest(method, srv.URL+"/work", body)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("%s /work: %v", method, err)
		}
		resp.Body.Close()
		res := <-results
		if res.readErr != nil || res.ctxErr != nil {
			t.Fatalf("%s /work: a handler past the deadline must keep its context: read %v, ctx %v",
				method, res.readErr, res.ctxErr)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s /work: %d", method, resp.StatusCode)
		}
	}
}
