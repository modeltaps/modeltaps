package vercel

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

// TestVercelClaudeNativePassThrough 核验 Claude 格式请求直达网关 /v1/messages，
// Bearer 鉴权，客户端的 cache_control 断点原样保留。
func TestVercelClaudeNativePassThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	if requester.HTTPClient == nil {
		requester.InitHttpClient()
	}

	var gotPath, gotAuth string
	var gotBody []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","type":"message","role":"assistant","model":"anthropic/claude-opus-5",` +
			`"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn","usage":{"input_tokens":10,"output_tokens":2}}`))
	}))
	defer upstream.Close()

	baseURL := upstream.URL
	channel := &model.Channel{Type: config.ChannelTypeVercelAIGateway, Key: "vck_test", BaseURL: &baseURL}
	p := VercelProviderFactory{}.Create(channel).(*VercelProvider)

	body := []byte(`{"model":"anthropic/claude-opus-5","max_tokens":64,` +
		`"system":[{"type":"text","text":"sys","cache_control":{"type":"ephemeral"}}],` +
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
	if _, errWithCode := p.CreateClaudeChat(req); errWithCode != nil {
		t.Fatalf("CreateClaudeChat error: %+v", errWithCode)
	}
	if gotPath != "/v1/messages" || gotAuth != "Bearer vck_test" {
		t.Errorf("request = %s %q", gotPath, gotAuth)
	}
	if got := gjson.GetBytes(gotBody, "system.0.cache_control.type").String(); got != "ephemeral" {
		t.Errorf("system cache_control = %q, want ephemeral", got)
	}
}
