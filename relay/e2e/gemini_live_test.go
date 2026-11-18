package e2e

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
)

const geminiLivePath = "/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContent"

// fakeGeminiLive stands in for the Live API: it records the setup and the API key, answers
// setupComplete (as a binary frame, as Google does), and after the first client message sends a
// model turn followed by its usage. With reject set it closes the session on setup instead.
type fakeGeminiLive struct {
	server *httptest.Server
	reject *websocket.CloseError

	mu     sync.Mutex
	setups []string
	keys   []string
	inputs []string
}

func newFakeGeminiLive(t *testing.T) *fakeGeminiLive {
	t.Helper()
	f := &fakeGeminiLive{}
	upgrader := websocket.Upgrader{}
	mux := http.NewServeMux()
	mux.HandleFunc(geminiLivePath, func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		_, setup, err := conn.ReadMessage()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.setups = append(f.setups, string(setup))
		f.keys = append(f.keys, r.Header.Get("x-goog-api-key"))
		f.mu.Unlock()

		if f.reject != nil {
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(f.reject.Code, f.reject.Text), time.Now().Add(time.Second))
			return
		}
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte(`{"setupComplete":{}}`))

		_, input, err := conn.ReadMessage()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.inputs = append(f.inputs, string(input))
		f.mu.Unlock()
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte(`{"serverContent":{"modelTurn":{"parts":[{"text":"page 12"}]},"turnComplete":true}}`))
		_ = conn.WriteMessage(websocket.BinaryMessage, []byte(`{"usageMetadata":{"promptTokenCount":100,"responseTokenCount":40,"totalTokenCount":140,"promptTokensDetails":[{"modality":"AUDIO","tokenCount":60},{"modality":"TEXT","tokenCount":40}]}}`))
		// Wait for the client to close.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func newGeminiLiveHarness(t *testing.T, upstream string, extra ...ChannelSpec) (*Harness, *httptest.Server) {
	t.Helper()
	channels := append(extra, ChannelSpec{
		Name:         "gemini-live",
		Type:         config.ChannelTypeGemini,
		BaseURL:      upstream,
		Key:          "AIza-e2e-channel-key",
		ModelMapping: `{"gemini-live-test":"gemini-live-upstream"}`,
	})
	// A mapped model is billed by its upstream name.
	prices := []PriceSpec{{Model: "gemini-live-test", Input: 1, Output: 1}, {Model: "gemini-live-upstream", Input: 1, Output: 1}}
	h := NewHarness(t, HarnessOptions{Model: "gemini-live-test", Channels: channels, Prices: prices})
	gateway := httptest.NewServer(h.Engine)
	t.Cleanup(gateway.Close)
	return h, gateway
}

func dialGeminiLive(t *testing.T, gateway *httptest.Server, key string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(gateway.URL, "http") + "/gemini" + geminiLivePath + "?key=" + key
	conn, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		t.Fatalf("dial gateway: %v (status %d)", err, status)
	}
	// Cleanups run last-in first-out: the connection closes first, then the session's background
	// settlement finishes before the harness swaps the global settings back.
	t.Cleanup(func() {
		time.Sleep(50 * time.Millisecond)
		common.WaitTrackedGoroutines(5 * time.Second)
	})
	t.Cleanup(func() { conn.Close() })
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	return conn
}

func readCloseError(t *testing.T, conn *websocket.Conn) *websocket.CloseError {
	t.Helper()
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue
		}
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) {
			t.Fatalf("expected a close frame, got %v", err)
		}
		return closeErr
	}
}

