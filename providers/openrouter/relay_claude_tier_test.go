package openrouter

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// TestClaudeNativePremiumTier: on the byte pass-through path speed=fast and
// inference_geo=us are dropped unless the channel allows them, and the speed the
// upstream reports in usage reaches billing.
func TestClaudeNativePremiumTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}

	for _, allow := range []bool{false, true} {
		var gotBody []byte
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"anthropic/claude-opus-5-5",` +
				`"content":[{"type":"text","text":"hi"}],"stop_reason":"end_turn",` +
				`"usage":{"input_tokens":10,"output_tokens":2,"speed":"fast","inference_geo":"us"}}`))
		}))

		baseURL := upstream.URL + "/api"
		channel := &model.Channel{Type: config.ChannelTypeOpenRouter, Key: "sk-or-test", BaseURL: &baseURL, AllowServiceTier: allow}
		p := OpenRouterProviderFactory{}.Create(channel).(*OpenRouterProvider)

		body := []byte(`{"model":"claude-opus-5-5","max_tokens":64,"speed":"fast","inference_geo":"us",` +
			`"messages":[{"role":"user","content":"hi"}]}`)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(config.GinRequestBodyKey, body)
		p.SetContext(c)
		p.SetUsage(&types.Usage{})

		req := &claude.ClaudeRequest{}
		if err := json.Unmarshal(body, req); err != nil {
			t.Fatal(err)
		}
		req.Model = "anthropic/claude-opus-5-5"

		if _, errWithCode := p.CreateClaudeChat(req); errWithCode != nil {
			t.Fatalf("allow=%v CreateClaudeChat error: %+v", allow, errWithCode)
		}
		upstream.Close()

		for _, field := range []string{"speed", "inference_geo"} {
			if gjson.GetBytes(gotBody, field).Exists() != allow {
				t.Errorf("allow=%v: upstream body has %s=%v (%s)", allow, field, !allow, gotBody)
			}
		}
		usage := p.GetUsage()
		if usage.ServiceTier != "fast" || usage.InferenceGeo != "us" {
			t.Errorf("allow=%v: reported tier not carried: tier=%q geo=%q", allow, usage.ServiceTier, usage.InferenceGeo)
		}
	}
}
