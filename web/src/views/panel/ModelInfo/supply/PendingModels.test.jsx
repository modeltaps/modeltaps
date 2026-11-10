// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import { toast } from '@/components/ui/sonner';
import PendingModels from './PendingModels';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));
// The supply switches are read from the root-only options; the list does not depend on them.
vi.mock('./supplyParts', async (importOriginal) => ({ ...(await importOriginal()), useSupplySwitches: () => null }));

const PENDING = [
  {
    upstream_model: 'anthropic/claude-opus-5',
    suggestion: { model: 'claude-opus-5', source: 'seed', exists: true, status: 'published' },
    channels: [
      { channel_id: 1, channel_name: 'main', channel_status: 1 },
      { channel_id: 2, channel_name: 'backup', channel_status: 2 }
    ]
  },
  {
    upstream_model: 'brand-new-1',
    suggestion: { model: 'brand-new-1', source: '', exists: false },
    channels: [{ channel_id: 1, channel_name: 'main', channel_status: 1 }]
  }
];

const CATALOG = [{ model: 'claude-opus-5' }, { model: 'opus', alias_of: 'claude-opus-5' }, { model: 'auto/coder', kind: 'routing_policy' }];

let pending;
beforeEach(() => {
  vi.clearAllMocks();
  pending = PENDING;
  API.get.mockImplementation((url) => {
    if (url === '/api/model_supply/pending') return Promise.resolve({ data: { success: true, data: pending } });
    if (url === '/api/model_info/catalog') return Promise.resolve({ data: { success: true, data: CATALOG } });
    return Promise.resolve({ data: { success: true, data: [] } });
  });
  API.post.mockResolvedValue({ data: { success: true, data: { model: 'claude-opus-5', supplies: [{ id: 9 }], skipped: [] } } });
});

afterEach(cleanup);

const renderTab = () => render(createElement(MemoryRouter, null, createElement(PendingModels)));
const t = (key, options) => i18n.t(`modelSupply.pendingPage.${key}`, options);

describe('PendingModels', () => {
  it('lists waiting upstream models with their channels and suggested public model', async () => {
    renderTab();
    await waitFor(() => expect(screen.getAllByTestId('pending-model')).toHaveLength(2));
    const [first, second] = screen.getAllByTestId('pending-model');
    expect(first.textContent).toContain('anthropic/claude-opus-5');
    expect(first.textContent).toContain('main');
    expect(first.textContent).toContain('backup');
    expect(first.textContent).toContain(t('source.seed'));
    expect(first.textContent).not.toContain(t('isNew'));
    expect(second.textContent).toContain(t('isNew'));
  });

  it('shows the empty state', async () => {
    pending = [];
    renderTab();
    expect(await screen.findByText(t('empty'))).toBeTruthy();
  });

  it('attaches to the suggested model on every channel', async () => {
    renderTab();
    await waitFor(() => expect(screen.getAllByTestId('pending-model')).toHaveLength(2));
    fireEvent.click(screen.getAllByRole('button', { name: t('attach') })[0]);
    expect(await screen.findByText(t('attachTitle', { model: 'anthropic/claude-opus-5' }))).toBeTruthy();
    fireEvent.click(screen.getByRole('button', { name: t('attachButton') }));
    await waitFor(() =>
      expect(API.post).toHaveBeenCalledWith('/api/model_supply/pending/attach', {
        upstream_model: 'anthropic/claude-opus-5',
        model: 'claude-opus-5',
        channel_ids: [1, 2],
        create: false
      })
    );
    expect(toast.success).toHaveBeenCalledWith(t('attached', { model: 'claude-opus-5', count: 1 }));
  });

  it('creates a model that is not public yet, and refuses a routing policy', async () => {
    renderTab();
    await waitFor(() => expect(screen.getAllByTestId('pending-model')).toHaveLength(2));
    fireEvent.click(screen.getAllByRole('button', { name: t('attach') })[1]);
    const input = await screen.findByLabelText(t('publicModel'));
    expect(screen.getByText(t('willCreate', { model: 'brand-new-1' }))).toBeTruthy();

    fireEvent.change(input, { target: { value: 'auto/coder' } });
    expect(screen.getByText(t('isPolicy', { model: 'auto/coder' }))).toBeTruthy();
    expect(screen.getByRole('button', { name: t('attachButton') }).disabled).toBe(true);

    fireEvent.change(input, { target: { value: 'brand-new' } });
    fireEvent.click(screen.getByRole('button', { name: t('attachButton') }));
    await waitFor(() =>
      expect(API.post).toHaveBeenCalledWith('/api/model_supply/pending/attach', {
        upstream_model: 'brand-new-1',
        model: 'brand-new',
        channel_ids: [1],
        create: true
      })
    );
  });

  it('ignores a model everywhere it waits', async () => {
    API.post.mockResolvedValue({ data: { success: true, data: { channels: 2 } } });
    renderTab();
    await waitFor(() => expect(screen.getAllByTestId('pending-model')).toHaveLength(2));
    fireEvent.click(screen.getAllByRole('button', { name: t('ignore') })[0]);
    await waitFor(() =>
      expect(API.post).toHaveBeenCalledWith('/api/model_supply/pending/ignore', {
        upstream_model: 'anthropic/claude-opus-5',
        channel_ids: []
      })
    );
    expect(toast.success).toHaveBeenCalledWith(t('ignored', { model: 'anthropic/claude-opus-5' }));
  });
});