func TestGeminiLiveRelaysSessionAndBills(t *testing.T) {
	upstream := newFakeGeminiLive(t)
	h, gateway := newGeminiLiveHarness(t, upstream.server.URL)
	quotaBefore := h.UserQuota(t)

	conn := dialGeminiLive(t, gateway, h.Token.Key)
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"setup":{"model":"models/gemini-live-test","generationConfig":{"responseModalities":["AUDIO"]}}}`)); err != nil {
		t.Fatal(err)
	}
	_, first, err := conn.ReadMessage()
	if err != nil || !gjson.GetBytes(first, "setupComplete").Exists() {
		t.Fatalf("expected setupComplete, got %s (%v)", first, err)
	}

	frame := `{"realtimeInput":{"video":{"data":"/9j/AAAA","mimeType":"image/jpeg"}}}`
	if err := conn.WriteMessage(websocket.TextMessage, []byte(frame)); err != nil {
		t.Fatal(err)
	}
	if _, msg, err := conn.ReadMessage(); err != nil || !strings.Contains(string(msg), "page 12") {
		t.Fatalf("expected the model turn, got %s (%v)", msg, err)
	}
	if _, msg, err := conn.ReadMessage(); err != nil || !gjson.GetBytes(msg, "usageMetadata").Exists() {
		t.Fatalf("expected the usage message, got %s (%v)", msg, err)
	}
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	conn.Close()

	// Settlement runs in the background once the session closes; wait for its log.
	var logs []model.Log
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		logs = nil
		if err := model.DB.Where("user_id = ?", h.User.Id).Find(&logs).Error; err != nil {
			t.Fatal(err)
		}
		if len(logs) > 0 || time.Now().After(deadline) {
			break
		}
	}
	// The settlement goroutine is registered by now; let it finish before the next test swaps
	// the global settings.
	common.WaitTrackedGoroutines(5 * time.Second)

	upstream.mu.Lock()
	defer upstream.mu.Unlock()
	if len(upstream.setups) != 1 || gjson.Get(upstream.setups[0], "setup.model").String() != "models/gemini-live-upstream" {
		t.Fatalf("upstream setup: %v", upstream.setups)
	}
	if gjson.Get(upstream.setups[0], "setup.generationConfig.responseModalities.0").String() != "AUDIO" {
		t.Fatalf("setup fields were not passed through: %s", upstream.setups[0])
	}
	if upstream.keys[0] != "AIza-e2e-channel-key" {
		t.Fatalf("upstream key: %q", upstream.keys[0])
	}
	if len(upstream.inputs) != 1 || upstream.inputs[0] != frame {
		t.Fatalf("upstream input: %v", upstream.inputs)
	}

	if len(logs) != 1 || logs[0].PromptTokens != 100 || logs[0].CompletionTokens != 40 {
		t.Fatalf("logs: %+v", logs)
	}
	if after := h.UserQuota(t); after >= quotaBefore {
		t.Fatalf("quota not charged: before=%d after=%d", quotaBefore, after)
	}
}

func TestGeminiLiveRequiresSetupModel(t *testing.T) {
	upstream := newFakeGeminiLive(t)
	h, gateway := newGeminiLiveHarness(t, upstream.server.URL)

	conn := dialGeminiLive(t, gateway, h.Token.Key)
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"setup":{}}`))
	if closeErr := readCloseError(t, conn); closeErr.Code != websocket.CloseInvalidFramePayloadData || !strings.Contains(closeErr.Text, "setup.model") {
		t.Fatalf("close: %+v", closeErr)
	}
}

func TestGeminiLiveRejectsUnknownMethod(t *testing.T) {
	upstream := newFakeGeminiLive(t)
	h, gateway := newGeminiLiveHarness(t, upstream.server.URL)

	url := "ws" + strings.TrimPrefix(gateway.URL, "http") + "/gemini/ws/google.ai.generativelanguage.v1beta.GenerativeService.BidiGenerateContentConstrained?key=" + h.Token.Key
	_, resp, err := websocket.DefaultDialer.Dial(url, nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %v", err)
	}
}

// An upstream that refuses the setup as invalid is the client's error: the close frame reaches
// the client and nothing is charged.
func TestGeminiLiveForwardsInvalidSetupClose(t *testing.T) {
	upstream := newFakeGeminiLive(t)
	upstream.reject = &websocket.CloseError{Code: websocket.CloseInvalidFramePayloadData, Text: "Request contains an invalid argument."}
	h, gateway := newGeminiLiveHarness(t, upstream.server.URL)
	quotaBefore := h.UserQuota(t)

	conn := dialGeminiLive(t, gateway, h.Token.Key)
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"setup":{"model":"models/gemini-live-test"}}`))
	closeErr := readCloseError(t, conn)
	if closeErr.Code != websocket.CloseInvalidFramePayloadData || closeErr.Text != "Request contains an invalid argument." {
		t.Fatalf("close: %+v", closeErr)
	}
	if after := h.UserQuota(t); after != quotaBefore {
		t.Fatalf("refused setup was charged: before=%d after=%d", quotaBefore, after)
	}
}

// A channel that carries the model but cannot serve Live (here an OpenAI channel) is skipped.
func TestGeminiLiveSkipsChannelsWithoutLive(t *testing.T) {
	upstream := newFakeGeminiLive(t)
	h, gateway := newGeminiLiveHarness(t, upstream.server.URL, ChannelSpec{Name: "openai-same-model", BaseURL: "http://127.0.0.1:1", Priority: 10})
	setAndRestore(t, &config.RetryTimes, 2)

	conn := dialGeminiLive(t, gateway, h.Token.Key)
	_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"setup":{"model":"models/gemini-live-test"}}`))
	_, first, err := conn.ReadMessage()
	if err != nil || !gjson.GetBytes(first, "setupComplete").Exists() {
		t.Fatalf("expected setupComplete from the Gemini channel, got %s (%v)", first, err)
	}
}
