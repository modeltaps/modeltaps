// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Provider } from 'react-redux';
import { legacy_createStore as createStore } from 'redux';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import NewTokenDialog from './NewTokenDialog';
import ManagementKeyList from './ManagementKeyList';
import { MANAGEMENT_EXPIRY_DEFAULT, managementExpiryFromPreset, managementKeyTypeLabel } from './tokenData';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));

const MGMT_KEY = 'mk-' + 'A'.repeat(56);

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(cleanup);

const openDialog = (props, siteInfo = {}) =>
  render(
    createElement(
      Provider,
      { store: createStore(() => ({ siteInfo })) },
      createElement(NewTokenDialog, {
        onClose: vi.fn(),
        onCreated: vi.fn(),
        onOpen: vi.fn(),
        onCopy: vi.fn(),
        baseUrl: 'http://x/v1',
        ...props
      })
    )
  );

describe('NewTokenDialog key type', () => {
  it('has no type selector in an organization', () => {
    openDialog({ allowManagement: false });
    expect(screen.queryByTestId('key-type-trigger')).toBeNull();
  });

  it('defaults to an API key and creates it on the API key endpoint', async () => {
    API.post.mockResolvedValue({ data: { success: true, data: { id: 9, key: 'newkey' } } });
    openDialog({ allowManagement: true });
    expect(screen.getByTestId('key-type-trigger').textContent).toBe(i18n.t('apiKey.type.api'));
    fireEvent.change(document.getElementById('nt-name'), { target: { value: 'ci' } });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('apiKey.new.create') }));
    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    expect(API.post.mock.calls[0][0]).toBe('/api/token/');
  });

  it('creates a read-write management key with the default 90-day expiry and shows it once', async () => {
    API.post.mockResolvedValue({ data: { success: true, data: { id: 4, key: MGMT_KEY, key_hint: 'mk-A****AAAA' } } });
    const onCreated = vi.fn();
    const onManagementCreated = vi.fn();
    openDialog({ allowManagement: true, onCreated, onManagementCreated });

    fireEvent.click(screen.getByTestId('key-type-trigger'));
    fireEvent.click(await screen.findByRole('option', { name: i18n.t('apiKey.type.write'), hidden: true }));
    expect(screen.getByTestId('key-type-trigger').textContent).toBe(i18n.t('apiKey.type.write'));
    // A management key has no spending limit, and its expiry is required.
    expect(document.querySelector('input[name="nt-quota"]')).toBeNull();
    expect(document.getElementById('nt-expiry').textContent).toBe(i18n.t(`apiKey.expiry.${MANAGEMENT_EXPIRY_DEFAULT}`));

    fireEvent.change(document.getElementById('nt-name'), { target: { value: 'agent' } });
    const before = Math.floor(Date.now() / 1000);
    fireEvent.click(screen.getByRole('button', { name: i18n.t('apiKey.new.create') }));

    await waitFor(() => expect(API.post).toHaveBeenCalledTimes(1));
    const [url, body] = API.post.mock.calls[0];
    expect(url).toBe('/api/management_key/');
    expect(body.name).toBe('agent');
    expect(body.scope).toBe('write');
    expect(body.expired_time).toBeGreaterThanOrEqual(before + 90 * 86400);
    expect(body.expired_time).toBeLessThanOrEqual(before + 90 * 86400 + 5);

    // The key is shown as is (no "sk-"), with the management note and no base URL or details link.
    expect(await screen.findByTestId('new-token-key')).toHaveProperty('textContent', MGMT_KEY);
    expect(screen.getByTestId('key-once-note').textContent).toBe(i18n.t('apiKey.management.onceNote'));
    expect(screen.queryByText('http://x/v1')).toBeNull();
    expect(screen.queryByRole('button', { name: i18n.t('apiKey.new.openDetails') })).toBeNull();
    expect(onManagementCreated).toHaveBeenCalledTimes(1);
    expect(onCreated).not.toHaveBeenCalled();
  });
});

