// ==============================|| AGENT DEVICE — CLIENT RECOGNITION ||============================== //
// Which agent is asking, from what it sent (User-Agent first, then the name it gave), so the page
// can show its mark and a plain product name. Everything here is self-reported by the agent: the
// page says so and never treats a match as proof.

// Order matters where names overlap (opencode before openai-style matches).
const KNOWN_CLIENTS = [
  { match: /claude[-_ ]?(cli|code)/i, name: 'Claude Code', brandKey: 'claude' },
  { match: /codex/i, name: 'Codex', brandKey: 'codex' },
  { match: /gemini[-_ ]?cli/i, name: 'Gemini CLI', brandKey: 'geminicli' },
  { match: /copilot/i, name: 'GitHub Copilot', brandKey: 'githubcopilot' },
  { match: /qwen[-_ ]?code/i, name: 'Qwen Code', brandKey: 'qwen' },
  { match: /kimi/i, name: 'Kimi CLI', brandKey: 'kimi' },
  { match: /openclaw/i, name: 'OpenClaw', domain: 'openclaw.ai' },
  { match: /opencode/i, name: 'OpenCode', domain: 'opencode.ai' },
  { match: /cursor/i, name: 'Cursor', domain: 'cursor.com' },
  { match: /hermes/i, name: 'Hermes', brandKey: 'nousresearch' }
];

const find = (text) => (text ? KNOWN_CLIENTS.find((c) => c.match.test(text)) : undefined);

// "claude-cli/2.1.3 (external, cli)" → "2.1.3": the version of the User-Agent's first product.
export function userAgentVersion(userAgent) {
  const m = /^[^\s/]+\/v?(\d+(?:\.\d+){0,3})/.exec(String(userAgent || '').trim());
  return m ? m[1] : '';
}

// { name, brandKey?, domain?, version } for a known agent, or null.
export function recognizeClient(clientName, userAgent) {
  const byAgent = find(userAgent);
  const known = byAgent || find(clientName);
  if (!known) return null;
  return { name: known.name, brandKey: known.brandKey, domain: known.domain, version: byAgent ? userAgentVersion(userAgent) : '' };
}
