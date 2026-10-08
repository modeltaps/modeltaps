// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render as rtlRender, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it } from 'vitest';

import 'i18n/i18n';
import CompareCodeView from './CompareCodeView';
import CompareWorkspace from './CompareWorkspace';
import { INITIAL_STATE } from './useCompareSession';

// 对比页的两种落地形态：默认两列 / 没有可用的对话模型；以及「查看代码」的列切换。
// 用 jsdom 客户端渲染：服务端渲染会让 react-router 的 useLayoutEffect 报 SSR 告警。

afterEach(cleanup);

const OPTIONS = [
  { id: 'gpt-a', endpoints: ['chat'] },
  { id: 'gpt-b', endpoints: ['chat'] }
];

const noop = () => {};

const session = (overrides = {}) => ({
  ...INITIAL_STATE,
  columns: INITIAL_STATE.columns.map((column, i) => ({ ...column, modelId: OPTIONS[i].id })),
  options: OPTIONS,
  running: false,
  hasContent: false,
  activeIndex: 0,
  addColumn: noop,
  removeColumn: noop,
  setColumnModel: noop,
  setOverride: noop,
  setActive: noop,
  setInput: noop,
  setSystem: noop,
  updateSettings: noop,
  setSync: noop,
  sendAll: noop,
  sendTo: noop,
  stop: noop,
  stopAll: noop,
  clearAll: noop,
  ...overrides
});

const noModels = {
  title: '暂无可用的对比模型',
  description: '你所在的分组还没有开放对话模型',
  actionLabel: '查看可用模型',
  actionHref: '/panel/model_price'
};

const render = (props = {}) =>
  rtlRender(
    createElement(
      MemoryRouter,
      { future: { v7_startTransition: true, v7_relativeSplatPath: true } },
      createElement(CompareWorkspace, { session: session(), noModels, ...props })
    )
  ).container.innerHTML;

describe('CompareWorkspace', () => {
  it('默认两列各带自己的模型，2 列时「×」禁用，输入框在底部', () => {
    const html = render();

    expect(html.match(/data-compare-column=/g)).toHaveLength(2);
    expect(html).toContain('gpt-a');
    expect(html).toContain('gpt-b');
    expect(html).toContain('<textarea');
    expect(html.match(/<button[^>]*disabled=""[^>]*aria-label="删除此列"/g)).toHaveLength(2);
    expect(html).not.toContain(noModels.title);
  });

  it('只有一个模型时第二列写「选择模型」', () => {
    const html = render({
      session: session({
        options: [OPTIONS[0]],
        columns: INITIAL_STATE.columns.map((column, i) => ({ ...column, modelId: i === 0 ? 'gpt-a' : '' }))
      })
    });

    expect(html).toContain('选择模型');
  });

  it('没有可用的对话模型时整页换成说明卡，不画列与输入框', () => {
    const html = render({ session: session({ options: [] }) });

    expect(html).toContain(noModels.title);
    expect(html).toContain(`href="${noModels.actionHref}"`);
    expect(html).not.toContain('data-compare-column');
    expect(html).not.toContain('<textarea');
  });
});

describe('CompareCodeView', () => {
  const codeSession = () =>
    session({
      input: 'hello',
      sync: false,
      activeId: INITIAL_STATE.columns[0].id,
      columns: INITIAL_STATE.columns.map((column, i) => ({
        ...column,
        modelId: OPTIONS[i].id,
        override: i === 1 ? { temperature: 1.3 } : {}
      }))
    });

  const code = () => document.querySelector('pre, code')?.textContent || '';

  it('分段标签为「列 N · 模型名」，默认显示激活列，切列后换成该列的模型与覆盖参数', () => {
    rtlRender(createElement(CompareCodeView, { session: codeSession(), baseUrl: 'https://api.test' }));

    const tabs = screen.getAllByRole('tab', { name: /^列 / });
    expect(tabs.map((tab) => tab.textContent)).toEqual(['列 1 · gpt-a', '列 2 · gpt-b']);
    expect(code()).toContain('gpt-a');
    expect(code()).toContain('hello');
    expect(code()).not.toContain('1.3');

    fireEvent.click(tabs[1]);

    expect(code()).toContain('gpt-b');
    expect(code()).toContain('1.3');
    // 同步输入关着时，还没发出去的那条只进激活列。
    expect(code()).not.toContain('hello');
  });
});
