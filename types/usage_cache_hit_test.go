package types

import (
	"encoding/json"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// DeepSeek's OpenAI format reports cache hits only as prompt_cache_hit_tokens.
func TestGetExtraTokensDeepSeekCacheHit(t *testing.T) {
	var u Usage
	body := `{"prompt_tokens":1200,"completion_tokens":50,"total_tokens":1250,"prompt_cache_hit_tokens":1000,"prompt_cache_miss_tokens":200}`
	if err := json.Unmarshal([]byte(body), &u); err != nil {
		t.Fatal(err)
	}
	if got := u.GetExtraTokens()[config.UsageExtraCache]; got != 1000 {
		t.Fatalf("cache hits should bill as cached input, got %d", got)
	}

	// cached_tokens wins when the upstream sends both.
	var both Usage
	body = `{"prompt_tokens":1200,"prompt_tokens_details":{"cached_tokens":900},"prompt_cache_hit_tokens":1000}`
	if err := json.Unmarshal([]byte(body), &both); err != nil {
		t.Fatal(err)
	}
	if got := both.GetExtraTokens()[config.UsageExtraCache]; got != 900 {
		t.Fatalf("cached_tokens should take precedence, got %d", got)
	}
}

// Requesty's OpenAI format reports Claude cache writes as caching_tokens, split by TTL in
// caching_token_details (bodies as Requesty returned them for claude-haiku-5-5).
func TestGetExtraTokensRequestyCacheWrite(t *testing.T) {
	var write Usage
	body := `{"completion_tokens":4,"prompt_tokens":20317,"prompt_tokens_details":{"cached_tokens":0,` +
		`"caching_token_details":{"caching_5m_tokens":20313,"caching_1h_tokens":0},"caching_tokens":20313},"total_tokens":20321}`
	if err := json.Unmarshal([]byte(body), &write); err != nil {
		t.Fatal(err)
	}
	extra := write.GetExtraTokens()
	if extra[config.UsageExtraCachedWrite] != 20313 || extra[config.UsageExtraCachedWrite1h] != 0 {
		t.Fatalf("cache write should bill as a 5m write, got %v", extra)
	}

	var read Usage
	body = `{"prompt_tokens":20317,"prompt_tokens_details":{"cached_tokens":20313,` +
		`"caching_token_details":{"caching_5m_tokens":0,"caching_1h_tokens":0},"caching_tokens":0}}`
	if err := json.Unmarshal([]byte(body), &read); err != nil {
		t.Fatal(err)
	}
	if extra := read.GetExtraTokens(); extra[config.UsageExtraCache] != 20313 || extra[config.UsageExtraCachedWrite] != 0 {
		t.Fatalf("cache read = %v", extra)
	}

	var oneHour Usage
	body = `{"prompt_tokens_details":{"caching_tokens":300,"caching_token_details":{"caching_5m_tokens":100,"caching_1h_tokens":200}}}`
	if err := json.Unmarshal([]byte(body), &oneHour); err != nil {
		t.Fatal(err)
	}
	if extra := oneHour.GetExtraTokens(); extra[config.UsageExtraCachedWrite] != 100 || extra[config.UsageExtraCachedWrite1h] != 200 {
		t.Fatalf("TTL split = %v", extra)
	}
}

// Together AI reports cache hits at the top level of usage for some models.
func TestGetExtraTokensTopLevelCachedTokens(t *testing.T) {
	var u Usage
	if err := json.Unmarshal([]byte(`{"prompt_tokens":5000,"completion_tokens":20,"cached_tokens":4096}`), &u); err != nil {
		t.Fatal(err)
	}
	if got := u.GetExtraTokens()[config.UsageExtraCache]; got != 4096 {
		t.Fatalf("top-level cached_tokens should bill as cached input, got %d", got)
	}
}

// Venice reports Claude cache writes inside prompt_tokens_details under Anthropic's name.
func TestGetExtraTokensVeniceCacheWrite(t *testing.T) {
	var u Usage
	body := `{"prompt_tokens":5200,"completion_tokens":10,"prompt_tokens_details":{"cached_tokens":0,"cache_creation_input_tokens":5100}}`
	if err := json.Unmarshal([]byte(body), &u); err != nil {
		t.Fatal(err)
	}
	if extra := u.GetExtraTokens(); extra[config.UsageExtraCachedWrite] != 5100 || extra[config.UsageExtraCache] != 0 {
		t.Fatalf("cache write = %v", extra)
	}
}

// Venice reports one cache hit twice, as cached_tokens and as cache_read_input_tokens (body as
// Venice returned it for deepseek-v4-1-flash-uncensored). It counts once, as a cache read.
func TestGetExtraTokensVeniceCacheHitCountsOnce(t *testing.T) {
	var u Usage
	body := `{"prompt_tokens":3931,"completion_tokens":3,"total_tokens":3934,"prompt_tokens_details":{"cached_tokens":3840},"cache_read_input_tokens":3840}`
	if err := json.Unmarshal([]byte(body), &u); err != nil {
		t.Fatal(err)
	}
	extra := u.GetExtraTokens()
	if extra[config.UsageExtraCachedRead] != 3840 {
		t.Fatalf("cached_read = %d, want 3840", extra[config.UsageExtraCachedRead])
	}
	if _, ok := extra[config.UsageExtraCache]; ok {
		t.Fatalf("the hit must not also count as cached_tokens: %v", extra)
	}
	// A second call (billing reads it more than once) gives the same answer.
	if _, ok := u.GetExtraTokens()[config.UsageExtraCache]; ok {
		t.Fatalf("second call counted the hit twice")
	}

	// OpenAI's own format, cached_tokens alone, is unchanged.
	var openai Usage
	if err := json.Unmarshal([]byte(`{"prompt_tokens":2000,"prompt_tokens_details":{"cached_tokens":1500}}`), &openai); err != nil {
		t.Fatal(err)
	}
	if got := openai.GetExtraTokens()[config.UsageExtraCache]; got != 1500 {
		t.Fatalf("cached_tokens = %d, want 1500", got)
	}
}
