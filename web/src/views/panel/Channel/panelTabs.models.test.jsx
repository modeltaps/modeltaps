// @vitest-environment jsdom
import { useState } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';

const fetchChannelUpstreamModels = vi.fn();
const fetchChannelModelDetails = vi.fn();
vi.mock('./channelApi', () => ({
  applyChannelPricing: vi.fn(),
  fetchChannelModelDetails: (...args) => fetchChannelModelDetails(...args),
  fetchChannelPrices: async () => ({ supported: false, prices: [] }),
  fetchChannelUpstreamModels: (...args) => fetchChannelUpstreamModels(...args),
  fetchUpstreamModelPicker: vi.fn(),
  previewChannelPricing: vi.fn(),
  refreshChannelPrices: vi.fn()
}));
vi.mock('./modelSuggestions', () => ({ buildModelSuggestions: () => [], fetchPriceList: async () => [] }));
vi.mock('../ModelInfo/supply/supplyApi', () => ({ attachPendingModel: vi.fn() }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() } }));

import { PANEL_TABS } from './panelTabs';

const modelsTab = PANEL_TABS.find((tab) => tab.key === 'models');
const ModelsBody = modelsTab.Body;

const channel = { id: 3, type: 1, models: 'gpt-4o', model_mapping: '', model_drift: { checked_at: 1700000000 } };

// The models tab with its draft kept in state, as the channel panel does; onDraft sees each change.
function Harness({ onDraft }) {
  const [draft, setDraft] = useState(() => modelsTab.make(channel));
  const ctx = { supplyRouting: true, rangeKey: '24h', metricsRows: [], modelsFocus: { filter: 'unlisted', n: 1 } };
  return (
    <MemoryRouter>
      <ModelsBody
        object={channel}
        draft={draft}
        setDraft={(next) => {
          onDraft(next);
          setDraft(next);
        }}
        ctx={ctx}
      />
    </MemoryRouter>
  );
}

describe('models tab: upstream models the channel does not list', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en_US');
    fetchChannelUpstreamModels.mockResolvedValue({
      checked_at: 1700000000,
      models: [
        { name: 'gpt-4o', state: 'in_use', public_model: 'gpt-4o' },
        { name: 'gpt-4o-2024-08-06', state: 'available', public_model: 'gpt-4o' },
        { name: 'o3', state: 'not_in_catalog' }
      ]
    });
    fetchChannelModelDetails.mockResolvedValue([]);
  });
  afterEach(cleanup);

  it('shows the public model each would serve', async () => {
    render(<Harness onDraft={() => {}} />);
    const rows = await screen.findAllByTestId('unlisted-model');
    expect(rows.map((r) => r.textContent)).toEqual([
      expect.stringContaining('Public as gpt-4o once added'),
      expect.stringContaining('Not in the catalog')
    ]);
  });

  it('adds a row to the list with one click, without ticking it first', async () => {
    const onDraft = vi.fn();
    render(<Harness onDraft={onDraft} />);
    const row = (await screen.findAllByTestId('unlisted-model')).find((r) => r.textContent.includes('gpt-4o-2024-08-06'));
    fireEvent.click(within(row).getByRole('button', { name: 'Add gpt-4o-2024-08-06' }));
    expect(onDraft).toHaveBeenCalledTimes(1);
    expect(onDraft.mock.calls[0][0].models).toBe('gpt-4o,gpt-4o-2024-08-06');
    // Added, it leaves the upstream-only rows; the other one stays.
    expect(screen.getAllByTestId('unlisted-model').map((r) => r.textContent)).toEqual([expect.stringContaining('o3')]);
  });

  it('adds a row when its switch is turned on', async () => {
    const onDraft = vi.fn();
    render(<Harness onDraft={onDraft} />);
    const row = (await screen.findAllByTestId('unlisted-model')).find((r) => r.textContent.includes('o3'));
    fireEvent.click(within(row).getByRole('switch', { name: 'Add o3' }));
    expect(onDraft.mock.calls[0][0].models).toBe('gpt-4o,o3');
  });
});
