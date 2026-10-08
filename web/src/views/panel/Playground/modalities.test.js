import { describe, expect, it } from 'vitest';

import { DEFAULT_MODALITY, MODALITIES, modalityDocsHref, modalityHref, resolveModality } from './modalities';

// 模态注册表的解析与回落:路由段 → 模态,未登记的值交回 null 由路由页回落。

describe('resolveModality', () => {
  it('缺省(裸 /panel/api)给默认模态', () => {
    expect(resolveModality(undefined)?.id).toBe(DEFAULT_MODALITY);
    expect(resolveModality('')?.id).toBe(DEFAULT_MODALITY);
  });

  it('三个可运行模态都能解析', () => {
    expect(resolveModality('chat')?.id).toBe('chat');
    expect(resolveModality('image')?.id).toBe('image');
    expect(resolveModality('speech')?.id).toBe('audio');
  });

  it('未登记的模态返回 null', () => {
    expect(resolveModality('not-a-modality')).toBeNull();
  });

  it('对比模态可路由,多列模态不回写 ?model=', () => {
    const compare = resolveModality('compare');
    expect(compare?.id).toBe('compare');
    expect(compare.multiModel).toBe(true);
    expect(compare.settings).toBeNull();
    expect(modalityHref(compare)).toBe('/panel/api/compare');
  });

  it('未上线的视频可路由,渲染空页面占位', () => {
    expect(resolveModality('video')?.id).toBe('video');
    const video = MODALITIES.find((m) => m.id === 'video');
    expect(video.comingSoon).toBe(true);
    expect(video.workspace).toBeTruthy();
  });
});

describe('modalityHref', () => {
  it('控制台与文档各走一套 /panel/api 路径', () => {
    expect(modalityHref(MODALITIES.find((m) => m.id === 'chat'))).toBe('/panel/api/chat');
    expect(modalityHref(MODALITIES.find((m) => m.id === 'audio'))).toBe('/panel/api/speech');
    expect(modalityDocsHref(MODALITIES.find((m) => m.id === 'audio'))).toBe('/panel/api/docs/speech');
  });
});

describe('registry 形状', () => {
  it('可运行模态都挂了工作区', () => {
    for (const entry of MODALITIES.filter((m) => !m.comingSoon)) {
      expect(entry.workspace, entry.id).toBeTruthy();
      expect(entry.mainTabKey, entry.id).toBeTruthy();
    }
  });

  it('对话模态没有设置栏:参数收在输入框的 chip 里', () => {
    expect(MODALITIES.find((m) => m.id === 'chat').settings).toBeNull();
  });
});
