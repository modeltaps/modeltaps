import { describe, expect, it } from 'vitest';

import { STT_PARAM_FIELDS, TTS_PARAM_FIELDS, clock, speechSummary, transcribeSummary } from './audioFields';

// 编辑卡上两颗 chip 的摘要与播放条的时间格式。音色单独一颗 chip，所以参数 chip 不含 voice。

describe('参数 chip 的字段', () => {
  it('合成参数把音色摘出去，转写参数照旧', () => {
    expect(TTS_PARAM_FIELDS.map((field) => field.key)).toEqual(['speed', 'response_format']);
    expect(STT_PARAM_FIELDS.map((field) => field.key)).toContain('language');
  });
});

describe('speechSummary', () => {
  it('语速一位小数 + 大写格式', () => {
    expect(speechSummary({ speed: 1, response_format: 'mp3' })).toBe('1.0x · MP3');
    expect(speechSummary({ speed: 1.25, response_format: 'wav' })).toBe('1.3x · WAV');
  });

  it('缺省按上游默认的 1.0 与 mp3 显示', () => {
    expect(speechSummary({})).toBe('1.0x · MP3');
  });
});

describe('transcribeSummary', () => {
  it('填了语言就带上，没填只报响应格式', () => {
    expect(transcribeSummary({ language: 'zh', response_format: 'verbose_json' })).toBe('zh · VERBOSE_JSON');
    expect(transcribeSummary({ language: '  ', response_format: 'json' })).toBe('JSON');
  });
});

describe('clock', () => {
  it('秒数折成 mm:ss，拿不到时长给 --:--', () => {
    expect(clock(0)).toBe('00:00');
    expect(clock(75.4)).toBe('01:15');
    expect(clock(undefined)).toBe('--:--');
    expect(clock(-1)).toBe('--:--');
  });
});
