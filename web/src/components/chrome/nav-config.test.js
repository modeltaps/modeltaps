import { describe, expect, it } from 'vitest';

import {
  commandSections,
  settingsCommandItems,
  contextSwitchTarget,
  deriveContext,
  findActiveItem,
  firstAdminUrl,
  isAdminPath,
  isSettingsPath,
  navSectionsFor
} from './nav-config';

const idsOf = (sections) => sections.map((s) => ({ id: s.id, items: s.items.map((i) => i.id) }));

const MAIN = { id: 'main', items: ['dashboard', 'usage', 'log'] };
const DEVELOP = { id: 'develop', items: ['token', 'model_price', 'playground', 'api_docs'] };

describe('deriveContext', () => {
  it('admin pages are the admin context for admins only', () => {
    expect(deriveContext({ pathname: '/panel/channel', isAdmin: true })).toBe('admin');
    expect(deriveContext({ pathname: '/panel/setting/site', isAdmin: true, orgId: 3 })).toBe('admin');
    expect(deriveContext({ pathname: '/panel/channel', isAdmin: false })).toBe('personal');
  });

  it('workbench pages follow the org context', () => {
    expect(deriveContext({ pathname: '/panel/dashboard', isAdmin: true })).toBe('personal');
    expect(deriveContext({ pathname: '/panel/dashboard', orgId: 7 })).toBe('org');
    expect(deriveContext({ pathname: '/panel/settings/account/profile', isAdmin: true, orgId: 7 })).toBe('org');
  });
});

describe('navSectionsFor (workbench)', () => {
  it('personal: main group and 开发, no 组织 group', () => {
    expect(idsOf(navSectionsFor({ context: 'personal' }))).toEqual([MAIN, DEVELOP]);
  });

  it('org member: the 组织 group only has the org overview', () => {
    expect(idsOf(navSectionsFor({ context: 'org', orgRole: 'member', orgId: 7 }))).toEqual([
      MAIN,
      DEVELOP,
      { id: 'organization', items: ['organization'] }
    ]);
  });

  it('org owner/admin: members & budget and billing; organization settings are in 设置, not the menu', () => {
    for (const orgRole of ['owner', 'admin']) {
      expect(idsOf(navSectionsFor({ context: 'org', orgRole, orgId: 7 }))).toEqual([
        MAIN,
        DEVELOP,
        { id: 'organization', items: ['organization', 'org_members', 'billing'] }
      ]);
    }
  });

  it('路由策略 is a workbench item in every context while the site allows own policies', () => {
    const features = { routingPolicy: true };
    for (const [context, orgRole] of [
      ['personal', null],
      ['org', 'owner'],
      ['org', 'member']
    ]) {
      const develop = navSectionsFor({ context, orgRole, orgId: 7, features }).find((s) => s.id === 'develop');
      expect(develop.items.map((i) => i.id)).toContain('routing');
      expect(develop.items.find((i) => i.id === 'routing').href).toBe('/panel/routing');
    }
    const off = navSectionsFor({ context: 'personal' }).find((s) => s.id === 'develop');
    expect(off.items.map((i) => i.id)).not.toContain('routing');
    expect(findActiveItem('/panel/routing')?.id).toBe('routing');
  });

  it('switching context stays on 路由策略 while it is available', () => {
    const pathname = '/panel/routing';
    const features = { routingPolicy: true };
    // null: stay on the page.
    expect(contextSwitchTarget({ pathname, to: { kind: 'org', orgId: 9, role: 'member' }, features })).toBeNull();
    expect(contextSwitchTarget({ pathname, to: { kind: 'personal' }, features })).toBeNull();
    expect(contextSwitchTarget({ pathname, to: { kind: 'personal' } })).toBe('/panel/dashboard');
  });

  it('org-scoped urls carry the active organization', () => {
    const org = navSectionsFor({ context: 'org', orgRole: 'owner', orgId: 7 }).find((s) => s.id === 'organization');
    expect(org.items.find((i) => i.id === 'org_members').href).toBe('/panel/settings/org/7/members');
  });

  it('workbench menus never contain admin pages', () => {
    const ids = navSectionsFor({ context: 'org', orgRole: 'owner', orgId: 1, isRoot: true }).flatMap((s) => s.items.map((i) => i.id));
    expect(ids).not.toContain('channel');
    expect(ids).not.toContain('platform_settings');
  });
});

