import { describe, expect, it } from 'vitest';

import {
  IMAGE_FIELDS,
  STT_FIELDS,
  TTS_FIELDS,
  TTS_FORMATS,
  TTS_VOICES,
  defaultTtsVoice,
  fieldDefault,
  fieldOptions,
  fieldParams,
  groupVoicesByLanguage,
  groupedFields,
  isChatImageModel,
  resolveValues,
  supportsImageToImage,
  ttsFormats,
  ttsVoices,
  visibleFields
} from './fieldSchema';

// 字段 schema 是「画出来的字段」与「发出去的字段」的唯一来源，所以三件事必须成立：
// showWhen 按模型 / 其他字段裁剪、默认值随模型回落、不可见与空值不进请求体。

const model = (id, extra = {}) => ({ id, endpoints: ['images'], info: { inputModalities: ['text'], capabilities: [] }, ...extra });

const gptImage = model('gpt-image-1');
const dalle3 = model('dall-e-3');
const generic = model('flux-pro');
const chatImage = {
  id: 'google/gemini-2.5-flash-image',
  endpoints: ['chat', 'responses'],
  info: { inputModalities: ['image', 'text'], outputModalities: ['image', 'text'], capabilities: [] }
};

describe('resolveValues', () => {
  it('没设过的字段取默认值', () => {
    expect(resolveValues(IMAGE_FIELDS, dalle3, {})).toMatchObject({ size: '1024x1024', n: 1, quality: 'standard', style: 'vivid' });
  });

  it('换模型后取值不在新家族的可选项里就回落默认值', () => {
    expect(resolveValues(IMAGE_FIELDS, gptImage, { quality: 'hd' }).quality).toBe('auto');
    expect(resolveValues(IMAGE_FIELDS, dalle3, { quality: 'hd' }).quality).toBe('hd');
  });

  it('用户设过且仍合法的取值原样保留', () => {
    expect(resolveValues(IMAGE_FIELDS, generic, { size: '512x512', n: 3 })).toMatchObject({ size: '512x512', n: 3 });
  });
});

describe('visibleFields', () => {
  it('quality / style 只对声明支持的家族出现', () => {
    const keys = (m) => visibleFields(IMAGE_FIELDS, m, resolveValues(IMAGE_FIELDS, m, {})).map((f) => f.key);
    expect(keys(dalle3)).toEqual(['size', 'n', 'quality', 'style', 'response_format']);
    // gpt-image 恒以 b64_json 回图，不接受 response_format。
    expect(keys(gptImage)).toEqual(['size', 'n', 'quality']);
    expect(keys(generic)).toEqual(['size', 'n', 'response_format']);
  });

  it('时间戳粒度只在 verbose_json 下出现（showWhen 读的是同一份取值）', () => {
    const keys = (values) => visibleFields(STT_FIELDS, null, values).map((f) => f.key);
    expect(keys({ response_format: 'json' })).not.toContain('timestamp_granularities');
    expect(keys({ response_format: 'verbose_json' })).toContain('timestamp_granularities');
  });
});

describe('fieldParams', () => {
  it('不可见字段不进请求体', () => {
    const values = resolveValues(IMAGE_FIELDS, gptImage, {});
    expect(fieldParams(IMAGE_FIELDS, gptImage, values)).toEqual({ size: '1024x1024', n: 1, quality: 'auto' });
  });

  it('空值不发，数值按 min / max 夹紧', () => {
    expect(fieldParams(IMAGE_FIELDS, generic, { size: '', n: 9, response_format: 'url' })).toEqual({ n: 4, response_format: 'url' });
  });

  it('omitDefault 的语速等于默认 1 时不发，改过才带上', () => {
    const base = resolveValues(TTS_FIELDS, null, {});
    expect(fieldParams(TTS_FIELDS, null, base).speed).toBeUndefined();
    expect(fieldParams(TTS_FIELDS, null, { ...base, speed: 1.25 }).speed).toBe(1.25);
  });

  it('转写的可选字段留空就不发', () => {
    const values = resolveValues(STT_FIELDS, null, {});
    expect(fieldParams(STT_FIELDS, null, values)).toEqual({ response_format: 'json' });
  });
});

describe('fieldDefault', () => {
  it('没写 default 的 select 取首个可选项，不支持该字段的模型取空串', () => {
    const quality = IMAGE_FIELDS.find((field) => field.key === 'quality');
    expect(fieldDefault(quality, dalle3)).toBe('standard');
    expect(fieldDefault(quality, generic)).toBe('');
  });
});

describe('groupedFields', () => {
  it('按 schema 顺序分组，空组不返回', () => {
    const groups = groupedFields(IMAGE_FIELDS, gptImage, resolveValues(IMAGE_FIELDS, gptImage, {}));
    expect(groups.map((group) => group.id)).toEqual(['generation']);
  });
});

describe('supportsImageToImage', () => {
  it('只有「images 接口 + 输入模态含 image」才算参考图可用', () => {
    expect(supportsImageToImage(generic)).toBe(false);
    expect(supportsImageToImage(model('gpt-image-1', { info: { inputModalities: ['text', 'image'] } }))).toBe(true);
    expect(supportsImageToImage(null)).toBe(false);
  });

  it('对话出图模型输入模态含 image 时参考图可用，纯 chat 模型不算', () => {
    expect(supportsImageToImage(chatImage)).toBe(true);
    expect(supportsImageToImage({ ...chatImage, info: { inputModalities: ['text'], outputModalities: ['image', 'text'] } })).toBe(false);
    expect(
      supportsImageToImage({ id: 'gpt-4o', endpoints: ['chat'], info: { inputModalities: ['text', 'image'], outputModalities: ['text'] } })
    ).toBe(false);
  });
});

