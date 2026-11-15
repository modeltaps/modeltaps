package types

import (
	"encoding/json"
	"strings"
	"testing"
)

// usage.cost is kept for the log and never sent on to the client.
func TestUsageKeepsUpstreamCost(t *testing.T) {
	var usage Usage
	body := `{"prompt_tokens":194,"completion_tokens":2,"total_tokens":196,"cost":0.00095,"prompt_tokens_details":{"cached_tokens":100}}`
	if err := json.Unmarshal([]byte(body), &usage); err != nil {
		t.Fatal(err)
	}
	if usage.UpstreamCost != 0.00095 || usage.PromptTokens != 194 || usage.PromptTokensDetails.CachedTokens != 100 {
		t.Fatalf("usage = %+v", usage)
	}
	out, _ := json.Marshal(usage)
	if strings.Contains(string(out), "cost") {
		t.Errorf("the client would see the upstream cost: %s", out)
	}

	// A usage inside a response is read the same way.
	var response struct {
		Usage *Usage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(`{"usage":`+body+`}`), &response); err != nil || response.Usage.UpstreamCost != 0.00095 {
		t.Fatalf("nested usage = %+v, %v", response.Usage, err)
	}
}

// A cost that is not a positive number is ignored; the rest of the usage still parses.
func TestUsageIgnoresOddCost(t *testing.T) {
	for _, cost := range []string{`"0.5"`, `null`, `{"total":1}`, `-1`, `[1]`} {
		usage := Usage{UpstreamCost: 9}
		body := `{"prompt_tokens":10,"completion_tokens":5,"cost":` + cost + `}`
		if err := json.Unmarshal([]byte(body), &usage); err != nil {
			t.Fatalf("cost %s broke the usage: %v", cost, err)
		}
		if usage.UpstreamCost != 0 || usage.PromptTokens != 10 || usage.CompletionTokens != 5 {
			t.Errorf("cost %s: usage = %+v", cost, usage)
		}
	}
	var none Usage
	if err := json.Unmarshal([]byte(`{"prompt_tokens":10}`), &none); err != nil || none.UpstreamCost != 0 {
		t.Errorf("no cost: %+v, %v", none, err)
	}
}

func TestResponsesUsageKeepsUpstreamCost(t *testing.T) {
	var usage ResponsesUsage
	body := `{"input_tokens":194,"output_tokens":2,"total_tokens":196,"cost":0.95,"input_tokens_details":{"cached_tokens":7}}`
	if err := json.Unmarshal([]byte(body), &usage); err != nil {
		t.Fatal(err)
	}
	converted := usage.ToOpenAIUsage()
	if converted.UpstreamCost != 0.95 || converted.PromptTokens != 194 || converted.PromptTokensDetails.CachedTokens != 7 {
		t.Fatalf("converted = %+v", converted)
	}
	out, _ := json.Marshal(usage)
	if strings.Contains(string(out), "cost") {
		t.Errorf("the client would see the upstream cost: %s", out)
	}
}
