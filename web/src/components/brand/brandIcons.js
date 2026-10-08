// ==============================|| BRAND KEY HELPERS ||============================== //
// Brand-key helpers for channel/vendor labels and callers that pass a `brandKey` to
// BrandIcon. Returned keys are resolved by BrandIcon against the backend manifest
// (keys or aliases, e.g. microsoftazure → azure), which also owns rendering metadata.

// Ordered brand-token rules (first substring match wins) shared by model names and
// channel display names. Tokens are lowercase; keep specific tokens before generic ones.
const BRAND_TOKEN_RULES = [
  {
    brand: 'openai',
    tokens: ['gpt', 'openai', 'chatgpt', 'davinci', 'dall-e', 'dalle', 'whisper', 'text-embedding', 'codex', 'o1-', 'o3-', 'o4-']
  },
  { brand: 'anthropic', tokens: ['claude', 'anthropic'] },
  { brand: 'googlegemini', tokens: ['gemini'] },
  { brand: 'google', tokens: ['palm', 'bison', 'gemma'] },
  { brand: 'deepseek', tokens: ['deepseek'] },
  { brand: 'alibabacloud', tokens: ['qwen', 'qwq', 'tongyi', 'alibaba'] },
  { brand: 'moonshotai', tokens: ['moonshot', 'kimi'] },
  { brand: 'mistralai', tokens: ['mistral', 'mixtral', 'codestral', 'ministral'] },
  { brand: 'meta', tokens: ['llama', 'codellama'] },
  { brand: 'x', tokens: ['grok'] },
  { brand: 'amazonwebservices', tokens: ['amazon', 'bedrock', 'titan'] },
  { brand: 'baidu', tokens: ['ernie', 'wenxin', 'baidu'] },
  { brand: 'minimax', tokens: ['minimax', 'abab'] },
  { brand: 'perplexity', tokens: ['perplexity', 'sonar'] },
  { brand: 'cloudflare', tokens: ['cloudflare'] },
  { brand: 'ollama', tokens: ['ollama'] }
];

// Channel type id (see constants/ChannelConstants.js) → brand key. Only entries with a
// verified mark are mapped; unmapped types fall back to name-token / letter resolution.
const CHANNEL_BRAND_BY_TYPE = {
  1: 'openai',
  59: 'openai',
  3: 'microsoftazure',
  55: 'microsoftazure',
  24: 'microsoftazure',
  54: 'microsoftazure',
  14: 'anthropic',
  58: 'anthropic',
  11: 'google',
  25: 'googlegemini',
  57: 'googlegemini',
  60: 'googlegemini',
  42: 'googlecloud',
  61: 'googlecloud',
  15: 'baidu',
  17: 'alibabacloud',
  27: 'minimax',
  28: 'deepseek',
  29: 'moonshotai',
  30: 'mistralai',
  32: 'amazonwebservices',
  35: 'cloudflare',
  38: 'coze',
  39: 'ollama',
  41: 'suno',
  49: 'github',
  52: 'replicate',
  56: 'x',
  20: 'openrouter'
};

// Resolve a brand key from free text (model name or channel name). Returns null when no
// token matches, letting the caller render a first-letter placeholder.
export function resolveBrandFromText(text) {
  if (!text) return null;
  const value = String(text).toLowerCase();
  for (const rule of BRAND_TOKEN_RULES) {
    if (rule.tokens.some((token) => value.includes(token))) return rule.brand;
  }
  return null;
}

// Resolve a brand key from a channel type id, falling back to null for unmapped types.
export function resolveChannelBrand(type) {
  const num = Number(type);
  if (!num || Number.isNaN(num)) return null;
  return CHANNEL_BRAND_BY_TYPE[num] || null;
}

// Human-readable brand names for resolved brand keys. Proper-noun brand names render
// literally (identical across locales), matching the "Works with" strip convention.
const BRAND_LABELS = {
  openai: 'OpenAI',
  anthropic: 'Anthropic',
  googlegemini: 'Google Gemini',
  google: 'Google',
  googlecloud: 'Google Cloud',
  deepseek: 'DeepSeek',
  alibabacloud: 'Alibaba Cloud',
  moonshotai: 'Moonshot AI',
  mistralai: 'Mistral AI',
  meta: 'Meta',
  x: 'xAI',
  amazonwebservices: 'AWS',
  baidu: 'Baidu',
  minimax: 'MiniMax',
  perplexity: 'Perplexity',
  ollama: 'Ollama',
  microsoftazure: 'Azure',
  cloudflare: 'Cloudflare',
  coze: 'Coze',
  suno: 'Suno',
  github: 'GitHub',
  replicate: 'Replicate',
  huggingface: 'Hugging Face',
  openrouter: 'OpenRouter'
};

// Resolve a provider brand display name from a channel type id (authoritative), falling
// back to the channel display name's brand token, then to null so the caller can degrade.
// Shares brand-key resolution with the icon so name and mark stay in lockstep.
export function resolveBrandLabel(type, name) {
  const key = resolveChannelBrand(type) || resolveBrandFromText(name);
  return key ? BRAND_LABELS[key] || null : null;
}
