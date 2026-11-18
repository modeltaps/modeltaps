package relay

import (
	"fmt"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"
	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/common/utils"
	"github.com/modeltaps/modeltaps/model"
	"github.com/modeltaps/modeltaps/relay/relay_util"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// A routing policy can name several models for one request: a fallback policy's models in order,
// or an auto policy's target and then its fallback model. The retry loop works through the lines
// of one model; these helpers move the request on to the next model when they are used up.

// setProviderOnChain picks a line for the request's model, and when the model has none (every
// line cooling down or paused) moves on along the routing chain until one has.
func setProviderOnChain(relay RelayBaseInterface) error {
	for {
		err := relay.setProvider(relay.getOriginalModel())
		if err == nil {
			return nil
		}
		c := relay.getContext()
		decision := relay_util.RoutingDecisionFromContext(c)
		from := ""
		if decision != nil {
			from = decision.TargetModel
		}
		next, ok := advanceRoutingChain(relay)
		if !ok {
			return err
		}
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("routing_fallback_no_channel from=%s to=%s error=\"%s\"", from, next, err.Error()))
	}
}

// switchRoutingModel moves the request to the next model of its chain and picks a line for it,
// skipping models that have none. It returns the model it settled on.
func switchRoutingModel(relay RelayBaseInterface) (string, error) {
	next, ok := advanceRoutingChain(relay)
	if !ok {
		return "", fmt.Errorf("no model left in the routing chain")
	}
	if err := setProviderOnChain(relay); err != nil {
		return "", err
	}
	if decision := relay_util.RoutingDecisionFromContext(relay.getContext()); decision != nil {
		next = decision.TargetModel
	}
	return next, nil
}

// advanceRoutingChain moves the decision to its next model, forgets the lines the previous model
// skipped (they are another model's lines) and updates the route header.
func advanceRoutingChain(relay RelayBaseInterface) (string, bool) {
	c := relay.getContext()
	decision := relay_util.RoutingDecisionFromContext(c)
	next, ok := decision.Advance()
	if !ok {
		return "", false
	}
	c.Set("skip_channel_ids", []int(nil))
	c.Header(relay_util.RoutingRouteHeader, decision.HeaderValue())
	return next, true
}

// requestModelsMax caps the fallback models a request may list of its own.
const requestModelsMax = 8

// requestModelsDecision reads the request's own fallback models: a top-level "models" array, as
// OpenRouter takes it. The field is removed from the body either way, so no upstream sees it.
// Each listed model must be one the request could have called by itself: published, allowed by
// the key, the organization and its approved models, and not a routing policy. Models that fail
// these checks are dropped (and logged); nil when none is left.
func requestModelsDecision(c *gin.Context, modelName string) *RoutingDecision {
	listed := stripRequestModels(c)
	if len(listed) == 0 {
		return nil
	}
	var guardrail *model.OrgMemberGuardrail
	if memberId := c.GetInt("token_created_by"); memberId > 0 {
		g, err := cacheGetOrgMemberGuardrailFn(c.GetInt("id"), memberId)
		if err != nil || g == nil {
			logger.LogWarn(c.Request.Context(), "request_models_ignored reason=guardrail_unavailable")
			return nil
		}
		guardrail = g
	}
	scope := routingScope(c)
	primary, _ := model.ResolveCanonicalModel(modelName)
	seen := map[string]bool{strings.ToLower(primary): true}
	chain := []string{modelName}
	for _, name := range listed {
		if len(chain) > requestModelsMax {
			break
		}
		canonical, _ := model.ResolveCanonicalModel(name)
		key := strings.ToLower(canonical)
		if seen[key] {
			continue
		}
		seen[key] = true
		if reason := requestModelRefusal(c, scope, guardrail, name, canonical); reason != "" {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("request_models_dropped model=%s reason=%s", name, reason))
			continue
		}
		chain = append(chain, canonical)
	}
	if len(chain) < 2 {
		return nil
	}
	return &RoutingDecision{Requested: modelName, TargetModel: modelName, Reason: RoutingReasonRequestModels, Chain: chain}
}

// requestModelRefusal says why a listed fallback model may not be used, "" when it may.
func requestModelRefusal(c *gin.Context, scope model.RoutingScope, guardrail *model.OrgMemberGuardrail, name, canonical string) string {
	if model.ResolveRoutingPolicy(scope, name) != nil {
		return "routing_policy"
	}
	if (config.ModelSupplyEnabled || config.CatalogEnforceHidden()) && model.IsModelHidden(canonical) {
		return "hidden"
	}
	if err := CheckLimitModel(c, name); err != nil {
		return "token_limit"
	}
	if guardrail != nil {
		listId := tokenAccessListIdFromContext(c)
		if !guardrail.ModelAllowed(name) || !guardrail.TokenListAllows(listId, name) || !guardrail.ApprovedAllows(canonical) {
			return "organization"
		}
	}
	return ""
}

// stripRequestModels removes the top-level "models" field from the cached request body (bytes and
// the pre-parsed map) and returns the model names it listed.
func stripRequestModels(c *gin.Context) []string {
	body, _ := utils.GetGinValue[[]byte](c, config.GinRequestBodyKey)
	field := gjson.GetBytes(body, "models")
	if !field.Exists() {
		return nil
	}
	if stripped, err := sjson.DeleteBytes(body, "models"); err == nil {
		c.Set(config.GinRequestBodyKey, stripped)
	}
	if raw, ok := utils.GetGinValue[map[string]any](c, config.GinRawMapBodyKey); ok && raw != nil {
		delete(raw, "models")
	}
	names := make([]string, 0)
	for _, item := range field.Array() {
		if name := strings.TrimSpace(item.String()); item.Type == gjson.String && name != "" {
			names = append(names, name)
		}
	}
	return names
}
