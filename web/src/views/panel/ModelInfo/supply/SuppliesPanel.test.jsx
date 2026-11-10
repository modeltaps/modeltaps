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
    upstream_model: 'gpt-a-2025',
    priority: 0,
    weight: 3,
    enabled: true,
    note: 'n'
  }
];

beforeEach(() => {
  vi.clearAllMocks();
  API.get.mockImplementation((url) => {
    if (url === '/api/model_supply/') return Promise.resolve({ data: { success: true, data: SUPPLIES } });
    if (url === '/api/supply_pause/')
      return Promise.resolve({
        data: { success: true, data: [{ id: 7, channel_id: 10, model: '', until: Math.floor(Date.now() / 1000) + 3600, rule: 'quota' }] }
      });
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
  it('lists the supplies by tier with their state', async () => {
    renderPanel();
    await waitFor(() => expect(screen.getAllByTestId('supply-row')).toHaveLength(2));
    expect(API.get).toHaveBeenCalledWith('/api/model_supply/', { params: { model: 'gpt-a' } });
    const [first, second] = screen.getAllByTestId('supply-row');
    expect(first.textContent).toContain('main');
    expect(first.textContent).toContain(i18n.t('modelSupply.state.wholeChannel'));
    expect(second.textContent).toContain('gpt-a-2025');
    expect(second.textContent).toContain(i18n.t('modelSupply.state.ok'));
    expect(screen.getByText(i18n.t('modelSupply.tier.first'))).toBeTruthy();
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
