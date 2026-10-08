import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';

import 'i18n/i18n';
import ModelPicker, { applyPick, menuKeyAction, resolveAlign, resolvePlacement, MENU_MAX_HEIGHT } from './ModelPicker';
import ModelRow, { formatContext, formatPricing } from './ModelRow';
import StatusDot from './StatusDot';
import { AVAILABILITY, normalizeModel } from '../shared/modelIndex';

// 选择器只吃归一后的行，静态渲染即可验「画了什么 / 过滤掉了什么 / 选中的是哪一行」；
// 键盘移动与过滤条件的组合由 modelIndex 的纯函数单测覆盖（见 shared/modelIndex.test.js）。

const row = (overrides = {}) =>
  normalizeModel({
    id: 'claude-sonnet-5',
    vendor: 'anthropic',
    ownedBy: 'Anthropic',
    groups: ['default'],
    aliases: [],
    price: { type: 'tokens', input: 1.5, output: 7.5 },
    info: { capabilities: ['reasoning'], contextLength: 200000 },
    ...overrides
  });

const MODELS = [
  row(),
  row({ id: 'gpt-5', vendor: 'openai', ownedBy: 'OpenAI', info: { capabilities: ['reasoning'], contextLength: 400000 } }),
  row({ id: 'legacy-model', groups: [], price: null, info: { capabilities: [] } })
];

const render = (props) => renderToStaticMarkup(createElement(ModelPicker, { open: true, items: MODELS, ...props }));

// 每行模型都是一个 role="option"，紧跟 aria-selected 与行内的模型名。
const selectedIds = (html) =>
  [...html.matchAll(/role="option" aria-selected="(true|false)"[\s\S]*?font-mono[^>]*>([^<]+)</g)]
    .filter(([, flag]) => flag === 'true')
    .map(([, , id]) => id);

describe('ModelPicker', () => {
  it('关闭时只留下锚点容器，不画浮层', () => {
    const html = render({ open: false });
    expect(html).not.toContain('role="dialog"');
    expect(html).not.toContain('claude-sonnet-5');
  });

  it('把触发器和浮层放进同一个 relative 容器', () => {
    const html = renderToStaticMarkup(
      createElement(ModelPicker, { open: true, items: MODELS }, createElement('button', { type: 'button' }, 'chip'))
    );
    expect(html).toContain('relative inline-flex');
    expect(html.indexOf('chip')).toBeLessThan(html.indexOf('role="dialog"'));
  });

  it('默认贴着触发器上方弹出，指定 bottom 时向下', () => {
    expect(render({})).toContain('bottom-full');
    expect(render({ placement: 'bottom' })).toContain('top-full');
  });

  it('画出搜索框、隐藏不可用开关与带口径的列标题', () => {
    const html = render({});
    expect(html).toContain('搜索模型或供应商');
    expect(html).toContain('隐藏不可用');
    expect(html).toContain('role="switch"');
    expect(html).toContain('价格为每 1M tokens 的输入 / 输出单价');
    expect(html).toMatch(/价格 \/ 1M tokens[\s\S]*上下文[\s\S]*输入[\s\S]*输出/);
  });

  it('列表为空时不画列标题', () => {
    expect(render({ items: [] })).not.toContain('价格 / 1M tokens');
  });

  it('供应商 tab 按模型数列出并带计数', () => {
    const html = render({});
    expect(html).toContain('anthropic');
    expect(html).toContain('openai');
    expect(html).toMatch(/全部<\/span><span[^>]*>3</);
  });

  it('默认隐藏不可用模型', () => {
    const html = render({});
    expect(html).toContain('claude-sonnet-5');
    expect(html).toContain('gpt-5');
    expect(html).not.toContain('legacy-model');
  });

  it('选中行标记 aria-selected', () => {
    expect(selectedIds(render({ value: 'gpt-5' }))).toEqual(['gpt-5']);
    expect(selectedIds(render({}))).toEqual([]);
  });

  it('多选模式下同时标记多行', () => {
    expect(selectedIds(render({ mode: 'multi', values: ['gpt-5', 'claude-sonnet-5'] }))).toEqual(['claude-sonnet-5', 'gpt-5']);
  });

  it('多选模式画出行内 checkbox 与底部已选计数', () => {
    const html = render({ mode: 'multi', values: ['gpt-5'] });
    expect([...html.matchAll(/role="checkbox"/g)]).toHaveLength(2);
    expect(html).toContain('aria-checked="true"');
    expect(html).toContain('已选 1 个');
  });

  it('单选模式既无 checkbox 也无已选计数', () => {
    const html = render({ value: 'gpt-5' });
    expect(html).not.toContain('role="checkbox"');
    expect(html).not.toContain('已选');
  });

  it('能力 chip 来自目录里出现过的能力，且只出现在顶部筛选区', () => {
    const html = render({});
    expect(html).toContain('推理');
    expect([...html.matchAll(/>推理</g)]).toHaveLength(1);
  });

  it('列表为空时给出空态而不是空白', () => {
    expect(render({ items: [] })).toContain('没有匹配的模型');
  });

  it('加载中与取数失败各有文案', () => {
    expect(render({ items: [], loading: true })).toContain('加载中');
    expect(render({ items: [], error: true })).toContain('模型列表加载失败');
  });
});

