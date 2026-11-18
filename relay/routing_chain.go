package relay

import (
	"fmt"

	"github.com/modeltaps/modeltaps/common/logger"
	"github.com/modeltaps/modeltaps/relay/relay_util"
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
