// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';

const api = { fetchChannelTopups: vi.fn(), addChannelTopup: vi.fn(), deleteChannelTopup: vi.fn() };
vi.mock('./channelApi', () => ({
  fetchChannelTopups: (...a) => api.fetchChannelTopups(...a),
  addChannelTopup: (...a) => api.addChannelTopup(...a),
  deleteChannelTopup: (...a) => api.deleteChannelTopup(...a)
}));

import ChannelTopups, { topupRate } from './ChannelTopups';

const topup = (id, paid, credited, note = '') => ({ id, channel_id: 7, paid, credited, note, created_at: 1763424000 });

describe('ChannelTopups', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en_US');
    Object.values(api).forEach((fn) => fn.mockReset());
  });
  afterEach(cleanup);

  it('works out the fee one top-up implies', () => {
    expect(topupRate({ paid: 105, credited: 100 })).toBeCloseTo(0.05);
    expect(topupRate({ paid: 100, credited: 110 })).toBeLessThan(0);
    expect(topupRate({ paid: 10, credited: 0 })).toBe(0);
  });

  it('lists the top-ups and the fee derived from them', async () => {
    api.fetchChannelTopups.mockResolvedValue({
      topups: [topup(2, 105.5, 100, 'card'), topup(1, 52, 50)],
      fee_rate: 0.0517,
      derived: true,
      window: 10
    });
    render(<ChannelTopups channelId={7} />);
    expect((await screen.findByTestId('topup-summary')).textContent).toBe('Fee from the latest 2 top-ups: 5.2%');
    const rows = screen.getAllByTestId('topup-row');
    expect(rows).toHaveLength(2);
    expect(rows[0].textContent).toContain('$105.50');
    expect(rows[0].textContent).toContain('5.5%');
  });

  it('says the fee is set by hand when nothing is recorded', async () => {
    api.fetchChannelTopups.mockResolvedValue({ topups: [], fee_rate: 0, derived: false, window: 10 });
    render(<ChannelTopups channelId={8} />);
    expect((await screen.findByTestId('topup-summary')).textContent).toMatch(/set by hand/);
  });

  it('records a top-up and reports the new summary', async () => {
    api.fetchChannelTopups.mockResolvedValue({ topups: [], fee_rate: 0, derived: false, window: 10 });
    const summary = { topups: [topup(3, 104, 100)], fee_rate: 0.04, derived: true, window: 10 };
    api.addChannelTopup.mockResolvedValue(summary);
    const onChanged = vi.fn();
    render(<ChannelTopups channelId={9} onChanged={onChanged} />);
    await screen.findByTestId('topup-summary');
    fireEvent.change(screen.getByLabelText('Paid'), { target: { value: '104' } });
    fireEvent.change(screen.getByLabelText('Credited'), { target: { value: '100' } });
    fireEvent.click(screen.getByRole('button', { name: /Add/ }));
    await waitFor(() => expect(onChanged).toHaveBeenCalledWith(summary));
    expect(api.addChannelTopup).toHaveBeenCalledWith(9, { paid: 104, credited: 100, note: '' });
    expect(screen.getByTestId('topup-summary').textContent).toBe('Fee from the latest top-up: 4.0%');
  });
});
