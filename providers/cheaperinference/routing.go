package cheaperinference

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/modeltaps/modeltaps/model"
)

// routing is what a channel asks of Cheaper Inference's routing on every inference request
// (checked 2026-11-19 against the OpenAPI spec of chat completions, completions, responses and
// messages; embeddings take neither field):
//
//   - zdr: only zero-data-retention providers serve the request, at the ZDR price; the
//     marketplace and unverified sellers are left out. With no such supply the request fails
//     with 503 zdr_capacity_unavailable instead of falling back.
//   - min_discount_percent: only supply priced at least this far under the lab's list price
//     serves the request, and the bill is capped at that rate. With none it fails with 503
//     min_discount_unavailable.
//
// Both are set by the channel and win over the same fields from anywhere else (a client body
// passed through, the channel's custom parameters), so a ZDR channel cannot be talked out of it.
type routing struct {
	zdr         bool
	minDiscount float64 // 0 = not set
}

func (r routing) empty() bool {
	return !r.zdr && r.minDiscount <= 0
}

// routingOf reads the channel's plugin settings. A minimum discount outside (0, 99.99] is
// ignored.
func routingOf(channel *model.Channel) routing {
	var r routing
	opts := pluginOptions(channel, "cheaperinference_routing")
	if opts == nil {
		return r
	}
	switch v := opts["zdr"].(type) {
	case string:
		r.zdr = v == "on"
	case bool:
		r.zdr = v
	}
	if d, ok := percent(opts["min_discount_percent"]); ok {
		r.minDiscount = d
	}
	return r
}

// pluginOptions is one plugin's settings: "cheaperinference" (prompt caching) or
// "cheaperinference_routing" (zdr, minimum discount).
func pluginOptions(channel *model.Channel, plugin string) map[string]interface{} {
	if channel == nil || channel.Plugin == nil {
		return nil
	}
	opts, ok := channel.Plugin.Data()[plugin]
	if !ok {
		return nil
	}
	return opts
}

func percent(v interface{}) (float64, bool) {
	var d float64
	switch x := v.(type) {
	case float64:
		d = x
	case int:
		d = float64(x)
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(x), "%")), 64)
		if err != nil {
			return 0, false
		}
		d = parsed
	default:
		return 0, false
	}
	if math.IsNaN(d) || d <= 0 || d > 99.99 {
		return 0, false
	}
	return math.Round(d*100) / 100, true
}

// routedPaths are the endpoints that take both fields.
var routedPaths = []string{"/v1/chat/completions", "/v1/completions", "/v1/responses", "/v1/messages"}

func routed(path string) bool {
	path = strings.TrimSuffix(path, "/")
	for _, p := range routedPaths {
		if strings.HasSuffix(path, p) {
			return true
		}
	}
	return false
}

// maxRoutedBody bounds the body the hook rewrites; the upstream refuses more than 20 MB anyway.
const maxRoutedBody = 32 << 20

// routingHook returns the requester hook that writes the channel's routing fields into the JSON
// body of each inference request; nil when the channel sets none.
func routingHook(channel *model.Channel) func(*http.Request) error {
	r := routingOf(channel)
	if r.empty() {
		return nil
	}
	return func(req *http.Request) error {
		if req.Method != http.MethodPost || req.Body == nil || !routed(req.URL.Path) {
			return nil
		}
		if ct := req.Header.Get("Content-Type"); ct != "" && !strings.Contains(ct, "json") {
			return nil
		}
		body, err := io.ReadAll(io.LimitReader(req.Body, maxRoutedBody+1))
		_ = req.Body.Close()
		if err != nil {
			return err
		}
		if len(body) > maxRoutedBody {
			return errors.New("request body too large to apply the channel's routing settings")
		}
		body, err = r.apply(body)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		req.Header.Set("Content-Length", strconv.Itoa(len(body)))
		req.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(body)), nil }
		return nil
	}
}

// apply sets the fields on a JSON object body. A body that is not a JSON object is refused rather
// than sent without them: for zdr that would send the prompt where the channel says it must not go.
func (r routing) apply(body []byte) ([]byte, error) {
	if !gjson.ValidBytes(body) || !gjson.ParseBytes(body).IsObject() {
		return nil, errors.New("the channel's routing settings need a JSON object body")
	}
	var err error
	if r.zdr {
		if body, err = sjson.SetBytes(body, "zdr", true); err != nil {
			return nil, fmt.Errorf("set zdr: %w", err)
		}
	}
	if r.minDiscount > 0 {
		if body, err = sjson.SetBytes(body, "min_discount_percent", r.minDiscount); err != nil {
			return nil, fmt.Errorf("set min_discount_percent: %w", err)
		}
	}
	return body, nil
}
