package requester

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/modeltaps/modeltaps/types"
)

// wsPair returns a connection and a function that closes its peer.
func wsPair(t *testing.T) (*websocket.Conn, func()) {
	t.Helper()
	peers := make(chan *websocket.Conn, 1)
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		peers <- conn
	}))
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		server.Close()
		t.Fatal(err)
	}
	peer := <-peers
	server.Close() // the upgraded connections are no longer the server's
	return conn, func() { peer.Close() }
}

// Both transfer goroutines end once a session is over; none stays behind blocked on reporting
// that it is done.
func TestWSProxyLeavesNoGoroutineBehind(t *testing.T) {
	handler := func(MessageSource, int, []byte) (bool, *types.UsageEvent, []byte, error) {
		return true, nil, nil, nil
	}
	run := func() {
		user, closeUserPeer := wsPair(t)
		supplier, closeSupplierPeer := wsPair(t)
		proxy := NewWSProxy(user, supplier, time.Minute, handler, nil)
		proxy.Start()
		closeUserPeer()
		proxy.Wait()
		proxy.Close()
		closeSupplierPeer()
		<-proxy.SupplierClosed()
	}

	run() // warm up whatever the first connection starts for the process
	before := runtime.NumGoroutine()
	for i := 0; i < 20; i++ {
		run()
	}
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before+2 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before+2 {
		t.Fatalf("goroutines grew from %d to %d over 20 sessions", before, after)
	}
}
