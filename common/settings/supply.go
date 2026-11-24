package settings

// Settings page group "supply": which supply serves a request.
func init() { add(supply()...) }

func supply() []Setting {
	return []Setting{
		// ---------------------------------------------------------------- supply-mode
		{
			Key: "SupplyCostOrder", Scope: ScopeSite, Group: "supply", Card: "supply-mode", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "priority", Label: "Call order first", Description: "Supplies are called in the call order of the providers page; cost does not change the order."},
				{Value: "cost", Label: "Lowest cost", Description: "Supplies with a clearly lower cost that serve reliably are called ahead of the call order."},
			},
			Label: "Order by",
			Description: "Whether a supply's cost (its channel's list price with the channel's fee) affects the call order. " +
				"**Users always pay the catalog price times their group ratio**; this setting **affects only the platform's cost**. With " +
				"lowest cost, a supply is called earlier only when it **meets both the minimum saving and the minimum success rate**; " +
				"supplies of similar cost keep the call order and share a tier by weight, and supplies of unknown cost come last.",
			Examples: []Example{
				{Value: "priority", Effect: "Requesty (cost x1.05) and the official API (x1.00) at the same priority with weights 3 and 1 take about 75% and 25% of gpt-5.1's requests."},
				{Value: "cost", Effect: "A relay at x0.80 takes gpt-5.1's requests from Requesty at a higher priority while it succeeds 95% of the time; when it drops below, Requesty takes them back."},
			},
			Related:  []string{"SupplyCostMinSavingPercent", "SupplyCostMinSuccessPercent", "channel.fee_rate", "channel.priority"},
			Keywords: []string{"cheapest", "cost routing", "price routing", "lowest cost"},
			Docs:     "use/pricing_and_routing.md#which-supply-takes-a-request",
		},
		{
			Key: "SupplyCostMinSavingPercent", Scope: ScopeSite, Group: "supply", Card: "supply-mode", Type: TypePercent, Unit: "%",
			Min: num(0), Max: num(99),
			Label: "Minimum saving",
			Description: "With cost first, **how much cheaper a supply must be** than another to take requests ahead of it, as a " +
				"percentage of the dearer cost. Supplies closer than that count as the same cost and share requests by priority " +
				"and weight, so a slightly cheaper supply **does not take all of a model's traffic**. 0 puts any supply that is " +
				"cheaper by more than 0.1% ahead.",
			Examples: []Example{
				{Value: "5", Effect: "OpenRouter (fee 5.5%) and Requesty (fee 5%) count as the same cost; a relay at x0.90 is put ahead of both."},
				{Value: "0", Effect: "Requesty (x1.05) is put ahead of OpenRouter (x1.055)."},
			},
			Related:  []string{"SupplyCostOrder", "SupplyCostMinSuccessPercent"},
			Keywords: []string{"price difference", "threshold", "margin", "cost routing"},
			Docs:     "use/pricing_and_routing.md#which-supply-takes-a-request",
		},
		{
			Key: "SupplyCostMinSuccessPercent", Scope: ScopeSite, Group: "supply", Card: "supply-mode", Type: TypePercent, Unit: "%",
			Min: num(0), Max: num(100),
			Label: "Minimum success rate",
			Description: "With cost first, the success rate a supply needs over the current and the previous hour to be put " +
				"ahead for its cost. A cheaper supply below it **is ordered as if its cost were unknown**, behind the supplies that " +
				"serve well, until its rate recovers. A supply with fewer than 20 attempts in that time **is not judged**. 0 turns " +
				"the check off. It is also the uptime over the last day a host of a Vercel AI Gateway or " +
				"OpenRouter model needs to be in the order of hosts the channel asks for, whichever cost order is set.",
			Examples: []Example{
				{Value: "95", Effect: "A discount reseller that failed 12 of its last 100 requests stops taking requests first; the next healthy supply takes them."},
				{Value: "0", Effect: "The cheapest supply always leads, however often it fails; only pauses keep it out."},
			},
			Related:  []string{"SupplyCostOrder", "SupplyCostMinSavingPercent", "PauseRules"},
			Keywords: []string{"reliability", "stability", "error rate", "health", "unstable"},
			Docs:     "use/pricing_and_routing.md#which-supply-takes-a-request",
		},
		{
			Key: "ModelSupplyEnabled", Scope: ScopeSite, Group: "supply", Card: "supply-mode", Type: TypeBool,
			Label: "Route by supplies",
			Description: "Requests are **routed through each public model's supplies** instead of the channels' model lists and " +
				"model mappings. Turning it on first turns the channels' models and model mappings into supplies (only what is " +
				"still missing), so **every model a channel serves keeps working**. While on, requests are billed by the public " +
				"model, only published models are listed, the providers page orders the channels, and the call order, the " +
				"order-by setting, intake rules and pause rules decide where a request goes. New installations start with it on. Turning " +
				"it off goes back to the channels' model lists at once and **keeps the supplies**.",
			Examples: []Example{
				{Value: "false", Effect: "Requests follow each channel's models and model mapping; the supply pages only preview what would change."},
				{Value: "true", Effect: "A request for claude-sonnet-4-5 goes to the supplies of that public model, ordered by priority and cost."},
			},
			Related:  []string{"SupplyCostOrder", "SupplyIntakeRules", "PauseRules"},
			Keywords: []string{"supply routing", "migration", "public model", "model supplies"},
			Advanced: true,
			Docs:     "use/supply_routing.md",
		},

		// ---------------------------------------------------------------- supply-intake-rules
		{
			Key: "SupplyIntakeRules", Scope: ScopeSite, Group: "supply", Card: "supply-intake-rules", Type: TypeJSON,
			Label: "Intake rules for new upstream models",
			Description: "What happens to a model a channel's upstream starts listing, for channels that add new upstream models " +
				"on their own. It is a JSON array tried from the top, and **the first rule whose conditions all hold decides**; a model " +
				"no rule matches **waits for review**. Conditions: endpoints (abilities such as chat, responses, images, any of them), " +
				"vendors (vendor ids), families and patterns (globs such as claude-* or *-preview*), supplied (whether another " +
				"channel already supplies it). action is enable (routes at once), pending (added switched off until an admin " +
				"confirms), disable (added switched off) or skip (not added). placement is auto (last when others already supply " +
				"it), policy (where the allocation policy puts it) or last. Empty uses the built-in rules; **saved rules replace them** " +
				"as a whole, and a channel can use rules of its own.",
			Examples: []Example{
				{
					Value:  `[{"name":"Previews wait","patterns":["*-preview*","*-exp*"],"action":"pending"},{"name":"Chat joins","endpoints":["chat","responses"],"action":"enable","placement":"auto"},{"name":"Rest waits","action":"pending"}]`,
					Effect: "The built-in rules: gpt-5-preview waits for review, a new chat model routes at once (behind the channels that already supply it), an image model waits for review.",
				},
				{
					Value:  `[{"name":"Claude joins","families":["claude-*"],"action":"enable","placement":"last"},{"name":"Rest skipped","action":"skip"}]`,
					Effect: "A new Claude model routes at once after the model's other supplies; any other new model is not added and stays in the list of new upstream models.",
				},
			},
			Related:  []string{"ModelSupplyEnabled", "SupplyCostOrder"},
			Keywords: []string{"new models", "drift", "auto add", "review", "upstream models"},
		},

		// ---------------------------------------------------------------- session-affinity
		{
			Key: "SessionAffinityEnabled", Scope: ScopeSite, Group: "supply", Card: "session-affinity", Type: TypeBool,
			Label: "Keep each session on one channel",
			Description: "Requests of one client session **go to the channel that served it first**, so the upstream prompt cache " +
				"keeps hitting. A session is recognised by the Claude Code session in metadata.user_id, a session " +
				"header such as Codex's session_id, the OpenAI prompt_cache_key, or else the same system prompt and first user " +
				"message. Claude Code, Gemini CLI and Codex account channels are **always sticky**, whatever this says. When off, " +
				"each request is routed on its own.",
			Related:  []string{"SessionAffinityStrict", "SessionAffinityTTLSeconds", "ClaudePromptCachingEnabled"},
			Keywords: []string{"sticky session", "prompt cache", "cache hit", "affinity", "pinning"},
		},
		{
			Key: "SessionAffinityStrict", Scope: ScopeSite, Group: "supply", Card: "session-affinity", Type: TypeBool,
			Label: "Do not move a session on failure",
			Description: "When a session's channel fails, **the caller gets the error instead of a retry on another channel**. The " +
				"channel is not paused for that model either (channel-wide pauses still apply). Moving loses the upstream prompt cache and the " +
				"conversation state Codex keeps upstream. Errors that say the channel can no longer serve (a rejected key, an " +
				"empty balance) **still move the session**. Leave it off to favour availability.",
			Related:  []string{"SessionAffinityEnabled", "RetryTimes"},
			Keywords: []string{"strict", "no failover", "cache", "Codex"},
		},
		{
			Key: "SessionAffinityTTLSeconds", Scope: ScopeSite, Group: "supply", Card: "session-affinity", Type: TypeInt, Unit: "seconds", Min: num(0),
			Label: "Session lifetime",
			Description: "How long a session stays bound to its channel after its last request; every request starts the time " +
				"again. Account channels (Claude Code, Gemini CLI, Codex) **always keep 1 hour**. 0 uses 1 hour as well.",
			Examples: []Example{
				{Value: "3600", Effect: "A coding session paused for 50 minutes comes back to the same channel; after 70 minutes it may land elsewhere."},
				{Value: "300", Effect: "Matches the upstream's 5-minute prompt cache: past that the cache is gone, so the session is free to move."},
			},
			Related:  []string{"SessionAffinityEnabled"},
			Keywords: []string{"TTL", "session timeout", "sticky duration"},
		},

		// ---------------------------------------------------------------- auto-routing
		{
			Key: "RoutingDecisionBillingEnabled", Scope: ScopeSite, Group: "supply", Card: "routing-settings", Page: "routing-policies", Type: TypeBool,
			Label: "Charge the decision call",
			Description: "When on, **the caller pays for the decision call**, the call a routing policy makes to pick the target " +
				"model. Fee = usage × **the catalog price of the policy's decision model** × the caller's group ratio, " +
				"**charged once per session** as a consume log of its own; there is no fee to set. When off, the decision " +
				"still runs and **the platform pays for it**, with no consume log.",
			Examples: []Example{
				{Value: "true", Effect: "A policy whose decision model is gpt-5-mini: the caller pays gpt-5-mini's price for the decision, once per session, besides the request itself."},
				{Value: "false", Effect: "The same decision costs the caller nothing; the platform pays the upstream for it."},
			},
			Related:  []string{"UserRoutingPolicyEnabled"},
			Keywords: []string{"auto routing", "decision model", "classifier", "billing", "policy"},
		},
		{
			Key: "UserRoutingPolicyEnabled", Scope: ScopeSite, Group: "supply", Card: "routing-settings", Page: "routing-policies", Type: TypeBool,
			Label: "Own routing policies",
			Description: "Users and organizations can **create routing policies of their own**, called as policy/<name>. Users create " +
				"personal policies under Routing policies in the workbench, organization owners and admins organization policies; " +
				"an own policy **overrides a platform policy of the same name** for its own keys. When off, **own policies are kept but no longer apply**, and only platform " +
				"policies resolve.",
			Related:  []string{"PersonalRoutingPolicyMaxCount", "OrganizationRoutingPolicyMaxCount"},
			Keywords: []string{"routing policy", "policy/", "self-service", "organization policy"},
		},
		{
			Key: "PersonalRoutingPolicyMaxCount", Scope: ScopeSite, Group: "supply", Card: "routing-settings", Page: "routing-policies", Type: TypeInt, Min: num(0),
			Label: "Policies per user",
			Description: "How many personal routing policies one user can create. Policies a user already has are kept when the " +
				"cap is lowered; **only new ones are refused**. 0 sets no limit.",
			Examples: []Example{
				{Value: "20", Effect: "A user with 20 policies gets an error when creating the 21st."},
				{Value: "0", Effect: "Users can create as many policies as they like."},
			},
			Related:  []string{"UserRoutingPolicyEnabled", "OrganizationRoutingPolicyMaxCount"},
			Keywords: []string{"limit", "quota", "max policies"},
		},
		{
			Key: "OrganizationRoutingPolicyMaxCount", Scope: ScopeSite, Group: "supply", Card: "routing-settings", Page: "routing-policies", Type: TypeInt, Min: num(0),
			Label: "Policies per organization",
			Description: "How many routing policies one organization can create. Policies it already has are kept when the cap " +
				"is lowered; **only new ones are refused**. 0 sets no limit.",
			Examples: []Example{
				{Value: "20", Effect: "An organization with 20 policies gets an error when creating the 21st."},
				{Value: "5", Effect: "Keeps organization policies few enough to review by hand."},
			},
			Related:  []string{"UserRoutingPolicyEnabled", "PersonalRoutingPolicyMaxCount"},
			Keywords: []string{"limit", "organization", "max policies"},
		},
	}
}
