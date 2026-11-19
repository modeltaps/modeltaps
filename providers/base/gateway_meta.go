package base

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// A gateway upstream that routes one model to several hosting providers (Vercel AI Gateway) says
// in each response which provider served the request and what it charged, in a provider_metadata
// object whose place depends on the format: the choice's message (chat), a stream chunk's delta,
// or the top level (messages, responses), and only some formats repeat the charge in usage.cost.
// ObserveGatewayMeta reads both from the body as it passes, whatever the format, streamed or
// not, keeps them on the request context for the consume log (relay_util.ConsumeSnapshot), and
// removes provider_metadata from what goes on: it names the upstream model, the hosts and the
// charge, which a client of this gateway must not see, and formats passed through as they are
// (streamed chunks, Claude messages) would otherwise carry it to the client.

var (
	gatewayHostPattern = regexp.MustCompile(`"finalProvider"\s*:\s*"([^"\\]{1,64})"`)
	// A number in chat responses and usage, a string in messages and responses.
	gatewayCostPattern = regexp.MustCompile(`"(?:gatewayCost|gateway_cost)"\s*:\s*"?([0-9][0-9.eE+-]{0,30})`)
)

// maxGatewayJSONBody bounds a JSON body read whole to remove provider_metadata; a larger one is
// passed on as it is.
const maxGatewayJSONBody = 32 << 20

// ObserveGatewayMeta resets what an earlier attempt left on c and wraps resp's body to record the
// serving provider and the charge, and drop provider_metadata, as it passes.
func ObserveGatewayMeta(c *gin.Context, resp *http.Response) {
	if c == nil || resp == nil || resp.Body == nil {
		return
	}
	c.Set(config.GinUpstreamHostKey, "")
	c.Set(config.GinUpstreamGatewayCostKey, 0.0)
	contentType := resp.Header.Get("Content-Type")
	mode := gatewayRaw
	switch {
	case strings.Contains(contentType, "event-stream"):
		mode = gatewayStream
	case strings.Contains(contentType, "json"):
		mode = gatewayJSON
	default:
		// Audio, images and other bodies carry neither: passed on untouched.
		return
	}
	resp.Body = &gatewayMetaReader{src: resp.Body, lines: bufio.NewReaderSize(resp.Body, 64<<10), c: c, mode: mode}
	resp.ContentLength = -1
	resp.Header.Del("Content-Length")
}

// How the wrapper reads a body: line by line (server-sent events), whole (JSON), or passing it on.
const (
	gatewayRaw = iota
	gatewayStream
	gatewayJSON
)

type gatewayMetaReader struct {
	src   io.ReadCloser
	lines *bufio.Reader
	c     *gin.Context
	mode  int
	out   []byte
	done  bool
	err   error
}

func (r *gatewayMetaReader) Close() error { return r.src.Close() }

func (r *gatewayMetaReader) Read(p []byte) (int, error) {
	for len(r.out) == 0 && !r.done {
		r.fill()
	}
	if len(r.out) == 0 {
		return 0, r.err
	}
	n := copy(p, r.out)
	r.out = r.out[n:]
	return n, nil
}

// fill reads the next line of a stream, or a whole JSON body, records what it says and queues it
// without provider_metadata.
func (r *gatewayMetaReader) fill() {
	switch r.mode {
	case gatewayRaw:
		buf := make([]byte, 32<<10)
		n, err := r.lines.Read(buf)
		r.out = buf[:n]
		if err != nil {
			r.done, r.err = true, err
		}
		return
	case gatewayJSON:
		body, err := io.ReadAll(io.LimitReader(r.lines, maxGatewayJSONBody+1))
		r.observe(body)
		if err == nil && len(body) > maxGatewayJSONBody {
			// Too large to rewrite: the rest follows as it is.
			r.out, r.mode = body, gatewayRaw
			return
		}
		r.done, r.err = true, endErr(err)
		r.out = stripGatewayMeta(body)
		return
	}
	line, err := r.lines.ReadBytes('\n')
	if err != nil {
		r.done, r.err = true, endErr(err)
	}
	if len(line) == 0 {
		return
	}
	r.observe(line)
	if payload, ok := bytes.CutPrefix(line, []byte("data:")); ok && bytes.Contains(payload, []byte("provider_metadata")) {
		trimmed := bytes.TrimSpace(payload)
		stripped := stripGatewayMeta(trimmed)
		if !bytes.Equal(stripped, trimmed) {
			nl := line[len(bytes.TrimRight(line, "\r\n")):]
			line = append(append([]byte("data: "), stripped...), nl...)
		}
	}
	r.out = line
}

func endErr(err error) error {
	if err == nil {
		return io.EOF
	}
	return err
}

// observe records the serving host and the charge found in b; the last one wins, as a stream
// carries the final figures last.
func (r *gatewayMetaReader) observe(b []byte) {
	if m := gatewayHostPattern.FindAllSubmatch(b, -1); len(m) > 0 {
		r.c.Set(config.GinUpstreamHostKey, string(m[len(m)-1][1]))
	}
	if m := gatewayCostPattern.FindAllSubmatch(b, -1); len(m) > 0 {
		if cost, err := strconv.ParseFloat(string(m[len(m)-1][1]), 64); err == nil && cost >= 0 {
			r.c.Set(config.GinUpstreamGatewayCostKey, cost)
		}
	}
}

// stripGatewayMeta removes provider_metadata from a JSON object: at the top level (messages,
// responses), under message (a Claude stream's message_start) and under each choice's message or
// delta (chat). Anything that is not a JSON object is returned as it is.
func stripGatewayMeta(body []byte) []byte {
	if !bytes.Contains(body, []byte("provider_metadata")) || !gjson.ValidBytes(body) {
		return body
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		return body
	}
	paths := []string{"provider_metadata", "message.provider_metadata", "response.provider_metadata"}
	n := int(root.Get("choices.#").Int())
	for i := 0; i < n; i++ {
		paths = append(paths, "choices."+strconv.Itoa(i)+".message.provider_metadata", "choices."+strconv.Itoa(i)+".delta.provider_metadata")
	}
	for _, path := range paths {
		if gjson.GetBytes(body, path).Exists() {
			if out, err := sjson.DeleteBytes(body, path); err == nil {
				body = out
			}
		}
	}
	return body
}

// upstreamCostPaths are the fields routers add to a usage object about what they charged.
var upstreamCostPaths = []string{"usage.cost", "usage.cost_details", "usage.is_byok", "usage.market_cost", "usage.gateway_cost"}

// StripUpstreamCost removes the upstream's charge from a JSON object passed on to the client as
// it is (a streamed chunk): usage.cost and the fields routers send with it. Read the usage first.
func StripUpstreamCost(body []byte) []byte {
	if !bytes.Contains(body, []byte(`"usage"`)) || !bytes.Contains(body, []byte("cost")) && !bytes.Contains(body, []byte("is_byok")) {
		return body
	}
	for _, path := range upstreamCostPaths {
		if gjson.GetBytes(body, path).Exists() {
			if out, err := sjson.DeleteBytes(body, path); err == nil {
				body = out
			}
		}
	}
	return body
}
