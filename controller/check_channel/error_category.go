package check_channel

import "strings"

// classifyError sorts an upstream error into a stable category key so an admin can tell at a
// glance why a check failed; the web UI translates the key. Rules are tried in order and the first
// match wins. An empty string means the error matched no rule.
func classifyError(statusCode int, errType, message string) string {
	text := strings.ToLower(errType + " " + message)
	switch {
	case statusCode == 401 || containsAny(text, "unauthorized", "invalid api key", "invalid_api_key", "incorrect api key", "authentication"):
		return "auth"
	case containsAny(text, "insufficient balance", "insufficient_quota", "insufficient quota", "quota exceeded", "billing hard limit", "run out of credits", "余额不足"):
		return "quota"
	case containsAny(text, "content policy", "content_policy", "safety", "moderation"):
		return "content_policy"
	case containsAny(text, "does not exist", "model_not_found", "model not found", "unsupported model", "model not supported", "no available channel", "no such model"):
		return "model_unavailable"
	case containsAny(text, "context window", "context length", "maximum context", "too many tokens"):
		return "context_length"
	case statusCode == 429 || containsAny(text, "rate limit", "rate_limit", "overloaded", "high demand", "capacity", "concurrency limit"):
		return "rate_limit"
	case statusCode == 408 || statusCode == 504 || containsAny(text, "timeout", "timed out", "deadline exceeded", "gateway time-out"):
		return "timeout"
	case containsAny(text, "connection reset", "connection refused", "no such host", "tls", "eof"):
		return "network"
	case statusCode == 403:
		return "forbidden"
	case statusCode == 404:
		return "not_found"
	case statusCode == 400 || containsAny(text, "invalid_request", "invalid request", "invalid parameter", "unsupported parameter", "not supported"):
		return "bad_request"
	case statusCode >= 500:
		return "server_error"
	}
	return ""
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
