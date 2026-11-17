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

// TestResponsesPassThroughDropsBackground: the Responses byte pass-through never forwards
// background mode, which the gateway cannot bill (SEC-67); an explicit false passes as sent.
func TestResponsesPassThroughDropsBackground(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}

	for _, tc := range []struct {
		background string
		forwarded  bool
	}{
		{`true`, false},
		{`"true"`, false},
		{`false`, true},
	} {
		var gotBody []byte
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotBody, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":"resp_1","object":"response","model":"o3","status":"completed",` +
				`"output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hi"}]}],` +
				`"usage":{"input_tokens":10,"output_tokens":2,"total_tokens":12}}`))
		}))

		baseURL := upstream.URL
		channel := &model.Channel{Type: config.ChannelTypeOpenAI, Key: "sk-test", BaseURL: &baseURL}
		p := OpenAIProviderFactory{}.Create(channel).(*OpenAIProvider)

		body := []byte(`{"model":"o3","input":"hi","background":` + tc.background + `}`)
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
		c.Request.Header.Set("Content-Type", "application/json")
		c.Set(config.GinRequestBodyKey, body)
		p.SetContext(c)
		p.SetUsage(&types.Usage{})

		if _, errWithCode := p.CreateResponses(&types.OpenAIResponsesRequest{Model: "o3", Input: "hi"}); errWithCode != nil {
			t.Fatalf("background=%s: CreateResponses error: %+v", tc.background, errWithCode)
		}
		upstream.Close()

		if got := gjson.GetBytes(gotBody, "background"); got.Exists() != tc.forwarded {
			t.Errorf("background=%s: upstream body %s", tc.background, gotBody)
		}
	}
}
