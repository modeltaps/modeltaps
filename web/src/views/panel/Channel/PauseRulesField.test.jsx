// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import PauseRulesField, { pauseRulesPatch } from './PauseRulesField';
import { PANEL_TABS } from './panelTabs';
import { updateChannel } from './channelApi';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), put: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));

const t = (key) => key;
const monitorTab = PANEL_TABS.find((x) => x.key === 'monitor');
const STORED = '[{"name":"quota","status_codes":[402]}]';

describe('pauseRulesPatch', () => {
  it('leaves the field out when the rules are unchanged', () => {
    const channel = { pause_rules: STORED };
    expect(pauseRulesPatch(JSON.stringify(JSON.parse(STORED), null, 2), channel, t)).toEqual({});
    expect(pauseRulesPatch('', { pause_rules: '' }, t)).toEqual({});
    expect(pauseRulesPatch('', {}, t)).toEqual({});
  });

  it('sends changed rules, and "" to clear them', () => {
    expect(pauseRulesPatch(' [] ', { pause_rules: STORED }, t)).toEqual({ pause_rules: '[]' });
    expect(pauseRulesPatch('  ', { pause_rules: STORED }, t)).toEqual({ pause_rules: '' });
  });

  it('rejects anything but empty or a JSON array', () => {
    expect(pauseRulesPatch('{"name":"x"}', {}, t)).toEqual({ error: 'channels.pauseRules.notArray' });
    expect(pauseRulesPatch('[{', {}, t)).toEqual({ error: 'channels.pauseRules.invalidJson' });
  });
});

describe('monitor tab', () => {
  const channel = { id: 3, test_model: '', balance_threshold: 0, drift_auto_add: false, drift_ignore_rules: '', pause_rules: STORED };

  it('saves the other fields without pause_rules when only they changed', () => {
    const draft = { ...monitorTab.make(channel), test_model: 'gpt-a' };
    const patch = monitorTab.patch(draft, channel, { t });
    expect(patch.test_model).toBe('gpt-a');
    expect(patch).not.toHaveProperty('pause_rules');
  });

  it('blocks the save on invalid rules', () => {
    const draft = { ...monitorTab.make(channel), pause_rules: 'nope' };
    expect(monitorTab.patch(draft, channel, { t })).toEqual({ error: 'channels.pauseRules.invalidJson' });
  });

  it('does not resend stored rules from other tabs', async () => {
    API.put.mockResolvedValue({ data: { success: true } });
    await updateChannel(channel, { priority: 2 });
    expect(API.put.mock.calls.at(-1)[1]).not.toHaveProperty('pause_rules');
    await updateChannel(channel, { pause_rules: '' });
    expect(API.put.mock.calls.at(-1)[1].pause_rules).toBe('');
  });
});

describe('PauseRulesField', () => {
  beforeEach(async () => {
    await i18n.changeLanguage('en');
    API.get.mockReset();
    API.get.mockResolvedValue({
      data: { success: true, data: { enabled: true, custom: false, rules: [{ name: 'site-quota' }], channel_rules: [] } }
    });
  });
  afterEach(cleanup);

  it('shows the site rules for reference and flags a non-array value', async () => {
    const onChange = vi.fn();
    const { rerender } = render(createElement(PauseRulesField, { channelId: 3, value: '', onChange }));
    expect(API.get).toHaveBeenCalledWith('/api/supply_pause/rules', { params: { channel_id: 3 } });
    await waitFor(() => expect(screen.getByTestId('site-pause-rules').textContent).toContain('site-quota'));
    expect(screen.getByText('Site rules in force (1)')).toBeTruthy();

    fireEvent.change(screen.getByLabelText(/Pause rules/), { target: { value: '{}' } });
    expect(onChange).toHaveBeenCalledWith('{}');
    rerender(createElement(PauseRulesField, { channelId: 3, value: '{}', onChange }));
    expect(screen.getByText('The pause rules must be a JSON array')).toBeTruthy();
  });
});
