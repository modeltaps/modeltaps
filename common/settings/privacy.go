package settings

// Settings page group "privacy".
func init() {
	add(
		// Card "log-consume".
		Setting{
			Key: "LogConsumeEnabled", Scope: ScopeSite, Group: "privacy", Card: "log-consume", Type: TypeBool,
			Label: "Consume logs",
			Description: "Write a consume log for every charged request: user, API key, model, tokens, cost, channel and timing. " +
				"When off no new records are written and charging goes on as before, but the log list, the usage " +
				"statistics built from logs and per-user histories have a gap. Top-ups, sign-up bonuses and other system logs " +
				"are written either way.",
			Related:  []string{"LogAutoDeleteEnabled", "LogIOEnabled"},
			Keywords: []string{"usage log", "request log", "billing log", "audit"},
		},
		Setting{
			Key: "LogRelayErrorEnabled", Scope: ScopeSite, Group: "privacy", Card: "log-consume", Type: TypeBool,
			Label: "Log failed requests",
			Description: "Write an error log when a request still fails after its retries, with the error the client got and " +
				"every channel tried. Requests refused locally (insufficient quota, invalid parameters) are not logged.",
			Keywords: []string{"error log", "failed request", "upstream error", "troubleshooting"},
		},
		Setting{
			Key: "ChannelMetricsEnabled", Scope: ScopeSite, Group: "privacy", Card: "log-consume", Type: TypeBool,
			Label: "Hourly channel metrics",
			Description: "Count attempts, successes, latency, time to first token and output speed per channel, model and hour. " +
				"The counters are kept in memory, written every minute and read through GET /api/channel/metrics. When off " +
				"nothing new is counted; stored hours stay until they expire.",
			Related:  []string{"ChannelMetricsRetentionDays"},
			Keywords: []string{"success rate", "latency", "TTFT", "throughput", "channel health", "monitoring"},
		},
		Setting{
			Key: "ChannelMetricsRetentionDays", Scope: ScopeSite, Group: "privacy", Card: "log-consume", Type: TypeInt,
			Unit: "days", Min: num(0),
			Label:       "Keep metrics for (days)",
			Description: "How many days of hourly channel metrics are kept; older hours are deleted every night at 03:10 server time. 0 keeps them forever.",
			Examples: []Example{
				{Value: "30", Effect: "The default: at most 30 x 24 = 720 rows per channel and model."},
				{Value: "0", Effect: "Nothing is deleted: the table grows by one row per channel, model and hour in use."},
			},
			Related:  []string{"ChannelMetricsEnabled"},
			Keywords: []string{"metrics retention", "cleanup"},
		},
		Setting{
			Key: "LogAutoDeleteEnabled", Scope: ScopeSite, Group: "privacy", Card: "log-consume", Type: TypeBool,
			Label: "Delete old consume logs",
			Description: "Delete consume logs older than the retention days every night at 03:00 server time, with their kept " +
				"bodies. Error and system logs stay, and so do the daily usage statistics already " +
				"built from the deleted logs.",
			Related:  []string{"LogAutoDeleteDays", "LogConsumeEnabled"},
			Keywords: []string{"log retention", "cleanup", "purge", "data retention", "GDPR"},
		},
		Setting{
			Key: "LogAutoDeleteDays", Scope: ScopeSite, Group: "privacy", Card: "log-consume", Type: TypeInt,
			Unit: "days", Min: num(0),
			Label: "Keep consume logs for (days)",
			Description: "How many days of consume logs are kept when old logs are deleted automatically. Older logs go in the " +
				"nightly cleanup; 0 deletes nothing.",
			Examples: []Example{
				{Value: "30", Effect: "The default: a month of logs is kept; a log written on 1 May is gone by 1 June."},
				{Value: "90", Effect: "A quarter is kept, about three times the rows of the default."},
			},
			Related:  []string{"LogAutoDeleteEnabled"},
			Keywords: []string{"log retention", "retention days"},
		},
		// Card "log-io".
		Setting{
			Key: "LogIOEnabled", Scope: ScopeSite, Group: "privacy", Card: "log-io", Type: TypeBool,
			Label: "Allow keeping request bodies",
			Description: "Site-wide switch for keeping the full request and response bodies of API calls, for troubleshooting. " +
				"When off (the default, zero data retention) no API key keeps them and the option is hidden from personal and " +
				"organization settings. When on, each user's or organization's own choice decides, and the defaults for personal " +
				"and organization API keys apply until they choose. Bodies are cut at 1 MB each by default, kept 30 days, and " +
				"never include API keys or auth headers.",
			Related:  []string{"LogIODefaultUser", "OrganizationLogIODefault"},
			Keywords: []string{"zero data retention", "ZDR", "request body", "response body", "prompt logging", "privacy"},
		},
		Setting{
			Key: "LogIODefaultUser", Scope: ScopeSite, Group: "privacy", Card: "log-io", Type: TypeBool,
			Label: "Default for personal API keys",
			Description: "Whether personal API keys keep request and response bodies while their user has not chosen. Users can " +
				"change it any time in their personal settings. Needs Allow keeping request bodies.",
			Related:  []string{"LogIOEnabled", "OrganizationLogIODefault"},
			Keywords: []string{"request body", "retention default", "privacy"},
		},
		Setting{
			Key: "OrganizationLogIODefault", Scope: ScopeSite, Group: "privacy", Card: "log-io", Type: TypeBool,
			Label: "Default for organization API keys",
			Description: "Whether organization API keys keep request and response bodies while the organization has not chosen. " +
				"Organization admins can change it in the organization settings. Needs Allow keeping request bodies.",
			Related:  []string{"LogIOEnabled", "LogIODefaultUser"},
			Keywords: []string{"request body", "retention default", "organization", "privacy"},
		},
		// Card "safety".
		Setting{
			Key: "EnableSafe", Scope: ScopeSite, Group: "privacy", Card: "safety", Type: TypeBool,
			Label: "Content safety check",
			Description: "Check the text of every request with the safety tool before it goes to a channel, and refuse matches " +
				"with a 400 error. Messages, system prompts, tool definitions and the client events of Realtime and Gemini " +
				"Live sessions are checked; images and audio are not.",
			Related:  []string{"SafeToolName", "SafeKeyWords"},
			Keywords: []string{"moderation", "content filter", "keyword filter", "prompt safety", "censorship"},
		},
		Setting{
			Key: "SafeToolName", Scope: ScopeSite, Group: "privacy", Card: "safety", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "Keyword", Label: "Keyword", Description: "Refuses requests whose text contains a word of the blocked keywords."},
			},
			Advanced: true,
			Label:    "Safety tool",
			Description: "The tool that runs the content safety check. Keyword is the one built in. A tool that is not " +
				"installed refuses every request.",
			Related:  []string{"EnableSafe", "SafeKeyWords"},
			Keywords: []string{"moderation tool", "content filter"},
		},
		Setting{
			Key: "SafeKeyWords", Scope: ScopeSite, Group: "privacy", Card: "safety", Type: TypeList,
			Label: "Blocked keywords",
			Description: "The words the Keyword tool blocks, one per line. A request is refused when its text contains any of " +
				"them, ignoring case, also inside longer words. Empty lines are ignored.",
			Examples: []Example{
				{Value: "casino", Effect: "Refuses \"Plan a casino night\" and \"best casinos\"."},
				{Value: "ass", Effect: "Also refuses \"class\" and \"password\": short words match inside longer ones, so list whole, unusual words."},
			},
			Related:  []string{"EnableSafe", "SafeToolName"},
			Keywords: []string{"blocklist", "banned words", "keyword filter", "moderation"},
		},
	)
}
