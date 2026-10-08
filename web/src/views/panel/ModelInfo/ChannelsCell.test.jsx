// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { ChannelsCell, channelEditPath, describeChannel } from './ModelTable';

// 模型表「渠道」列:计数 chip「N 个渠道」只计启用渠道,附前两个启用渠道图标;
// 没有启用渠道显示「未绑定」警示;点击打开抽屉的渠道 Tab。

afterEach(cleanup);

const t = i18n.t.bind(i18n);

const CHANNELS = [
  { id: 1, name: 'openai-main', type: 1, status: 1, priority: 10, weight: 2 },
  { id: 2, name: 'claude one', type: 14, status: 1 },
  { id: 3, name: 'custom', type: 8, status: 1, base_url: 'https://api.example.com' },
  { id: 4, name: 'backup', type: 1, status: 2 },
  { id: 5, name: 'spare', type: 14, status: 3 }
];

const renderCell = (channels, onOpen = vi.fn()) =>
  render(createElement(MemoryRouter, null, createElement(ChannelsCell, { channels, onOpen })));

const chip = () => screen.getByRole('button');

describe('ChannelsCell', () => {
  it('shows the unbound warning when nothing is bound', () => {
    renderCell([]);
    expect(chip().textContent).toBe(t('modelsPage.unbound'));
    expect(chip().hasAttribute('data-zero')).toBe(true);
  });

  it('treats a model whose channels are all disabled as unbound', () => {
    renderCell([CHANNELS[3], CHANNELS[4]]);
    expect(chip().textContent).toBe(t('modelsPage.unbound'));
  });

  it('counts only enabled channels', () => {
    renderCell(CHANNELS);
    expect(chip().getAttribute('aria-label')).toBe(t('modelsPage.channelCount', { count: 3 }));
    expect(chip().hasAttribute('data-zero')).toBe(false);
  });

  it('shows at most two enabled channel icons', () => {
    const { container } = renderCell(CHANNELS);
    expect(container.querySelectorAll('button > span.inline-flex')).toHaveLength(2);
    expect(container.querySelector('.grayscale')).toBeNull();
  });

  it('calls onOpen when clicked', () => {
    const onOpen = vi.fn();
    renderCell(CHANNELS, onOpen);
    fireEvent.click(chip());
    expect(onOpen).toHaveBeenCalledTimes(1);
  });
});

describe('channelEditPath', () => {
  it('filters the channel page by name and opens the edit drawer by id', () => {
    expect(channelEditPath(CHANNELS[1])).toBe('/panel/channel?name=claude%20one&channel_id=2');
  });
});

describe('describeChannel', () => {
  it('uses the type name and omits missing priority/weight', () => {
    expect(describeChannel(t, CHANNELS[3])).toEqual(['backup', 'OpenAI', t('channel_row.manual')]);
  });

  it('falls back to #type for unknown types and treats missing status as enabled', () => {
    expect(describeChannel(t, { id: 9, name: 'x', type: 99999 })).toEqual(['x', '#99999', t('channel_index.enabled')]);
  });
});