describe('ModelPicker menu behaviour', () => {
  it('按键映射成菜单动作，其他键不拦截', () => {
    expect(menuKeyAction('ArrowDown')).toBe('next');
    expect(menuKeyAction('ArrowUp')).toBe('prev');
    expect(menuKeyAction('Enter')).toBe('pick');
    expect(menuKeyAction('Escape')).toBe('close');
    expect(menuKeyAction('a')).toBeNull();
  });

  it('单选选完即关，多选只切换 id 且保持打开', () => {
    const picked = { id: 'gpt-5' };
    expect(applyPick({ row: picked })).toEqual({ value: picked, close: true });
    expect(applyPick({ multi: true, selected: [], row: picked })).toEqual({ value: ['gpt-5'], close: false });
    expect(applyPick({ multi: true, selected: ['gpt-5', 'a'], row: picked })).toEqual({ value: ['a'], close: false });
  });

  it('首选方向放不下就翻面，两边都不够时留在余量大的一侧', () => {
    expect(resolvePlacement({ spaceAbove: 600, spaceBelow: 100 })).toBe('top');
    expect(resolvePlacement({ spaceAbove: 80, spaceBelow: 600 })).toBe('bottom');
    expect(resolvePlacement({ spaceAbove: 200, spaceBelow: 100 })).toBe('top');
    expect(resolvePlacement({ preferred: 'bottom', spaceAbove: 600, spaceBelow: 100 })).toBe('top');
    expect(resolvePlacement({ preferred: 'bottom', spaceBelow: MENU_MAX_HEIGHT })).toBe('bottom');
  });

  it('右对齐会溢出滚动容器左边时翻成左对齐', () => {
    // 语音页：内容列 left 464，两个 tab 的 chip 贴在列的左侧。
    const bounds = { boundsLeft: 464, boundsRight: 1424 };
    expect(resolveAlign({ triggerLeft: 490, triggerRight: 610, ...bounds })).toEqual({
      align: 'start',
      offset: 0,
      maxWidth: null
    });
  });

  it('首选方向放得下就不动，两侧都放不下时 clamp 进边界并收窄宽度', () => {
    expect(resolveAlign({ triggerLeft: 800, triggerRight: 1000, boundsLeft: 0, boundsRight: 1440 })).toEqual({
      align: 'end',
      offset: 0,
      maxWidth: null
    });
    const tight = resolveAlign({ triggerLeft: 100, triggerRight: 200, boundsLeft: 0, boundsRight: 300 });
    expect(tight.align).toBe('end');
    expect(200 - tight.maxWidth + tight.offset).toBe(8);
    expect(tight.maxWidth).toBe(284);
  });

  it('手机宽度下收窄后的浮层完整留在视口内，左右各留 8px', () => {
    for (const vw of [360, 390]) {
      const box = resolveAlign({ triggerLeft: vw - 150, triggerRight: vw - 60, boundsLeft: 0, boundsRight: vw });
      const right = vw - 60 + box.offset;
      expect(box.maxWidth).toBe(vw - 16);
      expect(right - box.maxWidth).toBe(8);
      expect(right).toBe(vw - 8);
    }
  });
});

