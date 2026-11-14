// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';

const fetchChannelCapabilities = vi.fn();
vi.mock('./channelApi', () => ({ fetchChannelCapabilities: () => fetchChannelCapabilities() }));

import BalanceAlertField from './BalanceAlertField';

const caps = (lowBalance = {}) => ({
  types: { balance: [1, 28, 62], usage_windows: [58] },
  low_balance: { refresh_minutes: 60, kind_on: true, targets: [{ name: 'Email', detail: 'ops@example.com', ready: true }], ...lowBalance }
});

const renderField = (channel) =>
  render(
    <MemoryRouter>
      <BalanceAlertField channel={channel} value="" onChange={() => {}} />
    </MemoryRouter>
  );

describe('BalanceAlertField', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en_US');
    fetchChannelCapabilities.mockReset();
  });
  afterEach(cleanup);

  it('says how often the balance is checked and where the alert goes', async () => {
    fetchChannelCapabilities.mockResolvedValue(caps());
    renderField({ type: 62 });
    expect((await screen.findByText(/every 60 min/)).textContent).toContain('Email ops@example.com');
    expect(screen.getByRole('spinbutton').disabled).toBe(false);
  });

  it('labels CNY balances and warns when nothing can send', async () => {
    fetchChannelCapabilities.mockResolvedValue(caps({ targets: [{ name: 'Email', ready: false }] }));
    renderField({ type: 28 });
    expect(screen.getByText('Low-balance alert (CNY)')).toBeTruthy();
    expect(await screen.findByText(/No notifier can send/)).toBeTruthy();
    expect(screen.getByRole('link', { name: 'SMTP settings' }).getAttribute('href')).toBe('/panel/setting/notify?highlight=smtp');
  });

  it('turns the field off for providers without a balance', async () => {
    fetchChannelCapabilities.mockResolvedValue(caps());
    renderField({ type: 14 });
    expect(await screen.findByText(/has no balance endpoint/)).toBeTruthy();
    expect(screen.getByRole('spinbutton').disabled).toBe(true);
  });

  it('explains that subscription channels report usage instead', async () => {
    fetchChannelCapabilities.mockResolvedValue(caps());
    renderField({ type: 58 });
    expect(await screen.findByText(/Subscription channels have no balance/)).toBeTruthy();
    expect(screen.getByRole('spinbutton').disabled).toBe(true);
  });
});
