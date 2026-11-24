package settings

// Settings page group "notify".
func init() {
	add(
		Setting{
			Key: "NotifyDisabledKinds", Scope: ScopeSite, Group: "notify", Card: "notifications", Type: TypeList,
			Label: "Turned-off notice kinds",
			Description: "The kinds of admin notices that are **not sent to any notice channel**; the card's switches write it. " +
				"Comma separated; **empty sends every kind**. The kinds are channel_disabled, channel_enabled, channel_test, " +
				"low_balance, subscription_usage, price_change, price_source, model_drift, model_consistency, credential, supply_paused, " +
				"supply_resumed, supply_failover, supply_above_quote and model_library. A notice channel can also take only " +
				"some kinds.",
			Examples: []Example{
				{Value: "price_change,model_drift", Effect: "Upstream price and model list changes are not sent; every other kind is."},
			},
			Related:  []string{"NotifyKindLimitPerHour"},
			Keywords: []string{"mute", "silence", "alerts", "notification types", "notice kinds"},
			Docs:     "deployment/notify.md",
		},
		Setting{
			Key: "NotifyKindLimitPerHour", Scope: ScopeSite, Group: "notify", Card: "notifications", Type: TypeInt, Min: num(0),
			Label: "Notices per kind per hour",
			Description: "The most notices of one kind sent in an hour; **the rest are dropped**. The hour starts with the first " +
				"notice of the kind, and the first notice after a limited hour says how many were dropped. Each kind counts " +
				"on its own, **on each node**. 0 means no limit.",
			Examples: []Example{
				{Value: "20", Effect: "The default: of 50 channel_disabled notices in one hour, 20 are sent; the next one after the hour says 30 were dropped."},
				{Value: "0", Effect: "Every notice is sent, however many."},
			},
			Related:  []string{"NotifyDisabledKinds"},
			Keywords: []string{"rate limit", "throttle", "flood", "spam"},
			Docs:     "deployment/notify.md",
		},
		Setting{
			Key: "UsageAlertTiers", Scope: ScopeSite, Group: "notify", Card: "notifications", Type: TypeList, Unit: "%",
			Label: "Subscription usage alert tiers",
			Description: "The usage levels, in percent, at which a subscription channel (Codex, Claude Code) sends a notice " +
				"for a usage window. Each tier **fires once per window period** and again after the window resets. Comma " +
				"separated, each between 1 and 100; empty means 50, 75, 100 and 0 turns the alerts off. A channel's own tiers " +
				"on its monitoring tab **win over these**. The notices are of the subscription_usage kind.",
			Examples: []Example{
				{Value: "80, 95", Effect: "A 5-hour window at 82% sends one notice and at 96% another; after the reset both can fire again."},
				{Value: "0", Effect: "No usage alerts, except from channels with tiers of their own."},
			},
			Related:  []string{"NotifyDisabledKinds", "NotifyKindLimitPerHour"},
			Keywords: []string{"usage alert", "quota window", "codex", "claude code", "subscription", "threshold"},
		},
	)
}
