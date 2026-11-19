package bedrock

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/providers/claude"
	"github.com/modeltaps/modeltaps/types"
)

func bedrockWithMode(t *testing.T, rawBody, mode string) *BedrockProvider {
	t.Helper()
	p := BedrockProviderFactory{}.Create(&model.Channel{Name: "bedrock-test", Type: config.ChannelTypeBedrock, Key: "us-east-1|AK|SK"}).(*BedrockProvider)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/claude/v1/messages", bytes.NewReader([]byte(rawBody)))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set(config.GinPromptCacheModeKey, mode)
	p.SetContext(c)
	return p
}

// A key or request set to "on" gets breakpoints on Claude over Bedrock, natively and from chat;
// "auto" with the site switch turned off leaves the body alone.
func TestBedrockClaudeBreakpointsFollowMode(t *testing.T) {
	old := config.ClaudePromptCachingEnabled
	config.ClaudePromptCachingEnabled = false
	t.Cleanup(func() { config.ClaudePromptCachingEnabled = old })
	const raw = `{"model":"claude-sonnet-4-5-20250929","max_tokens":64,"system":"be brief","messages":[{"role":"user","content":"hi"}]}`
	for mode, want := range map[string]bool{"on": true, "auto": false} {
		p := bedrockWithMode(t, raw, mode)
		request := &claude.ClaudeRequest{}
		if err := json.Unmarshal([]byte(raw), request); err != nil {
			t.Fatal(err)
		}
		body, _, errWithCode := p.buildInvokeBody(request)
		if errWithCode != nil {
			t.Fatal(errWithCode.Message)
		}
		if got := gjson.GetBytes(body, "system.0.cache_control.type").String() == "ephemeral"; got != want {
			t.Errorf("native, mode %s: breakpoint %v, want %v: %s", mode, got, want, body)
		}

		chat := bedrockWithMode(t, `{}`, mode)
		chatBody, _, errWithCode := chat.buildChatBody(&types.ChatCompletionRequest{Model: "claude-sonnet-4-5-20250929", MaxTokens: 64,
			Messages: []types.ChatCompletionMessage{{Role: "system", Content: "be brief"}, {Role: "user", Content: "hi"}}})
		if errWithCode != nil {
			t.Fatal(errWithCode.Message)
		}
		if got := gjson.GetBytes(chatBody, "system.0.cache_control.type").String() == "ephemeral"; got != want {
			t.Errorf("chat, mode %s: breakpoint %v, want %v: %s", mode, got, want, chatBody)
		}
	}
}
