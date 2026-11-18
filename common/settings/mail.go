package settings

// Settings page group "mail". The SMTP* options mirror the mail service in use (model/mail_service.go):
// they change with it on the Mail services card, and the option API refuses direct changes.
func init() {
	add(
		Setting{
			Key: "SMTPProvider", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeString,
			Label: "Mail service preset",
			Description: "Which preset the mail service in use was set up from, such as resend, ses, gmail or custom. " +
				"It only shapes the console form; sending reads the server, port and account. Like every SMTP setting it " +
				"mirrors the mail service in use and changes with it.",
			Related:  []string{"SMTPServer"},
			Keywords: []string{"mail provider", "email provider", "smtp preset"},
			Advanced: true,
			Docs:     "deployment/env.md#mail-sending",
		},
		Setting{
			Key: "SMTPServer", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeString,
			Label: "SMTP server",
			Description: "The host name of the SMTP server every email is sent through. It mirrors the mail service in use: " +
				"add, edit or switch services on the Mail services card. Empty means the site cannot send mail, so login " +
				"codes, password resets and quota warnings stop. When the deployment sets smtp.url (SMTP_URL), that wins and " +
				"the console is read-only.",
			Examples: []Example{
				{Value: "smtp.resend.com", Effect: "Mail goes out through Resend."},
			},
			Related:  []string{"SMTPPort", "SMTPTLSMode", "SMTPAccount"},
			Keywords: []string{"smtp host", "mail server", "email server", "outgoing mail"},
			Docs:     "deployment/env.md#mail-sending",
		},
		Setting{
			Key: "SMTPPort", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeInt, Min: num(1), Max: num(65535),
			Label: "SMTP port",
			Description: "The port of the SMTP server: usually 465 for SSL/TLS and 587 for STARTTLS. With encryption Auto the " +
				"port decides the encryption, and port 587 logs in with LOGIN instead of PLAIN. It mirrors the mail service " +
				"in use.",
			Examples: []Example{
				{Value: "465", Effect: "With Auto, the connection is TLS from the first byte."},
				{Value: "587", Effect: "With Auto, the connection must upgrade with STARTTLS and logs in with LOGIN."},
				{Value: "25", Effect: "With Auto, STARTTLS is still required; pick STARTTLS if offered for a relay without TLS."},
			},
			Related:  []string{"SMTPTLSMode", "SMTPServer"},
			Keywords: []string{"port", "465", "587"},
		},
		Setting{
			Key: "SMTPAccount", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeString,
			Label: "SMTP username",
			Description: "The user name the mail service logs in to the SMTP server with; for a mailbox it is the mailbox " +
				"address. When no sender address is set, mail is sent from it. It mirrors the mail service in use.",
			Related:  []string{"SMTPToken", "SMTPFrom"},
			Keywords: []string{"smtp user", "login", "account", "mailbox"},
		},
		Setting{
			Key: "SMTPFrom", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeString,
			Label: "Sender address",
			Description: "The address every email comes from. It must be an address the service lets you send from: one on " +
				"a domain you verified with it, or the mailbox itself. It mirrors the mail service in use.",
			Examples: []Example{
				{Value: "noreply@example.com", Effect: "Users get mail from noreply@example.com; example.com must be verified with the service."},
			},
			Related:  []string{"SMTPFromName", "SMTPReplyTo"},
			Keywords: []string{"from address", "sender", "noreply", "mail from"},
		},
		Setting{
			Key: "SMTPFromName", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeString,
			Label: "Sender name",
			Description: "The display name shown before the sender address. Empty shows the address alone. It mirrors the " +
				"mail service in use.",
			Examples: []Example{
				{Value: "Example AI", Effect: "Mail comes from Example AI <noreply@example.com>."},
			},
			Related:  []string{"SMTPFrom", "SystemName"},
			Keywords: []string{"from name", "display name", "sender"},
		},
		Setting{
			Key: "SMTPReplyTo", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeString,
			Label: "Reply-to address",
			Description: "Where replies to the site's emails go instead of the sender address. Empty sends replies to the " +
				"sender address. It mirrors the mail service in use.",
			Examples: []Example{
				{Value: "support@example.com", Effect: "A user who replies to a login code email writes to support@example.com."},
			},
			Related:  []string{"SMTPFrom"},
			Keywords: []string{"reply to", "support address"},
		},
		Setting{
			Key: "SMTPToken", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeSecret,
			Label: "SMTP password",
			Description: "The password, app password or API key the mail service logs in with. It is never shown again after " +
				"saving and is encrypted in the database when ENCRYPTION_KEY is set. It mirrors the mail service in use.",
			Related:  []string{"SMTPAccount"},
			Keywords: []string{"smtp password", "api key", "app password", "authorization code", "credentials"},
		},
		Setting{
			Key: "SMTPTLSMode", Scope: ScopeSite, Group: "mail", Card: "smtp", Type: TypeEnum,
			Enum: []EnumValue{
				{Value: "auto", Label: "Auto (by port)", Description: "SSL/TLS on port 465, required STARTTLS on any other port."},
				{Value: "ssl", Label: "SSL/TLS", Description: "TLS from the first byte, usually on port 465."},
				{Value: "starttls", Label: "STARTTLS (required)", Description: "Upgrades to TLS after connecting; sending fails when the server cannot."},
				{Value: "starttls_opportunistic", Label: "STARTTLS if offered", Description: "Upgrades when the server offers it, otherwise sends in plain text."},
				{Value: "none", Label: "None", Description: "Plain text, for a relay on a trusted local network or for development."},
			},
			Label: "Encryption",
			Description: "How the connection to the SMTP server is encrypted. Auto picks by port and suits almost every " +
				"service. It mirrors the mail service in use.",
			Related:  []string{"SMTPPort"},
			Keywords: []string{"tls", "ssl", "starttls", "encryption", "secure connection"},
		},
		Setting{
			Key: "MailTemplates", Scope: ScopeSite, Group: "mail", Card: "mail-templates", Type: TypeJSON,
			Label: "Email templates",
			Description: "Your own subject and body for each email users get, per console language. The emails are the " +
				"verification code, password reset, low balance and quota used up. Stored as JSON {\"<template>\": {\"<language>\": " +
				"{\"subject\", \"body\"}}}; a template or language left out, or an empty subject or body, uses the built-in " +
				"text of that language. Variables are a name in double curly braces, HTML-escaped in the body; the server adds the " +
				"shared header with the logo and the footer. Unknown templates or languages are refused on save.",
			Examples: []Example{
				{Value: `{"verification":{"en_US":{"subject":"Your {{site}} code","body":""}}}`, Effect: "English verification emails get the new subject and keep the built-in body; every other email stays built-in."},
			},
			Related:  []string{"DefaultLanguage", "SystemName", "Logo", "QuotaRemindEnabled"},
			Keywords: []string{"email template", "mail content", "verification email", "password reset email", "quota warning"},
		},
	)
}
