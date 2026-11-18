package settings

// Settings page group "gateway", tab "resilience": what happens when an upstream fails (retries,
// cooldowns, pause rules, channel auto disable and recovery, the error callers see).
func gatewayResilience() []Setting {
	return []Setting{
		// ---------------------------------------------------------------- retry
		{
			Key: "RetryTimes", Scope: ScopeSite, Group: "gateway", Card: "retry", Type: TypeInt, Min: num(0),
			Label: "Retries",
			Description: "How many other channels a failed request is tried on before the caller gets the error. " +
				"Rate limits, upstream 5xx errors, rejected keys and exhausted balances are retried; invalid requests (most 400s), " +
				"408 timeouts, the gateway's own errors and requests pinned to one channel are not. The retries never exceed the " +
				"channels that serve the model and stop once the retry time limit has passed. 0 returns the first failure.",
			Examples: []Example{
				{Value: "2", Effect: "A request to a model served by 5 channels is tried on at most 3 of them (the first plus 2 retries)."},
				{Value: "5", Effect: "With only 2 channels serving the model, the request is tried on both and then fails: retries stop when no untried channel is left."},
				{Value: "0", Effect: "No retry: the first failure is returned to the caller."},
			},
			Related:  []string{"RetryTimeOut", "RetryCooldownSeconds", "SessionAffinityStrict"},
			Keywords: []string{"retry", "failover", "fallback", "attempts", "max retries"},
		},
		{
			Key: "RetryCooldownSeconds", Scope: ScopeSite, Group: "gateway", Card: "retry", Type: TypeInt, Unit: "seconds", Min: num(0),
			Label: "Rate limit cooldown",
			Description: "How long a channel is kept away from a model after it answers that model with a 429 rate limit. " +
				"While it cools down, new requests for the model go to other channels; its other models are not affected. " +
				"When the upstream says when the limit resets, that time is used instead. With supply pause rules on, this is the " +
				"pause of the built-in rate_limit rule (custom rules carry their own seconds); with them off, a status code listed " +
				"under per-status cooldowns takes precedence. 0 only skips the channel for the failed request.",
			Examples: []Example{
				{Value: "5", Effect: "After a 429 for gpt-4o, that channel takes no gpt-4o requests for 5 seconds; it keeps serving its other models."},
				{Value: "60", Effect: "A channel with a per-minute limit sits out the rest of the minute instead of collecting more 429s."},
				{Value: "0", Effect: "No cooldown: the next request may land on the same channel and hit the limit again."},
			},
			Related:  []string{"RetryCooldownPerStatus", "PauseRulesEnabled", "PauseRules"},
			Keywords: []string{"429", "rate limit", "cooldown", "freeze", "backoff", "too many requests"},
		},
		{
			Key: "RetryTimeOut", Scope: ScopeSite, Group: "gateway", Card: "retry", Type: TypeInt, Unit: "seconds", Min: num(0),
			Label: "Retry time limit",
			Description: "No new retry starts once this many seconds have passed since the request arrived. It is checked " +
				"before each retry and does not cut an attempt that is already running, so long generations still finish. " +
				"When it stops the retries, the caller gets a 503 saying the upstream is at capacity. 0 starts no retry at all.",
			Examples: []Example{
				{Value: "10", Effect: "A first channel that fails after 12 seconds is not retried; one that fails after 3 seconds is, as long as the 10 seconds are not used up."},
				{Value: "120", Effect: "Suits long reasoning requests: a channel failing after a minute still leaves room for another try."},
			},
			Related:  []string{"RetryTimes"},
			Keywords: []string{"timeout", "deadline", "retry timeout", "total time"},
		},
		{
			Key: "RetryCooldownPerStatus", Scope: ScopeSite, Group: "gateway", Card: "retry", Type: TypeJSON,
			Label: "Per-status cooldown",
			Description: "Cooldowns in seconds for other HTTP status codes, as a JSON object of status code and seconds. " +
				"Like the rate limit cooldown it keeps the channel away from that one model. With supply pause rules on, the " +
				"built-in per_status rule applies it after the quota, credential, rate limit and model-not-found rules, so a 429 " +
				"entry is never reached there, and a listed 5xx code no longer waits for three failures in a minute. With pause " +
				"rules off, the order is the upstream's reset time, then this list, then the rate limit cooldown for 429. A code " +
				"set to 0 is never cooled. Codes must be 100-599 and seconds whole numbers of 0 or more; empty adds none.",
			Examples: []Example{
				{Value: `{"503": 120}`, Effect: "A channel answering a model with 503 takes no requests for it for 2 minutes, from the first 503 on."},
				{Value: `{"502": 60, "504": 300}`, Effect: "502 cools the model on that channel for 1 minute, 504 for 5 minutes."},
				{Value: `{"500": 0}`, Effect: "A 500 never pauses or cools the channel: the request moves on and the next one may use it again."},
			},
			Related:  []string{"RetryCooldownSeconds", "PauseRules", "PauseRulesEnabled"},
			Keywords: []string{"status code", "503", "502", "cooldown", "freeze"},
			Advanced: true,
		},
		{
			Key: "ModelNotFoundCooldownSeconds", Scope: ScopeSite, Group: "gateway", Card: "retry", Type: TypeInt, Unit: "seconds", Min: num(0),
			Label: "Model not found pause",
			Description: "How long a channel is kept away from a model after its upstream says the model does not exist. " +
				"There is no field for it on this page: change it through the option API (PUT /api/option/). It covers errors " +
				"such as \"model not found\", \"does not exist\" or \"no endpoints found\", but not when a 429 was seen in the same " +
				"request. With supply pause rules on, it is the pause of the built-in model_not_found rule, which also notifies " +
				"admins; custom rules carry their own seconds. 0 never pauses for it.",
			Examples: []Example{
				{Value: "21600", Effect: "A channel that lost access to gpt-4.5 stays out of gpt-4.5 requests for 6 hours, then gets tried again."},
				{Value: "600", Effect: "A model that was only briefly missing upstream comes back after 10 minutes."},
				{Value: "0", Effect: "The channel stays in rotation and keeps failing requests for the model until it is removed from the channel."},
			},
			Related:  []string{"PauseRulesEnabled", "PauseRules", "RetryCooldownSeconds"},
			Keywords: []string{"404", "model not found", "quarantine", "missing model", "no endpoints"},
			Advanced: true,
		},

		// ---------------------------------------------------------------- supply-pause-rules
		{
			Key: "PauseRulesEnabled", Scope: ScopeSite, Group: "gateway", Card: "supply-pause-rules", Type: TypeBool,
			Label: "Supply pause rules",
			Description: "Failed requests go through the pause rules, which pause a channel or one of its models and bring it " +
				"back on their own. When off, the older handling applies instead: automatic channel disabling " +
				"(if switched on), the rate limit and per-status cooldowns, and the model-not-found pause.",
			Examples: []Example{
				{Value: "true", Effect: "A channel whose key is rejected is paused for an hour and resumes after a passing test, without anyone re-enabling it."},
				{Value: "false", Effect: "A rejected key disables the channel when automatic disabling is on, and it stays off until a channel test passes."},
			},
			Related:  []string{"PauseRules", "AutomaticDisableChannelEnabled", "RetryCooldownSeconds"},
			Keywords: []string{"pause", "circuit breaker", "health", "auto disable"},
		},
		{
			Key: "PauseRules", Scope: ScopeSite, Group: "gateway", Card: "supply-pause-rules", Type: TypeJSON,
			Label: "Custom pause rules",
			Description: "A JSON array of pause rules that replaces the built-in rules as a whole; empty uses the built-in ones. " +
				"Rules are tried in order and the first match decides, after the channel's own rules. A rule matches on any of " +
				"status_codes, keywords (substrings of the error, \"re:\" for a regular expression), error_types or error_codes, plus " +
				"the flags unusable, model_not_found, rate_limit_reset and per_status_config; times and window_seconds make it count " +
				"matches first. action is \"pause\" (for seconds, on the whole channel or, by default, the model) or \"retry\" (move on " +
				"without pausing). probe sends a test request before resuming and doubles the pause when it fails; notify tells admins. " +
				"Pauses under a minute are kept in memory only. Saved rules carry their own seconds, so the rate limit and " +
				"model-not-found settings no longer apply to them.",
			Examples: []Example{
				{
					Value:  `[{"name":"quota","status_codes":[402],"keywords":["credit balance is too low"],"action":"pause","seconds":1800,"scope":"channel","probe":true,"notify":true,"category":"quota"}]`,
					Effect: "A 402 or a low-balance message pauses the whole channel for 30 minutes and notifies admins; a test request runs before it resumes, and a balance refresh that finds money ends the pause early.",
				},
				{
					Value:  `[{"name":"overloaded","status_codes":[529],"times":2,"window_seconds":60,"action":"pause","seconds":300,"probe":true}]`,
					Effect: "Two 529s for one model on one channel within a minute pause that model there for 5 minutes; if the test before resuming fails, the next pause lasts 10 minutes.",
				},
				{Value: `[]`, Effect: "No rule: nothing is ever paused, a failing channel is only skipped for the request that failed."},
			},
			Related:  []string{"PauseRulesEnabled", "RetryCooldownSeconds", "RetryCooldownPerStatus", "ModelNotFoundCooldownSeconds", "DisableChannelKeywords"},
			Keywords: []string{"pause rules", "JSON", "circuit breaker", "probe", "quota", "keyword"},
		},

		// ---------------------------------------------------------------- channel-auto
		{
			Key: "AutomaticDisableChannelEnabled", Scope: ScopeSite, Group: "gateway", Card: "channel-auto", Type: TypeBool,
			Label: "Disable channels that can no longer serve",
			Description: "Disables a channel whose upstream says it can no longer serve, such as a rejected key or an empty balance. " +
				"An exhausted quota, a disabled account or a disable keyword in the error count too. On live requests this only happens while supply pause " +
				"rules are off; with them on, such errors pause the channel instead. Channel tests disable for these errors too, " +
				"with or without pause rules. " +
				"A disabled channel comes back only through a passing test with automatic recovery on, or by hand.",
			Related:  []string{"AutomaticEnableChannelEnabled", "DisableChannelKeywords", "PauseRulesEnabled", "ChannelTestMode"},
			Keywords: []string{"auto disable", "disable channel", "dead key", "invalid key", "balance"},
		},
		{
			Key: "AutomaticEnableChannelEnabled", Scope: ScopeSite, Group: "gateway", Card: "channel-auto", Type: TypeBool,
			Label: "Re-enable channels that pass a test",
			Description: "A channel that was disabled automatically is enabled again when it passes a channel test. The test is " +
				"a test of all channels, scheduled or started by hand, and must answer within the maximum response time. Channels an admin disabled are never enabled " +
				"again on their own. When off, every disabled channel waits for an admin.",
			Related:  []string{"AutomaticDisableChannelEnabled", "ChannelTestMode", "ChannelDisableThreshold"},
			Keywords: []string{"auto enable", "recover", "re-enable", "restore channel"},
		},
		{
			Key: "ChannelDisableThreshold", Scope: ScopeSite, Group: "gateway", Card: "channel-auto", Type: TypeFloat, Unit: "seconds", Min: num(0),
			Label: "Maximum response time",
			Description: "A channel test slower than this many seconds disables an enabled channel and keeps a disabled one off. " +
				"It applies to tests of all channels (scheduled in the All channels mode, or started from the channels page), " +
				"whether or not automatic disabling is on; live requests are never timed against it. 0 sets no limit.",
			Examples: []Example{
				{Value: "5", Effect: "A channel that takes 7.3 seconds to answer the test request is disabled; one that answers in 2 seconds keeps serving."},
				{Value: "30", Effect: "Leaves slow reasoning models alone and only disables channels that hang."},
				{Value: "0", Effect: "Tests disable channels for errors only, never for slowness."},
			},
			Related:  []string{"ChannelTestMode", "AutomaticEnableChannelEnabled"},
			Keywords: []string{"slow", "latency", "response time", "threshold", "test"},
		},
		{
			Key: "DisableChannelKeywords", Scope: ScopeSite, Group: "gateway", Card: "channel-auto", Type: TypeList,
			Label: "Disable keywords",
			Description: "Error messages, one per line, that mark a channel as unable to serve when an upstream error contains one. " +
				"Matching is case-sensitive, so copy the upstream text exactly; a keyword wins over the built-in exceptions. A match " +
				"moves the request to another channel, pauses the channel through the built-in credential rule (1 hour, tested " +
				"before resuming) with supply pause rules on, and disables it with them off and automatic disabling on. " +
				"Empty leaves only the built-in checks of status codes and known messages.",
			Examples: []Example{
				{Value: "Your account is currently blocked", Effect: "An upstream error containing this text pauses the whole channel for an hour instead of failing request after request."},
				{Value: "Your credit balance is too low", Effect: "Also matched by the built-in quota rule, which comes first: the channel pauses for 30 minutes and resumes early once a balance refresh finds money."},
			},
			Related:  []string{"AutomaticDisableChannelEnabled", "PauseRules"},
			Keywords: []string{"keywords", "error message", "blocked", "banned", "suspended"},
		},
		{
			Key: "ChannelTestMode", Scope: ScopeSite, Group: "gateway", Card: "channel-auto", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "recovery", Label: "Auto-disabled channels only", Description: "Tests only channels that were disabled automatically and, with automatic recovery on, enables those that pass; healthy channels spend nothing."},
				{Value: "all", Label: "All channels", Description: "Tests every channel not disabled by hand: too slow ones are disabled, failing ones too when automatic disabling is on, and auto-disabled ones that pass come back."},
				{Value: "off", Label: "Off", Description: "No scheduled test: disabled channels stay off until an admin tests or enables them."},
			},
			Label: "Scheduled channel test",
			Description: "Which channels the gateway tests on its own, every test interval. Each test is one small request to the " +
				"channel's test model and is billed by the upstream. The CHANNEL_TEST_FREQUENCY environment variable, when set, " +
				"overrides this: all channels every that many minutes.",
			Related:  []string{"ChannelTestIntervalMinutes", "AutomaticEnableChannelEnabled", "ChannelDisableThreshold"},
			Keywords: []string{"health check", "test", "probe", "monitor", "CHANNEL_TEST_FREQUENCY"},
		},
		{
			Key: "ChannelTestIntervalMinutes", Scope: ScopeSite, Group: "gateway", Card: "channel-auto", Type: TypeInt, Unit: "minutes", Min: num(1),
			Label: "Test interval",
			Description: "Minutes between two scheduled channel tests, at least one. A new interval applies from the next round, " +
				"without a restart. Shorter intervals bring channels back sooner but send more test requests.",
			Examples: []Example{
				{Value: "10", Effect: "An auto-disabled channel whose upstream recovered is back within about 10 minutes."},
				{Value: "60", Effect: "With All channels and 20 channels, 480 test requests a day."},
			},
			Related:  []string{"ChannelTestMode"},
			Keywords: []string{"interval", "frequency", "schedule"},
		},
		{
			Key: "ChannelCredentialRefreshMinutes", Scope: ScopeSite, Group: "gateway", Card: "channel-auto", Type: TypeInt, Unit: "minutes", Min: num(0),
			Label: "Account credential refresh",
			Description: "How often the OAuth credentials of Codex, Claude Code, Gemini CLI and Antigravity channels are checked. " +
				"Credentials expiring before the next check (plus 15 minutes) are refreshed even on idle or auto-disabled channels, " +
				"so they do not fail on their first request after a quiet night; channels disabled by hand are skipped. A channel " +
				"whose refresh starts failing sends one channel credential notice. 0 turns the background refresh off.",
			Examples: []Example{
				{Value: "30", Effect: "Every 30 minutes, tokens that expire within the next 45 minutes are renewed in the background."},
				{Value: "0", Effect: "No background refresh: a token can expire while its channel is idle."},
			},
			Keywords: []string{"OAuth", "token refresh", "Codex", "Claude Code", "Gemini CLI", "account pool"},
			Advanced: true,
		},

		// ---------------------------------------------------------------- error-handling
		{
			Key: "ChannelFailErrorWrapEnabled", Scope: ScopeSite, Group: "gateway", Card: "error-handling", Type: TypeBool,
			Label: "Hide upstream errors",
			Description: "Callers get a generic error instead of the upstream's own text, which may name the provider or show " +
				"parts of a key. Upstream errors other than 400 become a 503 with the unified message (a 429 stays a 429 so " +
				"clients back off), rejected channel keys get a fixed message, and a missing model a 404. 400 errors pass through, " +
				"since they are about the request. Turn it off only for a while, to see what the upstream really says.",
			Related:  []string{"ChannelFailErrorMessage"},
			Keywords: []string{"error message", "mask", "sanitize", "upstream error", "debug"},
		},
		{
			Key: "ChannelFailErrorMessage", Scope: ScopeSite, Group: "gateway", Card: "error-handling", Type: TypeString,
			Label: "Unified error message",
			Description: "The message callers get when an upstream fails and upstream errors are hidden. The request id is " +
				"added to it so a caller can quote it. Empty uses \"Upstream for the current group is at capacity, please try " +
				"again later\". It has no effect while upstream errors are shown.",
			Examples: []Example{
				{Value: "Service busy, please retry in a minute", Effect: "A failed request answers 503 with \"Service busy, please retry in a minute (request id: ...)\"."},
			},
			Related:  []string{"ChannelFailErrorWrapEnabled"},
			Keywords: []string{"error text", "503", "capacity"},
		},
	}
}