describe('ManagementKeyList', () => {
  it('tags each key with its type and lists the masked key only', async () => {
    const now = Math.floor(Date.now() / 1000);
    API.get.mockResolvedValue({
      data: {
        success: true,
        data: [
          { id: 3, name: 'reader', scope: 'read', key_hint: 'mk-R****rrrr', expired_time: now + 86400, accessed_time: 0 },
          { id: 2, name: 'writer', scope: 'write', key_hint: 'mk-W****wwww', expired_time: now - 10, accessed_time: now - 60 },
          { id: 1, name: 'Access token (deprecated)', scope: 'legacy', key_hint: 'abcd****wxyz', expired_time: -1, accessed_time: 0 }
        ]
      }
    });
    render(createElement(ManagementKeyList, { reloadKey: 0 }));

    await waitFor(() => expect(screen.getAllByTestId('management-key-row')).toHaveLength(3));
    expect(API.get).toHaveBeenCalledWith('/api/management_key/');
    const tags = screen.getAllByTestId('management-key-type').map((el) => el.textContent);
    expect(screen.getByText(i18n.t('apiKey.management.legacyName'))).toBeTruthy();
    expect(tags).toEqual([i18n.t('apiKey.type.read'), i18n.t('apiKey.type.write'), i18n.t('apiKey.type.legacy')]);
    const rows = screen.getAllByTestId('management-key-row');
    expect(rows[0].textContent).toContain('mk-R****rrrr');
    expect(rows[1].textContent).toContain(i18n.t('apiKey.management.expired'));
    expect(rows[2].textContent).toContain(i18n.t('apiKey.management.never'));
  });

  it('shows nothing while the user has no management key', async () => {
    API.get.mockResolvedValue({ data: { success: true, data: [] } });
    render(createElement(ManagementKeyList, { reloadKey: 0 }));
    await waitFor(() => expect(API.get).toHaveBeenCalled());
    expect(screen.queryByTestId('management-keys')).toBeNull();
  });

  it('deletes a key after confirming', async () => {
    API.get.mockResolvedValue({
      data: {
        success: true,
        data: [{ id: 5, name: 'ci', scope: 'read', key_hint: 'mk-C****cccc', expired_time: 4102444800, accessed_time: 0 }]
      }
    });
    API.delete.mockResolvedValue({ data: { success: true } });
    render(createElement(ManagementKeyList, { reloadKey: 0 }));
    fireEvent.click(await screen.findByRole('button', { name: `${i18n.t('common.delete')} ci` }));
    fireEvent.click(screen.getByRole('button', { name: i18n.t('token_index.delete') }));
    await waitFor(() => expect(API.delete).toHaveBeenCalledWith('/api/management_key/5'));
    await waitFor(() => expect(screen.queryByTestId('management-keys')).toBeNull());
  });
});

describe('management key helpers', () => {
  it('counts expiry presets in days from now', () => {
    const now = new Date('2026-11-17T10:00:00Z');
    const base = Math.floor(now.getTime() / 1000);
    expect(managementExpiryFromPreset('month', now)).toBe(base + 30 * 86400);
    expect(managementExpiryFromPreset('quarter', now)).toBe(base + 90 * 86400);
    expect(managementExpiryFromPreset('year', now)).toBe(base + 365 * 86400);
    expect(managementExpiryFromPreset('bogus', now)).toBe(base + 90 * 86400);
  });

  it('labels read, write and anything else as a deprecated access token', () => {
    expect(managementKeyTypeLabel('read')).toBe('apiKey.type.read');
    expect(managementKeyTypeLabel('write')).toBe('apiKey.type.write');
    expect(managementKeyTypeLabel('legacy')).toBe('apiKey.type.legacy');
  });
});

// Keys have no retention switch: it is the account's or the organization's decision (ORG-10).
describe('NewTokenDialog retention', () => {
  it('has no retention switch, even while the site allows retention', () => {
    openDialog({}, { log_io_enabled: true });
    expect(document.getElementById('nt-log-io')).toBeNull();
    expect(screen.queryByText(i18n.t('profilePage.logIOToggle'))).toBeNull();
  });
});
