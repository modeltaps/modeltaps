package relay

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/modeltaps/modeltaps/model"
	providersBase "github.com/modeltaps/modeltaps/providers/base"
	"github.com/modeltaps/modeltaps/relay/relay_util"
	"github.com/modeltaps/modeltaps/types"

	"github.com/tidwall/gjson"
)

// A policy whose decision model is a decision model (served on /v1/decisions) asks it one choice
// question: the policy's question, with each route label as an option. The model answers with a
// probability for every label, so no prompt, logprobs or self-reported confidence is involved.

// routingDecisionsDefaultQuestion is asked when the policy has no question of its own.
const routingDecisionsDefaultQuestion = "Which label best describes this request?"

// isDecisionClassifier reports whether the policy's decision model is a decision model.
func isDecisionClassifier(name string) bool {
	return model.ModelKindOf(name) == model.ModelKindDecision
}

type decisionsChoice struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type decisionsQuestion struct {
	Type         string            `json:"type"`
	Name         string            `json:"name"`
	Instructions string            `json:"instructions"`
	Choices      []decisionsChoice `json:"choices"`
}

type decisionsRequest struct {
	Model     string              `json:"model"`
	Input     string              `json:"input"`
	Questions []decisionsQuestion `json:"questions"`
}

// buildDecisionsBody is the /v1/decisions request that classifies text by the policy's routes.
func buildDecisionsBody(policy *model.RoutingPolicy, upstreamModel, text string) ([]byte, error) {
	question := policy.Question
	if question == "" {
		question = routingDecisionsDefaultQuestion
	}
	choices := make([]decisionsChoice, len(policy.Routes))
	for i, route := range policy.Routes {
		choices[i] = decisionsChoice{Value: route.Label, Description: route.Description}
	}
	return json.Marshal(decisionsRequest{
		Model: upstreamModel,
		Input: text,
		Questions: []decisionsQuestion{{
			Type:         "choice",
			Name:         "route",
			Instructions: question,
			Choices:      choices,
		}},
	})
}

// parseDecisionsAnswer reads the choice answer: the chosen label with its probability, and the
// probability of every label. The confidence is the chosen label's probability when the answer
// lists it, so the threshold compares probabilities as in logprobs scoring.
func parseDecisionsAnswer(policy *model.RoutingPolicy, body []byte) (*ClassifyResult, error) {
	answer := gjson.GetBytes(body, "answers.0")
	if !answer.Exists() {
		return nil, errors.New("decision model returned no answer")
	}
	if answer.Get("type").String() == "refusal" {
		return nil, errors.New("decision model refused to answer")
	}
	label := answer.Get("choice").String()
	if label == "" {
		return nil, errors.New("decision model returned no choice")
	}
	confidence := answer.Get("confidence").Float()
	byLabel := map[string]float64{}
	coverage := 0.0
	answer.Get("probabilities").ForEach(func(_, item gjson.Result) bool {
		p := item.Get("probability").Float()
		byLabel[item.Get("value").String()] += p
		coverage += p
		return true
	})
	var probabilities []relay_util.RoutingLabelProbability
	if len(byLabel) > 0 {
		probabilities = make([]relay_util.RoutingLabelProbability, 0, len(policy.Routes))
		for _, route := range policy.Routes {
			probabilities = append(probabilities, relay_util.RoutingLabelProbability{Label: route.Label, Probability: byLabel[route.Label]})
		}
		if p, ok := byLabel[label]; ok {
			confidence = p
		}
	}
	return &ClassifyResult{
		Label:         label,
		Confidence:    math.Max(0, math.Min(confidence, 1)),
		Scoring:       model.RoutingScoringDecision,
		Probabilities: probabilities,
		Coverage:      math.Min(coverage, 1),
	}, nil
}

// classifyWithDecisionsAPI asks the decision model the policy's question on /v1/decisions.
func classifyWithDecisionsAPI(policy *model.RoutingPolicy, call *decisionCall, text string) (*ClassifyResult, error) {
	provider, ok := call.provider.(providersBase.DecisionsInterface)
	if !ok {
		return nil, fmt.Errorf("decision model %s: channel does not support /v1/decisions", policy.DecisionModel)
	}
	body, err := buildDecisionsBody(policy, call.upstreamModel, text)
	if err != nil {
		return nil, err
	}
	usage := &types.Usage{}
	var response []byte
	err = call.do(usage, func() error {
		var errWithCode *types.OpenAIErrorWithStatusCode
		response, errWithCode = provider.CreateDecision(body, call.upstreamModel)
		if errWithCode != nil {
			return errors.New(errWithCode.Message)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	result, err := parseDecisionsAnswer(policy, response)
	if err != nil {
		return nil, err
	}
	result.Usage = usage
	return result, nil
}
