import { createElement } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it, vi } from 'vitest';

import ComposerBar, { composerKeyAction } from './ComposerBar';

// 测试跑在 node 环境（见 vite.config.mjs 的 test.environment），没有 DOM：
// 形态断言走服务端渲染，交互断言直接遍历元素树取回调来调 —— ComposerBar 全受控、不含 hook，
// 因此可以当普通函数调用。

const walk = (node, visit) => {
  if (Array.isArray(node)) {
    node.forEach((child) => walk(child, visit));
    return;
  }
  if (!node || typeof node !== 'object') return;
  visit(node);
  walk(node.props?.children, visit);
};

const collect = (node, predicate) => {
  const found = [];
  walk(node, (item) => predicate(item) && found.push(item));
  return found;
};

const keyDownOf = (element) => collect(element, (node) => typeof node.props?.onKeyDown === 'function')[0].props.onKeyDown;
const buttonsOf = (element) => collect(element, (node) => node.type === 'button');

const enter = (overrides = {}) => ({ key: 'Enter', shiftKey: false, nativeEvent: {}, preventDefault: vi.fn(), ...overrides });

const render = (props) => renderToStaticMarkup(createElement(ComposerBar, props));

describe('composerKeyAction', () => {
  it('回车发送，Shift+回车留给换行', () => {
    expect(composerKeyAction(enter())).toBe('submit');
    expect(composerKeyAction(enter({ shiftKey: true }))).toBeNull();
  });

  it('输入法组字期间的回车是上屏，不是发送', () => {
    expect(composerKeyAction(enter({ nativeEvent: { isComposing: true } }))).toBeNull();
    expect(composerKeyAction(enter({ isComposing: true }))).toBeNull();
  });

  it('其他按键一律不发送', () => {
    expect(composerKeyAction({ key: 'a' })).toBeNull();
    expect(composerKeyAction(undefined)).toBeNull();
  });
});

describe('ComposerBar 键盘发送', () => {
  it('回车带着当前值回调 onSubmit', () => {
    const onSubmit = vi.fn();
    const event = enter();
    keyDownOf(ComposerBar({ value: '你好', onSubmit }))(event);
    expect(onSubmit).toHaveBeenCalledWith('你好');
    expect(event.preventDefault).toHaveBeenCalled();
  });

  it('Shift+回车不发送也不拦默认行为', () => {
    const onSubmit = vi.fn();
    const event = enter({ shiftKey: true });
    keyDownOf(ComposerBar({ value: '你好', onSubmit }))(event);
    expect(onSubmit).not.toHaveBeenCalled();
    expect(event.preventDefault).not.toHaveBeenCalled();
  });

  it('空白输入、禁用、运行中都不发送', () => {
    const onSubmit = vi.fn();
    keyDownOf(ComposerBar({ value: '   ', onSubmit }))(enter());
    keyDownOf(ComposerBar({ value: '你好', disabled: true, onSubmit }))(enter());
    keyDownOf(ComposerBar({ value: '你好', running: true, onSubmit }))(enter());
    expect(onSubmit).not.toHaveBeenCalled();
  });
});

describe('ComposerBar 发送键', () => {
  it('点击发送键等价于回车', () => {
    const onSubmit = vi.fn();
    const buttons = buttonsOf(ComposerBar({ value: '你好', onSubmit }));
    buttons[buttons.length - 1].props.onClick();
    expect(onSubmit).toHaveBeenCalledWith('你好');
  });

  it('运行中发送键变成停止键并回调 onStop', () => {
    const onStop = vi.fn();
    const onSubmit = vi.fn();
    const buttons = buttonsOf(ComposerBar({ value: '你好', running: true, onStop, onSubmit }));
    buttons[buttons.length - 1].props.onClick();
    expect(onStop).toHaveBeenCalled();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it('不可发送时发送键是 disabled 的', () => {
    expect(render({ value: '', sendLabel: '发送' })).toContain('disabled=""');
    expect(render({ value: '你好', sendLabel: '发送' })).not.toContain('disabled=""');
  });
});

describe('ComposerBar 形态', () => {
  it('画出 placeholder、左右 chip 与发送键无障碍名', () => {
    const html = render({
      value: '',
      placeholder: '问点什么',
      leading: createElement('span', null, '附件'),
      trailing: createElement('span', null, '模型'),
      sendLabel: '发送'
    });
    expect(html).toContain('placeholder="问点什么"');
    expect(html).toContain('附件');
    expect(html).toContain('模型');
    expect(html).toContain('aria-label="发送"');
  });

  it('门禁文案随 disabledReason 出现，textarea 同时禁用', () => {
    const html = render({ value: '', disabled: true, disabledReason: createElement('span', null, '请先创建令牌') });
    expect(html).toContain('请先创建令牌');
    expect(html).toMatch(/<textarea[^>]*disabled=""/);
  });
});