describe('navSectionsFor (platform admin)', () => {
  it('root sees every group', () => {
    expect(idsOf(navSectionsFor({ context: 'admin', isRoot: true }))).toEqual([
      { id: 'admin_main', items: ['analytics', 'systemInfo'] },
      { id: 'gateway', items: ['channel', 'pricing', 'allocation', 'routing_policy'] },
      { id: 'people', items: ['user', 'user_group', 'org_admin'] },
      { id: 'finance', items: ['payment', 'redemption', 'invite_code'] },
      { id: 'admin_settings', items: ['platform_settings'] }
    ]);
  });

  it('non-root admins lose 系统日志 and 组织', () => {
    const ids = navSectionsFor({ context: 'admin', isRoot: false }).flatMap((s) => s.items.map((i) => i.id));
    expect(ids).not.toContain('systemInfo');
    expect(ids).not.toContain('org_admin');
    expect(ids).toContain('analytics');
  });

  it('platform admin lands on 统计 for every admin', () => {
    expect(firstAdminUrl({ isRoot: true })).toBe('/panel/analytics');
    expect(firstAdminUrl({ isRoot: false })).toBe('/panel/analytics');
  });
});

describe('findActiveItem', () => {
  it('every API console resolves to 演练场, docs to 文档', () => {
    for (const m of ['chat', 'image', 'speech', 'compare', 'video']) expect(findActiveItem(`/panel/api/${m}`)?.id).toBe('playground');
    expect(findActiveItem('/panel/api/docs/chat')?.id).toBe('api_docs');
    expect(findActiveItem('/panel/api/docs/speech')?.id).toBe('api_docs');
    expect(findActiveItem('/panel/api')).toBeNull();
  });

  it('per-user stats share the 统计 item', () => {
    expect(findActiveItem('/panel/multi_user_stats')?.id).toBe('analytics');
    expect(findActiveItem('/panel/analytics')?.id).toBe('analytics');
  });

  it('platform settings groups resolve to one item', () => {
    expect(findActiveItem('/panel/setting')?.id).toBe('platform_settings');
    expect(findActiveItem('/panel/setting/gateway')?.id).toBe('platform_settings');
    expect(findActiveItem('/panel/invite_code')?.id).toBe('invite_code');
  });

  it('settings pages: personal vs organization', () => {
    expect(findActiveItem('/panel/settings')?.id).toBe('personal_settings');
    expect(findActiveItem('/panel/settings/account/security')?.id).toBe('personal_settings');
    expect(findActiveItem('/panel/settings/org/3/general')?.id).toBe('org_settings');
    expect(findActiveItem('/panel/settings/org/3/members')?.id).toBe('org_members');
    expect(findActiveItem('/panel/settings/org/3/danger')?.id).toBe('org_settings');
    // The old budget section redirects to the 常规 group of the organization settings.
    expect(findActiveItem('/panel/settings/org/3/budget')?.id).toBe('org_settings');
  });

  it('redirect-only legacy paths do not resolve', () => {
    for (const p of ['/panel/playground', '/panel/profile', '/panel/ledger', '/panel/invoice', '/panel/topup', '/panel/not-a-page']) {
      expect(findActiveItem(p)).toBeNull();
    }
  });

  it('a path only matches whole segments', () => {
    expect(findActiveItem('/panel/logout')).toBeNull();
    expect(findActiveItem('/panel/channel/12')?.id).toBe('channel');
  });
});

describe('isAdminPath', () => {
  it('recognises admin pages and legacy admin redirects', () => {
    for (const p of [
      '/panel/channel',
      '/panel/setting',
      '/panel/setting/site',
      '/panel/telegram',
      '/panel/model_info',
      '/panel/multi_user_stats'
    ]) {
      expect(isAdminPath(p)).toBe(true);
    }
    for (const p of ['/panel/settings', '/panel/settings/account/profile', '/panel/dashboard', '/panel/billing']) {
      expect(isAdminPath(p)).toBe(false);
    }
  });
});

describe('contextSwitchTarget', () => {
  it('entering platform admin lands on its first visible page', () => {
    expect(contextSwitchTarget({ pathname: '/panel/log', to: { kind: 'admin' }, isRoot: false })).toBe('/panel/analytics');
    expect(contextSwitchTarget({ pathname: '/panel/log', to: { kind: 'admin' }, isRoot: true })).toBe('/panel/analytics');
  });

  it('leaving platform admin goes to the dashboard', () => {
    expect(contextSwitchTarget({ pathname: '/panel/channel', to: { kind: 'personal' } })).toBe('/panel/dashboard');
    expect(contextSwitchTarget({ pathname: '/panel/setting/site', to: { kind: 'org', orgId: 2, role: 'owner' } })).toBe('/panel/dashboard');
  });

  it('shared workbench pages stay', () => {
    expect(contextSwitchTarget({ pathname: '/panel/log', to: { kind: 'org', orgId: 2, role: 'member' } })).toBeNull();
    expect(contextSwitchTarget({ pathname: '/panel/api/image', to: { kind: 'personal' } })).toBeNull();
    expect(contextSwitchTarget({ pathname: '/panel/settings/account/profile', to: { kind: 'personal' } })).toBeNull();
    expect(contextSwitchTarget({ pathname: '/panel/billing', to: { kind: 'personal' } })).toBeNull();
  });

  it('org admin pages follow the new organization or fall back to the dashboard', () => {
    expect(contextSwitchTarget({ pathname: '/panel/settings/org/1/budget', to: { kind: 'org', orgId: 2, role: 'admin' } })).toBe(
      '/panel/settings/org/2/budget'
    );
    expect(contextSwitchTarget({ pathname: '/panel/settings/org/1/general', to: { kind: 'org', orgId: 2, role: 'member' } })).toBe(
      '/panel/dashboard'
    );
    expect(contextSwitchTarget({ pathname: '/panel/settings/org/1/general', to: { kind: 'personal' } })).toBe('/panel/dashboard');
    expect(contextSwitchTarget({ pathname: '/panel/billing', to: { kind: 'org', orgId: 2, role: 'member' } })).toBe('/panel/dashboard');
  });
});

