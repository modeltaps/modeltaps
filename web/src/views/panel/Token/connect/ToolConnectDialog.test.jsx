// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import ToolConnectDialog from './ToolConnectDialog';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn() } }));
vi.mock('react-redux', () => ({
  useSelector: (select) => select({ siteInfo: { ClaudeAPIEnabled: true, server_address: 'https://gw.example.com', system_name: 'Acme' } })
}));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
vi.mock('@/components/brand/BrandIcon', () => ({ default: () => null }));

const token = { id: 7, name: 'ci', key_hint: 'sk-A****Rc6g', status: 1 };
const SECRET = 'sk-FRESHsecretFRESHsecretFRESHsecretFRESHsecret00';

beforeEach(async () => {
  await i18n.changeLanguage('en_US');
  API.get.mockResolvedValue({ data: { success: true, data: ['gpt-5.5', 'claude-sonnet-5-5'] } });
});
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

const open = (props) =>
  render(
    createElement(ToolConnectDialog, {
      open: true,
      onOpenChange: vi.fn(),
      onCopy: vi.fn(),
      serverAddress: 'https://gw.example.com',
      ...props
    })
  );

const search = () => screen.getByLabelText(i18n.t('connectTool.searchLabel'));

describe('ToolConnectDialog', () => {
  it('shows popular tools and every category, and search narrows them', () => {
    open({ token });
    for (const id of ['claude-code', 'codex', 'cursor', 'cline', 'goose', 'cherry-studio', 'lobechat', 'other'])
      expect(screen.getByTestId(`tool-${id}`)).toBeTruthy();
    fireEvent.change(search(), { target: { value: 'roo' } });
    expect(screen.getByTestId('tool-roo-code')).toBeTruthy();
    expect(screen.queryByTestId('tool-codex')).toBeNull();
    fireEvent.change(search(), { target: { value: 'zzz' } });
    expect(screen.getByText(i18n.t('connectTool.noMatch'))).toBeTruthy();
  });

  it('puts the import button first and folds the config file away', async () => {
    open({ token, secrets: { 7: SECRET } });
    fireEvent.click(screen.getByTestId('tool-claude-code'));
    const link = await screen.findByTestId('import-ccswitch');
    expect(link.getAttribute('href')).toMatch(/^ccswitch:\/\/v1\/import\?/);
    expect(link.getAttribute('href')).toContain(encodeURIComponent(SECRET));
    expect(screen.getByTestId('tool-primary').textContent).toContain(i18n.t('connectTool.method.deeplink'));
    expect(screen.getByTestId('tool-way-file').open).toBe(false);
    fireEvent.click(screen.getByTestId('tool-back'));
    expect(screen.getByTestId('tool-codex')).toBeTruthy();
  });

  it('disables the import until the key is known and offers a new key', async () => {
    const onCreateKey = vi.fn();
    open({ token, onCreateKey });
    fireEvent.click(screen.getByTestId('tool-codex'));
    const button = await screen.findByTestId('import-ccswitch');
    expect(button.getAttribute('href')).toBeNull();
    fireEvent.click(screen.getByText(i18n.t('connectTool.import.newKey')));
    expect(onCreateKey).toHaveBeenCalled();
  });

  it('lists steps and values for a tool with its own settings screen, the key masked', async () => {
    open({ token, secrets: { 7: SECRET } });
    fireEvent.click(screen.getByTestId('tool-goose'));
    await waitFor(() => expect(screen.getByTestId('tool-primary').textContent).toContain('claude-sonnet-5-5'));
    expect(screen.getByTestId('tool-primary').textContent).toContain('ANTHROPIC_HOSThttps://gw.example.com/coding-agent');
    const primary = screen.getByTestId('tool-primary').textContent;
    expect(primary).toContain('claude-sonnet-5-5');
    expect(primary).not.toContain(SECRET);
    expect(API.get).toHaveBeenCalledWith('/api/token/7/models');
  });

  it('opened from the page header, picks the first enabled key', async () => {
    open({ tokens: [{ ...token, id: 3, status: 2 }, token], secrets: { 7: SECRET } });
    fireEvent.click(screen.getByTestId('tool-codex'));
    await screen.findByTestId('import-ccswitch');
    expect(screen.getByTestId('import-ccswitch').getAttribute('href')).toContain(encodeURIComponent(SECRET));
  });

  it('says so when there is no enabled key', () => {
    open({ tokens: [] });
    fireEvent.click(screen.getByTestId('tool-other'));
    expect(screen.getByText(i18n.t('connectTool.noKey'))).toBeTruthy();
  });

  it('builds the agent prompt from the pairing code', async () => {
    open({ token, secrets: { 7: SECRET }, pairing: { code: 'pair-abc', expiresAt: 0 } });
    fireEvent.click(screen.getByTestId('tool-opencode'));
    await screen.findByTestId('tool-way-ai');
    expect(screen.getByTestId('tool-way-ai').textContent).toContain('pair-abc');
    expect(screen.getByTestId('tool-way-ai').textContent).toContain(i18n.t('connectTool.agent.promptTool', { tool: 'OpenCode' }));
  });
});
