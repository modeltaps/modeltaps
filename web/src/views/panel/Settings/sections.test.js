import { describe, expect, it } from 'vitest';

import {
  ACCOUNT_GROUPS,
  ORG_GROUPS,
  ORG_TREE,
  parseSettingsPath,
  resolveAccountSection,
  resolveOrgSection,
  settingsLanding,
  settingsOrg,
  settingsTree
} from './sections';

describe('personal and organization settings sections', () => {
  it('account groups stay, the old access token section lands on its card', () => {
    for (const group of ['profile', 'security', 'privacy', 'preferences']) expect(resolveAccountSection(group)).toBeNull();
    expect(resolveAccountSection('tokens')).toBe('/panel/settings/account/security?highlight=access-token');
    expect(resolveAccountSection('nope')).toBe('/panel/settings/account/profile');
  });

  it('organization groups and the members page stay, budget moved into 常规', () => {
    expect(resolveOrgSection(3, 'general')).toBeNull();
    expect(resolveOrgSection(3, 'danger')).toBeNull();
    expect(resolveOrgSection(3, 'members')).toBeNull();
    expect(resolveOrgSection(3, 'budget')).toBe('/panel/settings/org/3/general?highlight=team-budget');
    expect(resolveOrgSection(3, 'nope')).toBe('/panel/settings/org/3/general');
  });

  it('the old organization 自动路由 page redirects to the workbench 路由策略 page', () => {
    expect(resolveOrgSection(3, 'routing')).toBe('/panel/routing');
  });

  it('every organization group is in the tree, and personal and organization ids never collide', () => {
    expect(ORG_GROUPS.every((g) => ORG_TREE.some((item) => item.id === g.id))).toBe(true);
    const ids = [...ACCOUNT_GROUPS, ...ORG_TREE].map((g) => g.id);
    expect(new Set(ids).size).toBe(ids.length);
  });
});

describe('settingsOrg: the organization 设置 lists', () => {
  const managed = [
    { id: 4, name: 'Acme', role: 'owner' },
    { id: 9, name: 'Globex', role: 'admin' }
  ];

  it('none when the user manages no organization', () => {
    expect(settingsOrg([], { orgId: 4, currentOrgId: 4 })).toBeNull();
    expect(settingsOrg(undefined)).toBeNull();
  });

  it("the URL's organization on an organization group", () => {
    expect(settingsOrg(managed, { orgId: 9, currentOrgId: 4 }).id).toBe(9);
  });

  it('on a personal group: the active organization, else the first managed one', () => {
    expect(settingsOrg(managed, { currentOrgId: 9 }).id).toBe(9);
    expect(settingsOrg(managed, { currentOrgId: null }).id).toBe(4);
    // Active in an organization the user only belongs to: fall back to the first managed one.
    expect(settingsOrg(managed, { currentOrgId: 77 }).id).toBe(4);
  });
});

describe('the 设置 tree', () => {
  const orgs = [
    { id: 4, name: 'Acme', role: 'owner' },
    { id: 9, name: 'Globex', role: 'admin' },
    { id: 12, name: 'Initech', role: 'member' }
  ];
  const ids = (items) => items.map((i) => i.id);

  it('parses settings paths', () => {
    expect(parseSettingsPath('/panel/settings/org/9/members')).toEqual({ orgId: 9, section: 'members' });
    expect(parseSettingsPath('/panel/settings/account/security')).toEqual({ orgId: null, section: 'security' });
    expect(parseSettingsPath('/panel/settings')).toEqual({ orgId: null, section: null });
  });

  it('organization settings come first: general, members & budget, danger zone', () => {
    const tree = settingsTree({ pathname: '/panel/settings/org/9/general', organizations: orgs });
    expect(tree.org.id).toBe(9);
    expect(ids(tree.managedOrgs)).toEqual([4, 9]);
    expect(ids(tree.orgItems)).toEqual(['general', 'members', 'danger']);
    expect(tree.orgItems.find((i) => i.id === 'members').to).toBe('/panel/settings/org/9/members');
    expect(ids(tree.personalItems)).toEqual(['profile', 'security', 'privacy', 'preferences']);
    expect(tree.active.id).toBe('general');
    expect(tree.activeScope).toBe('org');
  });

  it('on a personal item the organization part shows the active (or first) managed organization', () => {
    const tree = settingsTree({ pathname: '/panel/settings/account/privacy', organizations: orgs, currentOrgId: 9 });
    expect(tree.org.id).toBe(9);
    expect(tree.orgItems.every((i) => !i.active)).toBe(true);
    expect(tree.active.id).toBe('privacy');
    expect(tree.activeScope).toBe('personal');
    expect(settingsTree({ pathname: '/panel/settings/account/privacy', organizations: orgs, currentOrgId: 12 }).org.id).toBe(4);
  });

  it('without a managed organization there is only the personal part', () => {
    const tree = settingsTree({ pathname: '/panel/settings/account/profile', organizations: [orgs[2]] });
    expect(tree.org).toBeNull();
    expect(tree.orgItems).toEqual([]);
  });

  it('opens on the first item: organization settings when the user manages one, else personal', () => {
    expect(settingsLanding({ organizations: orgs, currentOrgId: 9 })).toBe('/panel/settings/org/9/general');
    expect(settingsLanding({ organizations: orgs })).toBe('/panel/settings/org/4/general');
    expect(settingsLanding({ organizations: [orgs[2]] })).toBe('/panel/settings/account/profile');
    expect(settingsLanding()).toBe('/panel/settings/account/profile');
  });
});
