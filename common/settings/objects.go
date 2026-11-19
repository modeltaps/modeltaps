package settings

// Object fields: what operators set on channels, models, supplies, user groups and the other
// objects, rather than on the settings page.
func init() {
	add(channelFields()...)
	add(modelFields()...)
	add(userGroupFields()...)
	add(routingPolicyFields()...)
	add(organizationFields()...)
	add(tokenFields()...)
}

// Fields of a channel, on its panel in the Providers page.
func channelFields() []Setting {
	return []Setting{
		{
			Key: "channel.priority", Scope: ScopeChannel, Group: "channels", Card: "routing", Field: "priority",
			Type: TypeInt, Default: "0",
			Label: "Priority",
			Description: "The order channels are tried in for a model: the highest priority takes requests first. " +
				"A lower priority is used only while every channel above it is unavailable; channels of one priority " +
				"share requests by weight. The Usage choices (only provider, backup, share, prefer) set it for you. " +
				"While supplies route requests, the call order on the Providers page orders channels instead, and new " +
				"supplies of the channel start from its priority.",
			Examples: []Example{
				{Value: "10", Effect: "Tried before every channel at 0 that serves the same model in the same group."},
				{Value: "0", Effect: "The default. A backup at -1 is reached only while every channel at 0 fails."},
			},
			Related:  []string{"channel.weight", "supply.priority", "SupplyCostOrder"},
			Keywords: []string{"order", "fallback", "backup", "tier", "primary"},
		},
		{
			Key: "channel.weight", Scope: ScopeChannel, Group: "channels", Card: "routing", Field: "weight",
			Type: TypeInt, Min: num(0), Default: "1",
			Label: "Weight",
			Description: "How channels of the same priority share a model's requests: each takes its weight over the sum of " +
				"the weights. Channels of other priorities are not counted. While supplies route requests, the supplies' " +
				"own weights decide, and new supplies of the channel start from its weight.",
			Examples: []Example{
				{Value: "3", Effect: "Next to a channel of weight 1 at the same priority, takes about 3 / (3 + 1) = 75% of the requests."},
				{Value: "1", Effect: "The default: channels of one priority share requests equally."},
			},
			Related:  []string{"channel.priority", "supply.weight", "SupplyModeDefault"},
			Keywords: []string{"load balancing", "share", "split", "traffic"},
		},
		{
			Key: "channel.cost_ratio", Scope: ScopeChannel, Group: "channels", Card: "advanced", Field: "cost_ratio",
			Type: TypeRatio, Min: num(0), Default: "empty (unknown)",
			Label: "Cost ratio",
			Description: "What the channel's upstream charges as a multiple of the catalog price, for channels without a price list. " +
				"It is used only where the channel publishes no synced price: the channel cost is then catalog price x cost ratio " +
				"x (1 + fee rate). It feeds the profit figures, the supply cost and the warning on user groups sold below it; " +
				"users are never charged by it. Empty or 0 means unknown.",
			Examples: []Example{
				{Value: "0.8", Effect: "A relay at 80% of the official price: a $3 / $15 per million token model costs $2.40 / $12, or $2.52 / $12.60 with a 5% fee."},
				{Value: "1", Effect: "The upstream charges the official price; any group below ratio 1 is flagged as selling at a loss here."},
			},
			Related:  []string{"channel.fee_rate", "user_group.ratio", "model.price"},
			Keywords: []string{"cost", "discount", "relay price", "upstream price", "margin"},
			Docs:     "use/pricing_and_routing.md#where-to-set-what",
		},
		{
			Key: "channel.fee_rate", Scope: ScopeChannel, Group: "channels", Card: "advanced", Field: "fee_rate",
			Type: TypePercent, Unit: "%", Min: num(0), Default: "the provider's published fee",
			Label: "Fee rate",
			Description: "What the channel's provider charges on top of its list prices, usually when you buy credit: " +
				"cash paid / credit received - 1. A channel's cost is its list price x (1 + fee rate); costs decide which " +
				"supply takes a request and which ones the price check holds. Recorded top-ups set it from the latest 10.",
			Examples: []Example{
				{Value: "5", Effect: "Requesty: $105 buys $100 of credit."},
				{Value: "3", Effect: "Vercel AI Gateway card payments: a $3.00 processing fee on $100 of credit."},
				{Value: "0", Effect: "An official vendor API billed at list price."},
			},
			Related:  []string{"PaymentCostPercent", "SupplyCostOrder", "user_group.ratio"},
			Keywords: []string{"top-up fee", "processing fee", "markup", "channel cost"},
			Docs:     "use/pricing_and_routing.md#channel-fee-rates",
		},
		{
			Key: "channel.fee_note", Scope: ScopeChannel, Group: "channels", Card: "advanced", Field: "fee_note",
			Type:  TypeString,
			Label: "Fee source",
			Description: "Where the channel's fee rate comes from, such as a billing page or a checkout receipt. It is required " +
				"once the fee rate is above 0. Only admins see it; users see a generic label.",
			Related:  []string{"channel.fee_rate"},
			Keywords: []string{"fee note", "receipt", "source"},
		},
		{
			Key: "channel.group", Scope: ScopeChannel, Group: "channels", Card: "routing", Field: "group",
			Type: TypeList, Default: "default",
			Label: "Groups",
			Description: "The user groups the channel serves: a request is only sent to channels of the group it runs in. " +
				"A request runs in its API key's group (then the key's backup group), or the user's group when the key has none. " +
				"A channel needs at least one group.",
			Examples: []Example{
				{Value: "default", Effect: "Serves every user of the default group and keys set to it."},
				{Value: "default,vip", Effect: "Serves both groups; each request is billed at the ratio of the group it ran in."},
			},
			Related:  []string{"user_group.ratio", "token.group", "token.backup_group"},
			Keywords: []string{"user group", "channel group", "audience"},
		},
		{
			Key: "channel.model_mapping", Scope: ScopeChannel, Group: "channels", Card: "advanced", Field: "model_mapping",
			Type:  TypeJSON,
			Label: "Model mapping",
			Description: "Renames on this channel only: a requested model name is sent upstream as another name. Use it only when " +
				"the upstream wants its own names, such as deployment names; matching models to the catalog needs nothing here. " +
				"A target starting with + bills by the requested name. Requests routed through a supply use the supply's " +
				"upstream model instead. Saving a change rewrites the channel's model list to the requested names.",
			Examples: []Example{
				{Value: `{"gpt-4o": "my-gpt4o-deployment"}`, Effect: "Requests for gpt-4o reach the upstream as my-gpt4o-deployment."},
				{Value: `{"opus-latest": "+claude-opus-4-1"}`, Effect: "opus-latest is sent as claude-opus-4-1 and billed as opus-latest."},
			},
			Related:  []string{"supply.upstream_model"},
			Keywords: []string{"rename", "alias", "deployment name", "redirect"},
			Advanced: true,
		},
		{
			Key: "channel.pre_cost", Scope: ScopeChannel, Group: "channels", Card: "advanced", Field: "pre_cost",
			Type: TypeEnum, Default: "1",
			Enum: []EnumValue{
				{Value: "1", Label: "Normal billing", Description: "Count the input tokens, images included, before the request is sent."},
				{Value: "2", Label: "Exclude images", Description: "Count the text only; images are left out of the estimate."},
				{Value: "3", Label: "Exclude all", Description: "Count nothing beforehand; the reply's usage decides the charge."},
			},
			Label: "Pre-billing",
			Description: "How the gateway counts a request's input before sending it, to hold the user's credit until the reply's " +
				"usage is known. The count also stands in for the input when an upstream reports no usage, such as some streams. " +
				"Leaving images or everything out saves server work on heavy requests; the prompt token limit is still checked.",
			Related:  []string{"MaxPromptTokens"},
			Keywords: []string{"pre-consume", "token count", "estimate", "hold"},
			Advanced: true,
		},
		{
			Key: "channel.disabled_stream", Scope: ScopeChannel, Group: "channels", Card: "advanced", Field: "disabled_stream",
			Type:  TypeList,
			Label: "Models with streaming disabled",
			Description: "Models this channel must not stream: streaming requests for them skip it for another channel serving the model. " +
				"Requests without streaming still use it. Empty means every model streams.",
			Examples: []Example{
				{Value: "o1,o1-pro", Effect: "Streaming requests for o1 and o1-pro go to other channels; plain requests still come here."},
			},
			Keywords: []string{"stream", "no streaming", "sse"},
			Advanced: true,
		},
		{
			Key: "channel.prompt_caching", Scope: ScopeChannel, Group: "channels", Card: "advanced", Field: "plugin.prompt_caching.enabled",
			Type: TypeEnum, Default: "inherit",
			Enum: []EnumValue{
				{Value: "inherit", Label: "Follow the site", Description: "Inject when the site-wide Claude prompt caching switch is on."},
				{Value: "on", Label: "Always on", Description: "Inject on this channel whatever the site switch says."},
				{Value: "off", Label: "Always off", Description: "Never inject on this channel."},
			},
			Label: "Prompt caching injection",
			Description: "Whether the gateway adds cache_control marks to Claude requests on this channel, so the upstream caches the prompt. " +
				"Shown for Anthropic, Claude Code and OpenRouter channels, in the plugin settings. Requests that already carry " +
				"cache_control are left alone. The plugin also sets the cache lifetime (5 minutes or 1 hour), where the marks go " +
				"and which models match (Claude names by default).",
			Related:  []string{"ClaudePromptCachingEnabled"},
			Keywords: []string{"cache_control", "prompt cache", "anthropic", "claude", "cache"},
			Advanced: true,
		},
		{
			Key: "channel.proxy", Scope: ScopeChannel, Group: "channels", Card: "connection", Field: "proxy",
			Type:  TypeURL,
			Label: "Proxy",
			Description: "A proxy for this channel's upstream requests only, http or socks5. Only needed when this server cannot " +
				"reach the upstream directly. A %s in the address is replaced with a session ID derived from the key, for " +
				"proxies that pin one exit per session. Empty connects directly.",
			Examples: []Example{
				{Value: "http://127.0.0.1:1080", Effect: "Every request of the channel goes through the local HTTP proxy."},
				{Value: "socks5://user-%s:pass@proxy.example.com:1080", Effect: "Each key gets its own proxy session, so its exit address stays the same."},
			},
			Keywords: []string{"http proxy", "socks5", "network", "egress"},
		},
		{
			Key: "channel.test_model", Scope: ScopeChannel, Group: "channels", Card: "monitor", Field: "test_model",
			Type:  TypeString,
			Label: "Test model",
			Description: "The model connection tests and scheduled checks send their request with. Empty picks a cheap chat model " +
				"from the channel's list, else its first chat model; a channel without a chat model then cannot be tested.",
			Examples: []Example{
				{Value: "gpt-4o-mini", Effect: "Tests cost a fraction of a cent and do not touch the expensive models."},
			},
			Keywords: []string{"health check", "connection test", "probe"},
		},
	}
}

