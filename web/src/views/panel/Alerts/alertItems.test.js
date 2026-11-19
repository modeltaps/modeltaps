import { describe, expect, it } from 'vitest';
import { alertSubject, describeAlertItem } from './alertItems';

const t = (key, vars) => (vars ? `${key} ${JSON.stringify(vars)}` : key);

describe('official price dates', () => {
  it('shows a scheduled change like the other price changes', () => {
    const x = describeAlertItem(t, 'price_change', 'scheduled|gemini-4-argon|2|10|4|20');
    expect(x.tone).toBe('info');
    expect(x.text).toContain('alerts.item.price.scheduled');
    expect(x.text).toContain('"newOut":"20"');
  });

  it('asks to check a price that ended, with the vendor page', () => {
    const x = describeAlertItem(t, 'price_change', 'review|gpt-5.6-sol|2026-12-01|https://developers.openai.com/x');
    expect(x.tone).toBe('warn');
    expect(x.text).toContain('"day":"2026-12-01"');
    expect(x.href).toBe('https://developers.openai.com/x');
  });

  it('names the schedule source', () => {
    expect(alertSubject(t, { subject: 'source:official schedule' })).toContain('alerts.sources.official_schedule');
  });
});
