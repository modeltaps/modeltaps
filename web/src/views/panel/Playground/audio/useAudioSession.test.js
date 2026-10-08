import { describe, expect, it } from 'vitest';

import {
  buildSpeechBody,
  formatFromContentType,
  isPlayableSpeechFormat,
  speechResultMeta,
  transcriptionFields,
  transcriptionParams,
  transcriptionResultMeta
} from './useAudioSession';

// 会话 hook 的纯函数部分：合成请求体、转写表单字段（含 multipart 才需要的 `[]` 后缀）、
// 等效代码里的数组写法，以及两个方向各自的结果区元信息。

describe('buildSpeechBody', () => {
  it('模型与文本恒在，其余字段原样来自 schema 求值', () => {
    expect(buildSpeechBody({ model: 'tts-1', input: 'hi~', params: { voice: 'alloy', response_format: 'mp3' } })).toEqual({
      model: 'tts-1',
      input: 'hi~',
      voice: 'alloy',
      response_format: 'mp3'
    });
  });

  it('没有可选参数时只发模型与文本', () => {
    expect(buildSpeechBody({ model: 'tts-1', input: 'hi~' })).toEqual({ model: 'tts-1', input: 'hi~' });
  });
});

describe('transcriptionFields', () => {
  it('model 在最前，时间戳粒度在 multipart 里带 `[]` 后缀', () => {
    expect(
      transcriptionFields({ model: 'whisper-1', params: { response_format: 'verbose_json', timestamp_granularities: 'word' } })
    ).toEqual([
      ['model', 'whisper-1'],
      ['response_format', 'verbose_json'],
      ['timestamp_granularities[]', 'word']
    ]);
  });

  it('没有可选字段时只发 model', () => {
    expect(transcriptionFields({ model: 'whisper-1' })).toEqual([['model', 'whisper-1']]);
  });
});

describe('transcriptionParams', () => {
  it('等效代码里时间戳粒度写成数组，其余字段不动', () => {
    expect(transcriptionParams({ response_format: 'verbose_json', timestamp_granularities: 'segment' })).toEqual({
      response_format: 'verbose_json',
      timestamp_granularities: ['segment']
    });
    expect(transcriptionParams({ language: 'zh' })).toEqual({ language: 'zh' });
  });
});

describe('result meta', () => {
  it('合成给耗时 · 音色 · 格式', () => {
    expect(speechResultMeta({ elapsedMs: 1940, values: { voice: 'alloy', response_format: 'mp3' } })).toEqual(['1.94s', 'alloy', 'mp3']);
  });

  it('转写给耗时 · 分段数，缺的项不占位', () => {
    expect(transcriptionResultMeta({ elapsedMs: 3420, result: { segments: [{}, {}, {}] } })).toEqual(['3.42s', '3']);
    expect(transcriptionResultMeta({})).toEqual([]);
  });
});

describe('formatFromContentType', () => {
  it('按响应的 Content-Type 取扩展名，认不出返回空串', () => {
    expect(formatFromContentType('audio/mpeg')).toBe('mp3');
    expect(formatFromContentType('audio/wav; charset=binary')).toBe('wav');
    expect(formatFromContentType('audio/ogg')).toBe('opus');
    expect(formatFromContentType('audio/pcm;rate=24000;channels=1')).toBe('pcm');
    expect(formatFromContentType('application/octet-stream')).toBe('');
    expect(formatFromContentType('')).toBe('');
  });
});

describe('isPlayableSpeechFormat', () => {
  it('裸 PCM 浏览器放不了，其余格式可播', () => {
    expect(isPlayableSpeechFormat('pcm')).toBe(false);
    expect(isPlayableSpeechFormat('mp3')).toBe(true);
  });
});
