package settings

// Settings page group "billing".
func init() {
	add(
		// Card "quota".
		Setting{
			Key: "QuotaForNewUser", Scope: ScopeSite, Group: "billing", Card: "quota", Type: TypeInt, Min: num(0),
			Label: "New user quota",
			Description: "The quota a new account starts with, **granted once when it is created** and logged as a sign-up bonus. " +
				"In quota units: 500,000 = $1 at the default quota per unit; the field also takes dollars. Accounts an " +
				"organization creates for its members start at 0, and **changing it leaves existing balances alone**. 0 gives " +
				"new users nothing until they top up or redeem a code.",
			Examples: []Example{
				{Value: "1000000", Effect: "$2 of trial credit: about 119,000 output tokens of a $15 per million model sold at ratio 1.12."},
				{Value: "0", Effect: "New users can call nothing until they top up or redeem a code."},
			},
			Related:  []string{"QuotaPerUnit", "QuotaForInvitee", "PaymentMinAmount"},
			Keywords: []string{"sign-up bonus", "trial credit", "free credit", "initial balance", "welcome credit"},
		},
		Setting{
			Key: "PreConsumedQuota", Scope: ScopeSite, Group: "billing", Card: "quota", Type: TypeInt, Min: num(0),
			Advanced: true,
			Label:    "Fixed pre-charge",
			Description: "A fixed amount added to the up-front reservation of every token-priced request, on top of its estimated cost. " +
				"The reservation (estimated input + reserved output tokens + this) is held from the balance or the API key's " +
				"quota while the request runs and settled to the real cost when it ends, so it **decides who may start a request**, " +
				"**never what is charged**. A balance below the reservation **is refused with insufficient quota**. In quota units: " +
				"500 = $0.001. Requests priced per call or per minute reserve their own price instead.",
			Examples: []Example{
				{Value: "500", Effect: "The default. A request with 2,000 input tokens on a $3 / $15 per million model at ratio 1.12 holds 3,360 + 34,406 + 500 = 38,266 quota (about $0.077) until it settles."},
				{Value: "50000", Effect: "Every request needs $0.10 more than its estimate to start, so nearly empty balances are refused earlier."},
				{Value: "0", Effect: "Only the estimate is held: a balance just above it can still start a request."},
			},
			Related:  []string{"PreConsumedOutputTokens"},
			Keywords: []string{"pre-consume", "reservation", "hold", "insufficient quota", "deposit"},
		},
		Setting{
			Key: "PreConsumedOutputTokens", Scope: ScopeSite, Group: "billing", Card: "quota", Type: TypeInt, Min: num(0),
			Advanced: true,
			Label:    "Reserved output tokens",
			Description: "How many output tokens a token-priced request reserves up front, or the request's own max_tokens when smaller. " +
				"Reserving its likely output keeps concurrent requests from **driving a balance far below zero**; the charge is " +
				"**settled to the real usage** when the request ends. 0 reserves no output, only the input estimate and the fixed " +
				"pre-charge.",
			Examples: []Example{
				{Value: "4096", Effect: "The default. A $15 per million output model at ratio 1.12 reserves 4,096 x 8.4 = 34,406 quota (about $0.069) per request; a request with max_tokens 1,000 reserves 8,400."},
				{Value: "32000", Effect: "Long reasoning answers are covered up front: 32,000 x 8.4 = 268,800 quota (about $0.54) on the same model, so a balance below that cannot start a request."},
			},
			Related:  []string{"PreConsumedQuota"},
			Keywords: []string{"pre-consume", "max_tokens", "reservation", "output estimate"},
		},
		Setting{
			Key: "QuotaRemindEnabled", Scope: ScopeSite, Group: "billing", Card: "quota", Type: TypeBool,
			Label: "Quota warning emails",
			Description: "Email users when their balance falls below the warning threshold, and again when it runs out. " +
				"Each email is sent once; another goes out only after the balance has clearly recovered and fallen again. " +
				"**Needs a mail service**. Users can turn the emails off or set their own threshold in their personal settings; " +
				"**organization API keys never send them**. When off nobody gets them, and settlement skips the balance read the " +
				"check needs.",
			Related:  []string{"QuotaRemindThreshold"},
			Keywords: []string{"low balance", "balance alert", "reminder", "email", "running low", "used up"},
		},
		Setting{
			Key: "QuotaRemindThreshold", Scope: ScopeSite, Group: "billing", Card: "quota", Type: TypeInt, Min: num(0),
			Label: "Warning threshold",
			Description: "The balance below which a user gets the \"running low\" email, in quota units (500,000 = $1). " +
				"It is **the default for users who have not set their own threshold**. After an email the next one waits until the " +
				"balance has gone back above the threshold plus 10% of it (or plus the fixed pre-charge, if larger) and fallen " +
				"again. 0 **sends only the \"used up\" email**.",
			Examples: []Example{
				{Value: "1000", Effect: "The default: the email goes out when $0.002 is left, about when the balance runs out."},
				{Value: "5000000", Effect: "Users hear when less than $10 is left; the next email comes only after the balance went back above $11 and fell again."},
			},
			Related:  []string{"QuotaRemindEnabled", "QuotaPerUnit", "PreConsumedQuota"},
			Keywords: []string{"low balance", "balance alert", "reminder threshold"},
		},
		// Card "pricing".
		Setting{
			Key: "PaymentCostPercent", Scope: ScopeSite, Group: "billing", Card: "pricing", Type: TypePercent, Unit: "%",
			Min: num(0), Max: num(99.99),
			Label: "Payment cost",
			Description: "What taking a payment costs the site, as a percentage of the amount: card fees and currency " +
				"conversion. **Nobody is charged** it. It **only lowers the revenue** the price check and the margin figures " +
				"compare channel costs with: net revenue = catalog price x the default group's ratio x (1 - payment cost). " +
				"0 counts payments as free.",
			Examples: []Example{
				{Value: "4", Effect: "Card fee 3% plus 1% for currency conversion. With the default group at 1.12, $1 of catalog price brings in 1.12 x 0.96 = $1.0752."},
				{Value: "0", Effect: "Payments cost nothing: the price check compares channel costs with the full price."},
			},
			Related:  []string{"user_group.ratio", "channel.fee_rate"},
			Keywords: []string{"card fee", "stripe fee", "processing fee", "margin", "profit"},
			Docs:     "use/pricing_and_routing.md#the-price-check",
		},
		// Card "billing-rules".
		Setting{
			Key: "EmptyResponseBillingEnabled", Scope: ScopeSite, Group: "billing", Card: "billing-rules", Type: TypeBool,
			Label: "Charge empty replies",
			Description: "Charge token-priced requests whose reply has no output tokens for their input. When off such requests " +
				"**are free**, although **the upstream may still bill you for the input**. Requests priced per call or per minute are " +
				"charged either way, and a request with no tokens at all is never charged.",
			Examples: []Example{
				{Value: "true", Effect: "An empty reply to 10,000 input tokens of a $3 per million model costs the user $0.03 at ratio 1."},
				{Value: "false", Effect: "The same request is free for the user; you still pay the upstream for the input."},
			},
			Keywords: []string{"empty response", "no output", "zero tokens", "free request"},
		},
		Setting{
			Key: "ApproximateTokenEnabled", Scope: ScopeSite, Group: "billing", Card: "billing-rules", Type: TypeBool,
			Advanced: true,
			Label:    "Approximate token counting",
			Description: "Count tokens locally as 0.38 tokens per byte of text instead of running the model's tokenizer. " +
				"Local counts are used for the up-front reservation and when an upstream reply carries no usage; replies " +
				"that report their usage are billed by it. It saves CPU on busy instances **at the cost of accuracy**, worst " +
				"for text outside the Latin alphabet.",
			Examples: []Example{
				{Value: "true", Effect: "1,000 characters of English (1,000 bytes) count as 380 tokens; 1,000 Chinese characters (3,000 bytes) count as 1,140."},
			},
			Keywords: []string{"tokenizer", "tiktoken", "token count", "estimate"},
		},
		Setting{
			Key: "PriceSyncAutoApply", Scope: ScopeSite, Group: "billing", Card: "billing-rules", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "increases", Label: "Write increases, hold decreases", Description: "A price that went up is written at once, so you never sell below a new cost; a decrease waits for your review."},
				{Value: "all", Label: "Write every change", Description: "The price table follows the upstream price lists both ways."},
				{Value: "none", Label: "New models only", Description: "Every change to an existing price waits for your review."},
			},
			Label: "Automatic price sync",
			Description: "What the automatic price syncs (at startup, daily and when a channel is saved) write for models that " +
				"already have a price. **New models are always added** and **locked prices never change**. Changes that are not " +
				"written, and locked prices below the upstream price, are sent as notifications; review and apply them from " +
				"the channel's price sync.",
			Examples: []Example{
				{Value: "increases", Effect: "An upstream raises a model's output from $10 to $12 per million: written at once. It cuts it to $8: the table keeps $10 and you are notified."},
				{Value: "none", Effect: "Both changes wait in the notification until you apply them."},
			},
			Keywords: []string{"price update", "price sync", "pricing", "auto update", "price list"},
		},
		// Card "quota-reset".
		Setting{
			Key: "QuotaResetTimezone", Scope: ScopeSite, Group: "billing", Card: "quota-reset", Type: TypeString,
			Label: "Reset time zone",
			Description: "The time zone whose midnight starts each daily, weekly and monthly limit period. It applies to " +
				"API key period limits, organization budgets and member budgets. An IANA name; empty or unknown names " +
				"**fall back to UTC**. Changing it **moves the boundary of the current period**.",
			Examples: []Example{
				{Value: "UTC", Effect: "The default: daily limits reset at 00:00 UTC, which is 08:00 in Beijing."},
				{Value: "Asia/Shanghai", Effect: "Daily limits reset at 00:00 Beijing time, which is 16:00 UTC the day before."},
				{Value: "America/New_York", Effect: "Limits reset at local midnight through daylight saving time: 05:00 UTC in winter, 04:00 UTC in summer."},
			},
			Related:  []string{"QuotaResetWeekStart"},
			Keywords: []string{"timezone", "period limit", "daily limit", "budget reset", "midnight"},
		},
		Setting{
			Key: "QuotaResetWeekStart", Scope: ScopeSite, Group: "billing", Card: "quota-reset", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "monday", Label: "Monday"},
				{Value: "sunday", Label: "Sunday"},
			},
			Label: "Week starts on",
			Description: "The day weekly limit periods start, at midnight in the reset time zone. Daily and monthly periods " +
				"**are not affected**.",
			Examples: []Example{
				{Value: "monday", Effect: "A weekly API key limit used up on Saturday is available again on Monday at 00:00."},
				{Value: "sunday", Effect: "The same limit comes back on Sunday at 00:00."},
			},
			Related:  []string{"QuotaResetTimezone"},
			Keywords: []string{"weekly limit", "week start", "period limit"},
		},
		// Card "invite-reward".
		Setting{
			Key: "InviteRewardEnabled", Scope: ScopeSite, Group: "billing", Card: "invite-reward", Type: TypeBool,
			Label: "Invite rewards",
			Description: "Pay invite rewards: sign-up bonuses for the inviter and the invitee, and the inviter's rebate on the " +
				"first top-up. When off **none are paid and the site hides its invite entries**; who invited whom " +
				"is still recorded.",
			Related:  []string{"QuotaForInviter", "QuotaForInvitee", "InviterRewardType", "InviterRewardValue"},
			Keywords: []string{"referral", "invite", "affiliate", "aff", "invite code"},
		},
		Setting{
			Key: "QuotaForInviter", Scope: ScopeSite, Group: "billing", Card: "invite-reward", Type: TypeInt, Min: num(0),
			Label: "Inviter sign-up bonus",
			Description: "The quota an inviter gets each time someone signs up with their invite link or code. " +
				"In quota units (500,000 = $1). 0 pays nothing at sign-up; **the first top-up rebate is set separately**.",
			Examples: []Example{
				{Value: "1000000", Effect: "Each invited sign-up earns the inviter $2, whether or not the new user ever pays."},
				{Value: "0", Effect: "Inviters earn only the first top-up rebate, so fake sign-ups bring nothing."},
			},
			Related:  []string{"InviteRewardEnabled", "QuotaForInvitee", "InviterRewardValue"},
			Keywords: []string{"referral bonus", "inviter reward", "affiliate"},
		},
		Setting{
			Key: "QuotaForInvitee", Scope: ScopeSite, Group: "billing", Card: "invite-reward", Type: TypeInt, Min: num(0),
			Label: "Invitee sign-up bonus",
			Description: "Extra quota a new user gets for signing up with an invite link or code, **on top of the new user quota**. " +
				"In quota units (500,000 = $1). 0 gives invited users only the new user quota.",
			Examples: []Example{
				{Value: "500000", Effect: "With the new user quota at 1,000,000 ($2), an invited user starts with $3."},
			},
			Related:  []string{"InviteRewardEnabled", "QuotaForNewUser", "QuotaForInviter"},
			Keywords: []string{"invite code bonus", "referral bonus", "invitee reward"},
		},
		Setting{
			Key: "InviterRewardType", Scope: ScopeSite, Group: "billing", Card: "invite-reward", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "fixed", Label: "Fixed", Description: "The rebate is a quota amount, whatever the top-up."},
				{Value: "percentage", Label: "Percentage", Description: "The rebate is a percentage of the quota the top-up bought."},
			},
			Label: "First top-up rebate type",
			Description: "How the inviter's rebate on an invited user's first paid top-up is counted: **a fixed quota** or a " +
				"**percentage of the top-up**. The first top-up rebate sets the amount.",
			Related:  []string{"InviterRewardValue"},
			Keywords: []string{"rebate", "commission", "referral reward", "affiliate"},
		},
		Setting{
			Key: "InviterRewardValue", Scope: ScopeSite, Group: "billing", Card: "invite-reward", Type: TypeInt, Min: num(0),
			Label: "First top-up rebate",
			Description: "What an inviter earns when someone they invited makes a first paid top-up. For the fixed type it is " +
				"quota units (500,000 = $1); for the percentage type a whole percentage from 0 to 100 of the quota bought. " +
				"**Only the first paid top-up earns it**: later top-ups and redemption codes earn nothing. " +
				"0 **turns the rebate off**.",
			Examples: []Example{
				{Value: "10", Effect: "Percentage: an invited user's first $50 top-up (25,000,000 quota) earns the inviter 2,500,000 quota, $5."},
				{Value: "1000000", Effect: "Fixed: the inviter gets $2 for the first top-up, whether it is $1 or $500."},
			},
			Related:  []string{"InviterRewardType", "InviteRewardEnabled", "RechargeDiscount"},
			Keywords: []string{"rebate", "commission", "referral reward", "first top-up", "affiliate"},
		},
	)
}
