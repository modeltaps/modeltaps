package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modeltaps/modeltaps/common/requester"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/relay/relay_util"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gorilla/websocket"
)

// newWSPair 建一对已握手的 websocket 连接:返回服务端 conn(充当 realtime 的 userConn)与客户端 conn。
func newWSPair(t *testing.T) (*websocket.Conn, *websocket.Conn) {
	t.Helper()
	serverConnCh := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade failed: %v", err)
			return
		}
		serverConnCh <- conn
	}))
	t.Cleanup(srv.Close)

	clientConn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	t.Cleanup(func() { clientConn.Close() })
	serverConn := <-serverConnCh
	t.Cleanup(func() { serverConn.Close() })
	return serverConn, clientConn
}

// TestRealtimeEnterQuotaGuardRejects 修复 P0(B4):realtime 入场经过 PreQuotaConsumption,
// unpriced/额度不足/org 守护拒绝时连接被拒 —— 客户端收到错误事件后连接关闭,不转发任何消息。
// SEC-13 后守护先于建立上游连接执行(guardBeforeConnect),拒绝时 providerConn 必为 nil。
func TestRealtimeEnterQuotaGuardRejects(t *testing.T) {
	orig := preQuotaConsumptionFn
	defer func() { preQuotaConsumptionFn = orig }()

	cases := []struct {
		name    string
		code    string
		message string
		status  int
	}{
		{"unpriced 拒绝", "model_price_not_configured", "model gpt-4o-realtime has no configured price; pricing must be synced or set before use", http.StatusForbidden},
		{"额度不足拒绝", "insufficient_user_quota", "user quota is not enough", http.StatusPaymentRequired},
		{"org 守护拒绝", "org_member_budget_exceeded", "organization member budget exceeded for current period", http.StatusPaymentRequired},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			serverConn, clientConn := newWSPair(t)
			preQuotaConsumptionFn = func(q *relay_util.Quota) *types.OpenAIErrorWithStatusCode {
				return &types.OpenAIErrorWithStatusCode{
					OpenAIError: types.OpenAIError{Code: tc.code, Message: tc.message, Type: "modeltaps_api_error"},
					StatusCode:  tc.status,
					LocalError:  true,
				}
			}

			r := &RelayModeChatRealtime{userConn: serverConn, quota: &relay_util.Quota{}}
			if r.enterQuotaGuard() {
				t.Fatal("守护拒绝时 enterQuotaGuard 必须返回 false 拒绝连接")
			}

			clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, msg, err := clientConn.ReadMessage()
			if err != nil {
				t.Fatalf("客户端应收到错误事件: %v", err)
			}
			if !strings.Contains(string(msg), tc.message) {
				t.Fatalf("错误事件应含 %q, got %s", tc.message, msg)
			}
			if _, _, err := clientConn.ReadMessage(); err == nil {
				t.Fatal("守护拒绝后连接应关闭")
			}
		})
	}
}

// TestRealtimeEnterQuotaGuardPasses 个人令牌正常路径零回归:守护放行时返回 true、
// 不向用户连接写任何消息,且守护以 relay 自身的 quota 实例执行(orgGuardrail 填充生效)。
func TestRealtimeEnterQuotaGuardPasses(t *testing.T) {
	orig := preQuotaConsumptionFn
	defer func() { preQuotaConsumptionFn = orig }()

	var got *relay_util.Quota
	preQuotaConsumptionFn = func(q *relay_util.Quota) *types.OpenAIErrorWithStatusCode {
		got = q
		return nil
	}

	serverConn, clientConn := newWSPair(t)
	quota := &relay_util.Quota{}
	r := &RelayModeChatRealtime{userConn: serverConn, quota: quota}
	if !r.enterQuotaGuard() {
		t.Fatal("守护放行时必须继续建立连接")
	}
	if got != quota {
		t.Fatal("守护必须以 relay 的 quota 实例执行")
	}

	// 放行路径不写任何消息:随后服务端首条正常消息应原样到达客户端
	if err := serverConn.WriteMessage(websocket.TextMessage, []byte("ok")); err != nil {
		t.Fatalf("放行后连接应可用: %v", err)
	}
	clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := clientConn.ReadMessage()
	if err != nil || string(msg) != "ok" {
		t.Fatalf("放行后首条消息应为正常消息, got %q err=%v", msg, err)
	}
}

