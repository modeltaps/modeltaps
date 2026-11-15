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
