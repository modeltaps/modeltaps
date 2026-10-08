import i18n from 'i18n/i18n';
import { matchesRule } from 'views/panel/ApiCatalog/availability';

// ==============================|| PLAYGROUND — FIELD SCHEMA ||============================== //
// 运行设置栏的字段描述与求值：每个模态给一份 schema，设置栏由 FieldRenderer 按 schema 渲染，
// 请求体由 fieldParams 按同一份 schema 拼装——「画出来的字段」与「发出去的字段」同源。
// 字段形状：{ key, group, type, labelKey, options?, optionLabelKey?, min?, max?, step?, default?,
//            hintKey?, omitDefault?, showWhen?(model, values) }
// options / default 可以是值，也可以是 (model) => 值：不同模型家族的取值集合不同。
// 不被所选模型支持的字段既不渲染也不进请求体——多传一个字段会被上游整单拒掉。
// 本文件属 shared 层：只放纯函数与常量，不 import 任何 React 组件。

export const FIELD_TYPES = ['select', 'range', 'number', 'text', 'switch'];

// 四条模态规则与目录页同源（endpoints 词表见 model/model_endpoints.go）。
export const CHAT_RULE = { endpoints: ['chat'] };
export const IMAGE_RULE = { endpoints: ['images'] };
export const TTS_RULE = { endpoints: ['audio.speech'] };
export const STT_RULE = { endpoints: ['audio.transcription'] };

// 对话出图：声明 chat 接口且输出模态含 image 的模型，与目录页「图像生成 · Chat 写法」同源。
// 图像页候选取两条规则的并集；同时声明 images 的模型仍按专用 images 模型处理。
export const CHAT_IMAGE_RULE = { endpoints: ['chat'], outputModalities: ['image'] };
export const IMAGE_RULES = [IMAGE_RULE, CHAT_IMAGE_RULE];

export const isChatImageModel = (model) => matchesRule(model, CHAT_IMAGE_RULE) && !matchesRule(model, IMAGE_RULE);

// 目录里没有 image-to-image 这个词，等价信号是「images 接口 + 输入模态含 image」；
// 这类模型才画参考图上传区，并改走 /v1/images/edits。对话出图模型输入模态含 image 时，
// 参考图作为 image_url 内容块放进用户消息。
export const supportsImageToImage = (model) =>
  matchesRule(model, { endpoints: ['images'], inputModalities: ['image'] }) ||
  (isChatImageModel(model) && matchesRule(model, { inputModalities: ['image'] }));

// Images API 专用字段对对话出图模型一律隐藏：既不渲染也不进请求体。
const imagesApiOnly = (model) => !isChatImageModel(model);

export const IMAGE_SIZES = ['1024x1024', '1024x1536', '1536x1024', '1024x1792', '1792x1024', '512x512', '256x256'];
export const TTS_FORMATS = ['mp3', 'wav', 'opus', 'aac', 'flac', 'pcm'];
// 只给 json / verbose_json：text、srt、vtt 回的不是 JSON，结果区拿不到结构化分段。
export const STT_FORMATS = ['json', 'verbose_json'];

// 模型家族只看模型名前缀：gpt-image 与 dall-e-3 各自支持的可选参数不同，
// 其余（dall-e-2、第三方生图模型）一律只发通用字段。
export function imageModelFamily(model) {
  const id = String((typeof model === 'string' ? model : model?.id) || '').toLowerCase();
  if (id.startsWith('gpt-image')) return 'gpt-image';
  if (id.startsWith('dall-e-3')) return 'dall-e-3';
  return 'generic';
}

export function imageQualities(model) {
  const family = imageModelFamily(model);
  if (family === 'gpt-image') return ['auto', 'low', 'medium', 'high'];
  if (family === 'dall-e-3') return ['standard', 'hd'];
  return [];
}

export const imageStyles = (model) => (imageModelFamily(model) === 'dall-e-3' ? ['vivid', 'natural'] : []);

// 静态音色表：先内置 OpenAI 六音色（性别 + 用途标签），供应商还没有试听音频，
// previewUrl 占位为 null——有试听数据时音色卡直接播放，没有就退化成纯列表。
export const TTS_VOICE_TAGS = ['broadcast', 'support', 'audiobook', 'ad'];

