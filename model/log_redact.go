package model

import (
	"strings"

	"gorm.io/datatypes"
)

// Logs shown outside the site admin's log page (a user's own logs, an organization's logs, and
// their CSV exports) must not reveal how the gateway served a request: which channel or supply,
// the upstream model name, or what the request cost the gateway. These fields stay in the
// database for site admins.

// adminOnlyLogMetaKeys are metadata keys that describe the channel side of a request.
var adminOnlyLogMetaKeys = []string{
	"supply_id",               // the supply (channel × upstream model) that served the request
	"upstream_model",          // the model name sent to the upstream
	"response_model",          // the model the upstream said it used, when it did not match
	"response_model_mismatch", // flag for the above
	"session_affinity",        // whether the request reused the session's channel
	"upstream_host",           // the hosting provider a gateway upstream routed the request to
	LogMetaChannelFeeRate,     // the channel's fee rate in the logged cost
}

// RedactForMember clears the channel and cost fields of a log that is shown to a user or an
// organization member. Failed attempts of the retry chain keep their status code and cooldown
// flag, without the channel id.
func (l *Log) RedactForMember() {
	if l == nil {
		return
	}
	l.CostQuota = 0
	l.UpstreamRequestId = ""
	l.UpstreamCost = 0
	l.ExpectedCost = 0
	l.CostSource = ""
	l.ChannelId = 0
	l.Channel = nil

	meta := l.Metadata.Data()
	if meta == nil {
		return
	}
	redacted := make(map[string]any, len(meta))
	for k, v := range meta {
		redacted[k] = v
	}
	for _, k := range adminOnlyLogMetaKeys {
		delete(redacted, k)
	}
	if billing, ok := redacted[LogMetaBilling]; ok {
		if kept := redactBilling(billing); kept != nil {
			redacted[LogMetaBilling] = kept
		} else {
			delete(redacted, LogMetaBilling)
		}
	}
	if attempts, ok := redacted["retry_attempts"]; ok {
		if kept := redactRetryAttempts(attempts); len(kept) > 0 {
			redacted["retry_attempts"] = kept
		} else {
			delete(redacted, "retry_attempts")
		}
	}
	l.Metadata = datatypes.NewJSONType(redacted)
}

// RedactLogsForMember applies RedactForMember to every log.
func RedactLogsForMember(logs []*Log) {
	for _, l := range logs {
		l.RedactForMember()
	}
}

// redactRetryAttempts drops channel_id from each attempt ([]any of map[string]any once read back
// from JSON). Anything else is dropped whole, since its shape is unknown.
func redactRetryAttempts(v any) []any {
	list, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]any, 0, len(list))
	for _, item := range list {
		attempt, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kept := make(map[string]any, len(attempt))
		for k, val := range attempt {
			if k != "channel_id" {
				kept[k] = val
			}
		}
		out = append(out, kept)
	}
	return out
}

// redactBilling keeps the billing breakdown a user may see: the quoted amount, the amount
// charged and why. The channel cost is shown only when the request was charged at it, so a
// request charged the quote does not reveal the gateway's margin; admin keys never are.
func redactBilling(v any) map[string]any {
	billing, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	kept := make(map[string]any, len(billing))
	for k, val := range billing {
		if strings.HasPrefix(k, "_admin_") {
			continue
		}
		kept[k] = val
	}
	if kept["basis"] != "channel_cost" {
		delete(kept, "channel_cost_quota")
		delete(kept, "fee_rate")
	}
	return kept
}
