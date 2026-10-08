import { describe, expect, it } from 'vitest';

import {
  AUDIO_ACCEPT,
  MAX_AUDIO_BYTES,
  STT_FILE_PLACEHOLDER,
  buildSpeechBody,
  formatTimestamp,
  normalizeTranscription,
  parseSpeed,
  speechCurl,
  speechFileName,
  transcriptionCurl,
  transcriptionFields,
  validateAudioFile
} from './audio';

// 音频面板的纯函数回归：请求体只带用户真正设过的参数、上传前的类型与体积校验、
// 转写结果（含 verbose_json 的分段）归一化，以及等效 curl 不外泄真实 key。

const API_KEY = 'sk-YOUR_TOKEN';
const file = (name, { type = '', size = 1024 } = {}) => ({ name, type, size });

describe('parseSpeed', () => {
  it('留空或非数字视为不传', () => {
    expect(parseSpeed('')).toBeNull();
    expect(parseSpeed('  ')).toBeNull();
    expect(parseSpeed('fast')).toBeNull();
  });

  it('越界的值丢弃，区间内的值原样取用', () => {
    expect(parseSpeed('0.1')).toBeNull();
    expect(parseSpeed('9')).toBeNull();
    expect(parseSpeed('0.25')).toBe(0.25);
    expect(parseSpeed('1.5')).toBe(1.5);
  });
});

describe('buildSpeechBody', () => {
  it('固定字段齐全，语速留空时不出现在请求体里', () => {
    expect(buildSpeechBody({ model: 'tts-1', input: 'hi~', voice: 'alloy', format: 'mp3', speed: '' })).toEqual({
      model: 'tts-1',
      input: 'hi~',
      voice: 'alloy',
      response_format: 'mp3'
    });
  });

  it('语速等于默认的 1 同样不传，改过才带上', () => {
    expect(buildSpeechBody({ model: 'tts-1', input: 'hi~', voice: 'nova', format: 'wav', speed: '1' }).speed).toBeUndefined();
    expect(buildSpeechBody({ model: 'tts-1', input: 'hi~', voice: 'nova', format: 'wav', speed: '1.25' }).speed).toBe(1.25);
  });
});

describe('speechFileName', () => {
  it('按所选格式命名，未知格式回落 mp3', () => {
    expect(speechFileName('wav')).toBe('speech.wav');
    expect(speechFileName('pcm')).toBe('speech.pcm');
    expect(speechFileName('ogg')).toBe('speech.mp3');
  });
});

describe('transcriptionFields', () => {
  it('只发用户填过的可选字段', () => {
    expect(transcriptionFields({ model: 'whisper-1', responseFormat: 'json', language: '', prompt: '  ' })).toEqual([
      ['model', 'whisper-1'],
      ['response_format', 'json']
    ]);
  });

  it('语言与提示词去空白后追加在固定字段之后', () => {
    expect(transcriptionFields({ model: 'whisper-1', responseFormat: 'verbose_json', language: ' zh ', prompt: ' Modeltaps ' })).toEqual([
      ['model', 'whisper-1'],
      ['response_format', 'verbose_json'],
      ['language', 'zh'],
      ['prompt', 'Modeltaps']
    ]);
  });
});

describe('validateAudioFile', () => {
  it('没选文件时给出可读的 i18n key', () => {
    expect(validateAudioFile(null)).toEqual({ ok: false, i18nKey: 'playground.audio.stt.errors.missingFile' });
  });

  it('audio/* 与白名单扩展名都放行（mp4 / webm 的 MIME 是 video/*）', () => {
    expect(validateAudioFile(file('a.mp3', { type: 'audio/mpeg' })).ok).toBe(true);
    expect(validateAudioFile(file('a.MP4', { type: 'video/mp4' })).ok).toBe(true);
    expect(validateAudioFile(file('a.webm', { type: '' })).ok).toBe(true);
  });

  it('非音频文件按类型拒绝', () => {
    expect(validateAudioFile(file('a.pdf', { type: 'application/pdf' }))).toEqual({
      ok: false,
      i18nKey: 'playground.audio.stt.errors.unsupportedType'
    });
  });

  it('超过 25 MB 在本地就挡下，不发这次请求', () => {
    expect(validateAudioFile(file('a.mp3', { type: 'audio/mpeg', size: MAX_AUDIO_BYTES + 1 })).i18nKey).toBe(
      'playground.audio.stt.errors.tooLarge'
    );
    expect(validateAudioFile(file('a.mp3', { type: 'audio/mpeg', size: MAX_AUDIO_BYTES })).ok).toBe(true);
  });
});

describe('AUDIO_ACCEPT', () => {
  it('文件选择器同时接受 audio/* 与扩展名', () => {
    expect(AUDIO_ACCEPT).toContain('audio/*');
    expect(AUDIO_ACCEPT).toContain('.m4a');
  });
});

describe('normalizeTranscription', () => {
  it('普通 json 只有文本，分段为空', () => {
    expect(normalizeTranscription({ text: 'hello' })).toEqual({ text: 'hello', segments: [] });
  });

  it('verbose_json 的分段归一化成 id / 起止 / 文本', () => {
    const out = normalizeTranscription({
      text: 'hello there',
      segments: [
        { id: 0, start: 0, end: 1.5, text: ' hello ' },
        { start: '1.5', end: '3', text: 'there' }
      ]
    });
    expect(out.segments).toEqual([
      { id: 0, start: 0, end: 1.5, text: 'hello' },
      { id: 1, start: 1.5, end: 3, text: 'there' }
    ]);
  });

  it('缺字段或非对象的响应不抛', () => {
    expect(normalizeTranscription(null)).toEqual({ text: '', segments: [] });
    expect(normalizeTranscription('plain text')).toEqual({ text: 'plain text', segments: [] });
    expect(normalizeTranscription({ segments: 'nope' }).segments).toEqual([]);
  });
});

describe('formatTimestamp', () => {
  it('按 m:ss.s 展示，异常值归零', () => {
    expect(formatTimestamp(0)).toBe('0:00.0');
    expect(formatTimestamp(75.5)).toBe('1:15.5');
    expect(formatTimestamp(-1)).toBe('0:00.0');
    expect(formatTimestamp('x')).toBe('0:00.0');
  });
});

describe('curl 示例', () => {
  it('合成的 curl 带 --output，且只出现占位 key', () => {
    const body = buildSpeechBody({ model: 'tts-1', input: "what's up", voice: 'alloy', format: 'wav' });
    const curl = speechCurl('https://api.example.com', API_KEY, body, 'wav');
    expect(curl).toContain('https://api.example.com/v1/audio/speech');
    expect(curl).toContain('--output speech.wav');
    expect(curl).toContain(`Bearer ${API_KEY}`);
    expect(curl).not.toMatch(/sk-(?!YOUR_TOKEN)/);
  });

  it('转写的 curl 用 --form 与占位文件路径', () => {
    const fields = transcriptionFields({ model: 'whisper-1', responseFormat: 'json' });
    const curl = transcriptionCurl('https://api.example.com', API_KEY, fields);
    expect(curl).toContain(`--form 'file=@${STT_FILE_PLACEHOLDER}'`);
    expect(curl).toContain("--form 'model=whisper-1'");
    expect(curl).toContain("--form 'response_format=json'");
  });
});