// fakeRealtimeProvider 仅实现 CreateChatRealtime 并记录调用;嵌入接口使其满足
// RealtimeInterface,其余方法在守护单测中不会被触达。
type fakeRealtimeProvider struct {
	providersBase.ProviderInterface
	called bool
}

func (f *fakeRealtimeProvider) CreateChatRealtime(modelName string) (*websocket.Conn, requester.MessageHandler, *types.OpenAIErrorWithStatusCode) {
	f.called = true
	return nil, nil, &types.OpenAIErrorWithStatusCode{
		OpenAIError: types.OpenAIError{Code: "upstream_error", Message: "upstream connect failed", Type: "modeltaps_api_error"},
		StatusCode:  http.StatusBadGateway,
	}
}

// TestRealtimeGuardRejectsBeforeUpstreamConnect SEC-13「守护先行、连接在后」:
// 守护拒绝时不调用 CreateChatRealtime(零上游连接),用户只收到错误事件;
// 守护放行时才会尝试建立上游连接。
func TestRealtimeGuardRejectsBeforeUpstreamConnect(t *testing.T) {
	orig := preQuotaConsumptionFn
	defer func() { preQuotaConsumptionFn = orig }()

	t.Run("守护拒绝时零上游连接", func(t *testing.T) {
		serverConn, clientConn := newWSPair(t)
		preQuotaConsumptionFn = func(q *relay_util.Quota) *types.OpenAIErrorWithStatusCode {
			return &types.OpenAIErrorWithStatusCode{
				OpenAIError: types.OpenAIError{Code: "insufficient_user_quota", Message: "user quota is not enough", Type: "modeltaps_api_error"},
				StatusCode:  http.StatusPaymentRequired,
				LocalError:  true,
			}
		}

		fake := &fakeRealtimeProvider{}
		r := &RelayModeChatRealtime{userConn: serverConn, quota: &relay_util.Quota{}}
		_, _, _, guardPassed := r.guardBeforeConnect(fake)
		if guardPassed {
			t.Fatal("守护拒绝时 guardBeforeConnect 必须返回 guardPassed=false")
		}
		if fake.called {
			t.Fatal("守护拒绝时不得调用 CreateChatRealtime(零上游连接)")
		}
		if r.providerConn != nil {
			t.Fatal("守护拒绝时 providerConn 必为 nil")
		}

		clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := clientConn.ReadMessage()
		if err != nil {
			t.Fatalf("客户端应收到错误事件: %v", err)
		}
		if !strings.Contains(string(msg), "user quota is not enough") {
			t.Fatalf("错误事件应含守护拒绝原因, got %s", msg)
		}
	})

	t.Run("守护放行后才建立上游连接", func(t *testing.T) {
		serverConn, _ := newWSPair(t)
		preQuotaConsumptionFn = func(q *relay_util.Quota) *types.OpenAIErrorWithStatusCode {
			return nil
		}

		fake := &fakeRealtimeProvider{}
		r := &RelayModeChatRealtime{userConn: serverConn, quota: &relay_util.Quota{}}
		_, _, apiErr, guardPassed := r.guardBeforeConnect(fake)
		if !guardPassed {
			t.Fatal("守护放行时 guardPassed 必须为 true")
		}
		if !fake.called {
			t.Fatal("守护放行后必须尝试建立上游连接")
		}
		if apiErr == nil {
			t.Fatal("fake 上游失败应透传 apiErr 供重试逻辑处理")
		}
	})
}
