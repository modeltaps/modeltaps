// @vitest-environment jsdom
import { createElement } from 'react';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import NewTokenDialog from './NewTokenDialog';
import { AgentHandover, PairingCodeReveal } from './PairingCode';

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }));
vi.mock('react-redux', () => ({ useSelector: (select) => select({ siteInfo: { server_address: 'https://gw.example.com' } }) }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));

const CODE = 'pair-' + 'A'.repeat(40);

beforeEach(() => {
  vi.clearAllMocks();
});

afterEach(cleanup);

describe('NewTokenDialog pairing code (AGENT-7)', () => {
  it('shows the pairing code under the new key, with what redeeming it does', async () => {
    const expiresAt = Math.floor(Date.now() / 1000) + 600;
    API.post.mockResolvedValue({
      data: { success: true, data: { id: 9, key: 'newkey', key_hint: 'sk-n****ykey', pairing_code: CODE, pairing_expires_at: expiresAt } }
    });
    const onCopy = vi.fn();
    render(createElement(NewTokenDialog, { onClose: vi.fn(), onCreated: vi.fn(), onOpen: vi.fn(), onCopy, baseUrl: 'http://x/v1' }));
    fireEvent.change(document.getElementById('nt-name'), { target: { value: 'agent' } });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('apiKey.new.create') }));

    expect(await screen.findByTestId('pairing-code-value')).toHaveProperty('textContent', CODE);
    expect(screen.getByTestId('new-token-key').textContent).toBe('sk-newkey');
    const block = screen.getByTestId('pairing-code');
    expect(block.textContent).toContain('https://gw.example.com/skill.md');
    expect(block.textContent).toContain(i18n.t('apiKey.pairing.rotateNote'));

    const copy = block.querySelector('button');
    fireEvent.click(copy);
    expect(onCopy).toHaveBeenCalledWith(CODE, i18n.t('apiKey.pairing.label'));
  });

  it('shows no pairing block when the server sent no code', async () => {
    API.post.mockResolvedValue({ data: { success: true, data: { id: 9, key: 'newkey' } } });
    render(createElement(NewTokenDialog, { onClose: vi.fn(), onCreated: vi.fn(), onOpen: vi.fn(), onCopy: vi.fn() }));
    fireEvent.change(document.getElementById('nt-name'), { target: { value: 'agent' } });
    fireEvent.click(screen.getByRole('button', { name: i18n.t('apiKey.new.create') }));
    await screen.findByTestId('new-token-key');
    expect(screen.queryByTestId('pairing-code')).toBeNull();
  });
});

describe('AgentHandover (key details)', () => {
  const token = { id: 7, name: 'app', key_hint: 'sk-a****bcde' };

  it('shows who claimed the key', () => {
    const claimed = {
      ...token,
      agent_claim: { via: 'device_flow', client_name: 'Hermes', ip: '203.0.113.5', user_agent: 'hermes/1', claimed_at: 1700000000 }
    };
    render(createElement(AgentHandover, { token: claimed, canIssue: false, onCopy: vi.fn() }));
    expect(screen.getByTestId('agent-claim').textContent).toBe(
      i18n.t('apiKey.agent.claimed', { via: i18n.t('apiKey.agent.viaDeviceFlow') })
    );
    const section = screen.getByTestId('agent-handover');
    expect(section.textContent).toContain('Hermes');
    expect(section.textContent).toContain('203.0.113.5');
    expect(section.textContent).toContain('hermes/1');
    expect(screen.queryByRole('button', { name: i18n.t('apiKey.agent.newCode') })).toBeNull();
  });

  it('issues a new pairing code after confirming', async () => {
    vi.spyOn(window, 'confirm').mockReturnValue(true);
    API.post.mockResolvedValue({ data: { success: true, data: { pairing_code: CODE, pairing_expires_at: 1700000600 } } });
    render(createElement(AgentHandover, { token, canIssue: true, onCopy: vi.fn() }));
    expect(screen.getByTestId('agent-handover').textContent).toContain(i18n.t('apiKey.agent.notClaimed'));
    fireEvent.click(screen.getByRole('button', { name: i18n.t('apiKey.agent.newCode') }));
    await waitFor(() => expect(API.post).toHaveBeenCalledWith('/api/token/7/pairing_code'));
    expect(await screen.findByTestId('pairing-code-value')).toHaveProperty('textContent', CODE);
  });
});

describe('Copy for your agent (AGENT-6)', () => {
  const future = () => Math.floor(Date.now() / 1000) + 600;

  it('copies the guide, the pairing code and what to report, never the key', () => {
    const onCopy = vi.fn();
    render(createElement(PairingCodeReveal, { code: CODE, expiresAt: future(), secret: 'sk-secret', onCopy }));
    fireEvent.click(screen.getByTestId('copy-for-agent'));
    const [text, label] = onCopy.mock.calls[0];
    expect(label).toBe(i18n.t('apiKey.pairing.promptLabel'));
    expect(text).toContain('https://gw.example.com/skill.md');
    expect(text).toContain(CODE);
    expect(text).toContain(i18n.t('apiKey.pairing.prompt.report'));
    expect(text).not.toContain('sk-secret');
  });

  it('offers the copy with the key only while the key is known', () => {
    const onCopy = vi.fn();
    render(createElement(PairingCodeReveal, { code: CODE, expiresAt: future(), secret: 'sk-secret', onCopy }));
    fireEvent.click(screen.getByTestId('copy-for-agent-with-key'));
    expect(onCopy.mock.calls[0][0]).toContain('sk-secret');
    expect(onCopy.mock.calls[0][0]).not.toContain(CODE);
    cleanup();
    render(createElement(PairingCodeReveal, { code: CODE, expiresAt: future(), onCopy }));
    expect(screen.queryByTestId('copy-for-agent-with-key')).toBeNull();
  });

  it('asks for a new code once the code has expired', () => {
    render(createElement(PairingCodeReveal, { code: CODE, expiresAt: Math.floor(Date.now() / 1000) - 1, onCopy: vi.fn() }));
    expect(screen.getByTestId('copy-for-agent')).toHaveProperty('disabled', true);
    expect(screen.getByTestId('pairing-expired').textContent).toBe(i18n.t('apiKey.pairing.expired'));
  });
});
