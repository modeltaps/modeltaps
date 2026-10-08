import Decimal from 'decimal.js';

// ==============================|| LOG — HELPERS ||============================== //
// Ported from the v1 Log view (type/LogType.js + component/*). Pure logic + small
// presentational helpers so the table/detail components stay lean.

// log_type → { i18n text key, badge color token }. Mirrors v1 useLogType().
export const LOG_TYPES = [
  { value: '0', labelKey: 'logPage.logType.all', color: 'default' },
  { value: '1', labelKey: 'logPage.logType.recharge', color: 'primary' },
  { value: '2', labelKey: 'logPage.logType.consumption', color: 'orange' },
  { value: '3', labelKey: 'logPage.logType.management', color: 'info' },
  { value: '4', labelKey: 'logPage.logType.system', color: 'secondary' }
];

// 账务记录视图的类型子筛选;'0' 表示全部账务(充值/管理/系统,客户端合并 1/3/4)。
export const BILLING_LOG_TYPES = [
  { value: '0', labelKey: 'logPage.logType.allBilling' },
  { value: '1', labelKey: 'logPage.logType.recharge' },
  { value: '3', labelKey: 'logPage.logType.management' },
  { value: '4', labelKey: 'logPage.logType.system' }
];

// color token → Tailwind classes for soft badges.
const BADGE_CLASSES = {
  default: 'bg-muted text-foreground',
  primary: 'bg-muted text-foreground',
  orange: 'bg-orange-500/10 text-orange-600 dark:text-orange-400',
  info: 'bg-blue-500/10 text-blue-600 dark:text-blue-400',
  secondary: 'bg-muted text-muted-foreground',
  success: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400',
  error: 'bg-destructive/10 text-destructive'
};

export const badgeClass = (color) =>
  `inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium ${BADGE_CLASSES[color] || BADGE_CLASSES.default}`;

export function logTypeMeta(type) {
  return LOG_TYPES.find((o) => o.value === String(type)) || null;
}

// metadata.finish_reason(W8-B 归一值)→ { i18n text key, badge color token }。
// 值域固定为 stop|length|content_filter|tool_calls|error;筛选下拉也复用本列表。
export const FINISH_REASONS = [
  { value: 'stop', labelKey: 'logPage.finishReason.stop', color: 'success' },
  { value: 'length', labelKey: 'logPage.finishReason.length', color: 'orange' },
  { value: 'content_filter', labelKey: 'logPage.finishReason.contentFilter', color: 'error' },
  { value: 'tool_calls', labelKey: 'logPage.finishReason.toolCalls', color: 'info' },
  { value: 'error', labelKey: 'logPage.finishReason.error', color: 'error' }
];

// finish_reason → 展示元信息。旧日志(id≤39)与非 OpenAI 兼容 provider 无该字段:
// 传入空/未知值时返回 null,调用方据此不渲染 badge(容错,不报错)。
export function finishReasonMeta(reason) {
  if (!reason) return null;
  return FINISH_REASONS.find((o) => o.value === reason) || null;
}

// metadata.relay_mode(后端由请求路径判定的稳定标签)→ { i18n text key, badge color token }。
// 颜色按模态归类(文本=success,图像=info,音频=warning→orange,视频=secondary),筛选下拉复用本列表。
export const RELAY_MODES = [
  { value: 'chat_completions', labelKey: 'logPage.relayMode.chatCompletions', color: 'success' },
  { value: 'completions', labelKey: 'logPage.relayMode.completions', color: 'success' },
  { value: 'responses', labelKey: 'logPage.relayMode.responses', color: 'success' },
  { value: 'embeddings', labelKey: 'logPage.relayMode.embeddings', color: 'primary' },
  { value: 'rerank', labelKey: 'logPage.relayMode.rerank', color: 'primary' },
  { value: 'moderations', labelKey: 'logPage.relayMode.moderations', color: 'primary' },
  { value: 'image_generations', labelKey: 'logPage.relayMode.imageGenerations', color: 'info' },
  { value: 'image_edits', labelKey: 'logPage.relayMode.imageEdits', color: 'info' },
  { value: 'image_variations', labelKey: 'logPage.relayMode.imageVariations', color: 'info' },
  { value: 'audio_speech', labelKey: 'logPage.relayMode.audioSpeech', color: 'orange' },
  { value: 'audio_transcription', labelKey: 'logPage.relayMode.audioTranscription', color: 'orange' },
  { value: 'audio_translation', labelKey: 'logPage.relayMode.audioTranslation', color: 'orange' },
  { value: 'realtime', labelKey: 'logPage.relayMode.realtime', color: 'orange' },
  { value: 'video', labelKey: 'logPage.relayMode.video', color: 'secondary' }
];

// relay_mode → 展示元信息。旧日志与判不出模态的请求无该字段:传入空/未知值时返回 null,
// 调用方据此渲染占位符(容错,不报错)。
export function relayModeMeta(mode) {
  if (!mode) return null;
  return RELAY_MODES.find((o) => o.value === mode) || null;
}

// request_time (ms) → seconds color token. Mirrors v1 requestTimeLabelOptions.
export function requestTimeColor(requestTime) {
  if (requestTime === 0) return 'default';
  if (requestTime <= 10) return 'success';
  if (requestTime <= 50) return 'primary';
  if (requestTime <= 100) return 'secondary';
  return 'error';
}

export function requestTsColor(requestTs) {
  if (requestTs === 0) return 'default';
  if (requestTs <= 10) return 'error';
  if (requestTs <= 15) return 'secondary';
  if (requestTs <= 20) return 'primary';
  return 'success';
}

