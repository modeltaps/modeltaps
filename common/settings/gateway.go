package settings

// Settings page group "gateway", in the order of its tabs: resilience (gateway_resilience.go) and
// request (gateway_request.go).
func init() {
	add(gatewayResilience()...)
	add(gatewayRequest()...)
}
