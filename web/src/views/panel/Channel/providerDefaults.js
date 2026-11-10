import { CHANNEL_OPTIONS } from 'constants/ChannelConstants';
import { defaultConfig, typeConfig } from './typeConfig';

// Address a provider uses when the channel's base URL is empty (from the backend providers).
// Types without an entry have no fixed address: the field placeholder says so instead.
export const DEFAULT_BASE_URLS = {
  1: 'https://api.openai.com',
  11: 'https://generativelanguage.googleapis.com',
  14: 'https://api.anthropic.com',
  15: 'https://aip.baidubce.com',
  16: 'https://open.bigmodel.cn/api/paas/v4',
  17: 'https://dashscope.aliyuncs.com',
  20: 'https://openrouter.ai/api',
  23: 'https://hunyuan.cloud.tencent.com',
  25: 'https://generativelanguage.googleapis.com',
  26: 'https://api.baichuan-ai.com',
  27: 'https://api.minimax.chat',
  28: 'https://api.deepseek.com',
  29: 'https://api.moonshot.cn',
  30: 'https://api.mistral.ai',
  31: 'https://api.groq.com/openai',
  33: 'https://api.lingyiwanwu.com',
  36: 'https://api.cohere.ai',
  37: 'https://api.stability.ai/v2beta',
  38: 'https://api.coze.com/open_api',
  40: 'https://hunyuan.tencentcloudapi.com',
  47: 'https://api.jina.ai',
  49: 'https://models.inference.ai.azure.com',
  53: 'https://api.klingai.com',
  56: 'https://api.x.ai',
  58: 'https://api.anthropic.com',
  59: 'https://chatgpt.com',
  62: 'https://ai-gateway.vercel.sh'
};

// Types whose base URL must be filled in (the backend has no address for them).
export const BASE_URL_REQUIRED = new Set([3, 8]);
// Types that connect to an address of the user's own: the new channel sheet shows the field.
export const BASE_URL_SHOWN = new Set([3, 8, 34, 39, 41, 54, 55]);
export const OAUTH_TYPES = new Set([57, 58, 59, 60]);

// How the credential of a type is entered: oauth (account login), json (service account),
// parts (a region and key parts joined by |) or key (one key per line).
export function credentialKind(type) {
  const id = Number(type);
  if (OAUTH_TYPES.has(id)) return 'oauth';
  if (id === 42) return 'json';
  if (id === 32) return 'parts';
  return 'key';
}

// Host part of a URL for compact display; the input itself when it does not parse.
export function hostOf(url) {
  if (!url) return '';
  try {
    return new URL(url).host;
  } catch {
    return String(url).replace(/^https?:\/\//, '');
  }
}

// Label / prompt entries of the form config for one type (type entries win over defaults).
export function typeLabels(type) {
  const cfg = typeConfig[type] || {};
  return {
    label: { ...defaultConfig.inputLabel, ...(cfg.inputLabel || {}) },
    prompt: { ...defaultConfig.prompt, ...(cfg.prompt || {}) },
    hasOther: Boolean(cfg.inputLabel?.other || cfg.prompt?.other),
    canFetchModels: Boolean(cfg.inputLabel?.provider_models_list),
    defaultModels: cfg.input?.models || [],
    testModel: cfg.input?.test_model || ''
  };
}

// Where the provider's console is, for the "get a key" link; empty when unknown.
export function consoleUrl(type) {
  return CHANNEL_OPTIONS[type]?.url || '';
}
