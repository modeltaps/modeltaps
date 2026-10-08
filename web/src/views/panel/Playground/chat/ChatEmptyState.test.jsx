import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import 'i18n/i18n';
import ChatEmptyState, { CHAT_EXAMPLES } from './ChatEmptyState';

// 空态是 2×4 的示例卡网格：八张卡、卡面文案来自 i18n、点选交回整条示例（提示词 + 参数）。
// 渲染走服务端渲染（测试跑在 node 环境，见 vite.config.mjs）。

const render = (props = {}) => renderToStaticMarkup(createElement(ChatEmptyState, props));

describe('CHAT_EXAMPLES', () => {
  it('八条示例填满 2 行 × 4 列，id 不重复', () => {
    expect(CHAT_EXAMPLES).toHaveLength(8);
    expect(new Set(CHAT_EXAMPLES.map((example) => example.id)).size).toBe(8);
  });

  it('带参数的示例只写会话认识的键', () => {
    const keys = new Set(CHAT_EXAMPLES.flatMap((example) => Object.keys(example.settings || {})));
    for (const key of keys) expect(['temperature', 'topP', 'maxTokens', 'seed', 'thinking', 'jsonMode']).toContain(key);
  });
});

describe('ChatEmptyState', () => {
  it('画出标题、副标与八张示例卡', () => {
    const html = render();

    expect(html).toContain('从示例开始对话');
    expect(html.match(/<button/g)).toHaveLength(CHAT_EXAMPLES.length);
    expect(html).toContain('流式输出');
    expect(html).toContain('TypeScript');
  });

  it('示例卡带能力 tag', () => {
    const html = render();

    expect(html).toContain('代码');
    expect(html).toContain('JSON');
  });
});
