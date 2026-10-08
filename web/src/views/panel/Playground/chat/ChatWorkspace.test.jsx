import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { MemoryRouter } from 'react-router';
import { describe, expect, it } from 'vitest';

import 'i18n/i18n';
import ChatWorkspace, { threadSignal } from './ChatWorkspace';
import { DEFAULT_SETTINGS } from './useChatSession';

// 没有可用模型时对话页不白屏：外壳与输入框还在，示例位置换成说明卡，模型 chip 显示「暂无模型」。
// 渲染走服务端渲染（测试跑在 node 环境，见 vite.config.mjs）。

const session = (overrides = {}) => ({
  turns: [],
  input: '',
  system: '',
  settings: DEFAULT_SETTINGS,
  options: [],
  modelId: '',
  running: false,
  error: null,
  setInput: () => {},
  setSystem: () => {},
  updateSettings: () => {},
  run: () => {},
  rerun: () => {},
  abort: () => {},
  setSelectedModel: () => {},
  ...overrides
});

const noModels = {
  title: '暂无可用的对话模型',
  description: '你所在的分组还没有开放对话模型',
  actionLabel: '查看可用模型',
  actionHref: '/panel/model_price'
};

const render = (props = {}) =>
  renderToStaticMarkup(
    createElement(MemoryRouter, null, createElement(ChatWorkspace, { session: session(), noModels, ...props }))
  );

describe('ChatWorkspace 无可用模型', () => {
  it('画出说明卡与出口链接，示例网格不出现', () => {
    const html = render();

    expect(html).toContain(noModels.title);
    expect(html).toContain(noModels.actionLabel);
    expect(html).toContain(`href="${noModels.actionHref}"`);
    expect(html).not.toContain('从示例开始对话');
  });

  it('输入框置灰并给出同一句原因，模型 chip 写「暂无模型」', () => {
    const html = render();

    expect(html).toContain('<textarea');
    expect(html).toContain('disabled=""');
    expect(html).toContain(noModels.description);
    expect(html).toContain('暂无模型');
  });

  it('有模型时回到示例网格', () => {
    const html = render({ session: session({ options: [{ id: 'gpt-x', endpoints: ['/v1/chat/completions'] }], modelId: 'gpt-x' }) });

    expect(html).toContain('从示例开始对话');
    expect(html).not.toContain(noModels.title);
  });
});

describe('threadSignal', () => {
  it('新一轮与流式增量都换标识，内容没动就不换', () => {
    const one = [{ role: 'user', content: 'hi' }];
    const streaming = [...one, { role: 'assistant', content: 'he' }];

    expect(threadSignal([])).toBe(threadSignal([]));
    expect(threadSignal(one)).not.toBe(threadSignal(streaming));
    expect(threadSignal(streaming)).not.toBe(threadSignal([...one, { role: 'assistant', content: 'hell' }]));
    expect(threadSignal(streaming)).toBe(threadSignal([...one, { role: 'assistant', content: 'he' }]));
  });

  it('思考过程单独变长也算新增量', () => {
    const base = [{ role: 'assistant', content: '', reasoning: 'a' }];

    expect(threadSignal(base)).not.toBe(threadSignal([{ role: 'assistant', content: '', reasoning: 'ab' }]));
  });
});
