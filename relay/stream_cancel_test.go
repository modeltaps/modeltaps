package relay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
)

// endlessStream keeps producing chunks until it is closed, like an upstream still generating.
type endlessStream struct {
	data   chan string
	errs   chan error
	stop   chan struct{}
	closed atomic.Bool
	sent   atomic.Int64
}

func newEndlessStream() *endlessStream {
	s := &endlessStream{data: make(chan string), errs: make(chan error), stop: make(chan struct{})}
	go func() {
		defer close(s.data)
		for {
			select {
			case s.data <- `{"x":1}`:
				s.sent.Add(1)
			case <-s.stop:
				return
			}
		}
	}()
	return s
}

func (s *endlessStream) Recv() (<-chan string, <-chan error) { return s.data, s.errs }
func (s *endlessStream) Close() {
	if s.closed.CompareAndSwap(false, true) {
		close(s.stop)
	}
}

func cancelledContext(t *testing.T, cancelOn bool) (*gin.Context, context.CancelFunc) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil).WithContext(ctx)
	c.Set(config.GinCancelOnDisconnectKey, cancelOn)
	return c, cancel
}

// TestStreamStopsOnDisconnect: on a channel that cancels on disconnect, the stream loops return
// soon after the client goes away and close the upstream stream.
func TestStreamStopsOnDisconnect(t *testing.T) {
	for name, run := range map[string]func(*gin.Context, *endlessStream){
		"openai":  func(c *gin.Context, s *endlessStream) { responseStreamClient(c, s, nil) },
		"general": func(c *gin.Context, s *endlessStream) { responseGeneralStreamClient(c, s, nil) },
	} {
		c, cancel := cancelledContext(t, true)
		s := newEndlessStream()
		done := make(chan struct{})
		go func() { run(c, s); close(done) }()

		time.Sleep(20 * time.Millisecond)
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			s.Close()
			t.Fatalf("%s: stream loop kept reading after the client disconnected", name)
		}
		if !s.closed.Load() {
			t.Errorf("%s: upstream stream not closed", name)
		}
		if !c.GetBool(config.GinClientCancelledKey) {
			t.Errorf("%s: client_cancelled not set", name)
		}
	}
}

// TestStreamKeepsReadingWhenCancelOff: other channels keep reading to the end for the
// upstream's final usage, as before.
func TestStreamKeepsReadingWhenCancelOff(t *testing.T) {
	c, cancel := cancelledContext(t, false)
	s := newEndlessStream()
	done := make(chan struct{})
	go func() { responseStreamClient(c, s, nil); close(done) }()

	cancel()
	time.Sleep(50 * time.Millisecond)
	before := s.sent.Load()
	time.Sleep(50 * time.Millisecond)
	select {
	case <-done:
		t.Fatal("stream loop returned although the channel does not cancel on disconnect")
	default:
	}
	if s.sent.Load() <= before {
		t.Error("stream loop stopped reading the upstream")
	}
	if c.GetBool(config.GinClientCancelledKey) {
		t.Error("client_cancelled set on a channel that does not cancel")
	}
	s.Close()
	<-done
}

// TestStreamDisconnectClosesUpstreamConnection drives a real HTTP stream reader against a fake
// upstream that keeps generating: with cancel on, the upstream sees its connection go away soon
// after the client disconnects, which is what makes a real upstream stop generating.
func TestStreamDisconnectClosesUpstreamConnection(t *testing.T) {
	upstreamGone := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		ticker := time.NewTicker(5 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				close(upstreamGone)
				return
			case <-ticker.C:
				if _, err := w.Write([]byte("data: {\"x\":1}\n\n")); err != nil {
					close(upstreamGone)
					return
				}
				flusher.Flush()
			}
		}
	}))
	defer upstream.Close()

	resp, err := http.Get(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	stream, apiErr := requester.RequestStream(requester.NewHTTPRequester("", nil), resp,
		func(rawLine *[]byte, dataChan chan string, _ chan error) { dataChan <- string(*rawLine) })
	if apiErr != nil {
		t.Fatal(apiErr)
	}

	c, cancel := cancelledContext(t, true)
	done := make(chan struct{})
	go func() { responseStreamClient(c, stream, nil); close(done) }()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-upstreamGone:
	case <-time.After(time.Second):
		stream.Close()
		<-done
		t.Fatal("upstream connection still open a second after the client disconnected")
	}
	<-done
}