export const TTS_VOICES = [
  { id: 'alloy', gender: 'neutral', tags: ['broadcast', 'support'], previewUrl: null },
  { id: 'echo', gender: 'male', tags: ['audiobook'], previewUrl: null },
  { id: 'fable', gender: 'neutral', tags: ['audiobook', 'ad'], previewUrl: null },
  { id: 'onyx', gender: 'male', tags: ['ad', 'broadcast'], previewUrl: null },
  { id: 'nova', gender: 'female', tags: ['audiobook'], previewUrl: null },
  { id: 'shimmer', gender: 'female', tags: ['support'], previewUrl: null }
];

export const TTS_VOICE_IDS = TTS_VOICES.map((voice) => voice.id);

// 当前模型的音色：目录里带 tts_voices 就用它（{ id, language, gender }），没带的回落内置 OpenAI 六音色。
export function ttsVoices(model) {
  const voices = Array.isArray(model?.ttsVoices) ? model.ttsVoices.filter((voice) => voice?.id) : [];
  return voices.length ? voices : TTS_VOICES;
}

// 当前模型的输出格式：目录里带 tts_formats 就用它（如 OpenRouter 上的 kokoro 只有 mp3 / pcm），没带的给完整列表。
export function ttsFormats(model) {
  const formats = Array.isArray(model?.ttsFormats) ? model.ttsFormats.filter(Boolean) : [];
  return formats.length ? formats : TTS_FORMATS;
}

// 界面语言码（zh_CN / en_US …）取前两位，与音色的 ISO 639-1 语言码对齐。
export const uiLanguageCode = (lng) =>
  String(lng || '')
    .slice(0, 2)
    .toLowerCase();

// 默认音色：优先与界面语言一致的，否则取第一个。
export function defaultTtsVoice(voices, lng) {
  const code = uiLanguageCode(lng);
  return (code && voices.find((voice) => voice.language === code)?.id) || voices[0]?.id || '';
}

// 音色按语言分组：保持首次出现的顺序，语言为空的归到末尾的「其他」组（language 为 ''）。
export function groupVoicesByLanguage(voices) {
  const groups = [];
  let other = null;
  for (const voice of voices || []) {
    const language = voice.language || '';
    if (!language) {
      other = other || { language: '', voices: [] };
      other.voices.push(voice);
      continue;
    }
    const group = groups.find((g) => g.language === language);
    if (group) group.voices.push(voice);
    else groups.push({ language, voices: [voice] });
  }
  return other ? [...groups, other] : groups;
}

export const IMAGE_FIELDS = [
  {
    key: 'size',
    group: 'generation',
    type: 'select',
    labelKey: 'playgroundConsole.fields.size',
    options: IMAGE_SIZES,
    default: '1024x1024',
    showWhen: imagesApiOnly
  },
  {
    key: 'n',
    group: 'generation',
    type: 'range',
    labelKey: 'playgroundConsole.fields.count',
    min: 1,
    max: 4,
    step: 1,
    default: 1,
    hintKey: 'playgroundConsole.fields.countHint',
    showWhen: imagesApiOnly
  },
  {
    key: 'quality',
    group: 'generation',
    type: 'select',
    labelKey: 'playgroundConsole.fields.quality',
    options: imageQualities,
    showWhen: (model) => imagesApiOnly(model) && imageQualities(model).length > 0
  },
  {
    key: 'style',
    group: 'generation',
    type: 'select',
    labelKey: 'playgroundConsole.fields.style',
    options: imageStyles,
    showWhen: (model) => imagesApiOnly(model) && imageStyles(model).length > 0
  },
  {
    // gpt-image 系列恒以 b64_json 回图，不接受 response_format，整字段对它隐藏。
    key: 'response_format',
    group: 'output',
    type: 'select',
    labelKey: 'playgroundConsole.fields.responseFormat',
    options: ['url', 'b64_json'],
    default: 'url',
    hintKey: 'playgroundConsole.fields.responseFormatHint',
    showWhen: (model) => imagesApiOnly(model) && imageModelFamily(model) !== 'gpt-image'
  }
];

export const TTS_FIELDS = [
  {
    key: 'voice',
    group: 'voice',
    type: 'select',
    labelKey: 'playgroundConsole.fields.voice',
    options: (model) => ttsVoices(model).map((voice) => voice.id),
    default: (model) => defaultTtsVoice(ttsVoices(model), i18n.language)
  },
  {
    // 语速等于默认的 1 不进请求体：上游默认就是 1，改过才带上。
    key: 'speed',
    group: 'voice',
    type: 'range',
    labelKey: 'playgroundConsole.fields.speed',
    min: 0.25,
    max: 4,
    step: 0.05,
    default: 1,
    omitDefault: true,
    hintKey: 'playgroundConsole.fields.speedHint'
  },
  {
    key: 'response_format',
    group: 'output',
    type: 'select',
    labelKey: 'playgroundConsole.fields.format',
    options: ttsFormats,
    default: (model) => ttsFormats(model)[0]
  }
];

