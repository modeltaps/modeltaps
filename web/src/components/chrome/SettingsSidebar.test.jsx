// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { OrgContext } from 'contexts/OrgContext';
import { SettingsPlaneContext } from '@/components/settings/SettingsPlane';
import SettingsSidebar from './SettingsSidebar';

// On 设置 the sidebar is the settings tree: a top row (back link, search button, collapse button
// where needed), the organization part (picker + items, no heading) first, then 个人设置. Unsaved
// groups reported by the page get a dot.

afterEach(cleanup);

const orgs = [
  { id: 4, name: 'Acme', role: 'owner' },
  { id: 9, name: 'Globex', role: 'member' }
];

const renderAt = (path, { organizations = orgs, plane = null, showCollapse = false } = {}) =>
  render(
    createElement(
      MemoryRouter,
      { initialEntries: [path] },
      createElement(
        OrgContext.Provider,
        { value: { organizations, currentOrgId: null } },
        createElement(
          SettingsPlaneContext.Provider,
          { value: plane },
          createElement(SettingsSidebar, { showCollapse, onToggleCollapse: () => {} })
        )
      )
    )
  );

const linkNames = (group) =>
  within(group)
    .getAllByRole('link')
    .map((a) => a.textContent.trim());

describe('SettingsSidebar', () => {
  it('lists organization settings first, with the picker, then personal settings', () => {
    renderAt('/panel/settings/org/4/members');
    const [org, personal] = screen.getAllByRole('group');
    expect(org.getAttribute('aria-label')).toBe('组织设置');
    // The picker names the part; there is no visible heading.
    expect(org.textContent).not.toContain('组织设置');
    expect(within(org).getByRole('button', { name: '选择组织' }).textContent).toContain('Acme');
    expect(linkNames(org)).toEqual(['常规', '成员与预算', '危险操作']);
    expect(personal.getAttribute('aria-label')).toBe('个人设置');
    expect(personal.textContent).toContain('个人设置');
    expect(linkNames(personal)).toEqual(['资料', '登录与安全', '数据与隐私', '偏好']);
    expect(within(org).getByRole('link', { name: '成员与预算' }).getAttribute('aria-current')).toBe('page');
    expect(within(org).getByRole('link', { name: '成员与预算' }).getAttribute('href')).toBe('/panel/settings/org/4/members');
  });

  it('has a back row to the workbench and no footer', () => {
    renderAt('/panel/settings/account/profile');
    expect(screen.getByRole('link', { name: '返回工作台' }).textContent).toContain('工作台');
    expect(screen.queryByRole('button', { name: /账号菜单/ })).toBeNull();
  });

  it('shows only personal settings to users who manage no organization', () => {
    renderAt('/panel/settings/account/profile', { organizations: [orgs[1]] });
    expect(screen.getAllByRole('group').map((g) => g.getAttribute('aria-label'))).toEqual(['个人设置']);
  });

  it('marks groups with unsaved cards', () => {
    const plane = { query: '', setQuery: () => {}, publish: () => {}, searchable: true, dirtyGroups: ['general'], counts: null };
    renderAt('/panel/settings/org/4/general', { plane });
    expect(screen.getByRole('link', { name: /常规/ }).textContent).toContain('有未保存的改动');
    expect(screen.getByRole('link', { name: /资料/ }).textContent).not.toContain('有未保存的改动');
  });

  it('search is a button in the top row that turns the row into the search box', () => {
    const plane = { query: '', setQuery: () => {}, publish: () => {}, searchable: true, dirtyGroups: [], counts: null };
    renderAt('/panel/settings/org/4/general', { plane });
    expect(screen.queryByRole('searchbox')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '搜索设置' }));
    expect(screen.getByRole('searchbox', { name: '搜索设置' })).toBeTruthy();
    expect(screen.queryByRole('link', { name: '返回工作台' })).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '清除搜索' }));
    expect(screen.queryByRole('searchbox')).toBeNull();
    expect(screen.getByRole('link', { name: '返回工作台' })).toBeTruthy();
  });

  it('keeps the search box open while it holds a query, and has no search on pages without groups', () => {
    const plane = { query: '预算', setQuery: () => {}, publish: () => {}, searchable: true, dirtyGroups: [], counts: { general: 1 } };
    renderAt('/panel/settings/org/4/general', { plane });
    expect(screen.getByRole('searchbox', { name: '搜索设置' }).value).toBe('预算');
    cleanup();
    renderAt('/panel/settings/org/4/members', { plane: { ...plane, query: '', searchable: false } });
    expect(screen.queryByRole('button', { name: '搜索设置' })).toBeNull();
  });

  it('shows the collapse button only when asked (narrow screens, the drawer)', () => {
    renderAt('/panel/settings/account/profile');
    expect(screen.queryByRole('button', { name: '收起侧栏' })).toBeNull();
    cleanup();
    renderAt('/panel/settings/account/profile', { showCollapse: true });
    expect(screen.getByRole('button', { name: '收起侧栏' })).toBeTruthy();
  });
});
