import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import { ConsoleContext } from './ConsoleContext';
import { CHAT_RULE, IMAGE_RULE, IMAGE_RULES, STT_RULE, TTS_RULE } from './fieldSchema';
import useModalityModel, { resolveModalityModel } from './useModalityModel';

// 模态回落：顶栏选中的模型命中本模态候选就保留，不命中落到推荐项并要求同步回顶栏；
// rule 变化（语音页 tts ↔ stt）按新规则重算；无候选时给 null、不同步。

const MODELS = [
  { id: 'gpt-4o', endpoints: ['chat'] },
  { id: 'claude-sonnet', endpoints: ['chat'] },
  { id: 'gpt-image-1', endpoints: ['images'] },
  { id: 'tts-1', endpoints: ['audio.speech'] },
  { id: 'whisper-1', endpoints: ['audio.transcription'] }
];

const byId = (id) => MODELS.find((m) => m.id === id);

describe('resolveModalityModel', () => {
  it('选中模型在候选里：原样保留，不同步', () => {
    const out = resolveModalityModel(MODELS, CHAT_RULE, byId('claude-sonnet'));
    expect(out.options.map((m) => m.id)).toEqual(['gpt-4o', 'claude-sonnet']);
    expect(out.model.id).toBe('claude-sonnet');
    expect(out.sync).toBe(false);
  });

  it('选中模型不在候选里：落到推荐项，并要求同步回顶栏', () => {
    const out = resolveModalityModel(MODELS, IMAGE_RULE, byId('gpt-4o'));
    expect(out.model.id).toBe('gpt-image-1');
    expect(out.sync).toBe(true);
  });

  it('rule 变化时按新规则重算', () => {
    const tts = resolveModalityModel(MODELS, TTS_RULE, byId('tts-1'));
    expect(tts.model.id).toBe('tts-1');
    expect(tts.sync).toBe(false);

    const stt = resolveModalityModel(MODELS, STT_RULE, byId('tts-1'));
    expect(stt.options.map((m) => m.id)).toEqual(['whisper-1']);
    expect(stt.model.id).toBe('whisper-1');
    expect(stt.sync).toBe(true);
  });

  it('无候选：返回 null，不同步', () => {
    const out = resolveModalityModel(MODELS.slice(0, 2), IMAGE_RULE, byId('gpt-4o'));
    expect(out.options).toEqual([]);
    expect(out.model).toBeNull();
    expect(out.sync).toBe(false);
  });

  it('rule 为数组时取并集并保持目录顺序（图像页：images 模型 + 对话出图模型）', () => {
    const chatImage = { id: 'gemini-image', endpoints: ['chat'], info: { outputModalities: ['image', 'text'] } };
    const out = resolveModalityModel([chatImage, ...MODELS], IMAGE_RULES, byId('gpt-4o'));
    expect(out.options.map((m) => m.id)).toEqual(['gemini-image', 'gpt-image-1']);
  });

  it('尚未选中任何模型：落到推荐项并同步', () => {
    const out = resolveModalityModel(MODELS, CHAT_RULE, null);
    expect(out.model.id).toBe('gpt-4o');
    expect(out.sync).toBe(true);
  });
});

describe('useModalityModel', () => {
  it('从 ConsoleContext 取模型与选中项，返回候选与当前模型', () => {
    let result = null;
    const Probe = () => {
      result = useModalityModel(CHAT_RULE);
      return null;
    };
    const value = { models: MODELS, selectedModel: byId('claude-sonnet'), setSelectedModel: () => {} };
    renderToStaticMarkup(createElement(ConsoleContext.Provider, { value }, createElement(Probe)));
    expect(result.options.map((m) => m.id)).toEqual(['gpt-4o', 'claude-sonnet']);
    expect(result.model.id).toBe('claude-sonnet');
  });
});
