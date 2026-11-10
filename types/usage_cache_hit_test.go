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