describe('对话出图模型', () => {
  it('「chat 接口 + 输出模态含 image」才算；同时声明 images 的仍按专用 images 模型处理', () => {
    expect(isChatImageModel(chatImage)).toBe(true);
    expect(isChatImageModel({ ...chatImage, endpoints: ['chat', 'images'] })).toBe(false);
    expect(isChatImageModel(gptImage)).toBe(false);
    expect(isChatImageModel(null)).toBe(false);
  });

  it('Images API 专用字段既不渲染也不进请求体', () => {
    const values = resolveValues(IMAGE_FIELDS, chatImage, {});
    expect(visibleFields(IMAGE_FIELDS, chatImage, values)).toEqual([]);
    expect(fieldParams(IMAGE_FIELDS, chatImage, values)).toEqual({});
  });
});

describe('TTS_VOICES', () => {
  it('六个音色各有性别与用途标签，试听地址先留空', () => {
    expect(TTS_VOICES).toHaveLength(6);
    TTS_VOICES.forEach((voice) => {
      expect(voice.gender).toBeTruthy();
      expect(voice.tags.length).toBeGreaterThan(0);
      expect(voice.previewUrl).toBeNull();
    });
  });
});

describe('按模型的音色', () => {
  const kokoro = {
    id: 'kokoro',
    ttsVoices: [
      { id: 'af_heart', language: 'en', gender: 'female' },
      { id: 'zf_xiaoxiao', language: 'zh', gender: 'female' },
      { id: 'zm_yunxi', language: 'zh', gender: 'male' },
      { id: 'mystery', language: '', gender: '' }
    ]
  };

  it('模型带 tts_voices 就用它，没带回落内置六音色', () => {
    expect(ttsVoices(kokoro)).toEqual(kokoro.ttsVoices);
    expect(ttsVoices({ id: 'tts-1', ttsVoices: [] })).toBe(TTS_VOICES);
    expect(ttsVoices(null)).toBe(TTS_VOICES);
  });

  it('默认音色优先与界面语言一致，否则取第一个', () => {
    expect(defaultTtsVoice(kokoro.ttsVoices, 'zh_CN')).toBe('zf_xiaoxiao');
    expect(defaultTtsVoice(kokoro.ttsVoices, 'ja_JP')).toBe('af_heart');
    expect(defaultTtsVoice([], 'zh_CN')).toBe('');
  });

  it('音色只在不属于当前模型时回落该模型默认音色', () => {
    expect(resolveValues(TTS_FIELDS, kokoro, { voice: 'zm_yunxi' }).voice).toBe('zm_yunxi');
    expect(resolveValues(TTS_FIELDS, kokoro, { voice: 'alloy' }).voice).toBe(fieldDefault(TTS_FIELDS[0], kokoro));
    expect(resolveValues(TTS_FIELDS, { id: 'tts-1' }, { voice: 'zm_yunxi' }).voice).toBe('alloy');
  });

  it('按语言分组，保持首次出现顺序，语言为空的归到末尾', () => {
    const groups = groupVoicesByLanguage([kokoro.ttsVoices[3], ...kokoro.ttsVoices.slice(0, 3)]);
    expect(groups.map((g) => [g.language, g.voices.map((v) => v.id)])).toEqual([
      ['en', ['af_heart']],
      ['zh', ['zf_xiaoxiao', 'zm_yunxi']],
      ['', ['mystery']]
    ]);
  });
});

describe('按模型的输出格式', () => {
  const kokoro = { id: 'hexgrad/kokoro-82m', ttsFormats: ['mp3', 'pcm'] };
  const gemini = { id: 'google/gemini-3.1-flash-tts-preview', ttsFormats: ['wav', 'pcm'] };
  const openai = { id: 'gpt-4o-mini-tts' };
  const format = TTS_FIELDS.find((field) => field.key === 'response_format');

  it('模型带 tts_formats 就只给它，没带回落完整列表', () => {
    expect(ttsFormats(kokoro)).toEqual(['mp3', 'pcm']);
    expect(ttsFormats({ id: 'tts-1', ttsFormats: [] })).toBe(TTS_FORMATS);
    expect(ttsFormats(null)).toBe(TTS_FORMATS);
    expect(fieldOptions(format, kokoro)).toEqual(['mp3', 'pcm']);
    expect(fieldOptions(format, openai)).toEqual(TTS_FORMATS);
  });

  it('默认格式取该模型列表第一个', () => {
    expect(fieldDefault(format, kokoro)).toBe('mp3');
    expect(fieldDefault(format, gemini)).toBe('wav');
    expect(fieldDefault(format, openai)).toBe('mp3');
  });

  it('换模型后不在列表里的已存格式重置为列表第一个', () => {
    expect(resolveValues(TTS_FIELDS, kokoro, { response_format: 'wav' }).response_format).toBe('mp3');
    expect(resolveValues(TTS_FIELDS, kokoro, { response_format: 'pcm' }).response_format).toBe('pcm');
    expect(resolveValues(TTS_FIELDS, gemini, { response_format: 'mp3' }).response_format).toBe('wav');
    expect(resolveValues(TTS_FIELDS, gemini, { response_format: 'pcm' }).response_format).toBe('pcm');
    expect(resolveValues(TTS_FIELDS, openai, { response_format: 'wav' }).response_format).toBe('wav');
  });
});
