package relay

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/model"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/types"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// relayDecisions serves POST /v1/decisions, the OpenAI Decisions API: typed questions about a
// shared input, answered with probabilities, choices and scores. The body is relayed as the
// client sent it (only the model renamed) and billed by the input tokens the upstream reports.
type relayDecisions struct {
	relayBase
	request types.DecisionRequest
}

func NewRelayDecisions(c *gin.Context) *relayDecisions {
	relay := &relayDecisions{}
	relay.c = c
	return relay
}

func (r *relayDecisions) setRequest() error {
	if err := common.UnmarshalBodyReusable(r.c, &r.request); err != nil {
		return err
	}
	if !gjson.ParseBytes(r.request.Questions).IsArray() || len(gjson.ParseBytes(r.request.Questions).Array()) == 0 {
		return errors.New("questions must be a non-empty array")
	}
	if len(r.request.Input) == 0 || string(r.request.Input) == "null" {
		return errors.New("field input is required")
	}
	r.setOriginalModel(r.request.Model)
	return nil
}

func (r *relayDecisions) getRequest() any {
	return &r.request
}

// getPromptTokens estimates the input for the pre-charge: the text of the input and of the
// questions. The bill uses the upstream's usage.input_tokens when it reports one.
func (r *relayDecisions) getPromptTokens() (int, error) {
	var text strings.Builder
	collectText(gjson.ParseBytes(r.request.Input), &text)
	text.WriteString(string(r.request.Questions))
	return common.CountTokenText(text.String(), r.modelName), nil
}

// collectText gathers every string in v except base64 data URLs (images are not counted).
func collectText(v gjson.Result, out *strings.Builder) {
	switch {
	case v.IsArray() || v.IsObject():
		v.ForEach(func(_, item gjson.Result) bool {
			collectText(item, out)
			return true
		})
	case v.Type == gjson.String:
		if s := v.String(); !strings.HasPrefix(s, "data:") {
			out.WriteString(s)
			out.WriteByte('\n')
		}
	}
}

func (r *relayDecisions) send() (err *types.OpenAIErrorWithStatusCode, done bool) {
	provider, ok := r.provider.(providersBase.DecisionsInterface)
	if !ok {
		err = common.StringErrorWrapperLocal("channel not implemented", "channel_error", http.StatusServiceUnavailable)
		done = true
		return
	}
	raw, _ := r.c.Get(config.GinRequestBodyKey)
	body, _ := raw.([]byte)

	response, err := provider.CreateDecision(body, r.modelName)
	if err != nil {
		return
	}
	r.SetFirstResponseTime(time.Now())

	// The gateway's routing and cost details stay with us; the client sees the model it asked for.
	response, _ = sjson.DeleteBytes(response, "provider_metadata")
	response, _ = sjson.SetBytes(response, "model", r.getOriginalModel())
	r.c.Data(http.StatusOK, "application/json", response)
	return nil, false
}

// decisionEndpointMismatch explains why the catalog says name cannot be served on path, or "".
// A model whose endpoints the catalog does not state passes either way.
func decisionEndpointMismatch(path, name string) string {
	onDecisions := strings.HasPrefix(path, "/v1/decisions")
	if !onDecisions && model.IsDecisionOnlyModel(name) {
		return "model " + name + " is a decision model, not a language model; call POST /v1/decisions instead of " + path
	}
	if onDecisions {
		if endpoints := model.ModelEndpoints(name); len(endpoints) > 0 && !slices.Contains(endpoints, model.ModelEndpointDecisions) {
			return "model " + name + " is not a decision model; /v1/decisions takes decision models only"
		}
	}
	return ""
}
