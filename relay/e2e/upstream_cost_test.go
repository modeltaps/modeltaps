package e2e

import (
	"math"
	"net/http"
	"strings"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
)

// withUsageCost adds an upstream cost to the usage of a fake chat response.
func withUsageCost(response map[string]any, cost float64) map[string]any {
	response["usage"].(map[string]any)["cost"] = cost
	return response
}

// An OpenRouter channel's usage.cost is stored on the consume log next to the gateway's own
// estimate, for a plain and a streamed response, and is not passed on to the client.
func TestUpstreamCostIsLogged(t *testing.T) {
	const promptTokens, completionTokens = 1000, 500
	// Price ratio 1 is $2 per million tokens: 1000 input + 500 output at ratio 2 = $0.004.
	const wantExpected = 0.004
	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`

	run := func(t *testing.T, channelType int, stream bool) (cost, expected float64, responseBody string) {
		fake := NewFakeProvider(t)
		if stream {
			chunks := ChatStreamChunks("gpt-4o-mini", []string{"he", "llo"}, promptTokens, completionTokens)
			last := len(chunks) - 1
			chunks[last] = strings.Replace(chunks[last], `"usage":{`, `"usage":{"cost":0.0048,`, 1)
			fake.SetResponse(FakeResponse{Stream: chunks})
		} else {
			fake.SetResponse(FakeResponse{JSON: withUsageCost(ChatCompletionJSON("gpt-4o-mini", "hello", promptTokens, completionTokens), 0.0048)})
		}
		h := NewHarness(t, HarnessOptions{
			GroupRatio: 1.5,
			Channels:   []ChannelSpec{{Name: "upstream", Type: channelType, BaseURL: fake.Server.URL}},
			Prices:     []PriceSpec{{Model: "gpt-4o-mini", Input: 1, Output: 2}},
		})
		request := body
		if stream {
			request = strings.Replace(body, `{`, `{"stream":true,`, 1)
		}
		w := h.Post("/v1/chat/completions", request)
		if w.Code != http.StatusOK {
			t.Fatalf("status %d: %s", w.Code, w.Body.String())
		}
		log := soleConsumeLog(t)
		return log.UpstreamCost, log.ExpectedCost, w.Body.String()
	}

	for _, stream := range []bool{false, true} {
		cost, expected, responseBody := run(t, config.ChannelTypeOpenRouter, stream)
		if cost != 0.0048 || math.Abs(expected-wantExpected) > 1e-9 {
			t.Errorf("stream=%v: upstream cost = %v, expected cost = %v, want 0.0048 / %v", stream, cost, expected, wantExpected)
		}
		if strings.Contains(responseBody, `"cost"`) {
			t.Errorf("stream=%v: the client sees the upstream cost: %s", stream, responseBody)
		}
	}

	// The same field from an upstream whose unit is not verified is left alone.
	if cost, expected, _ := run(t, config.ChannelTypeOpenAI, false); cost != 0 || expected != 0 {
		t.Errorf("OpenAI channel: upstream cost = %v, expected cost = %v, want none", cost, expected)
	}
}
