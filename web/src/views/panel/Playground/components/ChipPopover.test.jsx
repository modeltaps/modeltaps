import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import 'i18n/i18n';
import ChipPopover from './ChipPopover';
import ModelChip from './ModelChip';
import ParamsChip from './ParamsChip';

// node 环境无 DOM：chip 的开合以受控 open 渲染两次来断言「浮层在不在」，
// 非受控分支（自己持有 state）与 Escape / 外部点击关闭依赖真实事件，留给浏览器走查。

const SCHEMA = [{ key: 'temperature', type: 'range', labelKey: 'playgroundConsole.fields.speed', min: 0, max: 2, step: 0.1 }];

const chipHtml = (props) =>
  renderToStaticMarkup(createElement(ChipPopover, { label: '参数', ...props }, createElement('p', null, '浮层内容')));

describe('ChipPopover', () => {
  it('关着的时候只有 chip，没有浮层', () => {
    const html = chipHtml({ open: false });
    expect(html).toContain('参数');
    expect(html).toContain('aria-expanded="false"');
    expect(html).not.toContain('role="dialog"');
    expect(html).not.toContain('浮层内容');
  });

  it('打开时画出浮层并标记 aria-expanded', () => {
    const html = chipHtml({ open: true });
    expect(html).toContain('aria-expanded="true"');
    expect(html).toContain('role="dialog"');
    expect(html).toContain('浮层内容');
  });

  it('浮层对齐方向跟着 align 走', () => {
    expect(chipHtml({ open: true, align: 'end' })).toContain('right-0');
    expect(chipHtml({ open: true })).toContain('left-0');
  });

  it('禁用时 chip 是 disabled 的', () => {
    expect(chipHtml({ open: false, disabled: true })).toContain('disabled=""');
  });
});

describe('ParamsChip', () => {
  const render = (props) =>
    renderToStaticMarkup(
      createElement(ParamsChip, { schema: SCHEMA, values: { temperature: 0.7 }, onChange: () => {}, summary: '0.7 · 4096', ...props })
    );

  it('chip 上显示摘要文字', () => {
    expect(render({ open: false })).toContain('0.7 · 4096');
  });

  it('关着时不渲染表单', () => {
    const html = render({ open: false });
    expect(html).not.toContain('type="range"');
  });

  it('打开时由 FieldRenderer 按 schema 画出表单', () => {
    const html = render({ open: true });
    expect(html).toContain('role="dialog"');
    expect(html).toContain('type="range"');
    expect(html).toContain('max="2"');
    expect(html).toContain('value="0.7"');
  });
});

describe('ModelChip', () => {
  const render = (props) => renderToStaticMarkup(createElement(ModelChip, { items: [], value: 'gpt-5', ...props }));

  it('chip 默认显示当前模型名，label 可覆盖', () => {
    expect(render({ open: false })).toContain('gpt-5');
    expect(render({ open: false, label: 'GPT-5' })).toContain('GPT-5');
  });

  it('关着时不渲染模型选择器', () => {
    expect(render({ open: false })).not.toContain('搜索模型或供应商');
  });

  it('打开时渲染目录式选择器', () => {
    const html = render({ open: true });
    expect(html).toContain('aria-expanded="true"');
    expect(html).toContain('搜索模型或供应商');
    expect(html).toContain('没有匹配的模型');
  });

  it('取数状态透传给选择器', () => {
    expect(render({ open: true, loading: true })).toContain('加载中');
    expect(render({ open: true, error: true })).toContain('模型列表加载失败');
  });
});
