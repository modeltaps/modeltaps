package settings

// Settings page group "site".
func init() {
	add(
		Setting{
			Key: "ServerAddress", Scope: ScopeSite, Group: "site", Card: "server-address", Type: TypeURL,
			Label: "Server address",
			Description: "The public address users reach the site at; every link and callback the server builds starts with it. " +
				"Sign-in callbacks, links in emails and Telegram replies, payment callbacks, generated image links and the docs " +
				"examples use it, and the CSRF check accepts requests from its origin. Enter the scheme and host as users type " +
				"them; a trailing slash is removed on save. The default http://localhost:3000 only works on the server itself. " +
				"Passkeys are bound to its host name: after a move to another domain, users register their passkeys again.",
			Examples: []Example{
				{Value: "https://api.example.com", Effect: "Password reset emails link to https://api.example.com/user/reset and an OIDC provider calls back to https://api.example.com/oauth/oidc/<slug>."},
			},
			Related:  []string{"PaymentCallbackAddress", "PasskeyLoginEnabled"},
			Keywords: []string{"domain", "base URL", "public URL", "site URL", "callback URL", "host"},
		},
		Setting{
			Key: "SystemName", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeString,
			Label: "Site name",
			Description: "The site's name in page titles, the console header and emails. It also names the site in payment " +
				"descriptions, authenticator apps, passkey prompts and notification webhooks. Empty shows Modeltaps.",
			Related:  []string{"Logo"},
			Keywords: []string{"system name", "brand", "title", "branding"},
		},
		Setting{
			Key: "Logo", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeURL,
			Label: "Logo URL",
			Description: "The image at the top of every email and on the signed-out and device authorization pages. " +
				"Use an absolute https address so mail clients can load it. Empty shows the built-in mark on those pages and " +
				"no image in emails.",
			Examples: []Example{
				{Value: "https://cdn.example.com/logo.svg", Effect: "Emails show the logo 130 px wide above the text; the signed-out page shows it instead of the built-in mark."},
			},
			Related:  []string{"SystemName"},
			Keywords: []string{"brand", "image", "icon", "branding"},
		},
		Setting{
			Key: "HomePageContent", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeText,
			Label: "Home page content",
			Description: "Your own home page in place of the built-in one. A value starting with http:// or https:// is " +
				"shown in a full-page frame; anything else is rendered as Markdown, or as sanitized HTML when it starts with a " +
				"tag. Empty keeps the built-in home page.",
			Examples: []Example{
				{Value: "https://www.example.com", Effect: "The home page shows your marketing site in a full-page frame under the site header."},
				{Value: "# Welcome\nSign in to get an API key.", Effect: "The home page shows a heading and one line of text."},
			},
			Related:  []string{"About", "Footer"},
			Keywords: []string{"landing page", "homepage", "index", "iframe", "markdown"},
		},
		Setting{
			Key: "About", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeText,
			Label: "About page",
			Description: "The content of the About page. A value starting with http:// or https:// is shown in a full-page " +
				"frame; anything else is rendered as Markdown, or as sanitized HTML when it starts with a tag. Empty shows the " +
				"built-in About text with a link to the project.",
			Examples: []Example{
				{Value: "Example AI is run by Example Inc. Contact: support@example.com", Effect: "The About page shows these two sentences."},
			},
			Related:  []string{"HomePageContent"},
			Keywords: []string{"about us", "contact", "company", "markdown"},
		},
		Setting{
			Key: "Footer", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeText,
			Label: "Footer",
			Description: "HTML that replaces the footer of the public pages. It is sanitized before display. Empty shows " +
				"the default footer with the copyright and the docs link. A custom footer also makes the home page render in " +
				"the console app instead of the prebuilt static page.",
			Examples: []Example{
				{Value: "© 2026 Example Inc · <a href=\"/about\">About</a>", Effect: "Every public page ends with this line instead of the default footer."},
			},
			Related:  []string{"HomePageContent", "DocsLink"},
			Keywords: []string{"copyright", "footer html", "ICP", "legal"},
		},
		Setting{
			Key: "AnalyticsCode", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeText,
			Label: "Analytics code",
			Description: "HTML added to the head of every page, for analytics scripts such as Google Analytics, Umami or " +
				"Plausible. It runs for every visitor with full access to the page, so paste only code from a provider you " +
				"trust. Empty adds nothing.",
			Examples: []Example{
				{Value: "<script defer src=\"https://umami.example.com/script.js\" data-website-id=\"<site id>\"></script>", Effect: "Umami counts the page views of the console and the public pages."},
			},
			Keywords: []string{"google analytics", "umami", "plausible", "tracking", "statistics", "script"},
		},
		Setting{
			Key: "BrandIconSyncEnabled", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeBool,
			Label: "Sync the icon library",
			Description: "Keeps the vendor and channel icon library up to date from the npm registry, checked every 24 hours. " +
				"A newer version is used once it is verified; a failed check keeps the current icons. Off keeps the icons embedded " +
				"in this build. Every node syncs on its own; browsers always load icons from this site.",
			Related:  []string{"BrandIconRegistry", "BrandIconFaviconFetchEnabled"},
			Keywords: []string{"lobehub", "icons", "provider icons", "vendor logo", "npm"},
			Advanced: true,
		},
		Setting{
			Key: "BrandIconRegistry", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeURL,
			Label: "Icon library registry",
			Description: "The npm registry the icon library sync downloads from. Enter an http or https address without a " +
				"query; a trailing slash is removed. Empty uses https://registry.npmmirror.com.",
			Examples: []Example{
				{Value: "https://registry.npmjs.org", Effect: "Downloads the icon library from the official npm registry."},
				{Value: "https://registry.npmmirror.com", Effect: "The default mirror, quicker to reach from mainland China."},
			},
			Related:  []string{"BrandIconSyncEnabled"},
			Keywords: []string{"npm registry", "mirror", "npmmirror", "icons"},
			Advanced: true,
		},
		Setting{
			Key: "BrandIconFaviconFetchEnabled", Scope: ScopeSite, Group: "site", Card: "branding", Type: TypeBool,
			Label: "Fetch icons for unknown vendors",
			Description: "When a vendor or channel without a built-in icon is saved, the server fetches its website's " +
				"favicon and caches it. Browsers still load every icon from this site. Off leaves those vendors without a " +
				"fetched icon.",
			Related:  []string{"BrandIconSyncEnabled"},
			Keywords: []string{"favicon", "icons", "vendor logo", "provider icons"},
			Advanced: true,
		},
		Setting{
			Key: "DisplayInCurrencyEnabled", Scope: ScopeSite, Group: "site", Card: "display", Type: TypeBool,
			Label: "Show amounts in US dollars",
			Description: "Shows balances, quotas and costs as US dollar amounts in the console, the logs and the billing API. " +
				"Each amount is quota / quota per dollar. Off shows raw quota numbers. Only the display changes, not what is charged.",
			Examples: []Example{
				{Value: "true", Effect: "With the default 500000 quota per dollar, a balance of 1,000,000 quota shows as $2.00."},
				{Value: "false", Effect: "The same balance shows as 1,000,000."},
			},
			Related:  []string{"QuotaPerUnit"},
			Keywords: []string{"currency", "USD", "dollar", "quota display", "balance"},
		},
		Setting{
			Key: "DisplayTokenStatEnabled", Scope: ScopeSite, Group: "site", Card: "display", Type: TypeBool,
			Label: "Report API key quota in the billing API",
			Description: "The billing API reports the calling API key's quota instead of the user's balance. This covers the " +
				"OpenAI-style endpoints /v1/dashboard/billing/subscription and /usage. A key with unlimited quota still reports the user's " +
				"balance, and organization keys always report their own quota. Off always reports the user's balance.",
			Related:  []string{"DisplayInCurrencyEnabled"},
			Keywords: []string{"billing API", "token quota", "API key quota", "subscription endpoint", "usage endpoint"},
		},
		Setting{
			Key: "QuotaPerUnit", Scope: ScopeSite, Group: "site", Card: "display", Type: TypeFloat, Min: num(1),
			Label: "Quota per dollar",
			Description: "How many quota units make one US dollar; every conversion between quota and dollars uses it. " +
				"A top-up of $1 credits this much quota, and dollar amounts in the console, logs and billing API are quota " +
				"divided by it. Balances are stored in quota, so changing it on a running site changes what every balance is " +
				"worth: keep 500000 unless the site is new.",
			Examples: []Example{
				{Value: "500000", Effect: "The default: a $10 top-up credits 5,000,000 quota, and 1,000 quota is $0.002."},
				{Value: "1000000", Effect: "On a running site, a stored balance of 5,000,000 quota drops from $10 to $5."},
			},
			Related:  []string{"DisplayInCurrencyEnabled"},
			Keywords: []string{"quota", "exchange rate", "credits", "points", "conversion", "unit"},
			Advanced: true,
		},
		Setting{
			Key: "DefaultLanguage", Scope: ScopeSite, Group: "site", Card: "site-language", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "en_US", Label: "English"},
				{Value: "zh_CN", Label: "Simplified Chinese"},
				{Value: "zh_HK", Label: "Traditional Chinese (Hong Kong)"},
				{Value: "ja_JP", Label: "Japanese"},
			},
			Label: "Default language",
			Description: "The language emails and the home page use when a person's language is unknown. " +
				"Users who picked a language in the console get emails in theirs; the home page uses it when the browser's " +
				"language is not supported. The email template editor opens on it. Empty means English.",
			Related:  []string{"MailTemplates"},
			Keywords: []string{"locale", "language", "i18n", "email language"},
		},
		Setting{
			Key: "TopUpLink", Scope: ScopeSite, Group: "site", Card: "external-links", Type: TypeURL,
			Label: "Top-up link",
			Description: "An outside shop where users buy redemption codes, opened in a new tab from the top-up page. " +
				"Empty hides the button.",
			Examples: []Example{
				{Value: "https://shop.example.com/codes", Effect: "The top-up page shows a button under the redemption code field that opens the shop."},
			},
			Keywords: []string{"recharge", "buy", "shop", "redemption code", "card store"},
		},
		Setting{
			Key: "DocsLink", Scope: ScopeSite, Group: "site", Card: "external-links", Type: TypeURL,
			Label: "Docs link",
			Description: "Where the site's documentation lives, linked from the footer, the account menu and the API pages. " +
				"/llms.txt and the agent skill file point to it too. Empty hides the external docs links; signed-in users " +
				"still get the console's built-in docs.",
			Examples: []Example{
				{Value: "https://docs.example.com", Effect: "Each API page links to its own page under https://docs.example.com."},
			},
			Related:  []string{"Footer"},
			Keywords: []string{"documentation", "docs", "help", "llms.txt"},
		},
		Setting{
			Key: "Notice", Scope: ScopeSite, Group: "site", Card: "version-notice", Type: TypeText,
			Label: "Announcement",
			Description: "A banner at the top of the console for signed-in users, until each one reads or closes it. " +
				"It shows again whenever the text changes. Markdown and HTML are supported. Empty shows no announcement.",
			Examples: []Example{
				{Value: "Maintenance on Saturday 02:00-03:00 UTC: requests may fail for a few minutes.", Effect: "Every signed-in user sees the banner once; editing the text shows it to everyone again."},
			},
			Keywords: []string{"notice", "banner", "announcement", "message", "maintenance"},
		},
	)
}
