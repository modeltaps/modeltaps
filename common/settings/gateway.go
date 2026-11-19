package settings

// Settings page group "gateway", in the order of its tabs: resilience (gateway_resilience.go),
// allocation (here) and request (gateway_request.go).
func init() {
	add(gatewayResilience()...)
	add(gatewayAllocation()...)
	add(gatewayRequest()...)
}

// Tab "allocation": which supply serves a request.
func gatewayAllocation() []Setting {
	return []Setting{
		// ---------------------------------------------------------------- supply-mode
		{
			Key: "SupplyModeDefault", Scope: ScopeSite, Group: "gateway", Card: "supply-mode", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "balance", Label: "Balance", Description: "Supplies of one tier share requests by weight."},
				{Value: "failover", Label: "Failover", Description: "The heaviest supply of a tier takes every request; the others back it up."},
			},
			Label: "Default supply mode",
			Description: "How the supplies that take a model's requests share them, for models that follow the default. " +
				"Only supplies that tie on priority (and on cost, see Cost and priority) share; a model can choose its own " +
				"mode on its Supplies tab.",
			Examples: []Example{
				{Value: "balance", Effect: "Two supplies with weights 3 and 1 take about 75% and 25% of the requests."},
				{Value: "failover", Effect: "The supply with weight 3 takes every request; the one with weight 1 is used only while it fails."},
			},
			Related:  []string{"SupplyCostOrder", "model.supply_mode"},
			Keywords: []string{"load balancing", "weight", "fallback", "primary", "backup"},
			Docs:     "use/pricing_and_routing.md#which-supply-takes-a-request",
		},
		{
			Key: "SupplyCostOrder", Scope: ScopeSite, Group: "gateway", Card: "supply-mode", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "priority", Label: "Priority first", Description: "Cost only orders supplies of the same priority."},
				{Value: "cost", Label: "Cost first", Description: "The cheapest supplies of a model take the requests whatever their priority."},
			},
			Label: "Cost and priority",
			Description: "How far a supply's cost (its channel's list price with the channel's fee) moves it ahead. Users pay the " +
				"catalog price times their group ratio either way; only your cost changes. Costs within 0.1% count as the same, " +
				"and supplies of unknown cost come last.",
			Examples: []Example{
				{Value: "priority", Effect: "Requesty (cost x1.05) at the top priority keeps the requests of gpt-5.1; the official API (x1.00) one priority lower is its backup."},
				{Value: "cost", Effect: "The official API (x1.00) takes the requests of gpt-5.1; Requesty becomes a backup marked dearer."},
			},
			Related:  []string{"SupplyModeDefault", "channel.fee_rate", "channel.priority"},
			Keywords: []string{"cheapest", "cost routing", "price routing", "lowest cost"},
			Docs:     "use/pricing_and_routing.md#which-supply-takes-a-request",
		},
		{
			Key: "ModelSupplyEnabled", Scope: ScopeSite, Group: "gateway", Card: "supply-mode", Type: TypeBool,
			Label: "Route by supplies",
			Description: "Requests are routed through each public model's supplies instead of the channels' model lists and " +
				"model mappings. Turning it on first turns the channels' models and model mappings into supplies (only what is " +
				"still missing), so every model a channel serves keeps working. While on, requests are billed by the public " +
				"model, only published models are listed, the providers page orders the channels, and the supply mode, cost " +
				"order, intake rules and pause rules decide where a request goes. New installations start with it on. Turning " +
				"it off goes back to the channels' model lists at once and keeps the supplies.",
			Examples: []Example{
				{Value: "false", Effect: "Requests follow each channel's models and model mapping; the supply pages only preview what would change."},
				{Value: "true", Effect: "A request for claude-sonnet-4-5 goes to the supplies of that public model, ordered by priority and cost."},
			},
			Related:  []string{"SupplyModeDefault", "SupplyCostOrder", "SupplyIntakeRules", "PauseRulesEnabled"},
			Keywords: []string{"supply routing", "migration", "public model", "model supplies"},
			Advanced: true,
			Docs:     "use/supply_routing.md",
		},

		// ---------------------------------------------------------------- supply-intake-rules
		{
			Key: "SupplyIntakeRules", Scope: ScopeSite, Group: "gateway", Card: "supply-intake-rules", Type: TypeJSON,
			Label: "Intake rules for new upstream models",
			Description: "What happens to a model a channel's upstream starts listing, for channels that add new upstream models " +
				"on their own. It is a JSON array tried from the top, and the first rule whose conditions all hold decides; a model " +
				"no rule matches waits for review. Conditions: endpoints (abilities such as chat, responses, images, any of them), " +
				"vendors (vendor ids), families and patterns (globs such as claude-* or *-preview*), supplied (whether another " +
				"channel already supplies it). action is enable (routes at once), pending (added switched off until an admin " +
				"confirms), disable (added switched off) or skip (not added). placement is auto (last when others already supply " +
				"it), policy (where the allocation policy puts it) or last. Empty uses the built-in rules; saved rules replace them " +
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
			Related:  []string{"ModelSupplyEnabled", "SupplyModeDefault"},
			Keywords: []string{"new models", "drift", "auto add", "review", "upstream models"},
		},

		// ---------------------------------------------------------------- session-affinity
		{
			Key: "SessionAffinityEnabled", Scope: ScopeSite, Group: "gateway", Card: "session-affinity", Type: TypeBool,
			Label: "Keep each session on one channel",
			Description: "Requests of one client session go to the channel that served it first, so the upstream prompt cache " +
				"keeps hitting. A session is recognised by the Claude Code session in metadata.user_id, a session " +
				"header such as Codex's session_id, the OpenAI prompt_cache_key, or else the same system prompt and first user " +
				"message. Claude Code, Gemini CLI and Codex account channels are always sticky, whatever this says. When off, " +
				"each request is routed on its own.",
			Related:  []string{"SessionAffinityStrict", "SessionAffinityTTLSeconds", "ClaudePromptCachingEnabled"},
			Keywords: []string{"sticky session", "prompt cache", "cache hit", "affinity", "pinning"},
		},
		{
			Key: "SessionAffinityStrict", Scope: ScopeSite, Group: "gateway", Card: "session-affinity", Type: TypeBool,
			Label: "Do not move a session on failure",
			Description: "When a session's channel fails, the caller gets the error instead of a retry on another channel. The " +
				"channel is not paused for that model either (channel-wide pauses still apply). Moving loses the upstream prompt cache and the " +
				"conversation state Codex keeps upstream. Errors that say the channel can no longer serve (a rejected key, an " +
				"empty balance) still move the session. Leave it off to favour availability.",
			Related:  []string{"SessionAffinityEnabled", "RetryTimes"},
			Keywords: []string{"strict", "no failover", "cache", "Codex"},
		},
		{
			Key: "SessionAffinityTTLSeconds", Scope: ScopeSite, Group: "gateway", Card: "session-affinity", Type: TypeInt, Unit: "seconds", Min: num(0),
			Label: "Session lifetime",
			Description: "How long a session stays bound to its channel after its last request; every request starts the time " +
				"again. Account channels (Claude Code, Gemini CLI, Codex) always keep 1 hour. 0 uses 1 hour as well.",
			Examples: []Example{
				{Value: "3600", Effect: "A coding session paused for 50 minutes comes back to the same channel; after 70 minutes it may land elsewhere."},
				{Value: "300", Effect: "Matches the upstream's 5-minute prompt cache: past that the cache is gone, so the session is free to move."},
			},
			Related:  []string{"SessionAffinityEnabled"},
			Keywords: []string{"TTL", "session timeout", "sticky duration"},
		},

		// ---------------------------------------------------------------- auto-routing
		{
			Key: "RoutingDecisionBillingEnabled", Scope: ScopeSite, Group: "gateway", Card: "auto-routing", Type: TypeBool,
			Label: "Charge the decision call",
			Description: "Bills the call an auto routing policy makes to pick a model to the caller, at the decision model's price. " +
				"It is charged once per session, as a log entry of its own. When off, the decision still runs, but the " +
				"site pays for it and no consume log is written.",
			Related:  []string{"UserRoutingPolicyEnabled"},
			Keywords: []string{"auto routing", "decision model", "classifier", "billing", "policy"},
		},
		{
			Key: "UserRoutingPolicyEnabled", Scope: ScopeSite, Group: "gateway", Card: "auto-routing", Type: TypeBool,
			Label: "Own routing policies",
			Description: "Users and organizations can create routing policies of their own, called as policy/<name>. Users create " +
				"personal policies under Routing policies in the workbench, organization owners and admins organization policies; " +
				"an own policy overrides a platform policy of the same name for its own keys. When off, own policies are kept but no longer apply, and only platform " +
				"policies resolve.",
			Related:  []string{"PersonalRoutingPolicyMaxCount", "OrganizationRoutingPolicyMaxCount"},
			Keywords: []string{"routing policy", "policy/", "self-service", "organization policy"},
		},
		{
			Key: "PersonalRoutingPolicyMaxCount", Scope: ScopeSite, Group: "gateway", Card: "auto-routing", Type: TypeInt, Min: num(0),
			Label: "Policies per user",
			Description: "How many personal routing policies one user can create. Policies a user already has are kept when the " +
				"cap is lowered; only new ones are refused. 0 sets no limit.",
			Examples: []Example{
				{Value: "20", Effect: "A user with 20 policies gets an error when creating the 21st."},
				{Value: "0", Effect: "Users can create as many policies as they like."},
			},
			Related:  []string{"UserRoutingPolicyEnabled", "OrganizationRoutingPolicyMaxCount"},
			Keywords: []string{"limit", "quota", "max policies"},
		},
		{
			Key: "OrganizationRoutingPolicyMaxCount", Scope: ScopeSite, Group: "gateway", Card: "auto-routing", Type: TypeInt, Min: num(0),
			Label: "Policies per organization",
			Description: "How many routing policies one organization can create. Policies it already has are kept when the cap " +
				"is lowered; only new ones are refused. 0 sets no limit.",
			Examples: []Example{
				{Value: "20", Effect: "An organization with 20 policies gets an error when creating the 21st."},
				{Value: "5", Effect: "Keeps organization policies few enough to review by hand."},
			},
			Related:  []string{"UserRoutingPolicyEnabled", "PersonalRoutingPolicyMaxCount"},
			Keywords: []string{"limit", "organization", "max policies"},
		},
	}
}
