// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import SuppliesPanel from './SuppliesPanel';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));

const SUPPLIES = [
  {
    id: 1,
    model: 'gpt-a',
    channel_id: 10,
    channel_name: 'main',
    channel_status: 1,
    channel_type: 1,
    channel_group: 'default,vip',
    upstream_model: 'gpt-a',
    priority: 10,
    weight: 1,
    enabled: true,
    note: ''
  },
  {
    id: 2,
    model: 'gpt-a',
    channel_id: 20,
    channel_name: 'backup',
    channel_status: 1,
    channel_type: 1,
    channel_group: 'default',
    upstream_model: 'gpt-a-2025',
    priority: 0,
    weight: 3,
    enabled: true,
    note: 'n'
  }
];

const until = Math.floor(Date.now() / 1000) + 3600;

// The preview of a request from any group: the first tier is paused, so the backup takes it.
const PREVIEW = {
  model: 'gpt-a',
  status: 'published',
  mode: 'balance',
  active: true,
  first_choice: 2,
  outside_group: 0,
  tiers: [
    { priority: 10, supplies: [{ ...SUPPLIES[0], state: 'paused', until, pause_id: 7, pause_rule: 'quota', share: 0 }] },
    { priority: 0, supplies: [{ ...SUPPLIES[1], state: 'available', share: 1 }] }
  ]
};

// From group "vip" only the first supply is reachable, and it is paused.
const VIP_PREVIEW = { ...PREVIEW, group: 'vip', first_choice: undefined, outside_group: 1, tiers: [PREVIEW.tiers[0]] };

let preview;
beforeEach(() => {
  vi.clearAllMocks();
  preview = PREVIEW;
  API.get.mockImplementation((url, config) => {
    if (url === '/api/model_supply/preview')
      return Promise.resolve({ data: { success: true, data: config.params.group === 'vip' ? VIP_PREVIEW : preview } });
    if (url === '/api/supply_pause/')
      return Promise.resolve({ data: { success: true, data: [{ id: 7, channel_id: 10, model: '', until, rule: 'quota' }] } });
    if (url === '/api/group/')
      return Promise.resolve({ data: { success: true, data: [{ symbol: 'default' }, { symbol: 'vip', name: 'VIP' }] } });
    return Promise.resolve({ data: { success: true, data: [] } });
  });
  API.put.mockResolvedValue({ data: { success: true } });
});

afterEach(cleanup);

const renderPanel = (props = {}) =>
  render(
    createElement(
      MemoryRouter,
      null,
      createElement(SuppliesPanel, { model: 'gpt-a', supplyMode: 'balance', onSupplyModeChange: vi.fn(), channels: [], ...props })
    )
  );

describe('SuppliesPanel', () => {
  it('lists the supplies by tier with the state from the preview', async () => {
    renderPanel();
    await waitFor(() => expect(screen.getAllByTestId('supply-row')).toHaveLength(2));
    expect(API.get).toHaveBeenCalledWith('/api/model_supply/preview', { params: { model: 'gpt-a' } });
    const [first, second] = screen.getAllByTestId('supply-row');
    expect(first.textContent).toContain('main');
    expect(first.textContent).toContain(i18n.t('modelSupply.state.wholeChannel'));
    expect(first.textContent).toContain(i18n.t('modelSupply.channelGroups', { groups: 'default, vip' }));
    // The public name sent unchanged reads as "Same name"; a mapping shows the upstream name.
    expect(first.textContent).toContain(i18n.t('modelSupply.upstreamSame'));
    expect(i18n.t('modelSupply.upstreamSame')).not.toBe('modelSupply.upstreamSame');
    expect(second.textContent).toContain('gpt-a-2025');
    expect(second.textContent).not.toContain(i18n.t('modelSupply.upstreamSame'));
    expect(second.textContent).toContain(i18n.t('modelSupply.state.ok'));
    expect(second.textContent).toContain(i18n.t('modelSupply.preview.next'));
    expect(screen.getByText(i18n.t('modelSupply.tier.first'))).toBeTruthy();
    expect(screen.getByTestId('request-preview').textContent).toContain(i18n.t('modelSupply.preview.first', { channel: 'backup' }));
  });

  it('previews a request from one group', async () => {
    renderPanel();
    await waitFor(() => expect(screen.getAllByTestId('supply-row')).toHaveLength(2));
    fireEvent.click(await screen.findByRole('option', { name: 'VIP (vip)', hidden: true }));
    await waitFor(() => expect(API.get).toHaveBeenCalledWith('/api/model_supply/preview', { params: { model: 'gpt-a', group: 'vip' } }));
    const box = screen.getByTestId('request-preview');
    await waitFor(() => expect(box.textContent).toContain(i18n.t('modelSupply.preview.none')));
    expect(box.textContent).toContain(i18n.t('modelSupply.preview.outside', { count: 1, group: 'vip' }));
    const [, second] = screen.getAllByTestId('supply-row');
    expect(second.textContent).toContain(i18n.t('modelSupply.preview.outsideRow', { group: 'vip' }));
    expect(second.textContent).not.toContain(i18n.t('modelSupply.preview.next'));
  });

  it('notes when the preview is not in effect yet', async () => {
    preview = { ...PREVIEW, active: false };
    renderPanel();
    expect(await screen.findByText(i18n.t('modelSupply.switchOff'))).toBeTruthy();
    expect(screen.getByTestId('request-preview').textContent).toContain(i18n.t('modelSupply.preview.notActive'));
  });

  it('saves the enabled switch with every editable field', async () => {
    renderPanel();
    await waitFor(() => expect(screen.getAllByTestId('supply-row')).toHaveLength(2));
    const switches = screen.getAllByRole('switch', { name: i18n.t('modelSupply.enabled') });
    fireEvent.click(switches[1]);
    await waitFor(() =>
      expect(API.put).toHaveBeenCalledWith('/api/model_supply/', {
        id: 2,
        upstream_model: 'gpt-a-2025',
        priority: 0,
        weight: 3,
        enabled: false,
        note: 'n'
      })
    );
  });

  it('changes the supply mode through the callback', async () => {
    const onSupplyModeChange = vi.fn().mockResolvedValue();
    renderPanel({ onSupplyModeChange });
    await waitFor(() => expect(screen.getAllByTestId('supply-row')).toHaveLength(2));
    fireEvent.click(screen.getByRole('radio', { name: i18n.t('modelSupply.mode.failover') }));
    expect(onSupplyModeChange).toHaveBeenCalledWith('failover');
  });

  it('shows the note while supply routing is off', async () => {
    renderPanel({ switches: { supplyEnabled: false, pauseRulesEnabled: true } });
    expect(await screen.findByText(i18n.t('modelSupply.switchOff'))).toBeTruthy();
  });
});