// Fields of a public model, its supplies and the allocation policy.
func modelFields() []Setting {
	return []Setting{
		{
			Key: "model.price", Scope: ScopeModel, Group: "models", Card: "pricing", Field: "price",
			Type: TypeFloat, Unit: "USD / 1M tokens", Min: num(0),
			Label: "Catalog price",
			Description: "What the model costs before the group ratio, per million input and output tokens (or per request or minute). " +
				"Users pay catalog price x their group ratio, whichever channel serves the " +
				"request. It can be typed in USD, RMB or as a ratio (1 = $2 per million tokens). A model without a price is " +
				"refused by default.",
			Examples: []Example{
				{Value: "3 / 15", Effect: "$3 input and $15 output per million tokens; the default group at 1.12 pays $3.36 / $16.80."},
				{Value: "ratio 1.5 / 7.5", Effect: "The same price as ratios: 1.5 x $2 = $3 and 7.5 x $2 = $15 per million tokens."},
			},
			Related:  []string{"user_group.ratio", "channel.cost_ratio", "PaymentCostPercent"},
			Keywords: []string{"pricing", "model price", "input price", "output price", "official price"},
			Docs:     "use/pricing_and_routing.md#the-three-numbers",
		},
		{
			Key: "model.supply_mode", Scope: ScopeModel, Group: "models", Card: "supplies", Field: "supply_mode",
			Type: TypeEnum, Default: "default",
			Enum: []EnumValue{
				{Value: "default", Label: "Follow default", Description: "Use the default supply mode of the platform settings."},
				{Value: "balance", Label: "Balance", Description: "Supplies of one tier share requests by weight."},
				{Value: "failover", Label: "Failover", Description: "The heaviest supply of a tier takes every request; the others back it up."},
			},
			Label: "Supply mode",
			Description: "How the supplies of one tier share this model's requests. Tiers are always tried in order; the mode only " +
				"decides within one. It applies to models whose order is set by hand: a model that follows the call order " +
				"always shares a tier by weight.",
			Examples: []Example{
				{Value: "failover", Effect: "Supplies of weights 3 and 1 in one tier: the weight 3 supply takes every request while it works."},
				{Value: "balance", Effect: "The same supplies take about 75% and 25% of the requests."},
			},
			Related:  []string{"SupplyModeDefault", "model.allocation", "supply.weight"},
			Keywords: []string{"load balancing", "failover", "primary", "backup"},
			Docs:     "use/pricing_and_routing.md#which-supply-takes-a-request",
		},
		{
			Key: "model.allocation", Scope: ScopeModel, Group: "models", Card: "supplies", Field: "allocation",
			Type: TypeEnum, Default: "policy",
			Enum: []EnumValue{
				{Value: "policy", Label: "Call order", Description: "The supplies' priority and weight come from the call order on the Providers page."},
				{Value: "manual", Label: "Set by hand", Description: "The supplies keep the priority and weight stored on them."},
			},
			Label: "Supply order",
			Description: "Whether the model's supplies follow the call order of the Providers page or keep their own numbers. " +
				"Changing a supply's priority or weight on the Supplies tab sets the model to set by hand; switching back " +
				"to the call order drops those numbers.",
			Examples: []Example{
				{Value: "policy", Effect: "Moving a provider up on the Providers page moves it up for this model too."},
				{Value: "manual", Effect: "This model keeps its order when the call order changes; new channels join below its supplies."},
			},
			Related:  []string{"allocation.tiers", "supply.priority", "model.supply_mode"},
			Keywords: []string{"allocation policy", "call order", "manual order"},
		},
		{
			Key: "supply.priority", Scope: ScopeSupply, Group: "models", Card: "supplies", Field: "priority",
			Type:    TypeInt,
			Default: "the channel's priority; a supply added by hand joins below the lowest tier",
			Label:   "Priority",
			Description: "This supply's tier among the model's supplies: higher tiers take requests first, lower ones back them up. " +
				"A lower tier is used only while every supply above it is unavailable. Supplies of one priority form a tier and share it by the supply mode. " +
				"Under the call order the policy sets it; a supply with its own number no longer follows its channel.",
			Examples: []Example{
				{Value: "2", Effect: "Above two supplies at 1: they are used only while this supply fails."},
				{Value: "1", Effect: "Next to another supply at 1, the two form one tier and share its requests."},
			},
			Related:  []string{"supply.weight", "model.allocation", "SupplyCostOrder"},
			Keywords: []string{"tier", "order", "backup", "fallback"},
			Docs:     "use/pricing_and_routing.md#which-supply-takes-a-request",
		},
		{
			Key: "supply.weight", Scope: ScopeSupply, Group: "models", Card: "supplies", Field: "weight",
			Type: TypeInt, Min: num(0), Default: "the channel's weight",
			Label: "Weight",
			Description: "This supply's share of its tier: its weight over the tier's total under balance, all of it if heaviest under failover. " +
				"0 counts as 1. Under the call order a provider's weight is split equally " +
				"between its channels.",
			Examples: []Example{
				{Value: "3", Effect: "With a weight 1 supply in its tier: about 75% of the requests under balance, all of them under failover."},
				{Value: "1", Effect: "Two supplies of weight 1 share a tier 50 / 50 under balance."},
			},
			Related:  []string{"supply.priority", "model.supply_mode", "SupplyModeDefault"},
			Keywords: []string{"share", "split", "load balancing", "traffic"},
		},
		{
			Key: "supply.enabled", Scope: ScopeSupply, Group: "models", Card: "supplies", Field: "enabled",
			Type: TypeBool, Default: "on; off for a supply an intake rule adds for review",
			Label: "Use this supply",
			Description: "Whether the supply takes the model's requests. Off, it keeps its settings but is skipped. Supplies an intake " +
				"rule added wait switched off until you switch them on; switching a supply on also accepts its cost, so the " +
				"price check leaves it alone.",
			Related:  []string{"SupplyIntakeRules"},
			Keywords: []string{"enable", "disable", "review", "pending"},
		},
		{
			Key: "supply.upstream_model", Scope: ScopeSupply, Group: "models", Card: "supplies", Field: "upstream_model",
			Type: TypeString, Default: "filled in from the channel's mapping, its model list or its platform's naming",
			Label: "Upstream model",
			Description: "The model name this supply sends to its channel's upstream. The client still asks for the public name and is " +
				"billed by it; the channel's model mapping does not apply. A supply stops taking requests when the upstream stops " +
				"listing this name in two checks in a row.",
			Examples: []Example{
				{Value: "claude-sonnet-4-5@20250929", Effect: "A Vertex channel serves the public claude-sonnet-4-5 under its own versioned name."},
				{Value: "anthropic/claude-sonnet-4.5", Effect: "An aggregator that prefixes the vendor serves the same public model."},
			},
			Related:  []string{"channel.model_mapping"},
			Keywords: []string{"upstream name", "model id", "deployment name"},
		},
		{
			Key: "allocation.tiers", Scope: ScopeAllocation, Group: "allocation", Card: "call-order", Field: "tiers",
			Type: TypeJSON, Default: "created from the channels' priorities when supply routing starts",
			Label: "Call order",
			Description: "The order channels take requests in while supplies route them, edited as the list on the Providers page. " +
				"Each position is a provider (every channel of one type and address) or a single channel; positions shared by " +
				"several split their requests by weight, and a provider's weight is split equally between its channels. Channels " +
				"not placed come last, newest last. Models set by hand keep their own order.",
			Examples: []Example{
				{Value: `[[{"provider":"14:","weight":1}],[{"channel_id":7,"weight":1}]]`, Effect: "The official Anthropic API first; channel 7 only while it fails."},
				{Value: `[[{"channel_id":3,"weight":3},{"channel_id":5,"weight":1}]]`, Effect: "Channels 3 and 5 share the first position: about 75% and 25% of the requests."},
			},
			Related:  []string{"model.allocation", "supply.priority", "SupplyCostOrder"},
			Keywords: []string{"allocation policy", "call order", "provider order", "tiers"},
			Docs:     "use/pricing_and_routing.md#which-supply-takes-a-request",
		},
	}
}

