package openai

import (
	"io"
	"net/http"

	"github.com/modeltaps/modeltaps/common"
	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/types"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// CreateDecision sends an OpenAI Decisions API request. The client's body goes upstream as it
// is, with model set to the upstream name; providers without a Decisions path answer 501.
func (p *OpenAIProvider) CreateDecision(body []byte, modelName string) ([]byte, *types.OpenAIErrorWithStatusCode) {
	url, errWithCode := p.GetSupportedAPIUri(config.RelayModeDecisions)
	if errWithCode != nil {
		return nil, errWithCode
	}
	body, err := sjson.SetBytes(body, "model", modelName)
	if err != nil {
		return nil, common.ErrorWrapperLocal(err, "invalid_request_error", http.StatusBadRequest)
	}
	headers := p.GetRequestHeaders()
	headers["Content-Type"] = "application/json"
	req, err := p.Requester.NewRequest(http.MethodPost, p.GetFullRequestURL(url, modelName),
		p.Requester.WithBody(body), p.Requester.WithHeader(headers))
	if err != nil {
		return nil, common.ErrorWrapper(err, "new_request_failed", http.StatusInternalServerError)
	}
	defer req.Body.Close()

	resp, errWithCode := p.Requester.SendRequestRaw(req)
	if errWithCode != nil {
		return nil, errWithCode
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, common.ErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	if !gjson.ValidBytes(respBody) || !gjson.GetBytes(respBody, "answers").IsArray() {
		return nil, common.StringErrorWrapper("upstream returned no decision answers", "bad_response", http.StatusBadGateway)
	}

	if p.Usage != nil {
		input := int(gjson.GetBytes(respBody, "usage.input_tokens").Int())
		if input > 0 {
			p.Usage.PromptTokens = input
		}
		p.Usage.CompletionTokens = 0
		p.Usage.TotalTokens = p.Usage.PromptTokens
	}
	return respBody, nil
}
