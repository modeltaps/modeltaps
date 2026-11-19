package base

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
)

// observe passes body through the wrapper, one byte per read so every field is split across
// reads, and returns what comes out.
func observe(t *testing.T, c *gin.Context, body string) string {
	t.Helper()
	header := http.Header{}
	if strings.HasPrefix(body, "data:") {
		header.Set("Content-Type", "text/event-stream")
	} else {
		header.Set("Content-Type", "application/json")
	}
	resp := &http.Response{Header: header, Body: io.NopCloser(iotest.OneByteReader(strings.NewReader(body)))}
	ObserveGatewayMeta(c, resp)
	got, err := io.ReadAll(iotest.OneByteReader(resp.Body))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "provider_metadata") || strings.Contains(string(got), "finalProvider") {
		t.Errorf("provider_metadata passed on: %s", got)
	}
	return string(got)
}

// The serving host and the charge are read from any response shape, streamed or not, the last
// one winning; every response starts from nothing, so a retry does not inherit an earlier one.
func TestObserveGatewayMeta(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	observe(t, c, `{"choices":[{"message":{"content":"","provider_metadata":{"gateway":{"routing":{"resolvedProvider":"alibaba","finalProvider":"deepinfra"},"cost":"0.1","gatewayCost":0.000009825}}}}],"usage":{"cost":9.825e-06}}`)
	if c.GetString(config.GinUpstreamHostKey) != "deepinfra" || c.GetFloat64(config.GinUpstreamGatewayCostKey) != 0.000009825 {
		t.Fatalf("chat: host %q cost %v", c.GetString(config.GinUpstreamHostKey), c.GetFloat64(config.GinUpstreamGatewayCostKey))
	}
	stream := "data: {\"choices\":[{\"delta\":{\"provider_metadata\":{\"gateway\":{\"routing\":{\"finalProvider\":\"wafer\"}}}}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"provider_metadata\":{\"gateway\":{\"routing\":{\"finalProvider\":\"novita\"}}}}}]}\n\n" +
		"data: {\"usage\":{\"cost\":0.0000117,\"gateway_cost\":1.17e-05}}\n\ndata: [DONE]\n\n"
	out := observe(t, c, stream)
	if !strings.Contains(out, "data: [DONE]\n\n") || !strings.Contains(out, `"gateway_cost":1.17e-05`) || strings.Count(out, "data: ") != 4 {
		t.Errorf("stream out = %q", out)
	}
	if c.GetString(config.GinUpstreamHostKey) != "novita" || c.GetFloat64(config.GinUpstreamGatewayCostKey) != 1.17e-05 {
		t.Fatalf("stream: host %q cost %v", c.GetString(config.GinUpstreamHostKey), c.GetFloat64(config.GinUpstreamGatewayCostKey))
	}
	// Messages and responses: top-level provider_metadata, the charge as a string.
	observe(t, c, `{"type":"message","usage":{"input_tokens":49,"output_tokens":24},"provider_metadata":{"gateway":{"routing":{"finalProvider":"alibaba"},"cost":"0.0000845","gatewayCost":"0.0000845"}}}`)
	if c.GetString(config.GinUpstreamHostKey) != "alibaba" || c.GetFloat64(config.GinUpstreamGatewayCostKey) != 0.0000845 {
		t.Fatalf("messages: host %q cost %v", c.GetString(config.GinUpstreamHostKey), c.GetFloat64(config.GinUpstreamGatewayCostKey))
	}
	// A JSON body larger than the limit is passed on whole, as it is.
	big := `{"provider_metadata":{"gateway":{"routing":{"finalProvider":"zai"}}},"pad":"` + strings.Repeat("x", maxGatewayJSONBody) + `"}`
	resp := &http.Response{Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(big))}
	ObserveGatewayMeta(c, resp)
	if got, _ := io.ReadAll(resp.Body); string(got) != big {
		t.Errorf("large body: %d bytes out of %d", len(got), len(big))
	}
	// Other bodies are not wrapped.
	audio := &http.Response{Header: http.Header{"Content-Type": {"audio/mpeg"}}, Body: io.NopCloser(strings.NewReader("ID3"))}
	ObserveGatewayMeta(c, audio)
	if _, wrapped := audio.Body.(*gatewayMetaReader); wrapped {
		t.Error("audio body wrapped")
	}
	if out := observe(t, c, `{"error":{"message":"rate limited"}}`); out != `{"error":{"message":"rate limited"}}` {
		t.Errorf("error body changed: %s", out)
	}
	if c.GetString(config.GinUpstreamHostKey) != "" || c.GetFloat64(config.GinUpstreamGatewayCostKey) != 0 {
		t.Fatalf("reset: host %q cost %v", c.GetString(config.GinUpstreamHostKey), c.GetFloat64(config.GinUpstreamGatewayCostKey))
	}
}

// A streamed chunk loses the router's charge from its usage and keeps the token counts.
func TestStripUpstreamCost(t *testing.T) {
	in := `{"choices":[{"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":62,"completion_tokens":19,"cost":0.000088,"is_byok":false,"cost_details":{"upstream_inference_cost":null},"market_cost":0.000088,"gateway_cost":0.000088}}`
	out := string(StripUpstreamCost([]byte(in)))
	if strings.Contains(out, "cost") || strings.Contains(out, "is_byok") || !strings.Contains(out, `"completion_tokens":19`) {
		t.Errorf("out = %s", out)
	}
	plain := `{"choices":[{"delta":{"content":"cost"}}]}`
	if got := string(StripUpstreamCost([]byte(plain))); got != plain {
		t.Errorf("chunk without usage changed: %s", got)
	}
}