// Fields of a user group, on its panel in the User groups page.
func userGroupFields() []Setting {
	return []Setting{
		{
			Key: "user_group.ratio", Scope: ScopeUserGroup, Group: "user-groups", Card: "edit", Field: "ratio",
			Type: TypeRatio, Min: num(0), Default: "1",
			Label: "Group ratio",
			Description: "The multiplier on the catalog (official) price for users of the group: every request is charged " +
				"catalog price x ratio, whatever channel serves it. The default group's ratio is the site's normal price and " +
				"margin; other groups give discounts or charge more. 0 makes the group's requests free.",
			Examples: []Example{
				{Value: "1.12", Effect: "The default group sells at 12% over the official price: a $3 / $15 per million token model costs $3.36 / $16.80."},
				{Value: "1.05", Effect: "A VIP discount. With a 4% payment cost it brings in $1.008 per $1, below a 3% channel fee."},
				{Value: "1", Effect: "Sell at the official price: every channel fee and payment cost is a loss."},
			},
			Related:  []string{"PaymentCostPercent", "channel.fee_rate"},
			Keywords: []string{"markup", "margin", "discount", "price multiplier", "profit"},
			Docs:     "use/pricing_and_routing.md#examples",
		},
		{
			Key: "user_group.api_rate", Scope: ScopeUserGroup, Group: "user-groups", Card: "edit", Field: "api_rate",
			Type: TypeInt, Unit: "requests / minute", Min: num(0), Default: "300",
			Label: "API rate",
			Description: "How many API requests each user of the group may send per minute, counted per user across all their keys. " +
				"Requests over it get 429. Below 60 a fixed one-minute window counts them; from 60 a token bucket allows short bursts " +
				"of up to 10 seconds' worth.",
			Examples: []Example{
				{Value: "300", Effect: "Five requests a second on average per user."},
				{Value: "30", Effect: "The 31st request within a minute is refused until the minute is over."},
			},
			Keywords: []string{"rate limit", "rpm", "throttle", "429"},
		},
		{
			Key: "user_group.public", Scope: ScopeUserGroup, Group: "user-groups", Card: "edit", Field: "public",
			Type: TypeBool, Default: "off",
			Label: "Public",
			Description: "Whether users outside the group may pick it for their API keys, as a key's group or backup group. Off, " +
				"only users in the group use it. Requests through a picked group are billed at its ratio.",
			Related:  []string{"token.group", "token.backup_group", "user_group.ratio"},
			Keywords: []string{"selectable", "visible", "open group"},
		},
		{
			Key: "user_group.promotion", Scope: ScopeUserGroup, Group: "user-groups", Card: "edit", Field: "promotion",
			Type: TypeBool, Default: "off",
			Label: "Auto upgrade",
			Description: "Moves users into this group when their total paid top-ups fall between its minimum and maximum. It is " +
				"checked after each paid order; gifted credit (sign-up and invite bonuses, redemption codes, admin changes) does " +
				"not count. When several groups match, the one with the highest minimum wins. A user an admin put in another " +
				"group is moved too.",
			Related:  []string{"user_group.min", "user_group.max"},
			Keywords: []string{"promotion", "tier", "loyalty", "upgrade"},
		},
		{
			Key: "user_group.min", Scope: ScopeUserGroup, Group: "user-groups", Card: "edit", Field: "min",
			Type: TypeFloat, Unit: "$", Min: num(0), Default: "0",
			Label: "Minimum paid top-ups",
			Description: "The total paid top-ups from which auto upgrade moves a user into the group (inclusive). Only used with " +
				"auto upgrade on.",
			Examples: []Example{
				{Value: "100", Effect: "With a maximum of 500: a user who has paid $100 up to $499.99 in total joins the group."},
				{Value: "0", Effect: "Every paying user matches, up to the maximum."},
			},
			Related:  []string{"user_group.promotion", "user_group.max"},
			Keywords: []string{"threshold", "spend", "promotion"},
		},
		{
			Key: "user_group.max", Scope: ScopeUserGroup, Group: "user-groups", Card: "edit", Field: "max",
			Type: TypeFloat, Unit: "$", Min: num(0), Default: "0",
			Label: "Maximum paid top-ups",
			Description: "The total paid top-ups at which the group stops matching (exclusive), so a bigger spender moves on to the " +
				"next group. 0 means no upper bound. Only used with auto upgrade on.",
			Examples: []Example{
				{Value: "500", Effect: "A user reaching $500 in total moves to the group whose range starts at 500."},
				{Value: "0", Effect: "The top level: users stay however much they pay."},
			},
			Related:  []string{"user_group.promotion", "user_group.min"},
			Keywords: []string{"threshold", "spend", "promotion"},
		},
	}
}

