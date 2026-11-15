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
  62: 'https://ai-gateway.vercel.sh',
  63: 'https://router.requesty.ai',
  64: 'https://api.together.ai/v1',
  65: 'https://api.fireworks.ai/inference/v1',
  66: 'https://api.deepinfra.com/v1/openai',
  67: 'https://api.cerebras.ai/v1',
  68: 'https://api.tokenfactory.nebius.com/v1',
  69: 'https://api.novita.ai/openai/v1',
  70: 'https://api.sambanova.ai/v1',
  71: 'https://integrate.api.nvidia.com/v1',
  72: 'https://api-inference.modelscope.cn/v1',
  73: 'https://api.qnaigc.com/v1',
  74: 'https://router.huggingface.co/v1',
  75: 'https://llm.chutes.ai/v1',
  76: 'https://inference.baseten.co/v1',
  77: 'https://api.stepfun.com/v1',
  78: 'https://api.xiaomimimo.com/v1',
  79: 'https://api.longcat.chat/openai/v1',
  80: 'https://ark.cn-beijing.volces.com/api/v3',
  81: 'https://ark.ap-southeast.bytepluses.com/api/v3'
};

// Types whose upstream publishes a price list kept per channel: the gateways read their own list
// and also price the catalog, the preset platforms (64-81) are priced from models.dev.
export const CATALOG_PRICED_TYPES = new Set([20, 62, 63]);
const PRESET_TYPES = new Set(Array.from({ length: 18 }, (_, i) => 64 + i));

export function hasUpstreamPriceList(type) {
  return CATALOG_PRICED_TYPES.has(Number(type)) || PRESET_TYPES.has(Number(type));
}

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

// Host a channel sends to when it is not the type's own address (a relay or a self-hosted
// endpoint), empty otherwise: shown next to the provider name so relays of one type differ.
export function relayHost(channel) {
  const host = hostOf(String(channel?.base_url || '').trim());
  if (!host) return '';
  return host.toLowerCase() === hostOf(DEFAULT_BASE_URLS[channel.type] || '').toLowerCase() ? '' : host;
}

// Label / prompt entries of the form config for one type (type entries win over defaults).
export function typeLabels(type) {
  const cfg = typeConfig[type] || {};
  return {
    label: { ...defaultConfig.inputLabel, ...(cfg.inputLabel || {}) },
    prompt: { ...defaultConfig.prompt, ...(cfg.prompt || {}) },
    hasOther: Boolean(cfg.inputLabel?.other || cfg.prompt?.other),
    // The type reads account data (balance) with a management key of its own field.
    hasManageKey: Boolean(cfg.prompt?.manage_key),
    canFetchModels: Boolean(cfg.inputLabel?.provider_models_list),
    defaultModels: cfg.input?.models || [],
    testModel: cfg.input?.test_model || ''
  };
}

// The provider's website, for the channel menu; empty when unknown.
export function consoleUrl(type) {
  return CHANNEL_OPTIONS[type]?.url || '';
}

// The page where the provider's keys are created, for the "get a key" link; empty when the
// provider has no such page (login-based or self-hosted types).
export function keyPageUrl(type) {
  return CHANNEL_OPTIONS[type]?.keyUrl || '';
}
