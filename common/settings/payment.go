package settings

// Settings page group "payment".
func init() {
	add(
		// Card "payment".
		Setting{
			Key: "PaymentUSDRate", Scope: ScopeSite, Group: "payment", Card: "payment", Type: TypeFloat, Unit: "CNY/USD",
			Min:   num(0),
			Label: "CNY collection rate",
			Description: "How many yuan one US dollar of top-up costs at gateways that charge in CNY. A top-up of $X costs " +
				"X x this rate there, times the gateway's own rate, plus its fee. It changes neither balances nor the quota a " +
				"top-up buys, and USD gateways ignore it. Use the real exchange rate; for promotions use top-up discounts or " +
				"a gateway's rate.",
			Examples: []Example{
				{Value: "7.3", Effect: "A $100 top-up costs ¥730 at a CNY gateway and $100 at a USD gateway; both buy 50,000,000 quota."},
				{Value: "7.1", Effect: "The same top-up costs ¥710: you take ¥20 less for the same quota."},
			},
			Related:  []string{"RechargeDiscount", "PaymentMinAmount", "PaymentCostPercent"},
			Keywords: []string{"exchange rate", "CNY", "RMB", "yuan", "currency", "alipay", "wechat pay"},
		},
		Setting{
			Key: "PaymentMinAmount", Scope: ScopeSite, Group: "payment", Card: "payment", Type: TypeInt, Unit: "USD",
			Min:   num(0),
			Label: "Minimum top-up",
			Description: "The smallest top-up a user or an organization can pay, in whole US dollars. Smaller amounts are " +
				"refused before a payment is created. 0 and 1 both allow any amount from $1.",
			Examples: []Example{
				{Value: "1", Effect: "Users can top up any whole amount from $1 (500,000 quota)."},
				{Value: "10", Effect: "A $5 top-up is refused; $10 is the least, buying 5,000,000 quota."},
			},
			Related:  []string{"PaymentUSDRate", "RechargeDiscount"},
			Keywords: []string{"minimum recharge", "minimum payment", "top-up amount"},
		},
		Setting{
			Key: "PaymentCallbackAddress", Scope: ScopeSite, Group: "payment", Card: "payment", Type: TypeURL,
			Advanced: true,
			Label:    "Payment callback address",
			Description: "The address payment gateways send their payment notices to, when it differs from the server address. " +
				"Empty uses the gateway's own callback domain, then the server address. Notices go to " +
				"<address>/api/payment/notify/<gateway id> (EPay: <address>/api/user/epay/notify), which must be reachable " +
				"from the internet, or paid orders are never credited.",
			Examples: []Example{
				{Value: "https://pay.example.com", Effect: "Gateways call https://pay.example.com/api/payment/notify/<gateway id> while users keep using the main domain."},
			},
			Related:  []string{"ServerAddress"},
			Keywords: []string{"notify url", "webhook", "callback", "payment notification", "return url"},
		},
		Setting{
			Key: "RechargeDiscount", Scope: ScopeSite, Group: "payment", Card: "payment", Type: TypeJSON,
			Label: "Top-up discounts",
			Description: "Discounts for fixed top-up amounts: a JSON object from a whole-dollar amount to the share of the price " +
				"the user pays. Only those exact amounts are discounted; others pay full price. The quota is never " +
				"discounted: $100 always buys 100 x 500,000 quota at the default quota per unit. A gateway's percentage fee " +
				"is charged on the discounted price. Empty means no discounts.",
			Examples: []Example{
				{Value: `{"100":0.95}`, Effect: "A $100 top-up costs $95 (¥693.50 at a CNY rate of 7.3) and buys 50,000,000 quota; $99 or $101 pay full price."},
				{Value: `{"50":0.98,"200":0.9}`, Effect: "$50 costs $49 and $200 costs $180; every other amount pays full price."},
			},
			Related:  []string{"PaymentUSDRate", "PaymentMinAmount", "QuotaPerUnit"},
			Keywords: []string{"recharge discount", "bulk discount", "promotion", "top-up bonus"},
		},
	)
}
