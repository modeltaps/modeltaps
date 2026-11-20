// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';

const api = vi.hoisted(() => ({ get: vi.fn(), delete: vi.fn() }));
vi.mock('utils/api', () => ({ API: api }));
vi.mock('react-i18next', () => ({ useTranslation: () => ({ t: (key, vars) => (vars ? `${key} ${JSON.stringify(vars)}` : key) }) }));

const { default: DeleteUserGroupDialog, defaultSuccessor, hasReferences } = await import('./DeleteUserGroupDialog');

const groups = [
  { id: 3, symbol: 'vip', name: 'VIP' },
  { id: 2, symbol: 'svip', name: 'SVIP' },
  { id: 1, symbol: 'default', name: 'Default' }
];

function mockLoad(refs) {
  api.get.mockImplementation((url) =>
    Promise.resolve(url.endsWith('/references') ? { data: { success: true, data: refs } } : { data: { data: { data: groups } } })
  );
}

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DeleteUserGroupDialog', () => {
  it('offers default first, else another group', () => {
    expect(defaultSuccessor(groups, 'vip')).toBe('default');
    expect(defaultSuccessor(groups.slice(0, 2), 'vip')).toBe('svip');
    expect(hasReferences({ users: 0, channels: 0 })).toBe(false);
    expect(hasReferences({ users: 0, channels: 2 })).toBe(true);
  });

  it('lists what uses the group and moves it to the successor', async () => {
    mockLoad({ users: 2, shadow_accounts: 0, tokens: 1, backup_tokens: 0, channels: 3 });
    api.delete.mockResolvedValue({ data: { success: true } });
    const onDeleted = vi.fn();
    render(<DeleteUserGroupDialog group={groups[0]} onClose={() => {}} onDeleted={onDeleted} />);
    await screen.findByText(/userGroup.delete.refs.users/);
    expect(screen.getByText(/userGroup.delete.refs.channels/)).toBeTruthy();
    expect(screen.queryByText(/userGroup.delete.refs.shadow_accounts/)).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'userGroup.delete.moveAndDelete' }));
    await waitFor(() => expect(onDeleted).toHaveBeenCalled());
    expect(api.delete).toHaveBeenCalledWith('/api/user_group/3', { params: { successor: 'default' } });
  });

  it('closes after a delete without failing', async () => {
    mockLoad({ users: 2, shadow_accounts: 0, tokens: 0, backup_tokens: 0, channels: 0 });
    const { rerender } = render(<DeleteUserGroupDialog group={groups[0]} onClose={() => {}} onDeleted={() => {}} />);
    await screen.findByText(/userGroup.delete.refs.users/);
    rerender(<DeleteUserGroupDialog group={null} onClose={() => {}} onDeleted={() => {}} />);
    expect(screen.queryByText(/userGroup.delete.refs.users/)).toBeNull();
  });

  it('deletes an unused group without a successor', async () => {
    mockLoad({ users: 0, shadow_accounts: 0, tokens: 0, backup_tokens: 0, channels: 0 });
    api.delete.mockResolvedValue({ data: { success: true } });
    render(<DeleteUserGroupDialog group={groups[0]} onClose={() => {}} onDeleted={() => {}} />);
    await screen.findByText(/common.deleteConfirm/);
    fireEvent.click(screen.getByRole('button', { name: 'common.delete' }));
    await waitFor(() => expect(api.delete).toHaveBeenCalledWith('/api/user_group/3', { params: undefined }));
  });

  it('shows the server message when the delete fails', async () => {
    mockLoad({ users: 1, shadow_accounts: 0, tokens: 0, backup_tokens: 0, channels: 0 });
    api.delete.mockResolvedValue({ data: { success: false, message: 'nope' } });
    const onDeleted = vi.fn();
    render(<DeleteUserGroupDialog group={groups[0]} onClose={() => {}} onDeleted={onDeleted} />);
    await screen.findByText(/userGroup.delete.refs.users/);
    fireEvent.click(screen.getByRole('button', { name: 'userGroup.delete.moveAndDelete' }));
    await screen.findByText('nope');
    expect(onDeleted).not.toHaveBeenCalled();
  });
});
