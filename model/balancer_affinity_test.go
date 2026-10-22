package model

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

func newAffinityChooser(channelType int, ids ...int) *ChannelsChooser {
	weight := uint(1)
	cc := &ChannelsChooser{
		Rule:     map[string]map[string][][]int{"default": {"claude-opus-5-5": {ids}}},
		Channels: map[int]*ChannelChoice{},
	}
	for _, id := range ids {
		cc.Channels[id] = &ChannelChoice{Channel: &Channel{Id: id, Type: channelType, Weight: &weight, Status: config.ChannelStatusEnabled}}
	}
	return cc
}

func newAffinityCtx(tokenID int, sessionID string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set("token_id", tokenID)
	body := fmt.Sprintf(`{"model":"claude-opus-5-5","metadata":{"user_id":{"session_id":"%s"}},"messages":[{"role":"user","content":"hi"}]}`, sessionID)
	c.Set(config.GinRequestBodyKey, []byte(body))
	return c
}

func withAffinityConfig(t *testing.T, enabled bool) {
	t.Helper()
	oldEnabled, oldRedis := config.SessionAffinityEnabled, config.RedisEnabled
	config.SessionAffinityEnabled, config.RedisEnabled = enabled, false
	t.Cleanup(func() { config.SessionAffinityEnabled, config.RedisEnabled = oldEnabled, oldRedis })
	stickyMemory.Clear()
}

// TestGenericSessionAffinity: without Redis, every request of one Claude Code session on
// OpenRouter channels lands on the channel that served the first one.
func TestGenericSessionAffinity(t *testing.T) {
	withAffinityConfig(t, true)
	cc := newAffinityChooser(config.ChannelTypeOpenRouter, 1, 2, 3, 4, 5)

	for s := 0; s < 5; s++ {
		session := fmt.Sprintf("00000000-0000-4000-8000-%012d", s)
		c := newAffinityCtx(1, session)
		first, err := cc.NextByValidatedModel("default", "claude-opus-5-5", c)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.GetString(SessionAffinityContextKey); got != "bound" {
			t.Fatalf("first request affinity = %q, want bound", got)
		}
		for i := 0; i < 20; i++ {
			c := newAffinityCtx(1, session)
			ch, err := cc.NextByValidatedModel("default", "claude-opus-5-5", c)
			if err != nil {
				t.Fatal(err)
			}
			if ch.Id != first.Id {
				t.Fatalf("session %d moved from channel %d to %d", s, first.Id, ch.Id)
			}
			if got := c.GetString(SessionAffinityContextKey); got != "hit" {
				t.Fatalf("affinity = %q, want hit", got)
			}
		}
	}
}

// TestGenericSessionAffinityRebindsOnSkip: a retry that skips the bound channel picks
// another one and the session follows it afterwards.
func TestGenericSessionAffinityRebindsOnSkip(t *testing.T) {
	withAffinityConfig(t, true)
	cc := newAffinityChooser(config.ChannelTypeAnthropic, 1, 2, 3)
	session := "11111111-0000-4000-8000-000000000000"

	first, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session))
	retry, err := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session), FilterChannelId([]int{first.Id}))
	if err != nil || retry.Id == first.Id {
		t.Fatalf("retry = %v, %v; want a different channel", retry, err)
	}
	for i := 0; i < 10; i++ {
		ch, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session))
		if ch.Id != retry.Id {
			t.Fatalf("session went back to %d, want the retry channel %d", ch.Id, retry.Id)
		}
	}
}

// TestGenericSessionAffinityDisabled: with the switch off, sessions are spread by weight.
func TestGenericSessionAffinityDisabled(t *testing.T) {
	withAffinityConfig(t, false)
	cc := newAffinityChooser(config.ChannelTypeOpenRouter, 1, 2, 3, 4, 5)

	seen := map[int]bool{}
	for i := 0; i < 50; i++ {
		c := newAffinityCtx(1, "22222222-0000-4000-8000-000000000000")
		ch, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", c)
		seen[ch.Id] = true
		if c.GetString(SessionAffinityContextKey) != "" {
			t.Fatal("affinity recorded while disabled")
		}
	}
	if len(seen) < 2 {
		t.Fatalf("50 requests all went to one channel with affinity off: %v", seen)
	}
}

// TestAccountPoolAffinityWithoutRedis: Claude Code account channels keep their own
// stickiness and now work without Redis too, even with the generic switch off.
func TestAccountPoolAffinityWithoutRedis(t *testing.T) {
	withAffinityConfig(t, false)
	cc := newAffinityChooser(config.ChannelTypeClaudeCode, 1, 2, 3, 4, 5)
	session := "33333333-0000-4000-8000-000000000000"

	first, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session))
	for i := 0; i < 20; i++ {
		ch, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session))
		if ch.Id != first.Id {
			t.Fatalf("Claude Code session moved from %d to %d", first.Id, ch.Id)
		}
	}
}
