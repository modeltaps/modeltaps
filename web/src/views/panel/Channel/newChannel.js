// Pure helpers behind the new channel sheet: keys, names, allowed / blocked models and the
// key formats made of several parts.

// The backend stores one key per channel and creates one channel per key line. A JSON
// credential (service account, OAuth) is one key even when pasted over several lines, so it is
// compacted to a single line first.
export function normalizeKeyInput(text) {
  const raw = String(text || '').trim();
  if (!raw) return '';
  if (raw.startsWith('{')) {
    try {
      return JSON.stringify(JSON.parse(raw));
    } catch {
      return raw;
    }
  }
  return raw
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean)
    .join('\n');
}

// Number of channels a key input creates.
export function countKeys(text) {
  const key = normalizeKeyInput(text);
  return key ? key.split('\n').length : 0;
}

// Masked display of a key while it is not being edited: a short prefix and the last four
// characters. Short keys only keep their first characters.
export function maskInput(key) {
  const k = String(key || '');
  if (k.length <= 12) return `${k.slice(0, 3)}…`;
  return `${k.slice(0, k.length > 20 ? 8 : 4)}…${k.slice(-4)}`;
}

// Lower-case slug of a provider name, for channel names: "My Cloud AI" -> "my-cloud-ai".
export function slugify(label) {
  const s = String(label || '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
  return s || 'channel';
}

// First free name for a provider: the slug, then slug-2, slug-3, ...
export function autoChannelName(label, existingNames = []) {
  const base = slugify(label);
  const taken = new Set(existingNames);
  if (!taken.has(base)) return base;
  let i = 2;
  while (taken.has(`${base}-${i}`)) i++;
  return `${base}-${i}`;
}

// The allowed-models wildcard is a prefix match: a single trailing '*' (gpt-4.1*).
export function isValidWildcard(pattern) {
  const p = String(pattern || '').trim();
  return p.length > 1 && p.endsWith('*') && !p.slice(0, -1).includes('*');
}

// Entry typed in wildcard mode: a trailing '*' is added when missing.
export function toWildcard(text) {
  const p = String(text || '').trim();
  if (!p) return '';
  return p.endsWith('*') ? p : `${p}*`;
}

// Whether a model name is matched by an entry: exact, or a glob where '*' matches anything.
export function matchesEntry(model, entry) {
  if (!entry.includes('*')) return model === entry;
  const re = new RegExp(
    `^${entry
      .split('*')
      .map((s) => s.replace(/[.+?^${}()|[\]\\]/g, '\\$&'))
      .join('.*')}$`
  );
  return re.test(model);
}

// Blocked models have no field of their own: they are removed from `models` and written to the
// drift ignore rules (one per line, globs kept as globs) so the drift check never adds them back.
// Returns the models left and the merged rules text.
export function applyBlockedModels(models = [], blocked = [], ignoreRules = '') {
  const block = blocked.map((b) => String(b).trim()).filter(Boolean);
  const kept = models.filter((m) => !block.some((b) => matchesEntry(m, b)));
  const lines = String(ignoreRules || '')
    .split('\n')
    .map((l) => l.trim())
    .filter(Boolean);
  for (const b of block) if (!lines.includes(b)) lines.push(b);
  return { models: kept, driftIgnoreRules: lines.join('\n') };
}

// Models to store for the allowed list. ['*'] (all models) takes the upstream list, or the
// provider's defaults when the upstream list is unknown; drift auto-add is turned on so models
// the upstream adds later are offered too. Otherwise the entries are stored as they are.
// Model mapping request names are always added so mapped names can be called.
export function resolveAllowedModels({ allow = [], upstream = [], defaults = [], mappingKeys = [] }) {
  const all = allow.includes('*');
  const base = all ? (upstream.length ? upstream : defaults) : allow;
  return { models: Array.from(new Set([...base, ...mappingKeys].filter(Boolean))), driftAutoAdd: all };
}

// Keys made of parts (a cloud region plus credentials) are one string in one of these shapes:
//   apiKey: Region|BearerToken
//   access: Region|AccessKeyID|SecretAccessKey[|SessionToken]
//   role:   Region|AccessKeyID|SecretAccessKey[|SessionToken]|RoleArn
export const KEY_PART_MODES = {
  apiKey: ['region', 'token'],
  access: ['region', 'accessKey', 'secretKey', 'sessionToken'],
  role: ['region', 'accessKey', 'secretKey', 'sessionToken', 'roleArn']
};
const KEY_PART_OPTIONAL = new Set(['sessionToken']);

export function buildPartsKey(mode, parts = {}) {
  const fields = KEY_PART_MODES[mode] || [];
  const values = fields.map((f) => String(parts[f] || '').trim());
  if (fields.some((f, i) => !KEY_PART_OPTIONAL.has(f) && !values[i])) return '';
  return values.filter((v, i) => v || !KEY_PART_OPTIONAL.has(fields[i])).join('|');
}

// Model mapping { requestName: upstreamName } exposes the request name: it takes the place of
// its upstream name in the models list (as the earlier form did), or is added when the upstream
// name is not listed. A leading '+' on the upstream name (bill by the requested model) is
// ignored for matching.
export function applyMappingToModels(models = [], mapping = {}) {
  let list = [...models];
  for (const [from, to] of Object.entries(mapping || {})) {
    const alias = String(from).trim();
    const upstream = String(to || '')
      .replace(/^\+/, '')
      .trim();
    if (!alias || list.includes(alias)) continue;
    const i = list.indexOf(upstream);
    if (i >= 0) list[i] = alias;
    else list = [...list, alias];
  }
  return list;
}
