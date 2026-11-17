// @vitest-environment jsdom
import { cleanup, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import i18n from 'i18n/i18n';
import UsageAlertField from './UsageAlertField';

const caps = (usageAlert = {}) => ({
  types: { balance: [1], usage_windows: [58, 59] },
  usage_alert: {
    refresh_minutes: 60,
    kind_on: true,
    targets: [{ name: '', type: 'email', detail: 'ops@example.com', ready: true }],
    tiers: [50, 75, 100],
    ...usageAlert
  }
});

const renderField = (channel, capabilities, value = '') =>
  render(
    <MemoryRouter>
      <UsageAlertField channel={channel} caps={capabilities} value={value} onChange={() => {}} />
    </MemoryRouter>
  );

describe('UsageAlertField', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en_US');
  });
  afterEach(cleanup);

  it('says the alerts are on, at which tiers, how often the usage is checked and who gets them', () => {
    renderField({ type: 59, usage_windows: { fetched_at: 1_800_000_000 } }, caps());
    const line = screen.getByText(/Alerts at 50%, 75%, 100% of each window/).textContent;
    expect(line).toContain('(site tiers)');
    expect(line).toContain('usage checked every 60 min');
    expect(line).toContain('alerts go to Email ops@example.com');
    expect(screen.getByText(/Usage last updated/)).toBeTruthy();
    expect(screen.getByRole('textbox').getAttribute('placeholder')).toBe('50, 75, 100');
  });

  it('uses the channel tiers and says Claude Code usage comes from responses', () => {
    renderField({ type: 58 }, caps(), '80,95');
    const line = screen.getByText(/Alerts at 80%, 95% of each window/).textContent;
    expect(line).toContain('(this channel)');
    expect(line).toContain('usage read from responses, checked every 60 min');
  });

  it('says when the alerts are off for the channel or the site', () => {
    renderField({ type: 59 }, caps(), '0');
    expect(screen.getByText('Usage alerts are off for this channel.')).toBeTruthy();
    cleanup();
    renderField({ type: 59 }, caps({ tiers: [] }));
    expect(screen.getByText(/Usage alerts are off for the site/)).toBeTruthy();
    expect(screen.getByRole('textbox').getAttribute('placeholder')).toBe('Off');
  });

  it('warns about what keeps the alerts from arriving', () => {
    renderField({ type: 59 }, caps({ refresh_minutes: 0, kind_on: false, targets: [] }));
    expect(screen.getByText(/the usage is only checked when refreshed by hand/)).toBeTruthy();
    expect(screen.getByText(/Subscription usage notices are turned off/)).toBeTruthy();
    expect(screen.getByText(/No channel can send notices/)).toBeTruthy();
  });

  it('flags tiers it cannot read', () => {
    renderField({ type: 59 }, caps(), '50, 120');
    expect(screen.getByText(/Enter percentages from 1 to 100/)).toBeTruthy();
  });
});
