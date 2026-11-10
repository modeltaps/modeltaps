// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import QuotaRemindCard from './QuotaRemindCard';

// The personal quota warning card: a null threshold follows the site default, saving an amount
// sets it, "use site default" saves -1, the switch saves with the footer like the threshold, and
// the site switch disables the controls.

vi.mock('utils/api', () => ({ API: { get: vi.fn(), put: vi.fn() } }));
vi.mock('utils/common', () => ({ showError: vi.fn(), showSuccess: vi.fn(), renderSpend: (q) => '$' + q / 500000 }));
const selfUser = { inputs: { id: 1, email: 'alice@example.test' } };

const setting = (over = {}) => ({
  quota_remind_enabled: true,
  quota_remind_threshold: null,
  site_quota_remind_enabled: true,
  site_quota_remind_threshold: 500000,
  ...over
});

beforeEach(() => {
  vi.clearAllMocks();
  localStorage.setItem('display_in_currency', 'false');
  selfUser.inputs = { id: 1, email: 'alice@example.test' };
  API.put.mockImplementation((_url, body) =>
    Promise.resolve({
      data: {
        success: true,
        data: setting({
          quota_remind_enabled: body.quota_remind_enabled ?? true,
          quota_remind_threshold: body.quota_remind_threshold >= 0 ? body.quota_remind_threshold : null
        })
      }
    })
  );
});

afterEach(cleanup);

const renderCard = async (data) => {
  API.get.mockResolvedValue({ data: { success: true, data } });
  render(createElement(QuotaRemindCard, { self: selfUser }));
  await waitFor(() => expect(API.get).toHaveBeenCalled());
  await screen.findByText(i18n.t('settingsPage.preferences.quotaRemind.title'));
};

const toggle = () => document.getElementById('quota-remind-enabled');
const input = () => document.getElementById('quota-remind-threshold');
const button = (key) => screen.getByRole('button', { name: i18n.t(key) });

describe('QuotaRemindCard', () => {
  it('follows the site default until an amount is saved', async () => {
    await renderCard(setting());
    await screen.findByText(i18n.t('settingsPage.preferences.quotaRemind.followingSite', { amount: '$1' }));
    expect(input().value).toBe('');
    expect(button('common.save').disabled).toBe(true);

    fireEvent.change(input(), { target: { value: '2000000' } });
    fireEvent.click(button('common.save'));
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0][1]).toEqual({ quota_remind_threshold: 2000000 });
    await screen.findByRole('button', { name: i18n.t('settingsPage.preferences.quotaRemind.useSiteDefault') });
  });

  it('resets a personal threshold to the site default with -1', async () => {
    await renderCard(setting({ quota_remind_threshold: 2000000 }));
    await waitFor(() => expect(input().value).toBe('2000000'));
    fireEvent.click(button('settingsPage.preferences.quotaRemind.useSiteDefault'));
    expect(input().value).toBe('');
    fireEvent.click(button('common.save'));
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0][1]).toEqual({ quota_remind_threshold: -1 });
  });

  it('saves the switch with the footer', async () => {
    await renderCard(setting());
    await waitFor(() => expect(toggle().getAttribute('aria-checked')).toBe('true'));
    fireEvent.click(toggle());
    expect(API.put).not.toHaveBeenCalled();
    fireEvent.click(button('common.save'));
    await waitFor(() => expect(API.put).toHaveBeenCalledTimes(1));
    expect(API.put.mock.calls[0][1]).toEqual({ quota_remind_enabled: false });
    await waitFor(() => expect(input().disabled).toBe(true));
  });

  it('disables everything when the site turned warnings off', async () => {
    await renderCard(setting({ site_quota_remind_enabled: false }));
    await screen.findByText(i18n.t('settingsPage.preferences.quotaRemind.siteDisabled'));
    expect(toggle().disabled).toBe(true);
    expect(toggle().getAttribute('aria-checked')).toBe('false');
    expect(input().disabled).toBe(true);
  });

  it('asks for an email address when none is bound', async () => {
    selfUser.inputs = { id: 1, email: '' };
    await renderCard(setting());
    await screen.findByText(i18n.t('settingsPage.preferences.quotaRemind.noEmail'));
  });
});
