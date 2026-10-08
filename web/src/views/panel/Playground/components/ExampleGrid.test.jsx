import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import ExampleGrid from './ExampleGrid';

// node 环境无 DOM：形态走服务端渲染，点击回调靠遍历元素树取 onClick 直接调
// （ExampleGrid 纯展示、不含 hook，可以当普通函数调用）。

const walk = (node, visit) => {
  if (Array.isArray(node)) {
    node.forEach((child) => walk(child, visit));
    return;
  }
  if (!node || typeof node !== 'object') return;
  visit(node);
  walk(node.props?.children, visit);
};

const buttonsOf = (element) => {
  const found = [];
  walk(element, (node) => node.type === 'button' && found.push(node));
  return found;
};

const ITEMS = [
  { id: 'summarize', title: '长文摘要', description: '把长文压成要点', tags: ['系统提示词', '推理'] },
  { id: 'translate', title: '翻译', description: '中英互译' }
];

const render = (props) => renderToStaticMarkup(createElement(ExampleGrid, { items: ITEMS, ...props }));

describe('ExampleGrid', () => {
  it('画出标题、副标与每张卡的标题 / 说明 / tag', () => {
    const html = render({ title: '从示例开始', subtitle: '挑一个用例' });
    expect(html).toContain('从示例开始');
    expect(html).toContain('挑一个用例');
    expect(html).toContain('长文摘要');
    expect(html).toContain('把长文压成要点');
    expect(html).toContain('系统提示词');
  });

  it('每条示例一个按钮', () => {
    expect([...render({}).matchAll(/<button/g)]).toHaveLength(2);
  });

  it('列数只认 2 / 3 / 4，其余回落四列', () => {
    expect(render({ columns: 2 })).toContain('sm:grid-cols-2');
    expect(render({ columns: 3 })).toContain('lg:grid-cols-3');
    expect(render({})).toContain('lg:grid-cols-4');
  });

  it('空列表时只剩标题区', () => {
    const html = renderToStaticMarkup(createElement(ExampleGrid, { items: [], title: '从示例开始' }));
    expect(html).toContain('从示例开始');
    expect(html).not.toContain('<button');
  });

  it('点卡片把整条示例回调给 onPick', () => {
    const onPick = vi.fn();
    buttonsOf(ExampleGrid({ items: ITEMS, onPick }))[1].props.onClick();
    expect(onPick).toHaveBeenCalledWith(ITEMS[1]);
  });

  it('没有 onPick 时点击不抛', () => {
    expect(() => buttonsOf(ExampleGrid({ items: ITEMS }))[0].props.onClick()).not.toThrow();
  });
});