// Fields of an auto-routing policy, on its panel in the Routing policies page.
func routingPolicyFields() []Setting {
	return []Setting{
		{
			Key: "routing_policy.decision_model", Scope: ScopeRoutingPolicy, Group: "routing-policies", Card: "routes", Field: "decision_model",
			Type:  TypeString,
			Label: "Decision model",
			Description: "The model that reads a session's first user message and picks one of the routes. A decision model is " +
				"best: it answers with a probability for every label. A small, fast chat model also works. It is charged to the " +
				"caller once per session, and it cannot be another routing policy.",
			Examples: []Example{
				{Value: "gpt-4.1-mini", Effect: "A cheap chat model that returns logprobs, so the threshold is a real probability."},
			},
			Related:  []string{"routing_policy.scoring", "routing_policy.threshold", "routing_policy.fallback_model"},
			Keywords: []string{"classifier", "router model", "auto routing"},
		},
		{
			Key: "routing_policy.scoring", Scope: ScopeRoutingPolicy, Group: "routing-policies", Card: "routes", Field: "scoring",
			Type: TypeEnum, Default: "auto",
			Enum: []EnumValue{
				{Value: "auto", Label: "Auto", Description: "Logprobs where the channel returns them, else self-reported confidence for 24 hours."},
				{Value: "logprobs", Label: "Logprobs only", Description: "The label's real probability; requests fall back when logprobs are missing."},
				{Value: "self_reported", Label: "Self-reported only", Description: "The model answers its label and a confidence in JSON."},
			},
			Label: "Scoring",
			Description: "How the decision model's confidence is measured before it is compared with the threshold. Logprobs need a " +
				"non-reasoning model on a channel that returns them; Claude, reasoning models and most aggregators do not. " +
				"A decision model ignores this setting.",
			Related:  []string{"routing_policy.threshold", "routing_policy.decision_model"},
			Keywords: []string{"logprobs", "confidence", "probability"},
		},
		{
			Key: "routing_policy.threshold", Scope: ScopeRoutingPolicy, Group: "routing-policies", Card: "routes", Field: "threshold",
			Type: TypeFloat, Min: num(0), Max: num(1), Default: "0.5",
			Label: "Confidence threshold",
			Description: "The confidence the chosen route needs, from 0 to 1; below it the request goes to the fallback model. " +
				"Raise it to send more borderline requests to the fallback, lower it to trust the decision model more. " +
				"Self-reported confidence is only a ranking, so it compares less exactly than logprobs.",
			Examples: []Example{
				{Value: "0.5", Effect: "A message scored 62% coding / 38% writing goes to the coding route."},
				{Value: "0.8", Effect: "The same message goes to the fallback, since 62% is below 80%."},
				{Value: "0", Effect: "The top label always wins; only a failed decision uses the fallback."},
			},
			Related:  []string{"routing_policy.scoring", "routing_policy.fallback_model"},
			Keywords: []string{"confidence", "cutoff", "probability"},
		},
		{
			Key: "routing_policy.fallback_model", Scope: ScopeRoutingPolicy, Group: "routing-policies", Card: "routes", Field: "fallback_model",
			Type:  TypeString,
			Label: "Fallback model",
			Description: "Where a request goes when the confidence is below the threshold, the decision fails or the message has no text. " +
				"A decision that times out counts as failed. Put your safest model here; it cannot be another routing policy.",
			Examples: []Example{
				{Value: "claude-sonnet-4-5", Effect: "Unclear and failed decisions get a strong general model rather than a cheap one."},
			},
			Related:  []string{"routing_policy.threshold", "routing_policy.decision_model"},
			Keywords: []string{"default model", "fallback", "catch-all"},
		},
		{
			Key: "routing_policy.pin_ttl_seconds", Scope: ScopeRoutingPolicy, Group: "routing-policies", Card: "settings", Field: "pin_ttl_seconds",
			Type: TypeInt, Unit: "s", Min: num(0), Max: num(604800), Default: "3600",
			Label: "Pin per session",
			Description: "How long a session keeps the model chosen at its first message. Later turns skip the decision, keep the " +
				"upstream prompt cache warm and never switch models mid-conversation. 0 classifies every request; at most 7 days.",
			Examples: []Example{
				{Value: "3600", Effect: "A conversation stays on its model for an hour after its first message, then is classified again."},
				{Value: "0", Effect: "Every request pays for a decision and may land on another model."},
			},
			Related:  []string{"routing_policy.decision_model"},
			Keywords: []string{"session", "sticky", "pin", "ttl"},
		},
	}
}

