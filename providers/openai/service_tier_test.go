package openai

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// TestResponsesServiceTier: the Responses byte pass-through drops service_tier=priority
// unless the channel allows it, and the tier the upstream reports reaches billing.
func TestResponsesServiceTier(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}

	for _, allow := range []bool{false, true} {
		var gotBody []byte
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","model":"gpt-6.1-sol","status":"completed",` +
				`"service_tier":"priority","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],` +
				`"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`))
		}))

		baseURL := upstream.URL
		channel := &model.Channel{Type: config.ChannelTypeOpenAI, Key: "sk-test", BaseURL: &baseURL, AllowServiceTier: allow}
		p := OpenAIProviderFactory{}.Create(channel).(*OpenAIProvider)

		body := []byte(`{"model":"gpt-6.1-sol","input":"hi","service_tier":"priority"}`)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(config.GinRequestBodyKey, body)
		p.SetContext(c)
		p.SetUsage(&types.Usage{})

		if _, errWithCode := p.CreateResponses(&types.OpenAIResponsesRequest{Model: "gpt-6.1-sol", Input: "hi"}); errWithCode != nil {
			t.Fatalf("allow=%v CreateResponses error: %+v", allow, errWithCode)
		}
		upstream.Close()

		if got := gjson.GetBytes(gotBody, "service_tier"); got.Exists() != allow {
			t.Errorf("allow=%v: upstream service_tier = %q (%s)", allow, got.String(), gotBody)
		}
		if got := p.GetUsage().ServiceTier; got != "priority" {
			t.Errorf("allow=%v: usage.ServiceTier = %q, want priority", allow, got)
		}
	}
}

// TestHandlerChatStream_ServiceTierSurvivesUsageChunk: the tier comes on every chunk and
// must survive the trailing usage chunk that overwrites Usage.
func TestHandlerChatStream_ServiceTierSurvivesUsageChunk(t *testing.T) {
	h := &OpenAIStreamHandler{Usage: &types.Usage{}, ModelName: "gpt-6.1-sol"}

	feedStream(h, []string{
		`data: {"id":"1","object":"chat.completion.chunk","model":"gpt-6.1-sol","service_tier":"priority","choices":[{"index":0,"delta":{"content":"Hi"}}]}`,
		`data: {"id":"1","object":"chat.completion.chunk","model":"gpt-6.1-sol","service_tier":"priority","choices":[],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}`,
	})

	if h.Usage.ServiceTier != "priority" || h.Usage.CompletionTokens != 1 {
		t.Fatalf("tier=%q completion=%d", h.Usage.ServiceTier, h.Usage.CompletionTokens)
	}
}
