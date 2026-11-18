package relay

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/metrics"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/providers/gemini"
	"github.com/modeltaps/modeltaps/relay/relay_util"
	"github.com/modeltaps/modeltaps/types"
)

// geminiLiveRPC matches the method path of the Gemini Live API WebSocket, as the Google SDKs build
// it from a base URL: {base}/ws/google.ai.generativelanguage.{version}.GenerativeService.BidiGenerateContent.
var geminiLiveRPC = regexp.MustCompile(`^/google\.ai\.generativelanguage\.(v1|v1alpha|v1beta)\.GenerativeService\.BidiGenerateContent$`)

// geminiLiveUpstreamVersion maps the API version of the client's path to one the Gemini API
// serves Live on. The Google SDKs use v1 when given a custom base URL, but the Live endpoint only
// exists under v1beta and v1alpha, so v1 goes upstream as v1beta.
func geminiLiveUpstreamVersion(version string) string {
	if version == "v1" {
		return "v1beta"
	}
	return version
}

// geminiLiveHandshakeTimeout bounds the wait for the client's setup message and for the
// upstream's setupComplete.
const geminiLiveHandshakeTimeout = 30 * time.Second

// errGeminiLiveNotServed marks a channel that carries the model but cannot open a Live session.
var errGeminiLiveNotServed = errors.New("channel does not serve the Gemini Live API")

// relayGeminiLive relays a Gemini Live API session in its native protocol. The model is named in
// the client's first message (setup), so the channel is chosen after that message is read; the
// rest of the session (session limits, entry reservation, per-turn accounting, settlement) is
// shared with the OpenAI Realtime relay. Errors reach the client as WebSocket close frames, the
// way the Live API reports them.
type relayGeminiLive struct {
	RelayModeChatRealtime
	version string
	setup   []byte
}

// GeminiLive serves GET /gemini/ws/google.ai.generativelanguage.{version}.GenerativeService.BidiGenerateContent.
func GeminiLive(c *gin.Context) {
	match := geminiLiveRPC.FindStringSubmatch(c.Param("rpc"))
	if match == nil {
		common.AbortWithMessage(c, http.StatusNotFound, "unknown Live API method")
		return
	}

	release, ok := acquireRealtimeSession(c.GetInt("id"))
	if !ok {
		common.AbortWithMessage(c, http.StatusTooManyRequests, "too many open realtime sessions, close one and retry")
		return
	}
	defer release()

	userConn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.LogError(c.Request.Context(), "gemini live upgrade failed: "+err.Error())
		return
	}
	userConn.SetReadLimit(realtimeMaxMessageBytes())

	r := &relayGeminiLive{
		RelayModeChatRealtime: RelayModeChatRealtime{relayBase: relayBase{c: c}, userConn: userConn},
		version:               geminiLiveUpstreamVersion(match[1]),
	}
	if !r.readSetup() || !r.connect() {
		return
	}
	r.usage = &types.UsageEvent{}

	handler := moderateGeminiLiveClientMessages(geminiLiveMessageHandler, func(reason string) {
		closeWebSocket(r.userConn, websocket.ClosePolicyViolation, reason)
	})
	wsProxy := requester.NewWSProxy(r.userConn, r.providerConn, 2*time.Minute, handler, r.usageHandler)
	wsProxy.UseCloseFrames()
	wsProxy.Start()

	// Snapshot before the goroutine: once Wait returns the handler returns and gin may reuse c.
	snap := relay_util.NewConsumeSnapshot(r.c)
	common.TrackedGoroutine(func() {
		var closedBy string
		select {
		case <-wsProxy.UserClosed():
			closedBy = "user"
		case <-wsProxy.SupplierClosed():
			closedBy = "provider"
		}

		logger.LogInfo(snap.Ctx, fmt.Sprintf("Gemini Live connection closed by %s", closedBy))
		wsProxy.Close()
		r.quotaMu.Lock()
		defer r.quotaMu.Unlock()
		r.quota.ConsumeWithSnapshot(snap, r.usage.ToChatUsage(), false)
	})

	wsProxy.Wait()
}

// readSetup reads the client's first message, which must carry setup.model, and moderates the
// text in it. On failure the client connection is closed with the reason.
func (r *relayGeminiLive) readSetup() bool {
	_ = r.userConn.SetReadDeadline(time.Now().Add(geminiLiveHandshakeTimeout))
	_, message, err := r.userConn.ReadMessage()
	if err != nil {
		r.userConn.Close()
		return false
	}
	_ = r.userConn.SetReadDeadline(time.Time{})

	modelName, reason := parseGeminiLiveSetup(message)
	if reason != "" {
		closeWebSocket(r.userConn, websocket.CloseInvalidFramePayloadData, reason)
		return false
	}
	if apiErr := geminiLiveClientMessageSafety(message); apiErr != nil {
		closeWebSocket(r.userConn, websocket.ClosePolicyViolation, apiErr.OpenAIError.Message)
		return false
	}

	r.setup = message
	r.setOriginalModel(modelName)
	return true
}

