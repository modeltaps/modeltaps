// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import i18n from 'i18n/i18n';
import { API } from 'utils/api';
import OrgAdmin from './index';

// 组织成员上限:null=跟随站点默认,0=不限制,正数=上限。列表显示「成员数 / 生效上限」,
// 对话框三种模式分别提交 null / 0 / 正整数。

vi.mock('utils/api', () => ({ API: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() } }));
vi.mock('@/components/ui/sonner', () => ({ toast: { success: vi.fn(), error: vi.fn(), info: vi.fn(), warning: vi.fn() } }));

const ORG = {
  id: 7,
  name: 'Acme',
  slug: 'acme',
  status: 1,
  owner_id: 1,
  member_count: 2,
  quota: 0,
  used_quota: 0,
  created_time: 1792652404,
  max_members: null,
  effective_max_members: 10
};

const mockList = (org) => API.get.mockResolvedValue({ data: { success: true, data: { data: [org], page: 1, size: 10, total_count: 1 } } });

beforeEach(() => {
  vi.clearAllMocks();
  API.put.mockResolvedValue({ data: { success: true, message: '' } });
});

afterEach(cleanup);

const renderPage = () => render(createElement(MemoryRouter, null, createElement(OrgAdmin)));

const openDialog = async () => {
  const buttons = await screen.findAllByRole('button', { name: i18n.t('orgAdmin.maxMembers') });
  fireEvent.click(buttons[0]);
};

const chooseMode = (labelKey) => fireEvent.click(screen.getByRole('radio', { name: i18n.t(labelKey) }));

const submit = () => fireEvent.click(screen.getByRole('button', { name: i18n.t('common.submit') }));

describe('OrgAdmin member limit', () => {
  it('shows the effective limit and marks the site default', async () => {
    mockList(ORG);
    renderPage();
    expect((await screen.findAllByText(/2 \/ 10/)).length).toBeGreaterThan(0);
    expect(screen.getAllByText(i18n.t('orgAdmin.maxMembersDefaultTag')).length).toBeGreaterThan(0);
  });

  it('shows unlimited as ∞ without the default tag', async () => {
    mockList({ ...ORG, max_members: 0, effective_max_members: 0 });
    renderPage();
    expect((await screen.findAllByText(/2 \/ ∞/)).length).toBeGreaterThan(0);
    expect(screen.queryByText(i18n.t('orgAdmin.maxMembersDefaultTag'))).toBeNull();
  });

  it('submits 0 for unlimited', async () => {
    mockList(ORG);
    renderPage();
    await openDialog();
    chooseMode('orgAdmin.maxMembersModeUnlimited');
    submit();
    await waitFor(() => expect(API.put).toHaveBeenCalledWith('/api/organization/7/max_members', { max_members: 0 }));
  });

  it('submits a positive number for a custom limit and rejects 0', async () => {
    mockList(ORG);
    renderPage();
    await openDialog();
    chooseMode('orgAdmin.maxMembersModeCustom');
    const input = document.getElementById('org-max-members-value');
    fireEvent.change(input, { target: { value: '0' } });
    submit();
    expect(API.put).not.toHaveBeenCalled();

    fireEvent.change(input, { target: { value: '25' } });
    submit();
    await waitFor(() => expect(API.put).toHaveBeenCalledWith('/api/organization/7/max_members', { max_members: 25 }));
  });

  it('submits null to follow the site default', async () => {
    mockList({ ...ORG, max_members: 5, effective_max_members: 5 });
    renderPage();
    await openDialog();
    expect(document.getElementById('org-max-members-value').value).toBe('5');
    chooseMode('orgAdmin.maxMembersModeDefault');
    submit();
    await waitFor(() => expect(API.put).toHaveBeenCalledWith('/api/organization/7/max_members', { max_members: null }));
  });
});
