package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/requester"
	"github.com/modeltaps/modeltaps/model"

	"go.uber.org/zap"
)

// A test model that the relay sends over the Responses API (directly, or after
// channel model mapping) must be tested over /v1/responses as well. o1/o3/o4
// names are already routed there by getModelType; these cases are not.
func TestTestChannelUsesResponsesForResponsesOnlyModel(t *testing.T) {
	cases := []struct {
		name    string
		model   string
		mapping string
	}{
		{"codex-mini-latest", "codex-mini-latest", ""},
		{"mapped to o3-pro", "my-pro", `{"my-pro":"o3-pro"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testChannelOverResponses(t, tc.model, tc.mapping)
		})
	}
}

func testChannelOverResponses(t *testing.T, testModel, mapping string) {
	setModelInfoModeSwitch(t, false)
	oldLogger := logger.Logger
	logger.Logger = zap.NewNop()
	t.Cleanup(func() { logger.Logger = oldLogger })
	requester.InitHttpClient()

	var chatHits, responsesHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/responses":
			responsesHits.Add(1)
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "event: response.output_text.delta\n")
			fmt.Fprint(w, `data: {"type":"response.output_text.delta","delta":"hi"}`+"\n\n")
			fmt.Fprint(w, "event: response.completed\n")
			fmt.Fprint(w, `data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`+"\n\n")
		default:
			chatHits.Add(1)
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":{"message":"This model is only supported in v1/responses","type":"invalid_request_error"}}`)
		}
	}))
	t.Cleanup(upstream.Close)

	baseURL := upstream.URL
	channel := &model.Channel{
		Type:      config.ChannelTypeOpenAI,
		Key:       "sk-test",
		BaseURL:   &baseURL,
		Models:    testModel,
		TestModel: testModel,
	}
	if mapping != "" {
		channel.ModelMapping = &mapping
	}

	if _, err := testChannel(channel, testModel); err != nil {
		t.Fatalf("testChannel: %v", err)
	}
	if responsesHits.Load() != 1 || chatHits.Load() != 0 {
		t.Fatalf("responses hits = %d, chat hits = %d; want 1 and 0", responsesHits.Load(), chatHits.Load())
	}
}
