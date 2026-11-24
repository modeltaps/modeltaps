package controller

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/modeltaps/modeltaps/common/config"

	"github.com/gin-gonic/gin"
)

// /skill.md (AGENT-5) tells an open agent (one that takes third-party keys) how to connect itself to
// this gateway: get an API key without it passing through the chat (pairing code or device flow,
// AGENT-7), keep it, verify it and configure the provider. There is one file for the whole site,
// rendered from site settings only (name, public address, enabled protocols): it never holds user
// data or a key, so it can be cached, copied and shared. English only.

// siteDisplayName is the configured system name on one line, or the default brand.
func siteDisplayName() string {
	name := strings.Join(strings.Fields(config.SystemName), " ")
	if name == "" {
		return "Modeltaps"
	}
	return name
}

// skillSlug is the skill's name: the system name in lowercase letters, digits and dashes.
func skillSlug(systemName string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(systemName) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
			dash = false
		} else if b.Len() > 0 && !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "modeltaps"
	}
	return slug
}

// yamlQuote is s as a double-quoted YAML scalar.
func yamlQuote(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// SkillMd serves /skill.md.
func SkillMd(c *gin.Context) {
	c.Data(http.StatusOK, "text/markdown; charset=utf-8", []byte(renderSkillMd(publicSiteURL(c))))
}

func renderSkillMd(base string) string {
	name := siteDisplayName()
	slug := skillSlug(name)
	var b strings.Builder
	w := func(format string, args ...any) { fmt.Fprintf(&b, format, args...) }

	// 1. Frontmatter.
	w("---\n")
	w("name: %s\n", slug)
	w("description: %s\n", yamlQuote(fmt.Sprintf("Connect this agent to %s (%s), a multi-provider AI gateway, and use it as a model provider. "+
		"Use when the user asks to set up, connect or add %s, or to use models through it.", name, base, name)))
	w("compatibility: %s\n", yamlQuote(fmt.Sprintf("Needs an API key from %s (a call key, sk-...). Management keys (mk-...) cannot call models and are never handed to agents.", base)))
	w("---\n\n")

	w("# %s\n\n", name)

	// 2. Who we are.
	w("## 1. What %s is\n\n", name)
	w("%s is a multi-provider AI gateway at %s. One API key reaches the models of many providers through ", name, base)
	if config.ClaudeAPIEnabled {
		w("two protocols: the OpenAI API and the Anthropic (Claude) API.")
	} else {
		w("the OpenAI API.")
	}
	w(" Requests are billed to the user's %s account and limited by the key's own spending limit, expiry and model allow-list.\n\n", name)

	// 3. Base URLs.
	w("## 2. Base URLs\n\n")
	w("| Protocol | Base URL | Authentication |\n|---|---|---|\n")
	w("| OpenAI-compatible | `%s/v1` | `Authorization: Bearer <API key>` |\n", base)
	if config.ClaudeAPIEnabled {
		w("| Anthropic-compatible | `%s/claude` | `x-api-key: <API key>` or `Authorization: Bearer <API key>` |\n", base)
	}
	w("\n")
	if config.ClaudeAPIEnabled {
		w("Anthropic SDKs and clients append `/v1/messages` to the base URL themselves; give them `%s/claude`, not the full path.\n\n", base)
	}
	// Client-specific endpoints (AGENT-1): same behavior as the bare paths, and the console can
	// show usage per tool.
	w("Coding agents have endpoints of their own, with the same keys, models and prices; using them lets the user see usage per tool:\n\n")
	if config.ClaudeAPIEnabled {
		w("- Claude Code: `ANTHROPIC_BASE_URL=%s/claude-code` (model IDs are listed as `claude-code/<model>`).\n", base)
	}
	w("- Codex: a provider with `base_url = \"%s/codex/v1\"` and `wire_api = \"responses\"`.\n", base)
	w("- Other coding agents: `%s/coding-agent/v1` (OpenAI-compatible)", base)
	if config.ClaudeAPIEnabled {
		w(" or `%s/coding-agent` (Anthropic-compatible)", base)
	}
	w(".\n\n")

	// 4. Getting a key.
	w("## 3. Get an API key\n\n")
	w("Never ask the user to paste a key into the chat: conversations are stored, synced and may end up in memory. ")
	w("Use one of these, in this order:\n\n")

	w("### A. The user gave you a pairing code\n\n")
	w("A pairing code looks like `pair-` followed by 40 letters and digits. The console shows one when the user creates a key, ")
	w("and the key's details can issue a new one. It is valid for 10 minutes, works once, and only for that key.\n\n")
	w("```http\nPOST %s/api/agent/pair\nContent-Type: application/json\n\n{\"code\": \"<pairing code>\"}\n```\n\n", base)
	w("On success (`200`) the response holds the key in `access_token` (plus `key_name`, `key_hint`, `expires_at` in Unix seconds, `-1` for never, ")
	w("and `account`, the personal account or organization the key belongs to; name it to the user). ")
	w("Store it right away (section 4) and do not print it. Redeeming gives the key a fresh secret: from then on only you hold a working copy.\n\n")
	w("Errors come as `{\"error\": \"<code>\", \"error_description\": \"...\"}`:\n\n")
	w("- `pairing_code_expired` or `pairing_code_used`: ask the user to issue a new pairing code in the key's details.\n")
	w("- `pairing_code_invalid`: the code is wrong or void. Check it was copied completely; five wrong tries void a code.\n")
	w("- `pairing_key_unavailable`: the key is disabled, expired or deleted.\n")
	w("- HTTP `429`: wait for `Retry-After` seconds before trying again.\n\n")

	w("### B. The user has no key yet: device authorization\n\n")
	w("1. Start a request, naming yourself so the user recognizes it:\n\n")
	w("   ```http\n   POST %s/api/agent/device\n   Content-Type: application/json\n\n   {\"client_name\": \"<your agent name> on <host>\"}\n   ```\n\n", base)
	w("   The response has `device_code`, `user_code`, `verification_uri`, `verification_uri_complete`, `expires_in` and `interval` (seconds).\n")
	w("2. Ask the user to open `verification_uri_complete` (or `verification_uri` and enter `user_code`), sign in, check that the request shows your name, ")
	w("choose the key's spending limit, expiry and models, and approve. Keep `device_code` to yourself.\n")
	w("3. Poll every `interval` seconds:\n\n")
	w("   ```http\n   POST %s/api/agent/device/token\n   Content-Type: application/json\n\n   {\"grant_type\": \"urn:ietf:params:oauth:grant-type:device_code\", \"device_code\": \"<device_code>\"}\n   ```\n\n", base)
	w("   - `authorization_pending`: the user has not answered yet; keep polling.\n")
	w("   - `slow_down`: add 5 seconds to the interval, then keep polling.\n")
	w("   - `access_denied`: the user declined; stop.\n")
	w("   - `expired_token`: the request expired; start again from step 1 if the user still wants to.\n")
	w("   - `200` with `access_token`: that is the key; store it (section 4). The device code works only once.\n")
	w("   The response also has `account` (`type` is `personal` or `organization`, with its `name`). Tell the user which account approved the key; ")
	w("if it is not theirs, someone else used the code first: stop, and ask them to delete that key in the console and start again.\n\n")

	w("### C. Neither works\n\n")
	w("Ask the user to create an API key in the console (%s, API Keys) and to enter it through your own credential storage: ", base)
	w("a secret prompt, a command that reads the key from standard input, or the system keychain's own dialog. Never through the chat.\n\n")

	// 5. Key custody.
	w("## 4. Keep the key safe\n\n")
	w("How to store the key is your decision; pick the first of these your environment offers:\n\n")
	w("1. Your own credential store or secret reference (for example a secret store, a secret reference in your configuration, a password manager integration).\n")
	w("2. The operating system's keychain: macOS Keychain, Windows Credential Manager, or the Secret Service on Linux.\n")
	w("3. Only if neither exists: a file readable by the user alone (permissions `0600`), outside any repository or synced folder.\n\n")
	w("Configuration files then refer to the stored secret instead of holding its value. Never:\n\n")
	w("- write the key in plain text into a file that is committed or synced;\n")
	w("- write it into memory, notes, summaries or conversation history;\n")
	w("- show it in your output, logs or tool results (refer to it by the key name or `key_hint`);\n")
	w("- ask the user to paste a key into the chat.\n\n")
	w("If the key was exposed anyway, tell the user so they can reset it in the console.\n\n")

	// 6. Verification.
	w("## 5. Verify\n\n")
	w("1. `GET %s/v1/dashboard/billing/subscription` with `Authorization: Bearer <API key>` answers `200` with the key's limit (`hard_limit_usd`). ", base)
	w("`401` means the key is wrong, disabled or expired.\n")
	w("2. `GET %s/v1/models` with the same header lists the model IDs this key may call.\n\n", base)

	// 7. Provider configuration.
	w("## 6. Configure the provider\n\n")
	w("Register %s in your own model configuration, the place your runtime reads providers from (its config command, settings file or provider registry), so you and the user can pick its models. A note or a file of your own that nothing reads does not count. Point the provider's key at where you stored it in step 4 instead of copying the key in.\n\n", name)
	w("- Add %s as an OpenAI-compatible provider with base URL `%s/v1`", name, base)
	if config.ClaudeAPIEnabled {
		w(", and as an Anthropic-compatible provider with base URL `%s/claude` where you need that protocol", base)
	}
	w(".\n")
	w("- Take model IDs from `GET %s/v1/models` at run time; never hard-code them. The list depends on the key and changes over time.\n", base)
	w("- When a model the user asks for is missing, fetch the list again and offer the closest available ones instead of guessing.\n")
	w("- Finish with one short request through the configured provider (a cheap model, a one-word reply) to prove it works end to end.\n\n")

	// 8. Wrap-up.
	w("## 7. Report back to the user\n\n")
	w("When done, tell the user:\n\n")
	w("- which files or settings you changed;\n")
	w("- where the key is stored (the location, never the value) and under which key name;\n")
	w("- which models you configured;\n")
	w("- how to revoke it: in the %s console, API Keys, open the key, then disable, reset or delete it.\n\n", name)

	if docs := strings.TrimSpace(config.DocsLink); strings.HasPrefix(docs, "https://") || strings.HasPrefix(docs, "http://") {
		w("More: [%s documentation](%s), and `%s/llms.txt`.\n\n", name, docs, base)
	} else {
		w("More: `%s/llms.txt`.\n\n", base)
	}

	// Per-agent notes (AGENT-8): the same facts the console's "Connect a tool" dialog uses
	// (web/src/views/panel/Token/connect/tools.js); change both together.
	w("## Appendix: known agents\n\n")
	w("Where you are one of these agents, configure it this way; it keeps prompt caching working and lets the user see usage per tool.\n\n")
	if config.ClaudeAPIEnabled {
		w("- Claude Code: in `~/.claude/settings.json` under `env`, set `ANTHROPIC_BASE_URL` to `%s/claude-code` (no `/v1`), `ANTHROPIC_AUTH_TOKEN` to the key, `ANTHROPIC_API_KEY` to an empty string and `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY` to `1`. Check with `/model`.\n", base)
	}
	w("- Codex: in `~/.codex/config.toml`, a provider with `base_url = \"%s/codex/v1\"`, `wire_api = \"responses\"`, `model_catalog_url = \"%s/codex/v1/models\"` and `env_key` naming the variable that holds the key. Check with `/model`.\n", base, base)
	if config.ClaudeAPIEnabled {
		w("- Goose: for Claude models use the built-in `anthropic` provider with `ANTHROPIC_HOST` set to `%s/coding-agent` (no `/v1`); it adds prompt cache markers itself. ", base)
		w("For other models add a custom provider in `~/.config/goose/custom_providers/` with `engine: openai` and `base_url` `%s/coding-agent/v1/chat/completions`. Store the key through `goose configure` (system keychain).\n", base)
		w("- OpenCode: a provider in `opencode.json` with `options.baseURL` `%s/coding-agent/v1`; use the `@ai-sdk/anthropic` package for Claude models (so prompt caching works) and `@ai-sdk/openai-compatible` for others.\n", base)
	} else {
		w("- Goose: a custom provider in `~/.config/goose/custom_providers/` with `engine: openai` and `base_url` `%s/coding-agent/v1/chat/completions`. Store the key through `goose configure` (system keychain).\n", base)
		w("- OpenCode: a provider in `opencode.json` with `npm` `@ai-sdk/openai-compatible` and `options.baseURL` `%s/coding-agent/v1`.\n", base)
	}
	w("- OpenClaw: an OpenAI-compatible provider with base URL `%s/v1` and `api: openai-completions`.\n\n", base)

	// Appendix.
	w("## Appendix: reference commands\n\n")
	w("For reference only; prefer your own credential mechanism. Each reads the key from a prompt or standard input, so it does not end up in shell history or process lists.\n\n")
	w("- macOS Keychain: store with `security add-generic-password -U -s %s -a api-key -w` (prompts for the key), read with `security find-generic-password -s %s -a api-key -w`.\n", slug, slug)
	w("- Linux Secret Service: store with `secret-tool store --label='%s API key' service %s account api-key` (reads standard input), read with `secret-tool lookup service %s account api-key`.\n", name, slug, slug)
	w("- Windows (PowerShell SecretManagement): store with `Set-Secret -Name %s-api-key` (prompts), read with `Get-Secret -Name %s-api-key -AsPlainText`.\n", slug, slug)
	w("- File: `mkdir -p -m 700 ~/.config/%s && install -m 600 /dev/null ~/.config/%s/api-key`, then write the key into it from your process, not from a command line.\n", slug, slug)
	return b.String()
}
