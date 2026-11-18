package gemini

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/types"
)

// errLiveUnsupported is returned by channel types that reuse the Gemini provider but do not
// serve the Live API (Gemini CLI, Antigravity, Vertex AI express).
var errLiveUnsupported = errors.New("this channel does not serve the Gemini Live API")

// LiveRPCPath is the path of the Live API WebSocket for an API version (v1alpha or v1beta).
func LiveRPCPath(version string) string {
	return fmt.Sprintf("/ws/google.ai.generativelanguage.%s.GenerativeService.BidiGenerateContent", version)
}

// liveURL turns the channel base URL into the Live WebSocket URL: https becomes wss and http
// becomes ws, so a channel pointed at a reverse proxy works the same way.
func liveURL(baseURL, version string) string {
	baseURL = strings.TrimSuffix(baseURL, "/")
	switch {
	case strings.HasPrefix(baseURL, "https://"):
		baseURL = "wss://" + strings.TrimPrefix(baseURL, "https://")
	case strings.HasPrefix(baseURL, "http://"):
		baseURL = "ws://" + strings.TrimPrefix(baseURL, "http://")
	}
	return baseURL + LiveRPCPath(version)
}

// CreateGeminiLive opens the upstream Live API WebSocket of the channel. The caller sends the
// setup message and reads setupComplete; this only dials and authenticates (API key, or the
// OAuth token of a service account key).
func (p *GeminiProvider) CreateGeminiLive(version string) (*websocket.Conn, *types.OpenAIErrorWithStatusCode) {
	if p.Channel.Type != config.ChannelTypeGemini {
		return nil, common.ErrorWrapper(errLiveUnsupported, "channel_not_supported", http.StatusNotImplemented)
	}

	header := make(http.Header)
	for k, v := range p.GetRequestHeaders() {
		// The handshake is a GET without a body; these describe a JSON request body.
		if strings.EqualFold(k, "Content-Type") || strings.EqualFold(k, "Accept") {
			continue
		}
		header.Set(k, v)
	}

	conn, err := requester.NewWSRequester(p.Channel.GetProxy()).NewRequest(liveURL(p.GetBaseURL(), version), header)
	if err != nil {
		return nil, common.ErrorWrapper(err, "ws_request_failed", http.StatusBadGateway)
	}
	return conn, nil
}

// ParseLiveUsage reads the usageMetadata of a Live API server message. It returns nil when the
// message has none. The Gemini API reports the tokens of each turn (the whole context is read
// again on every turn and billed again), so the session total is the sum of these events.
func ParseLiveUsage(message []byte) *types.UsageEvent {
	meta := gjson.GetBytes(message, "usageMetadata")
	if !meta.Exists() || !meta.IsObject() {
		return nil
	}

	usage := &types.UsageEvent{}
	var detailInput int
	meta.Get("promptTokensDetails").ForEach(func(_, d gjson.Result) bool {
		n := int(d.Get("tokenCount").Int())
		detailInput += n
		switch d.Get("modality").String() {
		case "TEXT":
			usage.InputTokenDetails.TextTokens += n
		case "AUDIO":
			usage.InputTokenDetails.AudioTokens += n
		case "IMAGE", "VIDEO":
			usage.InputTokenDetails.ImageTokens += n
		}
		return true
	})
	meta.Get("responseTokensDetails").ForEach(func(_, d gjson.Result) bool {
		n := int(d.Get("tokenCount").Int())
		switch d.Get("modality").String() {
		case "TEXT":
			usage.OutputTokenDetails.TextTokens += n
		case "AUDIO":
			usage.OutputTokenDetails.AudioTokens += n
		case "IMAGE":
			usage.OutputTokenDetails.ImageTokens += n
		}
		return true
	})

	// Some Live models report a small promptTokenCount while the per-modality details carry the
	// context that was read; bill the larger of the two so the context is never left out.
	input := int(meta.Get("promptTokenCount").Int())
	if detailInput > input {
		input = detailInput
	}
	input += int(meta.Get("toolUsePromptTokenCount").Int())
	thoughts := int(meta.Get("thoughtsTokenCount").Int())
	output := int(meta.Get("responseTokenCount").Int()) + thoughts

	usage.InputTokens = input
	usage.OutputTokens = output
	usage.TotalTokens = input + output
	usage.InputTokenDetails.CachedTokens = int(meta.Get("cachedContentTokenCount").Int())
	usage.OutputTokenDetails.ReasoningTokens = thoughts

	if usage.TotalTokens == 0 {
		return nil
	}
	return usage
}