// Derive duration display strings (total / first-token / tokens-per-second).
export function deriveDuration(item) {
  const requestTime = item.request_time / 1000;
  const requestTimeStr = `${requestTime.toFixed(2)} S`;
  const firstTime = item.metadata?.first_response ? item.metadata.first_response / 1000 : 0;
  const firstTimeStr = firstTime ? `${firstTime.toFixed(2)} S` : '';
  const streamTime = requestTime - firstTime;

  let requestTs = 0;
  let requestTsStr = '';
  // streamTime > 0 守卫:firstTime === requestTime 时 streamTime 为 0 会算出 Infinity,
  // 置 0 与调用方(详情弹窗/吞吐列)语义一致 → 显示占位符而非 Infinity。
  if (firstTime > 0 && streamTime > 0 && item.completion_tokens > 0) {
    requestTs = item.completion_tokens / streamTime;
    requestTsStr = `${requestTs.toFixed(2)} t/s`;
  }
  return { requestTime, requestTimeStr, firstTimeStr, requestTs, requestTsStr };
}

// Compute display token totals incl. ratio-adjusted modality tokens. Ported verbatim
// from v1 TableRow.calculateTokens (used by the detail sheet).
export function calculateTokens(item) {
  const { prompt_tokens, completion_tokens, metadata } = item;
  if (prompt_tokens === undefined || prompt_tokens === null || !metadata) {
    return {
      totalInputTokens: prompt_tokens || 0,
      totalOutputTokens: completion_tokens || 0,
      show: false,
      tokenDetails: []
    };
  }

  let totalInputTokens = prompt_tokens;
  let totalOutputTokens = completion_tokens;
  let show = false;

  const ratios = {
    input_text_tokens: metadata?.input_text_tokens_ratio || 1,
    output_text_tokens: metadata?.output_text_tokens_ratio || 1,
    input_audio_tokens: metadata?.input_audio_tokens_ratio || 1,
    output_audio_tokens: metadata?.output_audio_tokens_ratio || 1,
    cached_tokens: metadata?.cached_tokens_ratio || 1,
    cached_write_tokens: metadata?.cached_write_tokens_ratio || 1,
    cached_write_1h_tokens: metadata?.cached_write_1h_tokens_ratio || 1,
    cached_read_tokens: metadata?.cached_read_tokens_ratio || 1,
    openai_cache_write_tokens: metadata?.openai_cache_write_tokens_ratio || 1,
    reasoning_tokens: metadata?.reasoning_tokens_ratio || 1,
    input_image_tokens: metadata?.input_image_tokens_ratio || 1,
    output_image_tokens: metadata?.output_image_tokens_ratio || 1
  };

  const labels = {
    input_text_tokens: 'logPage.inputTextTokens',
    output_text_tokens: 'logPage.outputTextTokens',
    input_audio_tokens: 'logPage.inputAudioTokens',
    output_audio_tokens: 'logPage.outputAudioTokens',
    cached_tokens: 'logPage.cachedTokens',
    cached_write_tokens: 'logPage.cachedWriteTokens',
    cached_write_1h_tokens: 'logPage.cachedWrite1hTokens',
    cached_read_tokens: 'logPage.cachedReadTokens',
    openai_cache_write_tokens: 'logPage.openaiCacheWriteTokens',
    reasoning_tokens: 'logPage.reasoningTokens',
    input_image_tokens: 'logPage.inputImageTokens',
    output_image_tokens: 'logPage.outputImageTokens'
  };

  const inputKeys = [
    'input_text_tokens',
    'output_text_tokens',
    'input_audio_tokens',
    'cached_tokens',
    'cached_write_tokens',
    'cached_write_1h_tokens',
    'cached_read_tokens',
    'openai_cache_write_tokens',
    'input_image_tokens'
  ];
  const outputKeys = ['output_audio_tokens', 'reasoning_tokens', 'output_image_tokens'];

  const tokenDetails = Object.keys(ratios)
    .filter((key) => metadata[key] > 0)
    .map((key) => {
      const rate = ratios[key];
      const tokens = Math.ceil(metadata[key] * (rate - 1));
      if (inputKeys.includes(key)) {
        totalInputTokens += tokens;
        show = true;
      } else if (outputKeys.includes(key)) {
        totalOutputTokens += tokens;
        show = true;
      }
      return { key, label: labels[key], tokens, value: metadata[key], rate, labelParams: { ratio: rate } };
    });

  return { totalInputTokens, totalOutputTokens, show, tokenDetails };
}

// Format a Decimal price string trimming trailing zeros. Ported from v1 calculatePrice.
export function calculatePrice(ratio, groupDiscount, isTimes) {
  let discount = new Decimal(ratio || 0).mul(groupDiscount || 0);
  if (!isTimes) discount = discount.mul(1000);
  const priceString = discount.mul(0.002).toFixed(6);
  return priceString.replace(/(\.\d*?[1-9])0+$|\.0*$/, '$1');
}

// original quota = actual quota / group ratio (fallback to metadata). Ported from v1.
export function calculateOriginalQuota(item) {
  if (!item?.quota || !item?.metadata?.group_ratio) {
    return item.metadata?.original_quota || item.metadata?.origin_quota || 0;
  }
  const groupRatio = item.metadata?.group_ratio || 1;
  if (groupRatio === 0) return item.quota;
  return item.quota / groupRatio || item.metadata?.original_quota || item.metadata?.origin_quota || 0;
}
