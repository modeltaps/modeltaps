// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import ToolConnectDialog from './ToolConnectDialog';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn() } }));
vi.mock('react-redux', () => ({
  useSelector: (select) => select({ siteInfo: { ClaudeAPIEnabled: true, server_address: 'https://gw.example.com' } })
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

describe('ToolConnectDialog', () => {
  it('lists the tools and shows nothing else until one is picked', () => {
    open({ token });
    for (const id of ['claude-code', 'codex', 'opencode', 'goose', 'openclaw', 'other'])
      expect(screen.getByTestId(`tool-${id}`)).toBeTruthy();
    expect(screen.queryByTestId('tool-cache')).toBeNull();
  });

  it('fills the Goose command with the Claude model, the host and the known key', async () => {
    open({ token, secrets: { 7: SECRET } });
    fireEvent.click(screen.getByTestId('tool-goose'));
    await waitFor(() => expect(document.body.textContent).toContain('export GOOSE_MODEL="claude-sonnet-5-5"'));
    expect(document.body.textContent).toContain('export ANTHROPIC_HOST="https://gw.example.com/coding-agent"');
    expect(document.body.textContent).toContain(`export ANTHROPIC_API_KEY="${SECRET}"`);
    expect(API.get).toHaveBeenCalledWith('/api/token/7/models');
  });

  it('uses the placeholder and asks for the key when it is unknown', async () => {
    open({ token });
    fireEvent.click(screen.getByTestId('tool-claude-code'));
    expect(await screen.findByLabelText(i18n.t('apiKey.secret.pasteLabel'))).toBeTruthy();
    expect(document.body.textContent).toContain('"ANTHROPIC_AUTH_TOKEN": "sk-xxxxxx"');
  });

  it('opened from the page header, picks the first enabled key', async () => {
    open({ tokens: [{ ...token, id: 3, status: 2 }, token], secrets: { 7: SECRET } });
    fireEvent.click(screen.getByTestId('tool-codex'));
    await waitFor(() => expect(document.body.textContent).toContain(`export MODELTAPS_API_KEY="${SECRET}"`));
    expect(document.body.textContent).toContain('model = "gpt-5.5"');
  });

  it('says so when there is no enabled key', () => {
    open({ tokens: [] });
    fireEvent.click(screen.getByTestId('tool-other'));
    expect(screen.getByText(i18n.t('connectTool.noKey'))).toBeTruthy();
  });

  it('builds the agent prompt from the pairing code, without the key', async () => {
    open({ token, secrets: { 7: SECRET }, pairing: { code: 'pair-abc', expiresAt: 0 } });
    fireEvent.click(screen.getByTestId('tool-opencode'));
    const tab = await screen.findByRole('tab', { name: i18n.t('connectTool.method.ai') });
    fireEvent.mouseDown(tab);
    fireEvent.click(tab);
    await waitFor(() => expect(document.body.textContent).toContain('pair-abc'));
    expect(document.body.textContent).toContain(i18n.t('connectTool.agent.promptTool', { tool: 'OpenCode' }));
  });
});