// Fields of an organization: the platform's limits on it (Organizations page) and its owners'
// own settings (organization settings).
func organizationFields() []Setting {
	return []Setting{
		{
			Key: "organization.max_members", Scope: ScopeOrganization, Group: "organizations", Card: "members", Field: "max_members",
			Type: TypeInt, Min: num(0), Default: "the site default",
			Label: "Member limit",
			Description: "The most members the organization may have, set by platform admins. It only stops new members from " +
				"joining; existing members are not removed. Follow the site default, unlimited (0) or a number.",
			Examples: []Example{
				{Value: "20", Effect: "An organization with 20 members cannot invite or add a 21st until one leaves."},
				{Value: "0", Effect: "No limit, whatever the site default says."},
			},
			Related:  []string{"OrganizationDefaultMaxMembers"},
			Keywords: []string{"seats", "team size", "members"},
		},
		{
			Key: "organization.approved_models", Scope: ScopeOrganization, Group: "organizations", Card: "models", Field: "approved_models",
			Type: TypeList, Default: "empty (every model)",
			Label: "Approved models",
			Description: "The most the organization may use, set by platform admins: every list of the organization stays inside it. " +
				"Its default, members' lists and keys are all checked against the model actually called. A name ending in * allows " +
				"a prefix, so new releases of a family follow; exact names need approving one by one. Empty means no limit.",
			Examples: []Example{
				{Value: "claude-*, gpt-5*", Effect: "Every Claude and GPT-5 model, including ones released later; nothing else."},
				{Value: "gpt-4.1-mini", Effect: "Only that model; a request for any other model is refused."},
			},
			Related:  []string{"organization.model_whitelist", "token.model_limits"},
			Keywords: []string{"allowlist", "model access", "whitelist", "approved"},
		},
		{
			Key: "organization.model_whitelist", Scope: ScopeOrganization, Group: "organizations", Card: "model-access", Field: "setting.model_whitelist",
			Type: TypeList, Default: "empty (every approved model)",
			Label: "Default model access",
			Description: "What members may use unless they have limits of their own, set by organization owners and admins. " +
				"It is a named list or a list of models. Members with their own limits are not affected, and keys can only narrow what " +
				"their creator may use. A name ending in * allows a prefix. Empty means every approved model.",
			Examples: []Example{
				{Value: "claude-haiku-*, gpt-4.1-mini", Effect: "Members use the cheap models only; a member given their own list can use more."},
			},
			Related:  []string{"organization.approved_models", "token.model_limits"},
			Keywords: []string{"allowlist", "model access", "whitelist", "member models"},
		},
		{
			Key: "organization.budget", Scope: ScopeOrganization, Group: "organizations", Card: "budget", Field: "setting.budget",
			Type: TypeFloat, Unit: "$", Min: num(0), Default: "none",
			Label: "Organization budget",
			Description: "The most all members together may spend per day, week or month; then the organization's requests are refused. " +
				"They are refused until the next period starts. Each member's own budget still applies. Periods start at midnight in the " +
				"platform's reset time zone; changing the period restarts the count. None means no organization-wide cap.",
			Examples: []Example{
				{Value: "1000 monthly", Effect: "When members have spent $1,000 in the month, every request of the organization is refused until the 1st."},
				{Value: "50 daily", Effect: "A runaway script stops the team at $50 a day."},
			},
			Related:  []string{"organization.member_budget", "QuotaResetTimezone", "QuotaResetWeekStart"},
			Keywords: []string{"spending cap", "budget", "limit", "team budget"},
		},
		{
			Key: "organization.member_budget", Scope: ScopeOrganization, Group: "organizations", Card: "members", Field: "member.setting.budget",
			Type: TypeFloat, Unit: "$", Min: num(0), Default: "none",
			Label: "Member budget",
			Description: "The most one member may spend per day, week or month in the organization, set on the member's limits. " +
				"Once used up, that member's requests are refused until the next period, while the organization budget caps " +
				"everyone together. Keys the organization owns have no member budget. None means no cap of the member's own.",
			Examples: []Example{
				{Value: "100 weekly", Effect: "The member can spend $100 from Monday (or Sunday, per the site's week start) to the next week start."},
			},
			Related:  []string{"organization.budget", "QuotaResetTimezone", "QuotaResetWeekStart"},
			Keywords: []string{"spending cap", "member limit", "budget"},
		},
		{
			Key: "organization.log_io_policy", Scope: ScopeOrganization, Group: "organizations", Card: "general", Field: "setting.log_io_policy",
			Type: TypeEnum, Default: "off",
			Enum: []EnumValue{
				{Value: "off", Label: "Keep for no key", Description: "No organization key keeps its request and response bodies."},
				{Value: "on", Label: "Keep for every key", Description: "Every organization key keeps its bodies, new keys included."},
				{Value: "selected", Label: "Keep for selected keys", Description: "Only the keys ticked in the list below keep their bodies; new keys start unticked."},
			},
			Label: "Request and response retention",
			Description: "Which of the organization's API keys keep the bodies of their requests and responses. Only owners and " +
				"admins set it, here: a key's creator cannot. Stored bodies never include API keys or auth headers, are deleted " +
				"after 30 days, and only owners and platform admins can read them. Members are told on their keys who turned " +
				"retention on and when. Shown only while the site allows retention.",
			Examples: []Example{
				{Value: "off", Effect: "Zero data retention for the whole organization."},
				{Value: "selected", Effect: "An admin ticks the key of the app being debugged; the other keys keep nothing."},
			},
			Related:  []string{"LogIOEnabled"},
			Keywords: []string{"log io", "retention", "request body", "privacy", "audit", "zero data retention"},
		},
	}
}