// parseGeminiLiveSetup returns the model named by a setup message ("models/{model}", the prefix
// is optional), or the reason the message is not a valid setup.
func parseGeminiLiveSetup(message []byte) (modelName string, reason string) {
	if !gjson.ValidBytes(message) {
		return "", "the first message must be a JSON setup message"
	}
	setup := gjson.GetBytes(message, "setup")
	if !setup.IsObject() {
		return "", "the first message must be a setup message"
	}
	modelName = strings.TrimPrefix(strings.TrimSpace(setup.Get("model").String()), "models/")
	if modelName == "" {
		return "", "setup.model is required"
	}
	return modelName, ""
}

// connect picks a channel, takes the entry reservation, opens the upstream Live session and
// completes the setup handshake, retrying other channels as the OpenAI Realtime relay does.
func (r *relayGeminiLive) connect() bool {
	retryTimes := config.RetryTimes
	if retryTimes == 0 {
		retryTimes = 1
	}

	lastReason := "no channel could open the Live session"
	for i := retryTimes; i > 0; i-- {
		if err := r.setProvider(r.getOriginalModel()); err != nil {
			closeWebSocket(r.userConn, websocket.ClosePolicyViolation, err.Error())
			return false
		}
		channel := r.provider.GetChannel()

		liveProvider, ok := r.provider.(providersBase.GeminiLiveInterface)
		if !ok {
			lastReason = errGeminiLiveNotServed.Error()
			r.skipChannelIds(channel.Id)
			continue
		}

		// Entry guard before any upstream connection, as in the OpenAI Realtime relay.
		r.quota = relay_util.NewQuota(r.getContext(), r.getModelName(), 0)
		if apiErr := preQuotaConsumptionFn(r.quota); apiErr != nil {
			closeWebSocket(r.userConn, websocket.ClosePolicyViolation, apiErr.OpenAIError.Message)
			return false
		}

		providerConn, apiErr := liveProvider.CreateGeminiLive(r.version)
		if apiErr != nil {
			r.quota.Undo(r.c)
			r.skipChannelIds(channel.Id)
			lastReason = apiErr.OpenAIError.Message
			logger.LogError(r.c.Request.Context(), fmt.Sprintf("using channel #%d(%s) Error: %s to retry (remain times %d)", channel.Id, channel.Name, apiErr.Error(), i))
			metrics.RecordProvider(r.c, apiErr.StatusCode)
			continue
		}

		completed, closeErr := r.handshake(providerConn)
		if completed {
			r.providerConn = providerConn
			metrics.RecordProvider(r.c, http.StatusOK)
			return true
		}

		r.quota.Undo(r.c)
		providerConn.Close()
		// An invalid setup is the client's error: another channel would refuse it the same way.
		if closeErr != nil && closeErr.Code == websocket.CloseInvalidFramePayloadData {
			closeWebSocket(r.userConn, closeErr.Code, closeErr.Text)
			return false
		}
		if closeErr != nil && closeErr.Text != "" {
			lastReason = closeErr.Text
		}
		r.skipChannelIds(channel.Id)
		logger.LogError(r.c.Request.Context(), fmt.Sprintf("using channel #%d(%s) Live setup failed: %s (remain times %d)", channel.Id, channel.Name, lastReason, i))
	}

	closeWebSocket(r.userConn, websocket.CloseInternalServerErr, lastReason)
	return false
}

// handshake sends the setup message upstream with the channel's model name and waits for
// setupComplete, which it forwards to the client. When the upstream closes instead, its close
// frame is returned.
func (r *relayGeminiLive) handshake(providerConn *websocket.Conn) (bool, *websocket.CloseError) {
	setup, err := sjson.SetBytes(r.setup, "setup.model", "models/"+r.modelName)
	if err != nil {
		return false, nil
	}
	if err := providerConn.WriteMessage(websocket.TextMessage, setup); err != nil {
		return false, nil
	}

	_ = providerConn.SetReadDeadline(time.Now().Add(geminiLiveHandshakeTimeout))
	messageType, message, err := providerConn.ReadMessage()
	if err != nil {
		var closeErr *websocket.CloseError
		if errors.As(err, &closeErr) {
			return false, closeErr
		}
		return false, nil
	}
	_ = providerConn.SetReadDeadline(time.Time{})

	if !gjson.GetBytes(message, "setupComplete").Exists() {
		return false, nil
	}
	if err := r.userConn.WriteMessage(messageType, message); err != nil {
		return false, nil
	}
	return true, nil
}

// geminiLiveMessageHandler reports the usage of each upstream message. The Live API sends its
// server messages as text or binary frames, so both are read.
func geminiLiveMessageHandler(source requester.MessageSource, messageType int, message []byte) (bool, *types.UsageEvent, []byte, error) {
	if source != requester.SupplierMessage {
		return true, nil, nil, nil
	}
	if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
		return true, nil, nil, nil
	}
	return true, gemini.ParseLiveUsage(message), nil, nil
}

// closeWebSocket sends a close frame with the reason (cut to the 123 bytes a close frame holds)
// and closes the connection.
func closeWebSocket(conn *websocket.Conn, code int, reason string) {
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(code, truncateCloseReason(reason)), time.Now().Add(time.Second))
	conn.Close()
}

func truncateCloseReason(reason string) string {
	const max = 123
	if len(reason) <= max {
		return reason
	}
	reason = reason[:max]
	for len(reason) > 0 && !utf8.ValidString(reason) {
		reason = reason[:len(reason)-1]
	}
	return reason
}
