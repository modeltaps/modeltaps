package settings

// Settings page group "auth".
func init() {
	add(
		// ---------------------------------------------------------------- account-system
		Setting{
			Key: "AccountSystem", Scope: ScopeSite, Group: "auth", Card: "account-system", Type: TypeEnum,
			Enum: []EnumValue{
				{
					Value: "builtin", Label: "Built-in accounts",
					Description: "This site manages users, passwords, email codes and passkeys, optionally with GitHub, WeChat " +
						"and other third-party sign-in. **No external service** is needed.",
				},
				{
					Value: "external", Label: "External identity provider",
					Description: "An OIDC service handles sign-in, sign-up, phone numbers, two-step verification and third-party " +
						"sign-in. This site shows **no sign-in form of its own**: /login goes straight to the provider.",
				},
			},
			Label: "Account system",
			Description: "Whether this site manages its own accounts or leaves them to an external identity provider. " +
				"With an external provider, the site's password, email code, passkey and social sign-in switches **stop applying** " +
				"to regular users; **only root keeps the emergency sign-in** at /login/admin. Switching to external needs an enabled " +
				"identity provider marked as the site identity, and saving asks for confirmation either way.",
			Related:  []string{"AdminLoginEnabled", "PasswordLoginEnabled", "RegisterEnabled"},
			Keywords: []string{"OIDC", "SSO", "identity provider", "IdP", "single sign-on", "login mode"},
		},

		// ---------------------------------------------------------------- login-methods
		Setting{
			Key: "PasswordLoginEnabled", Scope: ScopeSite, Group: "auth", Card: "login-methods", Type: TypeBool,
			Label: "Password sign-in",
			Description: "Users can sign in with their username or email and password. When off, regular users sign in " +
				"with an email code, a passkey or a social account; root can still use its password to reach the console. " +
				"Only applies **with built-in accounts**.",
			Related:  []string{"PasswordRegisterEnabled", "EmailCodeLoginEnabled", "PasskeyLoginEnabled", "AccountSystem"},
			Keywords: []string{"password login", "username", "local login"},
		},
		Setting{
			Key: "EmailCodeLoginEnabled", Scope: ScopeSite, Group: "auth", Card: "login-methods", Type: TypeBool,
			Label: "Email code sign-in",
			Description: "Users can sign in with a code sent to their verified email instead of a password. It needs a " +
				"**working mail service**: without one the option is not offered on the sign-in page. Only applies **with built-in accounts**.",
			Related:  []string{"PasswordLoginEnabled", "PasskeyLoginEnabled"},
			Keywords: []string{"magic code", "one-time code", "passwordless", "OTP", "login code"},
		},
		Setting{
			Key: "PasskeyLoginEnabled", Scope: ScopeSite, Group: "auth", Card: "login-methods", Type: TypeBool,
			Label: "Passkeys",
			Description: "Users can add passkeys (fingerprint, face or a hardware key) on their security page and sign in " +
				"with them. When off, **passkeys can neither be added nor used**. With an external identity provider only root " +
				"can use them, for the emergency sign-in.",
			Related:  []string{"PasswordLoginEnabled", "AdminLoginEnabled"},
			Keywords: []string{"WebAuthn", "FIDO2", "security key", "biometric", "Touch ID"},
		},
		Setting{
			Key: "RegisterEnabled", Scope: ScopeSite, Group: "auth", Card: "login-methods", Type: TypeBool,
			Label: "New user sign-up",
			Description: "Whether new accounts can be created at all. When off, **nobody can sign up**: not with a password, " +
				"not on a first social or identity provider sign-in, and organizations cannot create member accounts. " +
				"Existing users **still sign in**.",
			Related:  []string{"PasswordRegisterEnabled", "InviteCodeRegisterEnabled", "EmailVerificationEnabled"},
			Keywords: []string{"registration", "signup", "close registration", "new accounts"},
		},
		Setting{
			Key: "PasswordRegisterEnabled", Scope: ScopeSite, Group: "auth", Card: "login-methods", Type: TypeBool,
			Label: "Password sign-up",
			Description: "New users can sign up with a username and password. When off, new accounts come only from social " +
				"sign-in, and organizations cannot create member accounts. **Needs New user sign-up on**; **only applies with built-in accounts**.",
			Related:  []string{"RegisterEnabled", "EmailVerificationEnabled", "InviteCodeRegisterEnabled", "PasswordLoginEnabled"},
			Keywords: []string{"registration", "signup form"},
		},
		Setting{
			Key: "EmailVerificationEnabled", Scope: ScopeSite, Group: "auth", Card: "login-methods", Type: TypeBool,
			Label: "Email verification at sign-up",
			Description: "Password sign-up asks for an email and the code sent to it, and the account keeps that email as " +
				"verified. It **needs a working mail service**. Social sign-ups are not affected. With the email domain restriction " +
				"on, codes only go to allowed domains.",
			Related:  []string{"PasswordRegisterEnabled", "EmailDomainRestrictionEnabled", "EmailDomainWhitelist"},
			Keywords: []string{"verify email", "verification code", "confirm email"},
		},
		Setting{
			Key: "InviteCodeRegisterEnabled", Scope: ScopeSite, Group: "auth", Card: "login-methods", Type: TypeBool,
			Label: "Invite code required to sign up",
			Description: "New users **must enter a valid invite code**, both for password sign-up and for a first social sign-in. " +
				"Invite codes are managed on the invite codes page. Existing users and member accounts created by organizations " +
				"do not need one.",
			Related:  []string{"RegisterEnabled", "PasswordRegisterEnabled"},
			Keywords: []string{"invitation", "invite only", "closed beta", "referral code"},
		},
		Setting{
			Key: "TurnstileCheckEnabled", Scope: ScopeSite, Group: "auth", Card: "login-methods", Type: TypeBool,
			Label: "Bot check",
			Description: "Cloudflare Turnstile checks that a person is behind sign-in, sign-up, password reset and the emails " +
				"with codes. It **cannot be turned on before the Turnstile site key is saved**, and it **needs the secret key too**. " +
				"A passed check covers the session for 10 minutes by default.",
			Related:  []string{"TurnstileSiteKey", "TurnstileSecretKey"},
			Keywords: []string{"captcha", "Turnstile", "Cloudflare", "anti-bot", "human verification"},
		},

		// ---------------------------------------------------------------- admin-login
		Setting{
			Key: "AdminLoginEnabled", Scope: ScopeSite, Group: "auth", Card: "admin-login", Type: TypeBool,
			Label: "Admin emergency sign-in",
			Description: "Keeps /login/admin open so root can sign in with its password or passkey when the identity " +
				"provider is down. **Only root gets through**; account lockout, rate limits and the bot check still apply. " +
				"It **exists only with an external identity provider**. When off, **nobody can reach the console** while the provider " +
				"is down.",
			Related:  []string{"AccountSystem", "PasskeyLoginEnabled"},
			Keywords: []string{"break glass", "root login", "emergency access", "backdoor", "fallback login"},
		},

		// ---------------------------------------------------------------- github-oauth
		Setting{
			Key: "GitHubOAuthEnabled", Scope: ScopeSite, Group: "auth", Card: "github-oauth", Type: TypeBool,
			Label: "GitHub sign-in",
			Description: "Users can sign in and sign up with their GitHub account. It cannot be turned on before the GitHub " +
				"client ID is saved. Only applies **with built-in accounts**.",
			Related:  []string{"GitHubClientId", "GitHubClientSecret", "GitHubOldIdCloseEnabled"},
			Keywords: []string{"GitHub", "OAuth", "social login"},
		},
		Setting{
			Key: "GitHubClientId", Scope: ScopeSite, Group: "auth", Card: "github-oauth", Type: TypeString,
			Label: "GitHub client ID",
			Description: "The client ID of the OAuth app you registered on GitHub. Register it with this site's address as " +
				"the homepage URL and **<server address>/oauth/github as the callback URL**.",
			Related:  []string{"GitHubOAuthEnabled", "GitHubClientSecret"},
			Keywords: []string{"GitHub", "OAuth app", "client id"},
		},
		Setting{
			Key: "GitHubClientSecret", Scope: ScopeSite, Group: "auth", Card: "github-oauth", Type: TypeSecret,
			Label: "GitHub client secret",
			Description: "The client secret of the same GitHub OAuth app. It is **never shown again after saving**; leave the " +
				"field empty to keep the stored one.",
			Related:  []string{"GitHubOAuthEnabled", "GitHubClientId"},
			Keywords: []string{"GitHub", "OAuth app", "client secret"},
		},
		Setting{
			Key: "GitHubOldIdCloseEnabled", Scope: ScopeSite, Group: "auth", Card: "github-oauth", Type: TypeBool,
			Label: "Match GitHub accounts by ID only",
			Description: "GitHub sign-in no longer matches old accounts by their GitHub login name, only by GitHub's numeric " +
				"account ID. When off, an account from before IDs were stored is matched once by login name and then keeps " +
				"the ID. A login name can be renamed and **taken by someone else**, so **turn it on once old accounts have signed in**.",
			Related:  []string{"GitHubOAuthEnabled"},
			Keywords: []string{"legacy GitHub", "username", "impersonation", "old id"},
			Advanced: true,
		},

		// ---------------------------------------------------------------- wechat-auth
		Setting{
			Key: "WeChatAuthEnabled", Scope: ScopeSite, Group: "auth", Card: "wechat-auth", Type: TypeBool,
			Label: "WeChat sign-in",
			Description: "Users can sign in and sign up through your WeChat official account. They follow it, get a code " +
				"and enter it on the sign-in page. It **needs a WeChat login server** and cannot be turned on before its address " +
				"is saved. Only applies **with built-in accounts**.",
			Related:  []string{"WeChatServerAddress", "WeChatServerToken", "WeChatAccountQRCodeImageURL"},
			Keywords: []string{"WeChat", "Weixin", "official account", "social login"},
		},
		Setting{
			Key: "WeChatServerAddress", Scope: ScopeSite, Group: "auth", Card: "wechat-auth", Type: TypeURL,
			Label: "WeChat login server address",
			Description: "The address of the WeChat login server behind your official account. The site asks it which " +
				"WeChat user a code belongs to.",
			Examples: []Example{
				{Value: "https://wechat.example.com", Effect: "Codes are checked at https://wechat.example.com/api/wechat/user."},
			},
			Related:  []string{"WeChatAuthEnabled", "WeChatServerToken"},
			Keywords: []string{"WeChat", "server", "official account"},
		},
		Setting{
			Key: "WeChatServerToken", Scope: ScopeSite, Group: "auth", Card: "wechat-auth", Type: TypeSecret,
			Label: "WeChat login server token",
			Description: "The access token the site sends to the WeChat login server with every check, as set on that " +
				"server. It is **never shown again after saving**; leave the field empty to keep the stored one.",
			Related:  []string{"WeChatServerAddress"},
			Keywords: []string{"WeChat", "access token"},
		},
		Setting{
			Key: "WeChatAccountQRCodeImageURL", Scope: ScopeSite, Group: "auth", Card: "wechat-auth", Type: TypeURL,
			Label: "Official account QR code",
			Description: "A link to the QR code image of your WeChat official account. The WeChat sign-in dialog shows it " +
				"so users can scan it, follow the account and get their code.",
			Examples: []Example{
				{Value: "https://example.com/wechat-qr.png", Effect: "The sign-in dialog shows this image for users to scan."},
			},
			Related:  []string{"WeChatAuthEnabled"},
			Keywords: []string{"WeChat", "QR code", "official account"},
		},

		// ---------------------------------------------------------------- lark-auth
		Setting{
			Key: "LarkAuthEnabled", Scope: ScopeSite, Group: "auth", Card: "lark-auth", Type: TypeBool,
			Label: "Lark sign-in",
			Description: "Users can sign in and sign up with their Lark (Feishu) account. It uses the app whose App ID and " +
				"App Secret you enter on this card. Only applies **with built-in accounts**.",
			Related:  []string{"LarkClientId", "LarkClientSecret"},
			Keywords: []string{"Lark", "Feishu", "OAuth", "social login"},
		},
		Setting{
			Key: "LarkClientId", Scope: ScopeSite, Group: "auth", Card: "lark-auth", Type: TypeString,
			Label: "Lark App ID",
			Description: "The App ID of the Lark (Feishu) app users sign in with. Create a web app on the Lark open platform " +
				"and **set its redirect URL to <server address>/oauth/lark**.",
			Related:  []string{"LarkAuthEnabled", "LarkClientSecret"},
			Keywords: []string{"Lark", "Feishu", "app id", "client id"},
		},
		Setting{
			Key: "LarkClientSecret", Scope: ScopeSite, Group: "auth", Card: "lark-auth", Type: TypeSecret,
			Label: "Lark App Secret",
			Description: "The App Secret of the same Lark app. It is **never shown again after saving**; leave the field empty to " +
				"keep the stored one.",
			Related:  []string{"LarkAuthEnabled", "LarkClientId"},
			Keywords: []string{"Lark", "Feishu", "app secret", "client secret"},
		},

		// ---------------------------------------------------------------- linuxdo-oauth
		Setting{
			Key: "LinuxDoOAuthEnabled", Scope: ScopeSite, Group: "auth", Card: "linuxdo-oauth", Type: TypeBool,
			Label: "LINUX DO sign-in",
			Description: "Users can sign in and sign up with their LINUX DO account. It cannot be turned on before the client " +
				"ID and client secret are saved. Only applies **with built-in accounts**.",
			Related:  []string{"LinuxDoClientId", "LinuxDoClientSecret", "LinuxDoOAuthTrustLevelEnabled"},
			Keywords: []string{"LINUX DO", "linuxdo", "OAuth", "social login"},
		},
		Setting{
			Key: "LinuxDoClientId", Scope: ScopeSite, Group: "auth", Card: "linuxdo-oauth", Type: TypeString,
			Label: "LINUX DO client ID",
			Description: "The client ID of the OAuth app you registered on LINUX DO, with **<server address>/oauth/linuxdo** " +
				"as its callback URL.",
			Related:  []string{"LinuxDoOAuthEnabled", "LinuxDoClientSecret"},
			Keywords: []string{"LINUX DO", "OAuth app", "client id"},
		},
		Setting{
			Key: "LinuxDoClientSecret", Scope: ScopeSite, Group: "auth", Card: "linuxdo-oauth", Type: TypeSecret,
			Label: "LINUX DO client secret",
			Description: "The client secret of the same LINUX DO OAuth app. It is **never shown again after saving**; leave the " +
				"field empty to keep the stored one.",
			Related:  []string{"LinuxDoOAuthEnabled", "LinuxDoClientId"},
			Keywords: []string{"LINUX DO", "OAuth app", "client secret"},
		},
		Setting{
			Key: "LinuxDoOAuthTrustLevelEnabled", Scope: ScopeSite, Group: "auth", Card: "linuxdo-oauth", Type: TypeBool,
			Label: "Minimum trust level",
			Description: "LINUX DO accounts below the minimum trust level **cannot sign up or be linked**. Whether existing users are checked " +
				"too is up to the dynamic limit. It **needs LINUX DO sign-in on**.",
			Related:  []string{"LinuxDoOAuthLowestTrustLevel", "LinuxDoOAuthDynamicTrustLevel", "LinuxDoOAuthEnabled"},
			Keywords: []string{"LINUX DO", "trust level", "restriction"},
		},
		Setting{
			Key: "LinuxDoOAuthLowestTrustLevel", Scope: ScopeSite, Group: "auth", Card: "linuxdo-oauth", Type: TypeInt,
			Min: num(1), Max: num(4),
			Label: "Lowest LINUX DO trust level",
			Description: "The lowest LINUX DO trust level allowed in, from 1 to 4: 1 basic, 2 member, 3 regular, 4 leader. " +
				"**Only applies while the minimum trust level is on**.",
			Examples: []Example{
				{Value: "1", Effect: "Every account from basic users up gets in: only brand-new level 0 accounts are refused."},
				{Value: "2", Effect: "Members (level 2) and above get in; basic users (level 1) are refused."},
				{Value: "3", Effect: "Only regular users (level 3) and leaders (level 4) get in."},
			},
			Related:  []string{"LinuxDoOAuthTrustLevelEnabled", "LinuxDoOAuthDynamicTrustLevel"},
			Keywords: []string{"LINUX DO", "trust level"},
		},
		Setting{
			Key: "LinuxDoOAuthDynamicTrustLevel", Scope: ScopeSite, Group: "auth", Card: "linuxdo-oauth", Type: TypeBool,
			Label: "Check existing users too",
			Description: "The minimum trust level is checked on every LINUX DO sign-in, also for existing users. " +
				"An existing user **whose level is now too low is refused**. When off, only new sign-ups are checked. It **needs the minimum trust level on**.",
			Related:  []string{"LinuxDoOAuthTrustLevelEnabled", "LinuxDoOAuthLowestTrustLevel"},
			Keywords: []string{"LINUX DO", "trust level", "dynamic"},
			Advanced: true,
		},

		// ---------------------------------------------------------------- turnstile
		Setting{
			Key: "TurnstileSiteKey", Scope: ScopeSite, Group: "auth", Card: "turnstile", Type: TypeString,
			Label: "Turnstile site key",
			Description: "The site key of the widget you created in the Cloudflare Turnstile dashboard. The sign-in and " +
				"sign-up pages load the check with it. The **invisible widget type** disturbs users least.",
			Related:  []string{"TurnstileSecretKey", "TurnstileCheckEnabled"},
			Keywords: []string{"Turnstile", "Cloudflare", "captcha", "site key"},
		},
		Setting{
			Key: "TurnstileSecretKey", Scope: ScopeSite, Group: "auth", Card: "turnstile", Type: TypeSecret,
			Label: "Turnstile secret key",
			Description: "The secret key of the same Turnstile widget; the server uses it to confirm each check with " +
				"Cloudflare. It is **never shown again after saving**; leave the field empty to keep the stored one.",
			Related:  []string{"TurnstileSiteKey", "TurnstileCheckEnabled"},
			Keywords: []string{"Turnstile", "Cloudflare", "captcha", "secret key"},
		},

		// ---------------------------------------------------------------- email-domain
		Setting{
			Key: "EmailDomainRestrictionEnabled", Scope: ScopeSite, Group: "auth", Card: "email-domain", Type: TypeBool,
			Label: "Email domain restriction",
			Description: "Sign-up verification codes **only go to emails on the allowed domains**, which keeps out throwaway " +
				"mailboxes. Organizations cannot create member accounts with other emails either. It only stops password " +
				"sign-up while email verification is on, and it **cannot be turned on with an empty domain list**.",
			Related:  []string{"EmailDomainWhitelist", "EmailVerificationEnabled"},
			Keywords: []string{"allowlist", "whitelist", "disposable email", "temporary email", "domain"},
		},
		Setting{
			Key: "EmailDomainWhitelist", Scope: ScopeSite, Group: "auth", Card: "email-domain", Type: TypeList,
			Label: "Allowed email domains",
			Description: "The email domains allowed while the restriction is on, one per line (stored comma-separated). " +
				"A domain **matches exactly**: example.com does not allow mail.example.com.",
			Examples: []Example{
				{Value: "gmail.com,outlook.com", Effect: "alice@gmail.com and bob@outlook.com get codes; carol@tempmail.dev is refused."},
				{Value: "example.com", Effect: "Only company addresses like alice@example.com can sign up; alice@mail.example.com cannot."},
			},
			Related:  []string{"EmailDomainRestrictionEnabled"},
			Keywords: []string{"allowlist", "whitelist", "email domains"},
		},
	)
}
