import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { afterEach, describe, expect, it, vi } from 'vitest';

import CodeView, { LANGUAGE_TABS, copySnippet, readStoredLanguage, storeLanguage } from './CodeView';

// 测试跑在 node 环境（见 vite.config.mjs 的 test.environment），没有 DOM 也没有 Storage：
// 渲染断言走服务端渲染（受控 language 覆盖 tab 切换后的形态），交互回调则直接测组件导出的
// 纯函数（copySnippet / readStoredLanguage / storeLanguage）—— 按钮只是把它们接起来。

const SNIPPETS = {
  curl: 'curl https://api.example.com/v1/chat/completions',
  python: 'from openai import OpenAI',
  javascript: "import OpenAI from 'openai';"
};

const render = (props) => renderToStaticMarkup(createElement(CodeView, { snippets: SNIPPETS, ...props }));

const fakeStorage = (value) => ({ getItem: vi.fn(() => value), setItem: vi.fn(), removeItem: vi.fn() });

// 选中的那门语言：从 role="tab" 的按钮里挑 aria-selected="true" 的标签文本。
const selectedTab = (html) => html.match(/aria-selected="true"[^>]*>([^<]+)<\/button>/)?.[1] ?? '';

// 代码区经 hljs 高亮后被切成若干 span，断言代码文本前先去标签。
const text = (html) => html.replace(/<[^>]*>/g, '');

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe('语言 tab', () => {
  it('三门语言都出现，默认选中 curl 并渲染 curl 代码', () => {
    const html = render({});
    for (const tab of LANGUAGE_TABS) expect(html).toContain(`>${tab.label}</button>`);
    expect(selectedTab(html)).toBe('curl');
    expect(text(html)).toContain(SNIPPETS.curl);
    expect(text(html)).not.toContain(SNIPPETS.python);
  });

  it('切到 Python 后只渲染 Python 代码，选中态跟着走', () => {
    const html = render({ language: 'python' });
    expect(selectedTab(html)).toBe('Python');
    expect(text(html)).toContain(SNIPPETS.python);
    expect(text(html)).not.toContain(SNIPPETS.curl);
  });

  it('记忆上次语言：存过 javascript 就默认选它', () => {
    vi.stubGlobal('localStorage', fakeStorage('javascript'));
    expect(readStoredLanguage()).toBe('javascript');
    expect(selectedTab(render({}))).toBe('JavaScript');
  });

  it('存的值不是已知语言（或没有 Storage）时回落 curl，写入不抛', () => {
    vi.stubGlobal('localStorage', fakeStorage('rust'));
    expect(readStoredLanguage()).toBe('curl');

    const broken = {
      getItem: () => {
        throw new Error('denied');
      },
      setItem: () => {
        throw new Error('denied');
      }
    };
    vi.stubGlobal('localStorage', broken);
    expect(readStoredLanguage()).toBe('curl');
    expect(() => storeLanguage('python')).not.toThrow();
  });

  it('切换语言会写进 Storage', () => {
    const local = fakeStorage(null);
    vi.stubGlobal('localStorage', local);
    storeLanguage('python');
    expect(local.setItem).toHaveBeenCalledWith('playground.codeLanguage', 'python');
  });
});

describe('顶部一行', () => {
  it('当前模型 · 接口 · 同步提示拼在一起，缺项自动省略', () => {
    const html = render({ meta: { model: 'gpt-4o-mini', endpoint: 'POST /v1/chat/completions' }, labels: { sync: '随运行设置实时同步' } });
    expect(html).toContain('gpt-4o-mini · POST /v1/chat/completions · 随运行设置实时同步');

    expect(render({ meta: { model: 'gpt-4o-mini' } })).toContain('>gpt-4o-mini</p>');
  });
});

describe('复制', () => {
  it('写入剪贴板成功后回调 onCopy，参数是当前语言的代码', async () => {
    const clipboard = { writeText: vi.fn().mockResolvedValue(undefined) };
    const onCopy = vi.fn();

    await expect(copySnippet(SNIPPETS.python, { clipboard, onCopy })).resolves.toBe(true);
    expect(clipboard.writeText).toHaveBeenCalledWith(SNIPPETS.python);
    expect(onCopy).toHaveBeenCalledWith(SNIPPETS.python);
  });

  it('剪贴板不可用或写入失败时走 onCopyError，不抛给调用方', async () => {
    const onCopyError = vi.fn();
    const clipboard = { writeText: vi.fn().mockRejectedValue(new Error('denied')) };

    await expect(copySnippet('x', { clipboard, onCopyError })).resolves.toBe(false);
    expect(onCopyError).toHaveBeenCalled();

    onCopyError.mockClear();
    await expect(copySnippet('x', { clipboard: null, onCopyError })).resolves.toBe(false);
    expect(onCopyError).toHaveBeenCalled();
  });

  it('复制按钮用传入的文案', () => {
    expect(render({ labels: { copy: '复制' } })).toContain('复制');
  });
});