describe('ModelRow formatting', () => {
  it('上下文按 K / M 收敛', () => {
    expect(formatContext(200000)).toBe('200K');
    expect(formatContext(1000000)).toBe('1M');
    expect(formatContext(undefined)).toBeNull();
  });

  it('按量计费给输入 / 输出两个单价，按次计费只给一个', () => {
    const t = (key) => (key === 'modelpricePage.free' ? '免费' : key);
    expect(formatPricing({ type: 'tokens', input: 1.5, output: 7.5 }, t)).toBe('$3 / $15');
    expect(formatPricing({ type: 'times', input: 0.5 }, t)).toBe('$0.001');
    expect(formatPricing({ type: 'tokens', input: 0, output: 0 }, t)).toBe('免费 / 免费');
    expect(formatPricing(undefined, t)).toBeNull();
  });

  it('价格缺失时整列说明「未配置」而不是显示 0', () => {
    const html = renderToStaticMarkup(createElement(ModelPicker, { open: true, items: [row({ id: 'no-price', price: null })] }));
    expect(html).toContain('价格未配置');
  });

  it('按量计费拆成输入、输出两列', () => {
    const html = renderToStaticMarkup(createElement(ModelPicker, { open: true, items: [row()] }));
    expect(html).toMatch(/>\$3<\/span><span[^>]*>\$15</);
  });

  it('按次计费只占输入列，输出列标注按次', () => {
    const html = renderToStaticMarkup(
      createElement(ModelPicker, { open: true, items: [row({ id: 'per-call', price: { type: 'times', input: 0.5 } })] })
    );
    expect(html).toMatch(/>\$0\.001<\/span><span[^>]*>按次</);
  });

  it('不可用提示与多渠道入口都落在名称下方同一行，行内不画能力标签', () => {
    const meta = (html) => html.match(/<div class="h-4[^"]*">([\s\S]*?)<\/div>/)[1];
    const unavailable = renderToStaticMarkup(createElement(ModelRow, { model: row({ groups: [] }) }));
    const multiChannel = renderToStaticMarkup(
      createElement(ModelRow, {
        model: { ...row(), channels: [{ id: 'a' }, { id: 'b', pricing: { type: 'tokens', input: 1, output: 2 } }] }
      })
    );
    const plain = renderToStaticMarkup(createElement(ModelRow, { model: row() }));
    expect(meta(unavailable)).toBe('无可用渠道');
    expect(meta(multiChannel)).toContain('aria-expanded="false"');
    expect(meta(multiChannel)).toContain('2 渠道');
    expect(meta(plain)).toBe('');
    expect(plain).not.toContain('推理');
  });

  it('模型名带完整 id 的原生悬停提示，截断时也能看到全名', () => {
    const id = 'anthropic/claude-sonnet-5-20260901-extended-thinking-preview';
    const html = renderToStaticMarkup(createElement(ModelRow, { model: row({ id }) }));
    expect(html).toContain(`title="${id}">${id}<`);
  });
});

describe('StatusDot', () => {
  it('可用模型的点是绿的', () => {
    const html = renderToStaticMarkup(createElement(StatusDot, { availability: row().availability }));
    expect(html).toContain('bg-emerald-500');
  });

  it('可用性未知时按不可用画灰点', () => {
    const html = renderToStaticMarkup(createElement(StatusDot, {}));
    expect(html).toContain('bg-muted-foreground/40');
  });

  it('不可用模型的点是灰的', () => {
    const html = renderToStaticMarkup(createElement(StatusDot, { availability: row({ groups: [] }).availability }));
    expect(html).toContain('bg-muted-foreground/40');
    expect(row({ groups: [] }).availability).toBe(AVAILABILITY.unavailable);
  });
});
