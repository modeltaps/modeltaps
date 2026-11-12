// @vitest-environment jsdom
import { createElement } from 'react';
import { MemoryRouter } from 'react-router';
import { Provider } from 'react-redux';
import { legacy_createStore as createStore } from 'redux';
import { cleanup, fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { OrgContext } from 'contexts/OrgContext';
import { SettingsPlaneContext } from '@/components/settings/SettingsPlane';
import SettingsSidebar from './SettingsSidebar';

// On 设置 the sidebar is the settings tree, laid out like the workbench and platform admin sidebars:
// the brand row (brand, search button that opens the palette, collapse button where needed), the
// back row where the context switcher is, the organization picker under it, then the organization
// items and 个人设置. Unsaved groups reported by the page get a dot.

afterEach(cleanup);

const orgs = [
  { id: 4, name: 'Acme', role: 'owner' },
  { id: 9, name: 'Globex', role: 'member' }
];

// The brand row reads the site name.
const store = createStore(() => ({ siteInfo: { system_name: 'Modeltaps' } }));

const renderAt = (path, { organizations = orgs, plane = null, showCollapse = false, onOpenCommand = () => {}, collapsed = false } = {}) =>
  render(
    createElement(
      Provider,
      { store },
      createElement(
        MemoryRouter,
        { initialEntries: [path] },
        createElement(
          OrgContext.Provider,
          { value: { organizations, currentOrgId: null } },
          createElement(
            SettingsPlaneContext.Provider,
            { value: plane },
            createElement(SettingsSidebar, { showCollapse, collapsed, onToggleCollapse: () => {}, onOpenCommand })
          )
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
    // The picker names the part (it sits above, where the workbench has its context switcher);
    // there is no visible heading.
    expect(org.textContent).not.toContain('组织设置');
    const picker = screen.getByRole('button', { name: '选择组织' });
    expect(picker.textContent).toContain('Acme');
    expect(screen.getByRole('navigation').contains(picker)).toBe(false);
    expect(linkNames(org)).toEqual(['常规', '成员与预算', '危险操作']);
    expect(personal.getAttribute('aria-label')).toBe('个人设置');
    expect(personal.textContent).toContain('个人设置');
    expect(linkNames(personal)).toEqual(['个人信息', '登录与安全', '数据与隐私', '偏好']);
    expect(within(org).getByRole('link', { name: '成员与预算' }).getAttribute('aria-current')).toBe('page');
    expect(within(org).getByRole('link', { name: '成员与预算' }).getAttribute('href')).toBe('/panel/settings/org/4/members');
  });

  it('has a back row to the workbench and no footer', () => {
    renderAt('/panel/settings/account/profile');
    expect(screen.getByRole('link', { name: '返回工作台' }).textContent).toContain('工作台');
    // The same brand row as the workbench comes first.
    expect(screen.getAllByRole('link')[0].getAttribute('href')).toBe('/panel/dashboard');
    expect(screen.queryByRole('button', { name: /账号菜单/ })).toBeNull();
  });

  it('shows only personal settings to users who manage no organization', () => {
    renderAt('/panel/settings/account/profile', { organizations: [orgs[1]] });
    expect(screen.getAllByRole('group').map((g) => g.getAttribute('aria-label'))).toEqual(['个人设置']);
  });

  it('marks groups with unsaved cards', () => {
    const plane = { publish: () => {}, dirtyGroups: ['general'] };
    renderAt('/panel/settings/org/4/general', { plane });
    expect(screen.getByRole('link', { name: /常规/ }).textContent).toContain('有未保存的改动');
    expect(screen.getByRole('link', { name: /个人信息/ }).textContent).not.toContain('有未保存的改动');
  });

  it('search is a button in the first row that opens the search palette', () => {
    const opened = [];
    renderAt('/panel/settings/account/profile', { onOpenCommand: () => opened.push(true) });
    expect(screen.queryByRole('searchbox')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: '搜索' }));
    expect(opened).toHaveLength(1);
  });

  it('collapsed: the organization mark and search sit where the workbench has its switcher and search', () => {
    renderAt('/panel/settings/org/4/general', { collapsed: true });
    const picker = screen.getByRole('button', { name: '选择组织: Acme' });
    const search = screen.getByRole('button', { name: '搜索' });
    expect(picker.compareDocumentPosition(search) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(screen.getByRole('navigation').contains(picker)).toBe(false);
    expect(screen.queryByText('个人设置')).toBeNull();
  });

  it('shows the collapse button only when asked (narrow screens, the drawer)', () => {
    renderAt('/panel/settings/account/profile');
    expect(screen.queryByRole('button', { name: '收起侧栏' })).toBeNull();
    cleanup();
    renderAt('/panel/settings/account/profile', { showCollapse: true });
    expect(screen.getByRole('button', { name: '收起侧栏' })).toBeTruthy();
  });
});
