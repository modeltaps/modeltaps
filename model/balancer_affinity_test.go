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

// newTieredAffinityChooser builds a chooser whose channels are tried in order, one per tier.
func newTieredAffinityChooser(channelType int, ids ...int) *ChannelsChooser {
	cc := newAffinityChooser(channelType, ids...)
	tiers := make([][]int, len(ids))
	for i, id := range ids {
		tiers[i] = []int{id}
	}
	cc.Rule["default"]["claude-opus-5-5"] = tiers
	return cc
}

// TestSessionAffinityHoldsAcrossTiers: channels tried in order, each in a tier of its own. A
// session that moved to the second channel because the first failed its request stays there,
// where its prompt cache now is, instead of going back as soon as the first can be picked again;
// a new session starts on the first channel.
func TestSessionAffinityHoldsAcrossTiers(t *testing.T) {
	withAffinityConfig(t, true)
	cc := newTieredAffinityChooser(config.ChannelTypeAnthropic, 1, 2, 3)
	session := "44444444-0000-4000-8000-000000000000"

	first, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session))
	if first.Id != 1 {
		t.Fatalf("first request went to %d, want the first channel", first.Id)
	}
	retry, err := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session), FilterChannelId([]int{1}))
	if err != nil || retry.Id != 2 {
		t.Fatalf("retry = %v, %v; want the second channel", retry, err)
	}
	for i := 0; i < 5; i++ {
		c := newAffinityCtx(1, session)
		ch, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", c)
		if ch.Id != 2 {
			t.Fatalf("session went to %d, want it to stay on the second channel", ch.Id)
		}
		if got := c.GetString(SessionAffinityContextKey); got != "hit" {
			t.Fatalf("affinity = %q, want hit", got)
		}
	}
	other, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, "55555555-0000-4000-8000-000000000000"))
	if other.Id != 1 {
		t.Fatalf("a new session went to %d, want the first channel", other.Id)
	}
}

// TestSessionAffinityMovesOffCoolingChannel: a session leaves its channel while that channel
// cools down for the model, and keeps the channel it moved to once the cooldown is over.
func TestSessionAffinityMovesOffCoolingChannel(t *testing.T) {
	withAffinityConfig(t, true)
	cc := newTieredAffinityChooser(config.ChannelTypeAnthropic, 1, 2)
	session := "66666666-0000-4000-8000-000000000000"

	if first, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session)); first.Id != 1 {
		t.Fatalf("first request went to %d, want the first channel", first.Id)
	}
	cc.SetCooldownsWithDuration(1, "claude-opus-5-5", 60)
	c := newAffinityCtx(1, session)
	moved, err := cc.NextByValidatedModel("default", "claude-opus-5-5", c)
	if err != nil || moved.Id != 2 {
		t.Fatalf("while the first channel cools down: %v, %v; want the second channel", moved, err)
	}
	if got := c.GetString(SessionAffinityContextKey); got != "bound" {
		t.Fatalf("affinity = %q, want bound", got)
	}
	cc.Cooldowns.Delete("1:claude-opus-5-5")
	if back, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session)); back.Id != 2 {
		t.Fatalf("after the cooldown the session went to %d, want it to stay on the second channel", back.Id)
	}
}

// TestSessionAffinityDropsRemovedChannel: a binding to a channel that no longer serves the
// model is forgotten and the session is routed by order again.
func TestSessionAffinityDropsRemovedChannel(t *testing.T) {
	withAffinityConfig(t, true)
	cc := newTieredAffinityChooser(config.ChannelTypeAnthropic, 1, 2)
	session := "77777777-0000-4000-8000-000000000000"

	cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session))
	cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session), FilterChannelId([]int{1}))
	cc.Rule["default"]["claude-opus-5-5"] = [][]int{{1}}
	if ch, err := cc.NextByValidatedModel("default", "claude-opus-5-5", newAffinityCtx(1, session)); err != nil || ch.Id != 1 {
		t.Fatalf("after the second channel stopped serving the model: %v, %v; want the first channel", ch, err)
	}
}

// TestSessionAffinityBoundStaysBoundWithinRequest: a request that binds its session and then
// picks a channel again (chat completions look at the provider before reading the request) is
// still a new binding, not a session found on its channel; the session's next request is a hit.
func TestSessionAffinityBoundStaysBoundWithinRequest(t *testing.T) {
	withAffinityConfig(t, true)
	cc := newTieredAffinityChooser(config.ChannelTypeAnthropic, 1, 2)
	session := "88888888-0000-4000-8000-000000000000"

	c := newAffinityCtx(1, session)
	first, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", c)
	again, _ := cc.NextByValidatedModel("default", "claude-opus-5-5", c)
	if first.Id != again.Id {
		t.Fatalf("the second pick of one request went to %d, the first to %d", again.Id, first.Id)
	}
	if got := c.GetString(SessionAffinityContextKey); got != "bound" {
		t.Fatalf("affinity after the second pick = %q, want bound", got)
	}
	next := newAffinityCtx(1, session)
	cc.NextByValidatedModel("default", "claude-opus-5-5", next)
	if got := next.GetString(SessionAffinityContextKey); got != "hit" {
		t.Fatalf("affinity of the next request = %q, want hit", got)
	}
}
