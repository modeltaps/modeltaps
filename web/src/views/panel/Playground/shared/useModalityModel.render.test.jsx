// @vitest-environment jsdom
import { useCallback, useEffect, useMemo, useState } from 'react';
import { renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import { ConsoleContext, useConsole } from './ConsoleContext';
import { STT_RULE, TTS_RULE } from './fieldSchema';
import useModalityModel from './useModalityModel';

// hook 级回归：顶栏模型由路由页持有，首屏会在同一批里把空选中项补成目录首项，
// 可能盖掉模态的回落结果；hook 必须再次回落并同步回顶栏，且同步次数有限、不来回振荡。

const MODELS = [
  { id: 'gpt-4o', endpoints: ['chat'] },
  { id: 'tts-1', endpoints: ['audio.speech'] },
  { id: 'whisper-1', endpoints: ['audio.transcription'] }
];

// 模拟路由页(index.jsx)：selectedId 为状态，空时默认补 models[0]；
// setSelectedModel 额外记到 spy 上，只统计 hook 发起的同步。
function renderModalityModel({ initialId = '', rule = TTS_RULE } = {}) {
  const spy = vi.fn();
  const Page = ({ children }) => {
    const [selectedId, setSelectedId] = useState(initialId);
    useEffect(() => {
      if (!selectedId && MODELS.length > 0) setSelectedId(MODELS[0].id);
    }, [selectedId]);
    const selectedModel = useMemo(() => MODELS.find((m) => m.id === selectedId) || null, [selectedId]);
    const setSelectedModel = useCallback((model) => {
      spy(model);
      setSelectedId(typeof model === 'string' ? model : model?.id || '');
    }, []);
    const value = useMemo(() => ({ models: MODELS, selectedModel, setSelectedModel }), [selectedModel, setSelectedModel]);
    return <ConsoleContext.Provider value={value}>{children}</ConsoleContext.Provider>;
  };
  const view = renderHook(
    ({ rule: current }) => {
      const { options, model } = useModalityModel(current);
      return { options, model, selectedModel: useConsole().selectedModel };
    },
    { wrapper: Page, initialProps: { rule } }
  );
  return { ...view, spy };
}

describe('useModalityModel (rendered)', () => {
  it('路由页默认值盖掉回落结果时，再次回落并同步回 tts 候选', () => {
    const { result, spy } = renderModalityModel();
    expect(result.current.model.id).toBe('tts-1');
    expect(result.current.selectedModel?.id).toBe('tts-1');
    expect(spy).toHaveBeenCalledTimes(2);
    expect(spy.mock.calls.every(([m]) => m.id === 'tts-1')).toBe(true);
  });

  it('选中模型命中候选：不调用 setSelectedModel', () => {
    const { result, spy } = renderModalityModel({ initialId: 'tts-1' });
    expect(result.current.model.id).toBe('tts-1');
    expect(result.current.selectedModel.id).toBe('tts-1');
    expect(spy).not.toHaveBeenCalled();
  });

  it('rule 由 tts 切到 stt：重新回落并同步一次', () => {
    const { result, spy, rerender } = renderModalityModel({ initialId: 'tts-1' });
    expect(spy).not.toHaveBeenCalled();
    rerender({ rule: STT_RULE });
    expect(result.current.model.id).toBe('whisper-1');
    expect(result.current.selectedModel.id).toBe('whisper-1');
    expect(spy).toHaveBeenCalledTimes(1);
    expect(spy).toHaveBeenLastCalledWith(expect.objectContaining({ id: 'whisper-1' }));
  });

  it('同步次数有限：落定后反复重渲染不再同步', () => {
    const { result, spy, rerender } = renderModalityModel({ initialId: 'gpt-4o' });
    expect(spy).toHaveBeenCalledTimes(1);
    for (let i = 0; i < 3; i += 1) rerender({ rule: TTS_RULE });
    expect(spy).toHaveBeenCalledTimes(1);
    expect(result.current.selectedModel.id).toBe('tts-1');
  });
});