// Fields of an API key, on its panel in the API keys page.
func tokenFields() []Setting {
	return []Setting{
		{
			Key: "token.group", Scope: ScopeToken, Group: "tokens", Card: "access", Field: "group",
			Type: TypeString, Default: "empty (the user's group)",
			Label: "Group",
			Description: "The user group the key's requests run in: only channels of that group serve them, billed at its ratio. " +
				"It must be the user's own group or a public one. Empty uses the user's group.",
			Examples: []Example{
				{Value: "vip", Effect: "Requests use the vip channels and pay the vip ratio, even if the user is in default."},
			},
			Related:  []string{"token.backup_group", "user_group.public", "channel.group"},
			Keywords: []string{"user group", "channel group", "key group"},
		},
		{
			Key: "token.backup_group", Scope: ScopeToken, Group: "tokens", Card: "access", Field: "backup_group",
			Type:  TypeString,
			Label: "Backup group",
			Description: "A second group tried when the key's group has no channel that can take the request. A request served there " +
				"is billed at the backup group's ratio and marked in the logs. With a group or backup group set, the user's own " +
				"group is not tried.",
			Examples: []Example{
				{Value: "default", Effect: "Group vip with backup default: if every vip channel for the model fails, the request runs in default at its ratio."},
			},
			Related:  []string{"token.group", "user_group.ratio"},
			Keywords: []string{"fallback group", "failover", "backup"},
		},
		{
			Key: "token.model_limits", Scope: ScopeToken, Group: "tokens", Card: "access", Field: "setting.limits.limit_model_setting",
			Type:  TypeList,
			Label: "Model limits",
			Description: "The models the key may call; others are refused. A name ending in * allows a prefix. With the switch on and " +
				"no model selected, the key is not restricted. On an organization key it can only narrow what its creator may use, " +
				"and may be a named list.",
			Examples: []Example{
				{Value: "gpt-4.1-mini, claude-haiku-*", Effect: "A key for a chatbot that can only reach the two cheap model families."},
			},
			Related:  []string{"organization.model_whitelist", "organization.approved_models"},
			Keywords: []string{"allowlist", "model restriction", "whitelist"},
		},
		{
			Key: "token.quota", Scope: ScopeToken, Group: "tokens", Card: "basics", Field: "remain_quota",
			Type: TypeFloat, Unit: "$", Min: num(0), Default: "empty (no limit)",
			Label: "Spending limit",
			Description: "The most the key may spend; once used up, its requests are refused. With a reset period it is the limit " +
				"per period instead. It only caps the key: the account balance always applies. Empty means no limit of the key's own.",
			Examples: []Example{
				{Value: "10", Effect: "Without a period, the key stops for good after $10 of usage."},
				{Value: "5 daily", Effect: "The key can spend $5 a day; the count starts over at midnight."},
			},
			Related:  []string{"token.quota_reset"},
			Keywords: []string{"quota", "budget", "limit", "spend cap"},
		},
		{
			Key: "token.quota_reset", Scope: ScopeToken, Group: "tokens", Card: "basics", Field: "setting.quota_reset.period",
			Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "daily", Label: "Daily", Description: "Starts over at midnight."},
				{Value: "weekly", Label: "Weekly", Description: "Starts over at midnight on the first day of the week."},
				{Value: "monthly", Label: "Monthly", Description: "Starts over at midnight on the 1st."},
			},
			Label: "Reset period",
			Description: "How often the key's spending limit starts over. Without a period the limit is spent once. Periods start at " +
				"midnight in the platform's reset time zone, weeks on its week start; changing the period restarts the count. " +
				"A period needs a limit above 0.",
			Related:  []string{"token.quota", "QuotaResetTimezone", "QuotaResetWeekStart"},
			Keywords: []string{"reset", "period", "daily limit", "monthly limit"},
		},
	}
}
