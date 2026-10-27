// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import ModelSelectorModal from './ModelSelectorModal';

// 渠道模型挑选：打开即拉取上游（?detail=1），按家族热度排序、默认只显示各家族最新版本，
// 筛选后「全选当前结果」只加入显示中的行，提交时把选中的模型交回表单。

vi.mock('utils/api', () => ({ API: { post: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const item = (model, family, extra = {}) => ({
  model,
  family,
  version: '',
  latest: false,
  input_modalities: ['text'],
  output_modalities: ['text'],
  capabilities: [],
  open_weights: false,
  input_price: null,
  output_price: null,
  context_length: 0,
  other_channels: 0,
  ...extra
});

const DATA = {
  models: [
    item('anthropic/claude-opus-5.5', 'claude-opus', {
      version: '5.5',
      latest: true,
      input_price: 5,
      output_price: 25,
      context_length: 1000000
    }),
    item('anthropic/claude-opus-5', 'claude-opus', { version: '5' }),
    item('qwen/qwen3.8-27b', 'qwen-27b', { version: '3.8', latest: true, open_weights: true }),
    item('qwen/qwen3.6-27b', 'qwen-27b', { version: '3.6', open_weights: true }),
    item('openai/gpt-image-2', 'gpt-image', { version: '2', latest: true, output_modalities: ['image'] })
  ],
  families: [
    { family: 'qwen-27b', rank: 1, requests: 12, count: 2, latest: 'qwen/qwen3.8-27b' },
    { family: 'claude-opus', rank: 2, requests: 0, count: 2, latest: 'anthropic/claude-opus-5.5' },
    { family: 'gpt-image', rank: 3, requests: 0, count: 1, latest: 'openai/gpt-image-2' }
  ]
};

beforeAll(() => i18n.changeLanguage('en_US'));
afterAll(() => i18n.changeLanguage('zh_CN'));
beforeEach(() => {
  vi.clearAllMocks();
  API.post.mockResolvedValue({ data: { success: true, data: DATA } });
});
afterEach(cleanup);

const familyOrder = () => [...document.querySelectorAll('[data-family]')].map((el) => el.getAttribute('data-family'));
const shownModels = () => [...document.querySelectorAll('[data-model]')].map((el) => el.getAttribute('data-model'));

function renderModal(props = {}) {
  const onConfirm = vi.fn();
  render(
    <ModelSelectorModal
      open
      onClose={() => {}}
      onConfirm={onConfirm}
      channelValues={{ id: 7, type: 20, key: '', models: 'my-custom' }}
      {...props}
    />
  );
  return { onConfirm };
}

describe('ModelSelectorModal', () => {
  it('fetches the detailed list on open and shows the latest of each family by rank', async () => {
    renderModal();
    await screen.findByText('anthropic/claude-opus-5.5');
    expect(API.post).toHaveBeenCalledWith('/api/channel/provider_models_list?detail=1', expect.objectContaining({ id: 7, models: '' }));
    expect(familyOrder()).toEqual(['qwen-27b', 'claude-opus']);
    expect(shownModels()).toEqual(['qwen/qwen3.8-27b', 'anthropic/claude-opus-5.5']);
    expect(screen.getByTestId('picker-count').textContent).toBe('0 / 5 selected');
  });

  it('expands older versions of a family', async () => {
    renderModal();
    await screen.findByText('anthropic/claude-opus-5.5');
    const opus = document.querySelector('[data-family="claude-opus"]');
    fireEvent.click(within(opus).getByText('1 older'));
    expect(shownModels()).toContain('anthropic/claude-opus-5');
  });

  it('selects every shown result after filtering and submits it with the existing models', async () => {
    const { onConfirm } = renderModal();
    await screen.findByText('anthropic/claude-opus-5.5');
    fireEvent.click(screen.getByRole('button', { name: 'Open weights' }));
    expect(shownModels()).toEqual(['qwen/qwen3.8-27b']);
    fireEvent.click(screen.getByRole('button', { name: /Select results \(1\)/ }));
    expect(screen.getByTestId('picker-count').textContent).toBe('1 / 5 selected');
    fireEvent.click(screen.getByRole('button', { name: 'Submit' }));
    expect(onConfirm).toHaveBeenCalledWith(['my-custom', 'qwen/qwen3.8-27b'], false);
  });

  it('searches across all versions and switches the output filter', async () => {
    renderModal();
    await screen.findByText('anthropic/claude-opus-5.5');
    fireEvent.change(screen.getByLabelText('Search models'), { target: { value: 'qwen' } });
    expect(shownModels()).toEqual(['qwen/qwen3.8-27b', 'qwen/qwen3.6-27b']);
    fireEvent.change(screen.getByLabelText('Search models'), { target: { value: '' } });
    fireEvent.click(screen.getByRole('button', { name: 'Image' }));
    expect(shownModels()).toEqual(['openai/gpt-image-2']);
  });

  it('toggles a whole family from its header checkbox', async () => {
    renderModal();
    await screen.findByText('anthropic/claude-opus-5.5');
    fireEvent.click(screen.getByRole('switch', { name: 'Latest only' }));
    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all shown in claude-opus' }));
    expect(screen.getByTestId('picker-count').textContent).toBe('2 / 5 selected');
  });

  it('shows the error and an empty state when the fetch fails', async () => {
    API.post.mockResolvedValue({ data: { success: false, message: 'bad key' } });
    renderModal();
    await waitFor(() => expect(screen.getByText(/No models|no models/i)).toBeTruthy());
  });
});