export const STT_FIELDS = [
  {
    key: 'language',
    group: 'transcription',
    type: 'text',
    labelKey: 'playgroundConsole.fields.language',
    placeholderKey: 'playgroundConsole.fields.languagePlaceholder',
    default: ''
  },
  {
    key: 'prompt',
    group: 'transcription',
    type: 'text',
    labelKey: 'playgroundConsole.fields.sttPrompt',
    placeholderKey: 'playgroundConsole.fields.sttPromptPlaceholder',
    default: ''
  },
  {
    key: 'response_format',
    group: 'output',
    type: 'select',
    labelKey: 'playgroundConsole.fields.responseFormat',
    options: STT_FORMATS,
    default: STT_FORMATS[0]
  },
  {
    // 时间戳粒度只有 verbose_json 才有意义，普通 json 里上游会直接拒。
    key: 'timestamp_granularities',
    group: 'output',
    type: 'select',
    labelKey: 'playgroundConsole.fields.granularity',
    options: ['segment', 'word'],
    default: 'segment',
    showWhen: (model, values) => values?.response_format === 'verbose_json'
  }
];

// options 支持数组与 (model) => 数组两种写法，调用方一律经这里取。
export function fieldOptions(field, model) {
  const options = typeof field?.options === 'function' ? field.options(model) : field?.options;
  return Array.isArray(options) ? options : [];
}

// select 选项的文案 key：字段不声明 optionLabelKey 时选项直接显示取值（尺寸 / 格式这类
// 本身就是 API 字面量）；声明了就按 `${前缀}.${取值}` 走 i18n，空串取值记作 default。
export function fieldOptionLabelKey(field, option) {
  const prefix = field?.optionLabelKey;
  return prefix ? `${prefix}.${option === '' ? 'default' : option}` : null;
}

// 默认值：显式 default 优先；select 没写就取首个可选项，switch 取 false，其余取空串。
export function fieldDefault(field, model) {
  if (typeof field?.default === 'function') return field.default(model);
  if (field?.default !== undefined) return field.default;
  if (field?.type === 'switch') return false;
  if (field?.type === 'select') return fieldOptions(field, model)[0] ?? '';
  return '';
}

export const isFieldVisible = (field, model, values = {}) =>
  typeof field?.showWhen === 'function' ? Boolean(field.showWhen(model, values)) : true;

// 把「用户改过的值」与当前模型对齐：没设过的取默认值，设过但在当前模型的可选项里不存在的
// （换模型后 quality 从 hd 变成 high 这类）回落默认值。派生而非镜像，换模型不需要写回。
export function resolveValues(fields, model, stored = {}) {
  const out = {};
  for (const field of fields || []) {
    const fallback = fieldDefault(field, model);
    const value = stored?.[field.key];
    if (value === undefined) out[field.key] = fallback;
    else if (field.type === 'select' && !fieldOptions(field, model).includes(value)) out[field.key] = fallback;
    else out[field.key] = value;
  }
  return out;
}

export const visibleFields = (fields, model, values) => (fields || []).filter((field) => isFieldVisible(field, model, values));

// 分组渲染用：保持 schema 里的出现顺序，空组不返回。
export function groupedFields(fields, model, values) {
  const groups = [];
  for (const field of visibleFields(fields, model, values)) {
    const group = groups.find((g) => g.id === field.group);
    if (group) group.fields.push(field);
    else groups.push({ id: field.group, fields: [field] });
  }
  return groups;
}

// 请求参数：只取可见字段，空值不发；数值按 min / max 夹紧；omitDefault 的字段等于默认值时不发。
export function fieldParams(fields, model, values = {}) {
  const out = {};
  for (const field of visibleFields(fields, model, values)) {
    let value = values[field.key];
    if (value === undefined || value === null || value === '') continue;
    if (field.type === 'range' || field.type === 'number') {
      value = Number(value);
      if (!Number.isFinite(value)) continue;
      if (field.min !== undefined) value = Math.max(field.min, value);
      if (field.max !== undefined) value = Math.min(field.max, value);
    }
    if (field.omitDefault && value === fieldDefault(field, model)) continue;
    out[field.key] = value;
  }
  return out;
}