describe('commandSections', () => {
  it('non-admins get no platform admin entries', () => {
    const ids = commandSections({ isAdmin: false }).flatMap((s) => s.items.map((i) => i.id));
    expect(ids).toContain('personal_settings');
    expect(ids).not.toContain('org_settings');
    expect(ids).not.toContain('channel');
  });

  it('both parts of 设置: organization settings for owners and admins of the active organization', () => {
    const items = (orgRole) => commandSections({ orgId: 7, orgRole }).find((s) => s.id === 'account').items;
    const listed = (orgRole) => items(orgRole).filter((i) => !i.searchOnly);
    expect(listed('owner').map((i) => [i.id, i.href])).toEqual([
      ['org_settings', '/panel/settings/org/7/general'],
      ['personal_settings', '/panel/settings']
    ]);
    expect(listed('member').map((i) => i.id)).toEqual(['personal_settings']);
  });

  it('every item of the 设置 tree is searchable (shown while typing)', () => {
    const typed = (orgId, orgRole) =>
      commandSections({ orgId, orgRole })
        .find((s) => s.id === 'account')
        .items.filter((i) => i.searchOnly)
        .map((i) => [i.id, i.href, i.prefixKey]);
    expect(typed(7, 'admin')).toEqual([
      ['settings_org_general', '/panel/settings/org/7/general', 'settingsUi.sections.org'],
      ['settings_org_members', '/panel/settings/org/7/members', 'settingsUi.sections.org'],
      ['settings_org_danger', '/panel/settings/org/7/danger', 'settingsUi.sections.org'],
      ['settings_personal_profile', '/panel/settings/account/profile', 'settingsUi.sections.personal'],
      ['settings_personal_security', '/panel/settings/account/security', 'settingsUi.sections.personal'],
      ['settings_personal_privacy', '/panel/settings/account/privacy', 'settingsUi.sections.personal'],
      ['settings_personal_preferences', '/panel/settings/account/preferences', 'settingsUi.sections.personal']
    ]);
    expect(typed(7, 'member').map(([id]) => id)).not.toContain('settings_org_general');
    expect(typed(null, null).map(([id]) => id)).toHaveLength(4);
  });

  it('设置 pages are recognised by path', () => {
    for (const p of ['/panel/settings', '/panel/settings/account/profile', '/panel/settings/org/3/members'])
      expect(isSettingsPath(p)).toBe(true);
    for (const p of ['/panel/setting', '/panel/setting/site', '/panel/settingsx', '/panel/dashboard'])
      expect(isSettingsPath(p)).toBe(false);
  });

  it('admins can reach every admin page they may see', () => {
    const admin = commandSections({ isAdmin: true, isRoot: false }).find((s) => s.admin);
    expect(admin.items.map((i) => i.id)).toEqual([
      'admin_home',
      'analytics',
      'channel',
      'pricing',
      'allocation',
      'routing_policy',
      'user',
      'user_group',
      'payment',
      'redemption',
      'invite_code',
      'platform_settings'
    ]);
    expect(admin.items[0].href).toBe('/panel/analytics');
  });

  it('platform settings groups are listed; cards only show while typing', () => {
    const groupIds = (isRoot) =>
      commandSections({ isAdmin: true, isRoot })
        .find((s) => s.id === 'platform_settings')
        .items.filter((i) => !i.searchOnly)
        .map((i) => i.href);
    expect(groupIds(false)).toEqual(
      ['site', 'auth', 'billing', 'payment', 'gateway', 'privacy', 'notify'].map((g) => `/panel/setting/${g}`)
    );
    expect(groupIds(true)).toContain('/panel/setting/orgpolicy');
    const cards = settingsCommandItems({ isRoot: false }).filter((i) => i.searchOnly);
    expect(cards.find((i) => i.id === 'setting_card_smtp').href).toBe('/panel/setting/notify?highlight=smtp');
    expect(cards.some((i) => i.id === 'setting_card_org-policy')).toBe(false);
  });
});
