// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import TokenTable from './TokenTable';
import { KeySecretDialog, PasteKeyField, normalizePastedKey, pastedKeyMatches } from './KeySecret';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));

afterEach(cleanup);

// A full key that must never reach the DOM of a list, even if a response carried it.
const LEAKED = 'sk-LEAKEDfullkeyLEAKEDfullkeyLEAKEDfullkeyLEAKEDfullkey12';

const row = (over) => ({
  id: 3,
  name: 'ci',
  key_hint: 'sk-A****Rc6g',
  key: LEAKED.slice(3),
  status: 1,
  expired_time: -1,
  unlimited_quota: true,
  remain_quota: 0,
  used_quota: 0,
  accessed_time: 0,
  created_time: 0,
  group: '',
  setting: {},
  ...over
});

const renderTable = (props) =>
  render(
    createElement(
      MemoryRouter,
      null,
      createElement(TokenTable, {
        tokens: [row()],
        userGroup: {},
        onEdit: vi.fn(),
        onDelete: vi.fn(),
        onToggleStatus: vi.fn(),
        onShowUsage: vi.fn(),
        onConnectOpenClaw: vi.fn(),
        onOpen: vi.fn(),
        ...props
      })
    )
  );

describe('TokenTable key masking', () => {
  it('lists the masked hint only, never the key, and offers no copy', () => {
    renderTable({ onResetKey: vi.fn() });
    expect(screen.getByTestId('token-key-hint').textContent).toBe('sk-…Rc6g');
    expect(document.body.textContent).not.toContain(LEAKED.slice(3));
    expect(screen.queryByRole('button', { name: i18n.t('token_index.copy') })).toBeNull();
  });

  it('offers reset in the row menu only where the key may be reset', async () => {
    const onResetKey = vi.fn();
    renderTable({ onResetKey, canResetKey: () => true });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('token_index.actions') }));
    fireEvent.click(await screen.findByRole('menuitem', { name: i18n.t('apiKey.secret.reset') }));
    expect(screen.queryByRole('menuitem', { name: i18n.t('token_index.copy') })).toBeNull();
    expect(onResetKey).toHaveBeenCalledWith(expect.objectContaining({ id: 3 }));
    cleanup();

    renderTable({ onResetKey, canResetKey: () => false });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('token_index.actions') }));
    await screen.findByRole('menuitem', { name: i18n.t('common.delete') });
    expect(screen.queryByRole('menuitem', { name: i18n.t('apiKey.secret.reset') })).toBeNull();
  });
});

describe('KeySecretDialog', () => {
  it('shows the new key once, with copy and the "you will not see it again" note', () => {
    const onCopy = vi.fn();
    render(createElement(KeySecretDialog, { name: 'ci', secret: 'sk-newsecret', baseUrl: 'http://x/v1', onCopy, onClose: vi.fn() }));
    expect(screen.getByTestId('new-token-key').textContent).toBe('sk-newsecret');
    expect(screen.getByTestId('key-once-note').textContent).toBe(i18n.t('apiKey.secret.onceNote'));
    fireEvent.click(screen.getAllByRole('button', { name: i18n.t('token_index.copy') })[0]);
    expect(onCopy).toHaveBeenCalledWith('sk-newsecret', i18n.t('token_index.token'));
  });
});

describe('pasted keys', () => {
  it('normalizes the prefix and checks the key against the stored hint', () => {
    expect(normalizePastedKey('  Bearer sk-abc ')).toBe('sk-abc');
    expect(normalizePastedKey('abc')).toBe('sk-abc');
    expect(normalizePastedKey('')).toBe('');
    const key = 'sk-ABCDEFGHIJKLMNOPQRSTUVWXYZ0123';
    expect(pastedKeyMatches(key, 'sk-A****0123')).toBe(true);
    expect(pastedKeyMatches(key, 'sk-B****0123')).toBe(false);
  });

  it('warns when the pasted key is not the one of this entry', () => {
    const { rerender } = render(createElement(PasteKeyField, { id: 'k', value: '', onChange: vi.fn(), hint: 'sk-A****0123' }));
    expect(screen.getByText(i18n.t('apiKey.secret.pasteHint'))).toBeTruthy();
    rerender(createElement(PasteKeyField, { id: 'k', value: 'sk-ZZZZZZZZZZZZZZZZZZZZ9999', onChange: vi.fn(), hint: 'sk-A****0123' }));
    expect(screen.getByText(i18n.t('apiKey.secret.pasteMismatch', { hint: 'sk-A****0123' }))).toBeTruthy();
    expect(document.getElementById('k').getAttribute('type')).toBe('password');
  });
});
